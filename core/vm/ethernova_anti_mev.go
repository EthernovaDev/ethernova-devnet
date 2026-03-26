// Ethernova: Anti-MEV Fair Ordering (Phase 19)
//
// UPDATED after Gemini security review:
// Pure FIFO ordering creates a spam vector - bots flood the mempool with
// minimum-gas txs to get priority by arrival time instead of paying for it.
//
// Solution: Hybrid ordering
// - Base ordering is by arrival time (FIFO) = fair for users
// - BUT: rate limit per sender address in the mempool
//   - Max 16 pending txs per sender (prevents spam flooding)
//   - After 16 pending, new txs from that sender are dropped
// - Priority fee still accepted but NOT used for ordering
//   - Higher fee = higher chance of inclusion in full blocks
//   - But within the block, order is FIFO

package vm

import (
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

const (
	// MaxPendingPerSender limits how many pending txs one address can have.
	// Prevents spam attacks where bots flood mempool with minimum-gas txs.
	MaxPendingPerSender = 16
)

// FairOrderingConfig controls the anti-MEV fair ordering system.
type FairOrderingConfig struct {
	Enabled            bool
	OrderByArrival     bool
	RateLimitPerSender int // max pending txs per sender
}

// GlobalFairOrdering is the singleton config.
var GlobalFairOrdering = &FairOrderingConfig{
	Enabled:            true,
	OrderByArrival:     true,
	RateLimitPerSender: MaxPendingPerSender,
}

// TxArrivalTracker records when transactions arrive at the node.
type TxArrivalTracker struct {
	mu           sync.Mutex
	arrivals     map[common.Hash]time.Time
	senderCount  map[common.Address]int // track pending txs per sender
}

// GlobalTxArrivals tracks transaction arrival times.
var GlobalTxArrivals = &TxArrivalTracker{
	arrivals:    make(map[common.Hash]time.Time),
	senderCount: make(map[common.Address]int),
}

// RecordArrival marks when a transaction was first seen.
func (t *TxArrivalTracker) RecordArrival(txHash common.Hash) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, exists := t.arrivals[txHash]; !exists {
		t.arrivals[txHash] = time.Now()
	}
}

// CanAcceptFromSender returns true if the sender hasn't exceeded the rate limit.
func (t *TxArrivalTracker) CanAcceptFromSender(sender common.Address) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.senderCount[sender] < MaxPendingPerSender
}

// IncrementSender records a new pending tx from this sender.
func (t *TxArrivalTracker) IncrementSender(sender common.Address) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.senderCount[sender]++
}

// DecrementSender removes a pending tx count (tx mined or dropped).
func (t *TxArrivalTracker) DecrementSender(sender common.Address) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.senderCount[sender]--
	if t.senderCount[sender] <= 0 {
		delete(t.senderCount, sender)
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
