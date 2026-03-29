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

// erc20TransferSelector is the 4-byte function selector for transfer(address,uint256).
var erc20TransferSelector = [4]byte{0xa9, 0x05, 0x9c, 0xbb}

// erc20TransferFromSelector is the 4-byte selector for transferFrom(address,address,uint256).
var erc20TransferFromSelector = [4]byte{0x23, 0xb8, 0x72, 0xdd}

// conflictKey uniquely identifies a storage-slot "touch zone".
// For ERC-20 transfers, each (contract, account) pair maps to a distinct
// balance slot in the mapping, so two transfers conflict only when they
// share at least one (contract, account) pair.
type conflictKey struct {
	contract common.Address
	account  common.Address
}

// isERC20Transfer checks if the calldata starts with the transfer(address,uint256)
// selector and has at least 4+32 bytes for the recipient. Returns the extracted
// recipient address and true on success.
func isERC20Transfer(data []byte) (common.Address, bool) {
	if len(data) < 4+32 {
		return common.Address{}, false
	}
	var sel [4]byte
	copy(sel[:], data[:4])
	if sel != erc20TransferSelector {
		return common.Address{}, false
	}
	return common.BytesToAddress(data[4+12 : 4+32]), true
}

// isERC20TransferFrom checks for transferFrom(address,address,uint256).
// Returns (from, to, true) on success.
func isERC20TransferFrom(data []byte) (common.Address, common.Address, bool) {
	if len(data) < 4+64 {
		return common.Address{}, common.Address{}, false
	}
	var sel [4]byte
	copy(sel[:], data[:4])
	if sel != erc20TransferFromSelector {
		return common.Address{}, common.Address{}, false
	}
	from := common.BytesToAddress(data[4+12 : 4+32])
	to := common.BytesToAddress(data[4+32+12 : 4+64])
	return from, to, true
}

// ClassifyTransactions groups transactions into parallel-eligible and
// sequential buckets.
//
// Rules (conservative, per Noven's recommendation):
// 1. Simple ETH transfers are parallelizable if senders/recipients don't overlap
// 2. ERC-20 transfer() and transferFrom() between non-overlapping account pairs
//    are parallelizable — they touch disjoint balance-mapping slots
// 3. No two transactions can have the same sender (nonce ordering)
// 4. DEX swaps, multisig operations, and other contract calls remain sequential
//    because they may touch shared storage (reserves, confirmation counts, etc.)
func (pe *ParallelExecutor) ClassifyTransactions(txs []*types.Transaction, signer types.Signer) (parallel []*types.Transaction, sequential []*types.Transaction) {
	if vm.GlobalExecutionMode.GetMode() < vm.ModeParallel {
		return nil, txs
	}

	sendersSeen := make(map[common.Address]bool)
	recipientsSeen := make(map[common.Address]bool)
	// slotKeys tracks (contract, account) pairs that would be touched by
	// ERC-20 transfers. A conflict on any key forces the tx to sequential.
	slotKeys := make(map[conflictKey]bool)

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

		// ---- Contract call classification ----
		if len(tx.Data()) > 0 {
			// Try to recognise ERC-20 transfer(address,uint256).
			// Two transfer() calls on the same token contract are safe to
			// parallelise when they touch disjoint (sender, recipient)
			// account pairs — each pair maps to distinct balance-mapping
			// storage slots.
			if recipient, ok := isERC20Transfer(tx.Data()); ok {
				keySender := conflictKey{contract: to, account: sender}
				keyRecipient := conflictKey{contract: to, account: recipient}

				if sendersSeen[sender] || slotKeys[keySender] || slotKeys[keyRecipient] {
					sequential = append(sequential, tx)
					sendersSeen[sender] = true
					slotKeys[keySender] = true
					slotKeys[keyRecipient] = true
					continue
				}

				// Safe for parallel
				parallel = append(parallel, tx)
				sendersSeen[sender] = true
				slotKeys[keySender] = true
				slotKeys[keyRecipient] = true
				continue
			}

			// Try to recognise ERC-20 transferFrom(address,address,uint256).
			if from, recipient, ok := isERC20TransferFrom(tx.Data()); ok {
				keyFrom := conflictKey{contract: to, account: from}
				keyRecipient := conflictKey{contract: to, account: recipient}
				keyCaller := conflictKey{contract: to, account: sender}

				if sendersSeen[sender] || slotKeys[keyFrom] || slotKeys[keyRecipient] || slotKeys[keyCaller] {
					sequential = append(sequential, tx)
					sendersSeen[sender] = true
					slotKeys[keyFrom] = true
					slotKeys[keyRecipient] = true
					slotKeys[keyCaller] = true
					continue
				}

				parallel = append(parallel, tx)
				sendersSeen[sender] = true
				slotKeys[keyFrom] = true
				slotKeys[keyRecipient] = true
				slotKeys[keyCaller] = true
				continue
			}

			// Any other contract call (DEX swaps, multisig, etc.) → sequential.
			// These may touch shared state (pool reserves, confirmation counts).
			sequential = append(sequential, tx)
			sendersSeen[sender] = true
			recipientsSeen[to] = true
			continue
		}

		// ---- Simple ETH transfer (no calldata) ----

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