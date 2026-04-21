// Ethernova: Native Multi-Token Support (Phase 20) — StateDB-BACKED
//
// Previous versions stored token metadata, balances, and creation counts in
// the chain's LevelDB via core/rawdb helpers. That bypassed the state trie
// entirely, which meant:
//   - rawdb writes were NOT covered by block.stateRoot → two nodes could
//     hold different token balances while still agreeing on block hashes.
//   - rawdb writes were NOT reverted on transaction revert → every call
//     path through the precompile could leave partial state on failure.
//   - rawdb writes were NOT rolled back on reorg → an orphaned-branch
//     createToken would permanently prevent a fresh node from minting the
//     same tokenID on the canonical branch.
//
// All token state now lives at system address 0xAA25 via evm.StateDB,
// inside the Merkle Patricia Trie. Consensus reads happen through
// GetState, reverts roll back via the state journal, reorgs discard the
// state trie along with the orphaned block.
//
// ANTI-SPAM (retained from previous version):
//   - Token creation costs 500,000 gas.
//   - Each creator address limited to 100 tokens.

package vm

import (
	"encoding/binary"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// tokenManagerSystemAddr is the system account holding all token metadata,
// balances, and per-creator counters. 0xAA25 has no valid ECDSA preimage.
var tokenManagerSystemAddr = common.HexToAddress("0x000000000000000000000000000000000000AA25")

const tokenCreationLimit uint64 = 100

// tmEnsureSystemAccount makes the 0xAA25 account non-empty per EIP-161.
// Without this, Finalise(true) would delete the account and all token
// state at the end of every tx boundary. Same pattern as
// poEnsureRegistryExists in ethernova_protocol_objects.go.
func tmEnsureSystemAccount(sdb StateDB) {
	if !sdb.Exist(tokenManagerSystemAddr) {
		sdb.CreateAccount(tokenManagerSystemAddr)
	}
	if sdb.GetNonce(tokenManagerSystemAddr) == 0 {
		sdb.SetNonce(tokenManagerSystemAddr, 1)
	}
}

// Storage key builders.
func tmKeyCount(creator common.Address) common.Hash {
	return crypto.Keccak256Hash([]byte("token.count"), creator.Bytes())
}
func tmKeyMetaLen(tokenID common.Hash) common.Hash {
	return crypto.Keccak256Hash([]byte("token.meta.len"), tokenID.Bytes())
}
func tmKeyMetaChunk(tokenID common.Hash, idx uint64) common.Hash {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], idx)
	return crypto.Keccak256Hash([]byte("token.meta.chunk"), tokenID.Bytes(), buf[:])
}
func tmKeyBalance(tokenID common.Hash, holder common.Address) common.Hash {
	return crypto.Keccak256Hash([]byte("token.balance"), tokenID.Bytes(), holder.Bytes())
}

func tmReadUint64(sdb StateDB, key common.Hash) uint64 {
	return new(big.Int).SetBytes(sdb.GetState(tokenManagerSystemAddr, key).Bytes()).Uint64()
}
func tmWriteUint64(sdb StateDB, key common.Hash, v uint64) {
	sdb.SetState(tokenManagerSystemAddr, key, common.BigToHash(new(big.Int).SetUint64(v)))
}
func tmReadBigInt(sdb StateDB, key common.Hash) *big.Int {
	return new(big.Int).SetBytes(sdb.GetState(tokenManagerSystemAddr, key).Bytes())
}
func tmWriteBigInt(sdb StateDB, key common.Hash, v *big.Int) {
	sdb.SetState(tokenManagerSystemAddr, key, common.BigToHash(v))
}

// tmWriteMeta stores a token's metadata blob as length + 32-byte chunks.
func tmWriteMeta(sdb StateDB, tokenID common.Hash, meta []byte) {
	dataLen := uint64(len(meta))
	chunks := (dataLen + 31) / 32
	tmWriteUint64(sdb, tmKeyMetaLen(tokenID), dataLen)
	for i := uint64(0); i < chunks; i++ {
		start := i * 32
		end := start + 32
		if end > dataLen {
			end = dataLen
		}
		var chunk [32]byte
		copy(chunk[:], meta[start:end])
		sdb.SetState(tokenManagerSystemAddr, tmKeyMetaChunk(tokenID, i), common.BytesToHash(chunk[:]))
	}
}

// tmReadMeta returns nil if the token has no metadata (unknown tokenID).
func tmReadMeta(sdb StateDB, tokenID common.Hash) []byte {
	dataLen := tmReadUint64(sdb, tmKeyMetaLen(tokenID))
	if dataLen == 0 {
		return nil
	}
	chunks := (dataLen + 31) / 32
	out := make([]byte, 0, dataLen)
	for i := uint64(0); i < chunks; i++ {
		chunk := sdb.GetState(tokenManagerSystemAddr, tmKeyMetaChunk(tokenID, i))
		remaining := dataLen - uint64(len(out))
		if remaining >= 32 {
			out = append(out, chunk[:]...)
		} else {
			out = append(out, chunk[:remaining]...)
		}
	}
	return out
}

type novaTokenManager struct{}

func (c *novaTokenManager) RequiredGas(input []byte) uint64 {
	if len(input) < 1 {
		return 0
	}
	switch input[0] {
	case 0x01:
		return 500000 // createToken
	case 0x02:
		return 50000 // transfer (covers 2 SLOAD + 2 SSTORE)
	case 0x03:
		return 2000 // balanceOf
	case 0x04:
		return 5000 // tokenInfo (reads chunked metadata)
	default:
		return 0
	}
}

func (c *novaTokenManager) Run(input []byte) ([]byte, error) {
	return nil, errors.New("novaTokenManager: use RunStateful")
}

func (c *novaTokenManager) RunStateful(evm *EVM, caller common.Address, input []byte, readOnly bool) ([]byte, error) {
	if len(input) < 1 {
		return nil, errors.New("novaTokenManager: empty input")
	}
	sdb := evm.StateDB

	switch input[0] {
	case 0x01: // createToken
		if readOnly {
			return nil, ErrWriteProtection
		}
		if len(input) < 2 {
			return nil, errors.New("createToken: insufficient input")
		}

		tmEnsureSystemAccount(sdb)

		// Deterministic tokenID = keccak256(caller || rest of input).
		seed := append(caller.Bytes(), input[1:]...)
		tokenID := crypto.Keccak256Hash(seed)

		creationCount := tmReadUint64(sdb, tmKeyCount(caller))
		if creationCount >= tokenCreationLimit {
			return nil, errors.New("createToken: address has reached 100 token limit")
		}

		if tmReadUint64(sdb, tmKeyMetaLen(tokenID)) > 0 {
			return nil, errors.New("createToken: token already exists")
		}

		// Store metadata (creator prefix + caller's payload).
		meta := append(caller.Bytes(), input[1:]...)
		tmWriteMeta(sdb, tokenID, meta)
		tmWriteUint64(sdb, tmKeyCount(caller), creationCount+1)

		// If supply is encoded in the last 32 bytes, credit it to the creator.
		if len(input) >= 34 {
			supply := new(big.Int).SetBytes(input[len(input)-32:])
			if supply.Sign() > 0 {
				tmWriteBigInt(sdb, tmKeyBalance(tokenID, caller), supply)
			}
		}

		return tokenID.Bytes(), nil

	case 0x02: // transfer(tokenID32, to20, amount32)
		if readOnly {
			return nil, ErrWriteProtection
		}
		if len(input) < 85 {
			return nil, errors.New("transfer: need tokenID(32) + to(20) + amount(32)")
		}

		tmEnsureSystemAccount(sdb)

		tokenID := common.BytesToHash(input[1:33])
		to := common.BytesToAddress(input[33:53])
		amount := new(big.Int).SetBytes(input[53:85])

		if amount.Sign() <= 0 {
			return nil, errors.New("transfer: amount must be positive")
		}

		if tmReadUint64(sdb, tmKeyMetaLen(tokenID)) == 0 {
			return nil, errors.New("transfer: token does not exist")
		}

		senderBal := tmReadBigInt(sdb, tmKeyBalance(tokenID, caller))
		if senderBal.Cmp(amount) < 0 {
			return nil, errors.New("transfer: insufficient balance")
		}

		newSenderBal := new(big.Int).Sub(senderBal, amount)
		tmWriteBigInt(sdb, tmKeyBalance(tokenID, caller), newSenderBal)

		recipientBal := tmReadBigInt(sdb, tmKeyBalance(tokenID, to))
		newRecipientBal := new(big.Int).Add(recipientBal, amount)
		tmWriteBigInt(sdb, tmKeyBalance(tokenID, to), newRecipientBal)

		return common.LeftPadBytes([]byte{1}, 32), nil

	case 0x03: // balanceOf(tokenID32, address20)
		if len(input) < 53 {
			return nil, errors.New("balanceOf: need tokenID(32) + address(20)")
		}
		tokenID := common.BytesToHash(input[1:33])
		addr := common.BytesToAddress(input[33:53])
		balance := tmReadBigInt(sdb, tmKeyBalance(tokenID, addr))
		return common.LeftPadBytes(balance.Bytes(), 32), nil

	case 0x04: // tokenInfo(tokenID32) → up to 128 bytes of metadata
		if len(input) < 33 {
			return nil, errors.New("tokenInfo: need tokenID")
		}
		tokenID := common.BytesToHash(input[1:33])
		meta := tmReadMeta(sdb, tokenID)
		if meta == nil {
			return make([]byte, 32), nil
		}
		result := make([]byte, 128)
		if len(meta) > 128 {
			copy(result, meta[:128])
		} else {
			copy(result, meta)
		}
		return result, nil

	default:
		return nil, errors.New("novaTokenManager: unknown function")
	}
}

