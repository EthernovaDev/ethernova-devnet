// Ethernova: Phase 5 lifecycle hook for the lyra2 consensus engine.
//
// Pulls the lifecycle integration out of consensus.go so consensus.go
// stays focused on PoW + reward logic. The hook is called from BOTH
// Finalize and FinalizeAndAssemble at the same point relative to other
// state mutations (after rewards, before IntermediateRoot).
//
// CONSENSUS-CRITICAL: this file MUST execute identically on every
// node. The function reads the lifecycle config from
// params/ethernova constants, performs only deterministic integer
// arithmetic and external-LevelDB writes, and never mutates the
// state trie. State-root divergence cannot originate here.

package lyra2

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params/ethernova"
)

// runStateLifecycle is invoked from Finalize and FinalizeAndAssemble.
// It records the set of accounts touched during this block in the
// external Phase 15+5 index, then runs a bounded sweep that flips any
// still-untouched accounts into the Archived tier and snapshots their
// storage root.
//
// The function is intentionally tolerant: if the lifecycle engine
// can't be constructed (e.g. on a test path that runs a memory-only
// state), we log and return without touching anything. State-root
// formation is independent of this hook.
func runStateLifecycle(statedb *state.StateDB, currentBlock uint64) {
	if statedb == nil {
		return
	}
	disk := statedb.Database().DiskDB()
	if disk == nil {
		// In-memory state DB (test harness). Lifecycle is a no-op
		// here — there is nowhere to persist the index.
		return
	}
	cfg := state.LifecycleConfig{
		Thresholds: state.LifecycleThresholds{
			ActiveBlocks: ethernova.ActiveTierBlocks,
			WarmBlocks:   ethernova.WarmTierBlocks,
			ColdBlocks:   ethernova.ColdTierBlocks,
		},
		Fees: state.LifecycleFees{
			PerByte: ethernova.WarmingFeePerByte,
		},
		MaxSweepPerBlock: ethernova.MaxLifecycleSweepPerBlock,
	}
	engine := state.NewStateLifecycleEngine(disk, cfg)

	// 1. Ingest: record block touches collected by StateDB during the
	//    block. The list is sorted+deduped inside RecordBlockTouches.
	touched := statedb.LifecycleTouchedAddresses()
	if len(touched) > 0 {
		engine.RecordBlockTouches(currentBlock, touched)
	}

	// 2. Sweep: flip any still-untouched accounts into Archived tier
	//    in the external index. Bounded by MaxLifecycleSweepPerBlock.
	res := engine.ProcessLifecycle(currentBlock)
	if res.DemotedToArchive == 0 {
		return
	}

	// 3. Capture cold roots for newly-archived accounts. The sweep
	//    candidate list is the deterministic source of truth, so we
	//    re-read it (cheap LevelDB get) and only act on accounts that
	//    have an archive marker but no cold root yet.
	if res.BlockProcessed == 0 {
		return
	}
	addrs := rawdb.ReadBlockTouchedAddresses(disk, res.BlockProcessed)
	for _, addr := range addrs {
		if !engine.IsArchived(addr) {
			continue
		}
		if engine.ColdStorageRoot(addr) != (common.Hash{}) {
			continue
		}
		root := statedb.GetStorageRoot(addr)
		engine.CaptureColdRoot(addr, root)
	}
	log.Debug("StateLifecycle: sweep complete",
		"block", currentBlock,
		"candidate", res.BlockProcessed,
		"inspected", res.Inspected,
		"toWarm", res.DemotedToWarm,
		"toCold", res.DemotedToCold,
		"toArchive", res.DemotedToArchive)
}
