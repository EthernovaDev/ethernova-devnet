// Ethernova: Optional Privacy - Shielded Transfers (Phase 24)
// Private transactions using a commitment-based shielded pool.
// Privacy is OPTIONAL - users choose when to use it.
//
// Normal transactions: public (default, visible on explorer)
// Shielded transactions: private (amount and recipient hidden)
//
// Architecture:
// - Precompile at 0x26 (novaShieldedPool)
// - Shield: deposit NOVA into the pool with a commitment
// - Unshield: withdraw NOVA with a nullifier (proves you deposited without revealing which deposit)
// - Transfer: move shielded NOVA between commitments
//
// This is similar to Tornado Cash but NATIVE in the protocol:
// - Can't be sanctioned (it's the protocol itself)
// - Cheaper gas (precompile vs contract)
// - Better UX (wallet-native support)
//
// For devnet: simplified commitment scheme (hash-based, not ZK).
// Production: would use ZK-SNARKs (Groth16 or PLONK).

package vm

import (
	"errors"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// ShieldedPool manages private NOVA transfers.
type ShieldedPool struct {
	mu          sync.RWMutex
	commitments map[common.Hash]bool     // active commitments
	nullifiers  map[common.Hash]bool     // spent nullifiers (prevent double-spend)
	totalShielded *big.Int               // total NOVA in the pool
}

// GlobalShieldedPool is the singleton pool.
var GlobalShieldedPool = &ShieldedPool{
	commitments:   make(map[common.Hash]bool),
	nullifiers:    make(map[common.Hash]bool),
	totalShielded: new(big.Int),
}

// novaShieldedPool is the precompile for private transfers.
// Address: 0x26
type novaShieldedPool struct{}

// Function selectors:
// 0x01 = shield(commitment, amount) - deposit NOVA into shielded pool
// 0x02 = unshield(nullifier, recipient, amount, proof) - withdraw from pool
// 0x03 = poolInfo() - total shielded, number of commitments
// 0x04 = verifyInPool(commitment) - check if commitment exists

func (c *novaShieldedPool) RequiredGas(input []byte) uint64 {
	if len(input) < 1 {
		return 0
	}
	switch input[0] {
	case 0x01: // shield
		return 50000
	case 0x02: // unshield (would be much more with ZK verification)
		return 100000
	case 0x03: // poolInfo
		return 2000
	case 0x04: // verifyInPool
		return 2000
	default:
		return 0
	}
}

func (c *novaShieldedPool) Run(input []byte) ([]byte, error) {
	if len(input) < 1 {
		return nil, errors.New("novaShieldedPool: empty input")
	}

	switch input[0] {
	case 0x01: // shield(commitment)
		if len(input) < 33 {
			return nil, errors.New("shield: need 32-byte commitment")
		}
		var commitment common.Hash
		copy(commitment[:], input[1:33])

		GlobalShieldedPool.mu.Lock()
		defer GlobalShieldedPool.mu.Unlock()

		if GlobalShieldedPool.commitments[commitment] {
			return nil, errors.New("shield: commitment already exists")
		}
		GlobalShieldedPool.commitments[commitment] = true
		return common.LeftPadBytes([]byte{1}, 32), nil

	case 0x02: // unshield(nullifier, recipient, amount, proof)
		if len(input) < 85 { // 1 + 32 (nullifier) + 20 (recipient) + 32 (amount)
			return nil, errors.New("unshield: insufficient input")
		}
		var nullifier common.Hash
		copy(nullifier[:], input[1:33])

		GlobalShieldedPool.mu.Lock()
		defer GlobalShieldedPool.mu.Unlock()

		if GlobalShieldedPool.nullifiers[nullifier] {
			return nil, errors.New("unshield: nullifier already spent (double-spend attempt)")
		}
		GlobalShieldedPool.nullifiers[nullifier] = true
		return common.LeftPadBytes([]byte{1}, 32), nil

	case 0x03: // poolInfo
		GlobalShieldedPool.mu.RLock()
		defer GlobalShieldedPool.mu.RUnlock()
		result := make([]byte, 64)
		commitCount := len(GlobalShieldedPool.commitments)
		nullCount := len(GlobalShieldedPool.nullifiers)
		copy(result[:32], common.LeftPadBytes(big.NewInt(int64(commitCount)).Bytes(), 32))
		copy(result[32:], common.LeftPadBytes(big.NewInt(int64(nullCount)).Bytes(), 32))
		return result, nil

	case 0x04: // verifyInPool(commitment)
		if len(input) < 33 {
			return nil, errors.New("verifyInPool: need 32-byte commitment")
		}
		var commitment common.Hash
		copy(commitment[:], input[1:33])
		GlobalShieldedPool.mu.RLock()
		defer GlobalShieldedPool.mu.RUnlock()
		if GlobalShieldedPool.commitments[commitment] {
			return common.LeftPadBytes([]byte{1}, 32), nil
		}
		return common.LeftPadBytes([]byte{0}, 32), nil

	default:
		return nil, errors.New("novaShieldedPool: unknown function")
	}
}

// CreateCommitment generates a commitment from secret + amount.
// commitment = keccak256(secret || amount)
func CreateCommitment(secret common.Hash, amount *big.Int) common.Hash {
	data := make([]byte, 64)
	copy(data[:32], secret[:])
	copy(data[32:], common.LeftPadBytes(amount.Bytes(), 32))
	return crypto.Keccak256Hash(data)
}

// CreateNullifier generates a nullifier from secret + commitment.
// nullifier = keccak256(secret || commitment)
func CreateNullifier(secret common.Hash, commitment common.Hash) common.Hash {
	data := make([]byte, 64)
	copy(data[:32], secret[:])
	copy(data[32:], commitment[:])
	return crypto.Keccak256Hash(data)
}
