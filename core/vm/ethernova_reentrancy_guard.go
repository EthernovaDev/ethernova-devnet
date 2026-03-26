// Ethernova: Native Reentrancy Protection (Phase 17)
// Blocks reentrant calls by default at the EVM level.
// No more DAO hacks, no more Curve exploits.
// Contracts can opt-out via a special storage flag if they need reentrancy.

package vm

import (
	"sync"

	"github.com/ethereum/go-ethereum/common"
)

// ReentrancyGuard tracks which contracts are currently executing.
// If a contract tries to call itself (or be called while already executing),
// the call is rejected with an error.
type ReentrancyGuard struct {
	mu       sync.Mutex
	executing map[common.Address]bool
}

// GlobalReentrancyGuard is the per-block reentrancy tracker.
var GlobalReentrancyGuard = &ReentrancyGuard{
	executing: make(map[common.Address]bool),
}

// Enter marks a contract as currently executing.
// Returns false if the contract is already executing (reentrancy detected).
func (rg *ReentrancyGuard) Enter(addr common.Address) bool {
	rg.mu.Lock()
	defer rg.mu.Unlock()
	if rg.executing[addr] {
		return false // REENTRANCY BLOCKED
	}
	rg.executing[addr] = true
	return true
}

// Exit marks a contract as no longer executing.
func (rg *ReentrancyGuard) Exit(addr common.Address) {
	rg.mu.Lock()
	defer rg.mu.Unlock()
	delete(rg.executing, addr)
}

// Reset clears all tracking (called at start of each transaction).
func (rg *ReentrancyGuard) Reset() {
	rg.mu.Lock()
	defer rg.mu.Unlock()
	rg.executing = make(map[common.Address]bool)
}

// IsExecuting returns true if the contract is currently in the call stack.
func (rg *ReentrancyGuard) IsExecuting(addr common.Address) bool {
	rg.mu.Lock()
	defer rg.mu.Unlock()
	return rg.executing[addr]
}
