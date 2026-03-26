// Ethernova: Native Multi-Token Support (Phase 20) - FULL INTEGRATION
// Stateful precompile with caller awareness for balance verification.

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
	case 0x01: return 50000
	case 0x02: return 5000
	case 0x03: return 1000
	case 0x04: return 1000
	default:   return 0
	}
}

func (c *novaTokenManager) Run(input []byte) ([]byte, error) {
	return nil, errors.New("novaTokenManager: use RunStateful")
}

// RunStateful has access to the EVM and caller address.
func (c *novaTokenManager) RunStateful(evm *EVM, caller common.Address, input []byte) ([]byte, error) {
	if len(input) < 1 {
		return nil, errors.New("novaTokenManager: empty input")
	}
	if GlobalChainDB == nil {
		return nil, errors.New("novaTokenManager: database not initialized")
	}

	switch input[0] {
	case 0x01: // createToken(data...) -> tokenID
		if len(input) < 2 {
			return nil, errors.New("createToken: insufficient input")
		}
		// TokenID = keccak256(caller + input)
		seed := append(caller.Bytes(), input[1:]...)
		tokenID := crypto.Keccak256Hash(seed)

		// Check not already created
		if rawdb.ReadTokenMeta(GlobalChainDB, tokenID) != nil {
			return nil, errors.New("createToken: token already exists")
		}

		// Store metadata (includes creator info)
		meta := append(caller.Bytes(), input[1:]...)
		rawdb.WriteTokenMeta(GlobalChainDB, tokenID, meta)

		// If supply is specified (last 32 bytes), assign to creator
		if len(input) >= 34 {
			supply := new(big.Int).SetBytes(input[len(input)-32:])
			if supply.Sign() > 0 {
				rawdb.WriteTokenBalance(GlobalChainDB, tokenID, caller, supply)
			}
		}

		return tokenID.Bytes(), nil

	case 0x02: // transfer(tokenID32, to20, amount32)
		if len(input) < 85 {
			return nil, errors.New("transfer: need tokenID(32) + to(20) + amount(32)")
		}
		tokenID := common.BytesToHash(input[1:33])
		to := common.BytesToAddress(input[33:53])
		amount := new(big.Int).SetBytes(input[53:85])

		if amount.Sign() <= 0 {
			return nil, errors.New("transfer: amount must be positive")
		}

		// Check token exists
		if rawdb.ReadTokenMeta(GlobalChainDB, tokenID) == nil {
			return nil, errors.New("transfer: token does not exist")
		}

		// Check sender balance
		senderBal := rawdb.ReadTokenBalance(GlobalChainDB, tokenID, caller)
		if senderBal.Cmp(amount) < 0 {
			return nil, errors.New("transfer: insufficient balance")
		}

		// Deduct from sender
		newSenderBal := new(big.Int).Sub(senderBal, amount)
		rawdb.WriteTokenBalance(GlobalChainDB, tokenID, caller, newSenderBal)

		// Add to recipient
		recipientBal := rawdb.ReadTokenBalance(GlobalChainDB, tokenID, to)
		newRecipientBal := new(big.Int).Add(recipientBal, amount)
		rawdb.WriteTokenBalance(GlobalChainDB, tokenID, to, newRecipientBal)

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
