package vm

import (
	"sync"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/common"
)

// AdaptiveGasConfig controls the adaptive gas pricing system.
// When enabled, contracts that exhibit predictable, repetitive execution
// patterns receive a gas discount, incentivizing efficient code.
type AdaptiveGasConfig struct {
	Enabled         atomic.Bool
	DiscountPercent uint64 // e.g. 10 = 10% discount
}

var GlobalAdaptiveGas = &AdaptiveGasConfig{}

func init() {
	GlobalAdaptiveGas.Enabled.Store(false) // disabled by default, enable via RPC
	GlobalAdaptiveGas.DiscountPercent = 10 // 10% discount for optimized patterns
}

// ContractPattern tracks execution patterns for a contract address.
type ContractPattern struct {
	mu           sync.Mutex
	callCount    uint64
	lastOpcodes  [8]OpCode // last 8 opcodes executed
	patternScore uint64    // higher = more predictable
	totalOps     uint64
	pureOps      uint64 // opcodes that don't touch storage/external state
}

// PatternTracker tracks patterns across all contracts.
type PatternTracker struct {
	mu        sync.RWMutex
	contracts map[common.Address]*ContractPattern
}

var GlobalPatternTracker = &PatternTracker{
	contracts: make(map[common.Address]*ContractPattern),
}

// RecordOp records an opcode execution for pattern analysis.
func (pt *PatternTracker) RecordOp(addr common.Address, op OpCode) {
	if !GlobalAdaptiveGas.Enabled.Load() {
		return
	}

	pt.mu.RLock()
	cp, ok := pt.contracts[addr]
	pt.mu.RUnlock()

	if !ok {
		pt.mu.Lock()
		cp, ok = pt.contracts[addr]
		if !ok {
			cp = &ContractPattern{}
			pt.contracts[addr] = cp
		}
		pt.mu.Unlock()
	}

	cp.mu.Lock()
	defer cp.mu.Unlock()

	cp.totalOps++

	// Track if this is a "pure" opcode (no storage/external state)
	if isPureOpcode(op) {
		cp.pureOps++
	}

	// Shift opcode window
	copy(cp.lastOpcodes[1:], cp.lastOpcodes[:7])
	cp.lastOpcodes[0] = op

	// Update pattern score based on repetition
	if cp.totalOps > 100 {
		cp.patternScore = (cp.pureOps * 100) / cp.totalOps
	}
}

// RecordCall increments the call counter for pattern analysis.
func (pt *PatternTracker) RecordCall(addr common.Address) {
	if !GlobalAdaptiveGas.Enabled.Load() {
		return
	}

	pt.mu.RLock()
	cp, ok := pt.contracts[addr]
	pt.mu.RUnlock()

	if !ok {
		return
	}

	cp.mu.Lock()
	cp.callCount++
	cp.mu.Unlock()
}

// GetDiscount returns the gas discount percentage for a contract (0-DiscountPercent).
// A contract qualifies for a discount if:
// 1. It has been called at least 10 times
// 2. More than 70% of its opcodes are "pure" (no storage writes, no external calls)
func (pt *PatternTracker) GetDiscount(addr common.Address) uint64 {
	if !GlobalAdaptiveGas.Enabled.Load() {
		return 0
	}

	pt.mu.RLock()
	cp, ok := pt.contracts[addr]
	pt.mu.RUnlock()

	if !ok {
		return 0
	}

	cp.mu.Lock()
	defer cp.mu.Unlock()

	// Need at least 10 calls and 100 ops to qualify
	if cp.callCount < 10 || cp.totalOps < 100 {
		return 0
	}

	// Pattern score is percentage of pure opcodes
	if cp.patternScore >= 70 {
		return GlobalAdaptiveGas.DiscountPercent
	}

	return 0
}

// PatternStats holds pattern analysis for RPC reporting.
type PatternStats struct {
	Address      string `json:"address"`
	CallCount    uint64 `json:"callCount"`
	TotalOps     uint64 `json:"totalOps"`
	PureOps      uint64 `json:"pureOps"`
	PurePercent  uint64 `json:"purePercent"`
	PatternScore uint64 `json:"patternScore"`
	Discount     uint64 `json:"discountPercent"`
}

// GetAllPatterns returns pattern data for all tracked contracts.
func (pt *PatternTracker) GetAllPatterns() []PatternStats {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	var stats []PatternStats
	for addr, cp := range pt.contracts {
		cp.mu.Lock()
		var purePercent uint64
		if cp.totalOps > 0 {
			purePercent = (cp.pureOps * 100) / cp.totalOps
		}
		stats = append(stats, PatternStats{
			Address:      addr.Hex(),
			CallCount:    cp.callCount,
			TotalOps:     cp.totalOps,
			PureOps:      cp.pureOps,
			PurePercent:  purePercent,
			PatternScore: cp.patternScore,
			Discount:     pt.GetDiscount(addr),
		})
		cp.mu.Unlock()
	}
	return stats
}

// Reset clears all pattern data.
func (pt *PatternTracker) Reset() {
	pt.mu.Lock()
	pt.contracts = make(map[common.Address]*ContractPattern)
	pt.mu.Unlock()
}

// isPureOpcode returns true if the opcode does not modify external state.
// Pure opcodes: arithmetic, stack manipulation, memory, control flow.
// Non-pure: SSTORE, CALL, CREATE, LOG, SELFDESTRUCT, etc.
func isPureOpcode(op OpCode) bool {
	switch op {
	// Arithmetic
	case ADD, MUL, SUB, DIV, SDIV, MOD, SMOD, ADDMOD, MULMOD, EXP, SIGNEXTEND:
		return true
	// Comparison
	case LT, GT, SLT, SGT, EQ, ISZERO:
		return true
	// Bitwise
	case AND, OR, XOR, NOT, BYTE, SHL, SHR, SAR:
		return true
	// Stack
	case POP, PUSH0, PUSH1, PUSH2, PUSH3, PUSH4:
		return true
	case DUP1, DUP2, DUP3, DUP4, DUP5, DUP6, DUP7, DUP8:
		return true
	case DUP9, DUP10, DUP11, DUP12, DUP13, DUP14, DUP15, DUP16:
		return true
	case SWAP1, SWAP2, SWAP3, SWAP4, SWAP5, SWAP6, SWAP7, SWAP8:
		return true
	case SWAP9, SWAP10, SWAP11, SWAP12, SWAP13, SWAP14, SWAP15, SWAP16:
		return true
	// Memory (local, not persisted)
	case MLOAD, MSTORE, MSTORE8, MSIZE:
		return true
	// Control flow
	case JUMP, JUMPI, JUMPDEST, PC, GAS:
		return true
	// Data
	case CALLDATALOAD, CALLDATASIZE, CALLDATACOPY, CODESIZE, CODECOPY, RETURNDATASIZE, RETURNDATACOPY:
		return true
	// Hash
	case KECCAK256:
		return true
	// Return
	case RETURN, REVERT, STOP:
		return true
	// Read-only state (pure reads, no writes)
	case SLOAD, BALANCE, CALLVALUE, CALLER, ADDRESS, ORIGIN, GASPRICE:
		return true
	case BLOCKHASH, COINBASE, TIMESTAMP, NUMBER, DIFFICULTY, GASLIMIT, CHAINID, SELFBALANCE, BASEFEE:
		return true
	default:
		return false
	}
}
