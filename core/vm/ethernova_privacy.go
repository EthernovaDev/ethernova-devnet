// Ethernova: Optional Privacy - Shielded Transfers (Phase 24)
// Persistent commitment/nullifier storage in LevelDB.
// Precompile at 0x26 (novaShieldedPool)

package vm

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/crypto"
)

type novaShieldedPool struct{}

func (c *novaShieldedPool) RequiredGas(input []byte) uint64 {
	if len(input) < 1 {
		return 0
	}
	switch input[0] {
	case 0x01: return 50000   // shield
	case 0x02: return 100000  // unshield
	case 0x03: return 2000    // poolInfo
	case 0x04: return 2000    // verifyInPool
	default:   return 0
	}
}

func (c *novaShieldedPool) Run(input []byte) ([]byte, error) {
	if len(input) < 1 {
		return nil, errors.New("novaShieldedPool: empty input")
	}
	if GlobalChainDB == nil {
		return nil, errors.New("novaShieldedPool: database not initialized")
	}

	switch input[0] {
	case 0x01: // shield(commitment32)
		if len(input) < 33 {
			return nil, errors.New("shield: need 32-byte commitment")
		}
		commitment := common.BytesToHash(input[1:33])
		if rawdb.HasCommitment(GlobalChainDB, commitment) {
			return nil, errors.New("shield: commitment already exists")
		}
		rawdb.WriteCommitment(GlobalChainDB, commitment)
		// Update total shielded
		total := rawdb.ReadShieldedTotal(GlobalChainDB)
		if len(input) >= 65 {
			amount := new(big.Int).SetBytes(input[33:65])
			total.Add(total, amount)
			rawdb.WriteShieldedTotal(GlobalChainDB, total)
		}
		return common.LeftPadBytes([]byte{1}, 32), nil

	case 0x02: // unshield(nullifier32, recipient20, amount32)
		if len(input) < 85 {
			return nil, errors.New("unshield: need nullifier(32) + recipient(20) + amount(32)")
		}
		nullifier := common.BytesToHash(input[1:33])
		if rawdb.HasNullifier(GlobalChainDB, nullifier) {
			return nil, errors.New("unshield: nullifier already spent (double-spend blocked)")
		}
		rawdb.WriteNullifier(GlobalChainDB, nullifier)
		// Update total shielded
		amount := new(big.Int).SetBytes(input[53:85])
		total := rawdb.ReadShieldedTotal(GlobalChainDB)
		total.Sub(total, amount)
		if total.Sign() < 0 {
			total = new(big.Int)
		}
		rawdb.WriteShieldedTotal(GlobalChainDB, total)
		return common.LeftPadBytes([]byte{1}, 32), nil

	case 0x03: // poolInfo() -> totalShielded
		total := rawdb.ReadShieldedTotal(GlobalChainDB)
		return common.LeftPadBytes(total.Bytes(), 32), nil

	case 0x04: // verifyInPool(commitment32)
		if len(input) < 33 {
			return nil, errors.New("verifyInPool: need 32-byte commitment")
		}
		commitment := common.BytesToHash(input[1:33])
		if rawdb.HasCommitment(GlobalChainDB, commitment) {
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
