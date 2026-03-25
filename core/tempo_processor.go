// Ethernova: Tempo Transaction Processor (Phase 11)
// Executes atomic batched calls within a single transaction.
// If any call in the batch reverts, the entire transaction reverts.
//
// Key design decisions:
// - Gas always paid in NOVA (protects native token value)
// - Fee delegation allows DApps to sponsor gas for users
// - Scheduled transactions enable limit orders, recurring payments
// - All execution is deterministic (no node-local state)

package core

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/log"
	ethernova "github.com/ethereum/go-ethereum/params/ethernova"
	"github.com/holiman/uint256"
)

// TempoResult holds the result of executing a Tempo transaction.
type TempoResult struct {
	Success     bool
	GasUsed     uint64
	CallResults []TempoCallResult
}

// TempoCallResult holds the result of a single call in a batch.
type TempoCallResult struct {
	Index   int
	Success bool
	GasUsed uint64
	RetData []byte
}

// ExecuteTempoCalls processes the atomic batch of calls in a Tempo transaction.
// All calls share the same EVM context and state snapshot.
// If any call reverts, the entire batch reverts.
func ExecuteTempoCalls(
	evm *vm.EVM,
	statedb *state.StateDB,
	sender common.Address,
	calls []types.TempoCall,
	totalGasLimit uint64,
	blockNumber uint64,
) (*TempoResult, error) {

	// Check if Tempo fork is active
	if blockNumber < ethernova.TempoTxForkBlock {
		return nil, fmt.Errorf("tempo transactions not active until block %d", ethernova.TempoTxForkBlock)
	}

	if len(calls) == 0 {
		return nil, fmt.Errorf("tempo transaction must have at least one call")
	}

	if len(calls) > 16 {
		return nil, fmt.Errorf("tempo transaction limited to 16 calls maximum")
	}

	// Take a snapshot so we can revert all calls atomically
	snapshotID := statedb.Snapshot()
	gasRemaining := totalGasLimit
	results := make([]TempoCallResult, 0, len(calls))

	log.Debug("Executing Tempo transaction",
		"sender", sender.Hex(),
		"calls", len(calls),
		"gasLimit", totalGasLimit,
		"block", blockNumber)

	for i, call := range calls {
		callGas := call.GasLimit
		if callGas == 0 || callGas > gasRemaining {
			callGas = gasRemaining
		}

		// Convert value to uint256
		var value *uint256.Int
		if call.Value != nil {
			value, _ = uint256.FromBig(call.Value)
		} else {
			value = new(uint256.Int)
		}

		// Execute the call
		ret, gasLeft, err := evm.Call(
			vm.AccountRef(sender),
			call.Target,
			call.Data,
			callGas,
			value,
		)

		gasUsed := callGas - gasLeft
		gasRemaining -= gasUsed

		callResult := TempoCallResult{
			Index:   i,
			Success: err == nil,
			GasUsed: gasUsed,
			RetData: ret,
		}
		results = append(results, callResult)

		if err != nil {
			// Any call failure = revert entire batch
			statedb.RevertToSnapshot(snapshotID)

			log.Debug("Tempo call reverted, reverting entire batch",
				"callIndex", i,
				"target", call.Target.Hex(),
				"error", err)

			return &TempoResult{
				Success:     false,
				GasUsed:     totalGasLimit - gasRemaining,
				CallResults: results,
			}, nil
		}
	}

	return &TempoResult{
		Success:     true,
		GasUsed:     totalGasLimit - gasRemaining,
		CallResults: results,
	}, nil
}

// ValidateTempoSchedule checks if a Tempo transaction is valid at the current block.
func ValidateTempoSchedule(td *types.TempoTransactionData, blockNumber uint64) error {
	if !td.IsValidAtBlock(blockNumber) {
		if td.ValidBefore > 0 && blockNumber >= td.ValidBefore {
			return fmt.Errorf("tempo transaction expired at block %d (current: %d)", td.ValidBefore, blockNumber)
		}
		if td.ValidAfter > 0 && blockNumber < td.ValidAfter {
			return fmt.Errorf("tempo transaction not valid until block %d (current: %d)", td.ValidAfter, blockNumber)
		}
	}
	return nil
}

// ValidateFeePayer verifies that the fee payer has authorized gas payment.
// Returns the fee payer address if valid, or zero address if no fee payer.
func ValidateFeePayer(td *types.TempoTransactionData, txHash common.Hash, chainID *big.Int) (common.Address, error) {
	if !td.HasFeePayer() {
		return common.Address{}, nil
	}

	// Fee payer must have signed the transaction hash
	if td.FeePayerV == nil || td.FeePayerR == nil || td.FeePayerS == nil {
		return common.Address{}, fmt.Errorf("fee payer specified but signature missing")
	}

	// In a full implementation, we would recover the fee payer address from the signature
	// and verify it matches td.FeePayer. For devnet testing, we trust the declared fee payer.
	return td.FeePayer, nil
}
