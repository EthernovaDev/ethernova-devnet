package vm

import (
	"errors"

	"github.com/ethereum/go-ethereum/crypto"
)

// Ethernova custom precompiled contracts.
// Address 0x20: novaBatchHash  - batch keccak256 hashing
// Address 0x21: novaBatchVerify - batch ecrecover signature verification

// novaBatchHash hashes multiple 32-byte items in a single precompile call.
// Input: concatenated 32-byte chunks
// Output: concatenated 32-byte keccak256 hashes
// Gas: 30 per item (vs ~36 in Solidity with overhead)
type novaBatchHash struct{}

func (c *novaBatchHash) RequiredGas(input []byte) uint64 {
	if len(input) == 0 {
		return 0
	}
	items := (uint64(len(input)) + 31) / 32
	return items * 30
}

func (c *novaBatchHash) Run(input []byte) ([]byte, error) {
	if len(input) == 0 {
		return nil, nil
	}
	items := (len(input) + 31) / 32
	result := make([]byte, 0, items*32)
	for i := 0; i < items; i++ {
		start := i * 32
		end := start + 32
		if end > len(input) {
			// Pad with zeros
			chunk := make([]byte, 32)
			copy(chunk, input[start:])
			hash := crypto.Keccak256(chunk)
			result = append(result, hash...)
		} else {
			hash := crypto.Keccak256(input[start:end])
			result = append(result, hash...)
		}
	}
	return result, nil
}

// novaBatchVerify recovers addresses from multiple signatures in one call.
// Input format: for each signature, 32 bytes hash + 32 bytes r + 32 bytes s + 1 byte v (97 bytes per sig)
// Output: concatenated 20-byte recovered addresses (32 bytes each, left-padded)
// Gas: 2000 per signature (vs 3000 for individual ecrecover)
type novaBatchVerify struct{}

const sigSize = 97 // 32 (hash) + 32 (r) + 32 (s) + 1 (v)

func (c *novaBatchVerify) RequiredGas(input []byte) uint64 {
	if len(input) < sigSize {
		return 0
	}
	sigs := uint64(len(input)) / sigSize
	return sigs * 2000
}

func (c *novaBatchVerify) Run(input []byte) ([]byte, error) {
	if len(input) < sigSize {
		return nil, errors.New("input too short")
	}
	sigs := len(input) / sigSize
	result := make([]byte, 0, sigs*32)

	for i := 0; i < sigs; i++ {
		offset := i * sigSize
		hash := input[offset : offset+32]
		r := input[offset+32 : offset+64]
		s := input[offset+64 : offset+96]
		v := input[offset+96]

		// Normalize v (27/28 -> 0/1)
		if v >= 27 {
			v -= 27
		}
		if v > 1 {
			// Invalid v, return zero address
			result = append(result, make([]byte, 32)...)
			continue
		}

		// Construct 65-byte signature: r + s + v
		sig := make([]byte, 65)
		copy(sig[0:32], r)
		copy(sig[32:64], s)
		sig[64] = v

		pubkey, err := crypto.Ecrecover(hash, sig)
		if err != nil || len(pubkey) == 0 {
			result = append(result, make([]byte, 32)...)
			continue
		}

		// Hash public key to get address
		addr := crypto.Keccak256(pubkey[1:])[12:]
		padded := make([]byte, 32)
		copy(padded[12:], addr)
		result = append(result, padded...)
	}

	return result, nil
}
