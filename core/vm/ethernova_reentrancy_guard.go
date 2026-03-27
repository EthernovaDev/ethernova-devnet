// Ethernova: Native Reentrancy Protection (Phase 17)
//
// UPDATED after Gemini security review:
// Old behavior: blocked ALL reentrant calls (broke DeFi composability)
// New behavior: blocks SELF-reentrancy only (A->B->A blocked, A->B->C allowed)
//
// This preserves DeFi composability (flash loans, DEX aggregators, oracles)
// while still preventing the DAO hack and Curve-style reentrancy exploits.
//
// How it works:
// - When contract A is called, it's added to the call stack
// - If someone tries to call A again while A is still executing = BLOCKED
// - But A can call B, and B can call C, and C can call D (all different) = ALLOWED
// - When A finishes, it's removed from the stack
//
// This is "same-contract reentrancy protection" - the most dangerous kind
// of reentrancy (read-then-write on same storage) is blocked, while
// cross-contract calls (the basis of DeFi composability) work normally.

package vm

import (
	"sync"

	"github.com/ethereum/go-ethereum/common"
)

// ReentrancyGuard tracks which contracts are currently executing.
// Only blocks SELF-reentrancy (same contract called while already in call stack).
// Cross-contract calls are always allowed.
type ReentrancyGuard struct {
	mu        sync.Mutex
	callStack map[common.Address]int // address -> call depth count
}

// GlobalReentrancyGuard is DEPRECATED.
//
// DO NOT USE for consensus-critical execution paths.
//
// This was the root cause of BAD BLOCK errors: being a process-wide singleton,
// concurrent EVM instances (eth_call, miner, tracers) interfered with block
// processing, causing different execution paths on different nodes.
//
// Replaced by PerEVMReentrancyGuard (embedded in each EVM struct).
// Kept only for backward-compatible RPC inspection (IsExecuting).
var GlobalReentrancyGuard = &ReentrancyGuard{
	callStack: make(map[common.Address]int),
}

// Enter marks a contract as currently executing.
// Returns false ONLY if this exact contract is already in the call stack
// (self-reentrancy). Cross-contract calls always return true.
func (rg *ReentrancyGuard) Enter(addr common.Address) bool {
	rg.mu.Lock()
	defer rg.mu.Unlock()
	if rg.callStack[addr] > 0 {
		return false // SELF-REENTRANCY BLOCKED
	}
	rg.callStack[addr]++
	return true
}

// Exit marks a contract as no longer executing.
func (rg *ReentrancyGuard) Exit(addr common.Address) {
	rg.mu.Lock()
	defer rg.mu.Unlock()
	rg.callStack[addr]--
	if rg.callStack[addr] <= 0 {
		delete(rg.callStack, addr)
	}
}

// Reset clears all tracking (called at start of each transaction).
func (rg *ReentrancyGuard) Reset() {
	rg.mu.Lock()
	defer rg.mu.Unlock()
	rg.callStack = make(map[common.Address]int)
}

// IsExecuting returns true if the contract is currently in the call stack.
func (rg *ReentrancyGuard) IsExecuting(addr common.Address) bool {
	rg.mu.Lock()
	defer rg.mu.Unlock()
	return rg.callStack[addr] > 0
}

// ============================================================================
// PER-EVM REENTRANCY GUARD (consensus-safe replacement)
// ============================================================================
//
// CRITICAL CONSENSUS FIX:
// The GlobalReentrancyGuard above is UNSAFE for consensus because it is
// shared across ALL concurrent EVM instances (block processing, eth_call,
// miner, tracers). When multiple EVM instances run concurrently:
//   - eth_call executing contract A at depth>0 marks A as "executing"
//   - Block processing tries to call A at depth>0 → falsely blocked
//   - Block processing gets ErrExecutionReverted instead of normal execution
//   - Different execution path → different TraceCounters → different gas → BAD BLOCK
//
// PerEVMReentrancyGuard is embedded in the EVM struct (per-instance).
// No mutex needed: the EVM struct is documented as "not thread safe,
// should only ever be used *once*" — all calls within a single EVM
// instance are sequential (nested via the call stack, never concurrent).
//
// The GlobalReentrancyGuard is kept for backward compatibility (RPC
// inspection) but is NO LONGER used for consensus-critical execution.

// PerEVMReentrancyGuard tracks which contracts are currently executing
// within a single EVM instance. No mutex — EVM is single-threaded.
type PerEVMReentrancyGuard struct {
	callStack map[common.Address]int
}

// Init initializes the guard. Must be called once after EVM creation.
func (rg *PerEVMReentrancyGuard) Init() {
	rg.callStack = make(map[common.Address]int)
}

// Reset clears all tracking. Called at the start of each transaction.
func (rg *PerEVMReentrancyGuard) Reset() {
	// Re-use existing map by clearing entries (avoids allocation)
	for k := range rg.callStack {
		delete(rg.callStack, k)
	}
}

// Enter marks a contract as currently executing.
// Returns false if this contract is already in the call stack (self-reentrancy).
func (rg *PerEVMReentrancyGuard) Enter(addr common.Address) bool {
	if rg.callStack[addr] > 0 {
		return false // SELF-REENTRANCY BLOCKED
	}
	rg.callStack[addr]++
	return true
}

// Exit marks a contract as no longer executing.
func (rg *PerEVMReentrancyGuard) Exit(addr common.Address) {
	rg.callStack[addr]--
	if rg.callStack[addr] <= 0 {
		delete(rg.callStack, addr)
	}
}