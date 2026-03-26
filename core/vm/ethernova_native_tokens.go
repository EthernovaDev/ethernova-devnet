// Ethernova: Native Multi-Token Support (Phase 20)
// Tokens as protocol objects with persistent LevelDB storage.
// Precompile at 0x25 (novaTokenManager)
//
// Gas costs: 10x cheaper than ERC-20

package vm

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/crypto"
)

type novaTokenManager struct{}

func (c *novaTokenManager) RequiredGas(input []byte) uint64 {
	if len(input) < 1 {
		return 0
	}
	switch input[0] {
	case 0x01: return 50000  // createToken
	case 0x02: return 5000   // transfer
	case 0x03: return 1000   // balanceOf
	case 0x04: return 1000   // tokenInfo
	default:   return 0
	}
}

func (c *novaTokenManager) Run(input []byte) ([]byte, error) {
	if len(input) < 1 {
		return nil, errors.New("novaTokenManager: empty input")
	}
	if GlobalChainDB == nil {
		return nil, errors.New("novaTokenManager: database not initialized")
	}

	switch input[0] {
	case 0x01: // createToken(nameLen, name, symbolLen, symbol, decimals, supply32)
		if len(input) < 33 {
			return nil, errors.New("createToken: insufficient input")
		}
		// Generate deterministic tokenID
		tokenID := crypto.Keccak256Hash(input[1:])
		// Store metadata
		rawdb.WriteTokenMeta(GlobalChainDB, tokenID, input[1:])
		return tokenID.Bytes(), nil

	case 0x02: // transfer(tokenID32, to20, amount32)
		if len(input) < 85 {
			return nil, errors.New("transfer: need tokenID(32) + to(20) + amount(32)")
		}
		tokenID := common.BytesToHash(input[1:33])
		to := common.BytesToAddress(input[33:53])
		amount := new(big.Int).SetBytes(input[53:85])

		// Check token exists
		if rawdb.ReadTokenMeta(GlobalChainDB, tokenID) == nil {
			return nil, errors.New("transfer: token does not exist")
		}

		// Note: caller address not available in Run() - need StatefulPrecompiledContract
		// For now, this is a simplified version. Full version uses RunStateful.

		// Add to recipient balance
		currentBal := rawdb.ReadTokenBalance(GlobalChainDB, tokenID, to)
		newBal := new(big.Int).Add(currentBal, amount)
		rawdb.WriteTokenBalance(GlobalChainDB, tokenID, to, newBal)

		return common.LeftPadBytes([]byte{1}, 32), nil

	case 0x03: // balanceOf(tokenID32, address20)
		if len(input) < 53 {
			return nil, errors.New("balanceOf: need tokenID(32) + address(20)")
		}
		tokenID := common.BytesToHash(input[1:33])
		addr := common.BytesToAddress(input[33:53])
		balance := rawdb.ReadTokenBalance(GlobalChainDB, tokenID, addr)
		return common.LeftPadBytes(balance.Bytes(), 32), nil

	case 0x04: // tokenInfo(tokenID32)
		if len(input) < 33 {
			return nil, errors.New("tokenInfo: need tokenID")
		}
		tokenID := common.BytesToHash(input[1:33])
		meta := rawdb.ReadTokenMeta(GlobalChainDB, tokenID)
		if meta == nil {
			return make([]byte, 32), nil
		}
		// Return padded to 128 bytes
		result := make([]byte, 128)
		copy(result, meta)
		return result, nil

	default:
		return nil, errors.New("novaTokenManager: unknown function")
	}
}
