// Ethernova: Anti-MEV Fair Ordering (Phase 19)
// Prevents front-running and sandwich attacks at the protocol level.
//
// Mechanism: Commit-Reveal transaction ordering
// 1. Users submit encrypted tx hash (commit) - nobody can see the contents
// 2. After N blocks, the actual transaction is revealed
// 3. Transactions are ordered by commit time, not gas price
//
// For the devnet, we implement a simpler version:
// - Transaction ordering within a block is by ARRIVAL TIME, not gas price
// - The miner cannot reorder transactions to extract MEV
// - A "fair ordering" flag in the block header indicates this policy
//
// This eliminates:
// - Front-running (can't see pending txs before they're ordered)
// - Sandwich attacks (can't insert txs around a target)
// - Gas price bidding wars (ordering is first-come-first-serve)

package vm

import (
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// FairOrderingConfig controls the anti-MEV fair ordering system.
type FairOrderingConfig struct {
	Enabled        bool
	OrderByArrival bool // Order txs by arrival time, not gas price
}

// GlobalFairOrdering is the singleton config.
var GlobalFairOrdering = &FairOrderingConfig{
	Enabled:        true,
	OrderByArrival: true,
}

// TxArrivalTracker records when transactions arrive at the node.
// Used for fair ordering (first-come-first-serve instead of gas price).
type TxArrivalTracker struct {
	mu       sync.Mutex
	arrivals map[common.Hash]time.Time
}

// GlobalTxArrivals tracks transaction arrival times.
var GlobalTxArrivals = &TxArrivalTracker{
	arrivals: make(map[common.Hash]time.Time),
}

// RecordArrival marks when a transaction was first seen.
func (t *TxArrivalTracker) RecordArrival(txHash common.Hash) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, exists := t.arrivals[txHash]; !exists {
		t.arrivals[txHash] = time.Now()
	}
}

// GetArrival returns when a transaction was first seen.
func (t *TxArrivalTracker) GetArrival(txHash common.Hash) time.Time {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.arrivals[txHash]
}

// Cleanup removes old entries to prevent memory leak.
func (t *TxArrivalTracker) Cleanup(olderThan time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	cutoff := time.Now().Add(-olderThan)
	for hash, arrival := range t.arrivals {
		if arrival.Before(cutoff) {
			delete(t.arrivals, hash)
		}
	}
}
