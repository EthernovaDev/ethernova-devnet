// Ethernova: Optional Privacy (Phase 24) — StateDB-BACKED
//
// Previous versions stored commitments, nullifiers, and the total-shielded
// counter in core/rawdb while moving the actual NOVA balance through the
// state trie at 0xdEaD. Because rawdb writes are NOT covered by the state
// root and NOT reverted on tx failure, a caller could:
//   - Shield a commitment deep in a call chain, then force an outer revert
//     → caller's balance refunded, but the commitment slot remained
//     permanently occupied (denial-of-service vs any legitimate user who
//     later tried to use the same commitment).
//   - Unshield → pool debited, recipient credited → outer revert rolls
//     the state-trie moves back → but the nullifier write persisted →
//     victim's note permanently marked spent.
//
// All shielded-pool set membership now lives at system address 0xAA26 via
// StateDB. Balance transfers still use 0xdEaD (those were already in the
// state trie and correct). Because commitments and nullifiers live in the
// same trie as balances, revert/reorg semantics now match the rest of the
// EVM: either the whole operation lands or none of it does.
//
// SAFETY (retained):
//  1. Double-spend check on nullifier before any balance move.
//  2. Pool accounting: trackedTotal must cover any requested withdrawal.
//  3. Max 10,000 NOVA per unshield (circuit breaker).
//  4. Commitment uniqueness (prevents collision griefing).

package vm

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
)

// shieldMetaAddr holds commitment/nullifier set membership and the
// total-shielded counter. The NOVA balance itself still sits at
// shieldPoolAddress (below).
var shieldMetaAddr = common.HexToAddress("0x000000000000000000000000000000000000AA26")

// shieldPoolAddress is where the actual shielded NOVA balance lives.
// Kept at 0xdEaD for continuity — deposits add NOVA here, withdrawals
// take it out.
var shieldPoolAddress = common.HexToAddress("0x000000000000000000000000000000000000dEaD")

func spEnsureMetaAccount(sdb StateDB) {
	if !sdb.Exist(shieldMetaAddr) {
		sdb.CreateAccount(shieldMetaAddr)
	}
	if sdb.GetNonce(shieldMetaAddr) == 0 {
		sdb.SetNonce(shieldMetaAddr, 1)
	}
}

func spKeyCommitment(commitment common.Hash) common.Hash {
	return crypto.Keccak256Hash([]byte("shield.commitment"), commitment.Bytes())
}
func spKeyNullifier(nullifier common.Hash) common.Hash {
	return crypto.Keccak256Hash([]byte("shield.nullifier"), nullifier.Bytes())
}
func spKeyTotal() common.Hash {
	return crypto.Keccak256Hash([]byte("shield.total"))
}

func spHasCommitment(sdb StateDB, commitment common.Hash) bool {
	return sdb.GetState(shieldMetaAddr, spKeyCommitment(commitment)) != (common.Hash{})
}
func spHasNullifier(sdb StateDB, nullifier common.Hash) bool {
	return sdb.GetState(shieldMetaAddr, spKeyNullifier(nullifier)) != (common.Hash{})
}
func spReadTotal(sdb StateDB) *big.Int {
	return new(big.Int).SetBytes(sdb.GetState(shieldMetaAddr, spKeyTotal()).Bytes())
}
func spWriteTotal(sdb StateDB, v *big.Int) {
	sdb.SetState(shieldMetaAddr, spKeyTotal(), common.BigToHash(v))
}

type novaShieldedPool struct{}

func (c *novaShieldedPool) RequiredGas(input []byte) uint64 {
	if len(input) < 1 {
		return 0
	}
	switch input[0] {
	case 0x01:
		return 50000
	case 0x02:
		return 100000
	case 0x03:
		return 2000
	case 0x04:
		return 2000
	default:
		return 0
	}
}

func (c *novaShieldedPool) Run(input []byte) ([]byte, error) {
	return nil, errors.New("novaShieldedPool: use RunStateful")
}

func (c *novaShieldedPool) RunStateful(evm *EVM, caller common.Address, input []byte, readOnly bool) ([]byte, error) {
	if len(input) < 1 {
		return nil, errors.New("novaShieldedPool: empty input")
	}
	sdb := evm.StateDB

	switch input[0] {
	case 0x01: // shield(commitment32, amount32) — deposit NOVA into the pool
		if readOnly {
			return nil, ErrWriteProtection
		}
		if len(input) < 65 {
			return nil, errors.New("shield: need commitment(32) + amount(32)")
		}

		spEnsureMetaAccount(sdb)

		commitment := common.BytesToHash(input[1:33])
		amount := new(big.Int).SetBytes(input[33:65])

		if amount.Sign() <= 0 {
			return nil, errors.New("shield: amount must be positive")
		}

		if spHasCommitment(sdb, commitment) {
			return nil, errors.New("shield: commitment already exists")
		}

		amountU256, overflow := uint256.FromBig(amount)
		if overflow {
			return nil, errors.New("shield: amount overflow")
		}
		callerBalance := sdb.GetBalance(caller)
		if callerBalance.Cmp(amountU256) < 0 {
			return nil, errors.New("shield: insufficient NOVA balance")
		}

		// Transfer NOVA from caller to pool.
		sdb.SubBalance(caller, amountU256)
		sdb.AddBalance(shieldPoolAddress, amountU256)

		// Record commitment (non-zero marker).
		sdb.SetState(shieldMetaAddr, spKeyCommitment(commitment), common.BytesToHash([]byte{0x01}))

		// Update total shielded.
		total := spReadTotal(sdb)
		total.Add(total, amount)
		spWriteTotal(sdb, total)

		return common.LeftPadBytes([]byte{1}, 32), nil

	case 0x02: // unshield(nullifier32, recipient20, amount32)
		if readOnly {
			return nil, ErrWriteProtection
		}
		if len(input) < 85 {
			return nil, errors.New("unshield: need nullifier(32) + recipient(20) + amount(32)")
		}

		spEnsureMetaAccount(sdb)

		nullifier := common.BytesToHash(input[1:33])
		recipient := common.BytesToAddress(input[33:53])
		amount := new(big.Int).SetBytes(input[53:85])

		if amount.Sign() <= 0 {
			return nil, errors.New("unshield: amount must be positive")
		}

		maxWithdraw := new(big.Int).Mul(big.NewInt(10000), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
		if amount.Cmp(maxWithdraw) > 0 {
			return nil, errors.New("unshield: exceeds max withdrawal (10,000 NOVA)")
		}

		if spHasNullifier(sdb, nullifier) {
			return nil, errors.New("unshield: nullifier already spent (DOUBLE SPEND BLOCKED)")
		}

		trackedTotal := spReadTotal(sdb)
		if trackedTotal.Cmp(amount) < 0 {
			return nil, errors.New("unshield: tracked pool total < withdrawal (accounting error detected)")
		}

		amountU256, overflow := uint256.FromBig(amount)
		if overflow {
			return nil, errors.New("unshield: amount overflow")
		}
		poolBalance := sdb.GetBalance(shieldPoolAddress)
		if poolBalance.Cmp(amountU256) < 0 {
			return nil, errors.New("unshield: pool insufficient balance")
		}

		sdb.SubBalance(shieldPoolAddress, amountU256)
		sdb.AddBalance(recipient, amountU256)

		sdb.SetState(shieldMetaAddr, spKeyNullifier(nullifier), common.BytesToHash([]byte{0x01}))

		total := spReadTotal(sdb)
		total.Sub(total, amount)
		if total.Sign() < 0 {
			total = new(big.Int)
		}
		spWriteTotal(sdb, total)

		return common.LeftPadBytes([]byte{1}, 32), nil

	case 0x03: // poolInfo()
		total := spReadTotal(sdb)
		return common.LeftPadBytes(total.Bytes(), 32), nil

	case 0x04: // verifyInPool(commitment32)
		if len(input) < 33 {
			return nil, errors.New("verifyInPool: need commitment")
		}
		commitment := common.BytesToHash(input[1:33])
		if spHasCommitment(sdb, commitment) {
			return common.LeftPadBytes([]byte{1}, 32), nil
		}
		return common.LeftPadBytes([]byte{0}, 32), nil

	default:
		return nil, errors.New("novaShieldedPool: unknown function")
	}
}

// CreateCommitment generates a commitment from secret + amount.
func CreateCommitment(secret common.Hash, amount *big.Int) common.Hash {
	data := make([]byte, 64)
	copy(data[:32], secret[:])
	copy(data[32:], common.LeftPadBytes(amount.Bytes(), 32))
	return crypto.Keccak256Hash(data)
}

// CreateNullifier generates a nullifier from secret + commitment.
func CreateNullifier(secret common.Hash, commitment common.Hash) common.Hash {
	data := make([]byte, 64)
	copy(data[:32], secret[:])
	copy(data[32:], commitment[:])
	return crypto.Keccak256Hash(data)
}
