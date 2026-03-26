// Ethernova: Parallel Transaction Execution (Phase 23)
//
// IMPORTANT (Gemini review): This is ANALYSIS ONLY - transactions still execute
// sequentially for consensus safety. The engine reports how much parallelism
// is available but does NOT change execution order.
//
// Why not actual parallel execution?
// EVM contracts make dynamic CALL to addresses computed at runtime.
// It's impossible to predict 100% which storage slots a tx will touch
// before executing it. If two "independent" txs collide at runtime,
// the state root would be non-deterministic = BAD BLOCK.
//
// Future: Implement optimistic parallel execution with abort-and-retry.
// If a collision is detected during parallel execution, abort the
// conflicting tx and re-execute it sequentially at the end of the block.
// This is what Monad and Block-STM (Aptos) do.

package vm

import (
	"sort"
	"sync"

	"github.com/ethereum/go-ethereum/common"
)

// TxAccessSet tracks which storage slots a transaction reads and writes.
type TxAccessSet struct {
	TxIndex int
	Reads   map[common.Address]map[common.Hash]bool
	Writes  map[common.Address]map[common.Hash]bool
}

// ParallelGroup is a set of transactions that can execute in parallel.
type ParallelGroup struct {
	TxIndices []int
}

// ClassifyTransactions groups transactions into parallel execution groups.
// Transactions in the same group have no state conflicts and can run simultaneously.
func ClassifyTransactions(accessSets []TxAccessSet) []ParallelGroup {
	n := len(accessSets)
	if n == 0 {
		return nil
	}

	// Build conflict graph
	conflicts := make([][]bool, n)
	for i := range conflicts {
		conflicts[i] = make([]bool, n)
	}

	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if hasConflict(accessSets[i], accessSets[j]) {
				conflicts[i][j] = true
				conflicts[j][i] = true
			}
		}
	}

	// Greedy coloring to assign groups
	colors := make([]int, n)
	for i := range colors {
		colors[i] = -1
	}

	for i := 0; i < n; i++ {
		used := make(map[int]bool)
		for j := 0; j < n; j++ {
			if conflicts[i][j] && colors[j] >= 0 {
				used[colors[j]] = true
			}
		}
		// Find first available color
		color := 0
		for used[color] {
			color++
		}
		colors[i] = color
	}

	// Build groups from colors
	groupMap := make(map[int][]int)
	for i, c := range colors {
		groupMap[c] = append(groupMap[c], i)
	}

	// Sort groups by first tx index for determinism
	keys := make([]int, 0, len(groupMap))
	for k := range groupMap {
		keys = append(keys, k)
	}
	sort.Ints(keys)

	groups := make([]ParallelGroup, 0, len(keys))
	for _, k := range keys {
		groups = append(groups, ParallelGroup{TxIndices: groupMap[k]})
	}
	return groups
}

func hasConflict(a, b TxAccessSet) bool {
	// Conflict if A writes something B reads/writes, or B writes something A reads
	for addr, slots := range a.Writes {
		if bSlots, ok := b.Reads[addr]; ok {
			for slot := range slots {
				if bSlots[slot] {
					return true
				}
			}
		}
		if bSlots, ok := b.Writes[addr]; ok {
			for slot := range slots {
				if bSlots[slot] {
					return true
				}
			}
		}
	}
	for addr, slots := range b.Writes {
		if aSlots, ok := a.Reads[addr]; ok {
			for slot := range slots {
				if aSlots[slot] {
					return true
				}
			}
		}
	}
	return false
}

// ParallelStats tracks how much parallelism is available per block.
type ParallelStats struct {
	mu              sync.Mutex
	TotalBlocks     uint64
	TotalTxs        uint64
	ParallelTxs     uint64
	LastBlockNumber uint64
	LastBlockTxs    int
	LastBlockParallel int
}

// GlobalParallelStats is the singleton stats tracker.
var GlobalParallelStats = &ParallelStats{}

// RecordBlock records parallel execution statistics for a block.
func (ps *ParallelStats) RecordBlock(blockNumber uint64, totalTxs, parallelTxs int) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.TotalBlocks++
	ps.TotalTxs += uint64(totalTxs)
	ps.ParallelTxs += uint64(parallelTxs)
	ps.LastBlockNumber = blockNumber
	ps.LastBlockTxs = totalTxs
	ps.LastBlockParallel = parallelTxs
}

// GetStats returns a copy of the current stats.
func (ps *ParallelStats) GetStats() map[string]interface{} {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ratio := float64(0)
	if ps.TotalTxs > 0 {
		ratio = float64(ps.ParallelTxs) / float64(ps.TotalTxs) * 100
	}
	return map[string]interface{}{
		"totalBlocks":       ps.TotalBlocks,
		"totalTxs":          ps.TotalTxs,
		"parallelTxs":       ps.ParallelTxs,
		"parallelRatio":     ratio,
		"lastBlock":         ps.LastBlockNumber,
		"lastBlockTxs":      ps.LastBlockTxs,
		"lastBlockParallel": ps.LastBlockParallel,
	}
}
