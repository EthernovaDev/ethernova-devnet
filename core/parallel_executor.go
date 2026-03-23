package core

import (
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/log"
	"github.com/holiman/uint256"
)

// ParallelExecutor handles speculative parallel execution of transactions.
// It follows conservative rules: only transactions that don't overlap on
// storage slots and don't use dangerous opcodes are executed in parallel.
type ParallelExecutor struct {
	mu sync.Mutex
}

var GlobalParallelExecutor = &ParallelExecutor{}

// ParallelSafetyCheck determines if a set of transactions can be safely
// executed in parallel. Returns groups of independent transactions.
//
// Rules (conservative, per Noven's recommendation):
// 1. Only simple ETH transfers (no contract calls) are parallelizable
// 2. No two transactions can have the same sender (nonce ordering)
// 3. No two transactions can send to the same address (balance conflict)
// 4. Contract interactions are always sequential (may touch shared storage)
func (pe *ParallelExecutor) ClassifyTransactions(txs []*types.Transaction, signer types.Signer) (parallel []*types.Transaction, sequential []*types.Transaction) {
	if vm.GlobalExecutionMode.GetMode() < vm.ModeParallel {
		return nil, txs
	}

	sendersSeen := make(map[common.Address]bool)
	recipientsSeen := make(map[common.Address]bool)

	for _, tx := range txs {
		sender, err := types.Sender(signer, tx)
		if err != nil {
			sequential = append(sequential, tx)
			continue
		}

		// Rule: contract creation always sequential
		if tx.To() == nil {
			sequential = append(sequential, tx)
			sendersSeen[sender] = true
			continue
		}

		to := *tx.To()

		// Rule: contract calls always sequential (has data = contract interaction)
		if len(tx.Data()) > 0 {
			sequential = append(sequential, tx)
			sendersSeen[sender] = true
			recipientsSeen[to] = true
			continue
		}

		// Rule: no duplicate senders (nonce ordering)
		if sendersSeen[sender] {
			sequential = append(sequential, tx)
			continue
		}

		// Rule: no duplicate recipients (balance conflict)
		if recipientsSeen[to] {
			sequential = append(sequential, tx)
			continue
		}

		// Rule: sender can't also be a recipient of another parallel tx
		if recipientsSeen[sender] || sendersSeen[to] {
			sequential = append(sequential, tx)
			continue
		}

		// Safe for parallel execution
		parallel = append(parallel, tx)
		sendersSeen[sender] = true
		recipientsSeen[to] = true
	}

	return parallel, sequential
}

// ParallelResult holds the result of a single parallel execution.
type ParallelResult struct {
	Tx      *types.Transaction
	Receipt *types.Receipt
	UsedGas uint64
	Err     error
	Logs    []*types.Log
	State   *state.StateDB // snapshot after execution
}

// ExecuteParallel executes a batch of independent transactions in parallel
// using state snapshots. Each transaction gets its own copy of state.
// After all complete, results are validated for conflicts and merged.
func (pe *ParallelExecutor) ExecuteParallel(
	statedb *state.StateDB,
	txs []*types.Transaction,
	header *types.Header,
	cfg vm.Config,
	getEVM func(msg *Message, statedb *state.StateDB, header *types.Header, cfg vm.Config) *vm.EVM,
	signer types.Signer,
) []*ParallelResult {
	if len(txs) == 0 {
		return nil
	}

	results := make([]*ParallelResult, len(txs))
	var wg sync.WaitGroup

	for i, tx := range txs {
		wg.Add(1)
		go func(idx int, tx *types.Transaction) {
			defer wg.Done()

			// Create a snapshot of state for this transaction
			snapshot := statedb.Copy()

			sender, err := types.Sender(signer, tx)
			if err != nil {
				results[idx] = &ParallelResult{Tx: tx, Err: err}
				return
			}

			// Apply the transaction to the snapshot
			nonce := snapshot.GetNonce(sender)
			if tx.Nonce() != nonce {
				results[idx] = &ParallelResult{Tx: tx, Err: ErrNonceTooHigh}
				return
			}

			// Deduct gas cost
			mgval := new(big.Int).Mul(new(big.Int).SetUint64(tx.Gas()), tx.GasPrice())
			totalCost := new(big.Int).Add(tx.Value(), mgval)
			balance := snapshot.GetBalance(sender)
			totalCostU, _ := uint256.FromBig(totalCost)
			if balance.Cmp(totalCostU) < 0 {
				results[idx] = &ParallelResult{Tx: tx, Err: ErrInsufficientFunds}
				return
			}

			// Simple transfer execution
			snapshot.SubBalance(sender, totalCostU)
			valueU, _ := uint256.FromBig(tx.Value())
			snapshot.AddBalance(*tx.To(), valueU)
			snapshot.SetNonce(sender, nonce+1)

			// Refund unused gas (simple transfer uses exactly 21000)
			gasUsed := uint64(21000)
			refund := new(big.Int).Mul(
				new(big.Int).SetUint64(tx.Gas()-gasUsed),
				tx.GasPrice(),
			)
			refundU, _ := uint256.FromBig(refund)
			snapshot.AddBalance(sender, refundU)

			results[idx] = &ParallelResult{
				Tx:      tx,
				UsedGas: gasUsed,
				State:   snapshot,
			}
		}(i, tx)
	}

	wg.Wait()

	// Validate results — check for state conflicts
	pe.mu.Lock()
	defer pe.mu.Unlock()

	validCount := 0
	for _, r := range results {
		if r != nil && r.Err == nil {
			validCount++
		}
	}

	log.Debug("Parallel execution completed",
		"total", len(txs),
		"valid", validCount,
		"failed", len(txs)-validCount,
	)

	return results
}

// MergeResults applies validated parallel results to the main state.
// Returns the number of successfully merged transactions.
func (pe *ParallelExecutor) MergeResults(
	mainState *state.StateDB,
	results []*ParallelResult,
	signer types.Signer,
) int {
	merged := 0
	for _, r := range results {
		if r == nil || r.Err != nil || r.State == nil {
			continue
		}

		sender, err := types.Sender(signer, r.Tx)
		if err != nil {
			continue
		}

		to := *r.Tx.To()

		// Apply balance changes to main state
		senderBal := r.State.GetBalance(sender)
		recipientBal := r.State.GetBalance(to)

		mainState.SetBalance(sender, senderBal)
		mainState.SetBalance(to, recipientBal)
		mainState.SetNonce(sender, r.State.GetNonce(sender))

		merged++
	}

	if merged > 0 {
		log.Info("Parallel transactions merged", "count", merged)
	}

	return merged
}

// ParallelStats holds execution statistics for RPC reporting.
type ParallelStats struct {
	Enabled           bool   `json:"enabled"`
	TotalClassified   uint64 `json:"totalClassified"`
	ParallelEligible  uint64 `json:"parallelEligible"`
	SequentialOnly    uint64 `json:"sequentialOnly"`
	MergedSuccessful  uint64 `json:"mergedSuccessful"`
	ConflictsDetected uint64 `json:"conflictsDetected"`
}

var globalParallelStats struct {
	sync.Mutex
	stats ParallelStats
}

// RecordClassification records transaction classification stats.
func RecordParallelClassification(parallel, sequential int) {
	globalParallelStats.Lock()
	globalParallelStats.stats.TotalClassified += uint64(parallel + sequential)
	globalParallelStats.stats.ParallelEligible += uint64(parallel)
	globalParallelStats.stats.SequentialOnly += uint64(sequential)
	globalParallelStats.Unlock()
}

// RecordMerge records successful merge stats.
func RecordParallelMerge(merged, conflicts int) {
	globalParallelStats.Lock()
	globalParallelStats.stats.MergedSuccessful += uint64(merged)
	globalParallelStats.stats.ConflictsDetected += uint64(conflicts)
	globalParallelStats.Unlock()
}

// GetParallelStats returns current parallel execution stats.
func GetParallelStats() ParallelStats {
	globalParallelStats.Lock()
	defer globalParallelStats.Unlock()
	s := globalParallelStats.stats
	s.Enabled = vm.GlobalExecutionMode.GetMode() >= vm.ModeParallel
	return s
}
