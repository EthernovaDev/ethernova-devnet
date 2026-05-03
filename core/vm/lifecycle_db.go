// Ethernova: Phase 5 chain-DB registration for the EVM gas surcharge.
//
// applyLifecycleSurcharge runs inside the EVM gas-calculation path,
// which means it has no direct access to the chain DB. Originally we
// tried to recover the DB via type assertion + Database().DiskDB(),
// but this fails silently during eth_call / eth_estimateGas / debug
// trace simulations where the StateDB may be wrapped (parallel TX
// executor cache layer, override layer, etc.) and DiskDB() returns
// nil — the surcharge then silently drops to zero, leaving estimate
// gas LOWER than actual mined gas, and ultimately breaking UX
// because user wallets size gas limits from the estimate.
//
// Package-global registration is the simplest robust fix: the chain
// DB is opened exactly once at node startup; we register it here at
// that moment; every subsequent gas calculation reads the global
// regardless of how the StateDB is wrapped.
//
// CONSENSUS-CRITICAL: SetLifecycleDB MUST be called once and exactly
// once during node initialization, BEFORE any block is processed.
// Calling it twice with different DBs is a misconfiguration. Calling
// it never (e.g. test harness) means surcharge will be a no-op,
// which is the correct conservative fallback.

package vm

import (
	"sync/atomic"

	"github.com/ethereum/go-ethereum/ethdb"
)

// lifecycleDiskDB holds a pointer to the chain's underlying
// ethdb.KeyValueStore. Stored as atomic.Pointer so reads from the EVM
// gas path are lock-free.
var lifecycleDiskDB atomic.Pointer[ethdb.KeyValueStore]

// SetLifecycleDB registers the chain's disk DB so that the Phase 5
// SLOAD warming-fee surcharge can look up tier markers regardless of
// which StateDB (real, copy, override, simulated) the EVM is using.
//
// Called from eth/backend.go right after eth.chainDb is opened.
func SetLifecycleDB(db ethdb.KeyValueStore) {
	if db == nil {
		return
	}
	lifecycleDiskDB.Store(&db)
}

// getLifecycleDB returns the registered chain DB or nil if not yet
// set (test harness, or pre-init code paths). Callers must treat nil
// as "lifecycle disabled" and apply no surcharge.
func getLifecycleDB() ethdb.KeyValueStore {
	p := lifecycleDiskDB.Load()
	if p == nil {
		return nil
	}
	return *p
}
