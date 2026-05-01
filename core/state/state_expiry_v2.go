// Ethernova: State Expiry v2 (Phase 15) — Phase 5 transitional shim.
//
// As of NIP-0004 Phase 5, this file is a thin compatibility wrapper
// that delegates to the StateLifecycleEngine (in state_lifecycle.go).
// The Phase 15 contract (TouchContract / RecordBlockTouches /
// SweepExpired) remains as an external API but is implemented by
// routing through the lifecycle engine's external LevelDB index.
//
// Why a shim instead of an outright rewrite: removing the type and
// methods would break any out-of-tree callers that already reference
// StateExpiryEngine. Keeping the shape identical means downstream
// upgrades to v1.1.x (Phase 5) are a recompile, not a refactor.
//
// SweepExpired is intentionally a no-op. The destructive trie-
// deleting sweep was disabled in v1.0.7 due to the cross-platform
// RLP encoding bug; Phase 5 supersedes it with a non-destructive
// tier marker + witness restoration model. Restoring contracts is
// now done via the 0x2F precompile (novaStateWitness selector 0x02).

package state

import (
	"bytes"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/ethdb"
)

// StateExpiryEngine is the Phase 15 external-index expiry engine,
// preserved for API compatibility. Internally it shares the same
// LevelDB prefix space as the Phase 5 StateLifecycleEngine ('X' for
// per-account last-touched, 'x' for per-block touch lists, 'A' for
// archived account body).
type StateExpiryEngine struct {
	db ethdb.Database
}

// NewStateExpiryEngine creates a Phase 15 shim. New code should use
// NewStateLifecycleEngine directly; this constructor is kept for
// backward compatibility with v1.0.8 callers.
func NewStateExpiryEngine(db ethdb.Database) *StateExpiryEngine {
	return &StateExpiryEngine{db: db}
}

// TouchContract records that a contract was accessed at the given
// block. Routes through the same 'X' prefix the Phase 5 engine
// reads, so a TouchContract by Phase 15 callers is visible to the
// Phase 5 tier classifier and vice versa.
func (e *StateExpiryEngine) TouchContract(addr common.Address, blockNumber uint64) {
	if rawdb.ReadLastTouched(e.db, addr) == blockNumber {
		return
	}
	batch := e.db.NewBatch()
	rawdb.WriteLastTouched(batch, addr, blockNumber)
	_ = batch.Write()
}

// RecordBlockTouches saves the sorted, deduplicated list of contracts
// touched during a block. Output is bit-identical to the Phase 5
// engine's equivalent — both write through rawdb.WriteBlockTouched
// Addresses with the same sort order.
func (e *StateExpiryEngine) RecordBlockTouches(blockNumber uint64, addresses []common.Address) {
	if len(addresses) == 0 {
		return
	}
	sort.Slice(addresses, func(i, j int) bool {
		return bytes.Compare(addresses[i][:], addresses[j][:]) < 0
	})
	unique := make([]common.Address, 0, len(addresses))
	for i, addr := range addresses {
		if i == 0 || addr != addresses[i-1] {
			unique = append(unique, addr)
		}
	}
	batch := e.db.NewBatch()
	rawdb.WriteBlockTouchedAddresses(batch, blockNumber, unique)
	_ = batch.Write()
}

// SweepExpired is a no-op as of Phase 5. The destructive trie sweep
// it used to implement is replaced by:
//
//   - Phase 5 tier markers in the 'T' prefix (non-destructive).
//   - Cold storage roots in the 'C' prefix (witness anchor).
//   - 0x2F precompile selector 0x02 for resurrection (witness path).
//
// Returning nil keeps callers happy without forcing them to be aware
// of the supersession; they will simply observe that no addresses
// are returned.
func (e *StateExpiryEngine) SweepExpired(
	statedb *StateDB,
	currentBlock uint64,
	expiryPeriod uint64,
) []common.Address {
	_ = statedb
	_ = currentBlock
	_ = expiryPeriod
	return nil
}
