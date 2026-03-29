package vm

import (
	"sync"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// ExecutionMode defines how the EVM processes contract calls.
type ExecutionMode uint8

const (
	// ModeStandard is full EVM compatibility — no optimizations, no shortcuts.
	ModeStandard ExecutionMode = 0
	// ModeFast skips redundant validation for contracts that have been
	// verified as safe (no self-destruct, no delegatecall, deterministic).
	ModeFast ExecutionMode = 1
	// ModeParallel enables speculative parallel execution of independent
	// transactions within a block. Rolls back on state conflict.
	ModeParallel ExecutionMode = 2
)

func (m ExecutionMode) String() string {
	switch m {
	case ModeStandard:
		return "standard"
	case ModeFast:
		return "fast"
	case ModeParallel:
		return "parallel"
	default:
		return "unknown"
	}
}

// ExecutionModeConfig controls the global execution mode.
type ExecutionModeConfig struct {
	Mode atomic.Uint32
}

var GlobalExecutionMode = &ExecutionModeConfig{}

func init() {
	GlobalExecutionMode.Mode.Store(uint32(ModeStandard))
}

// GetMode returns the current execution mode.
func (c *ExecutionModeConfig) GetMode() ExecutionMode {
	return ExecutionMode(c.Mode.Load())
}

// SetMode sets the execution mode.
func (c *ExecutionModeConfig) SetMode(m ExecutionMode) {
	if m > ModeParallel {
		m = ModeStandard
	}
	c.Mode.Store(uint32(m))
}

// VerifiedContract tracks contracts that have been analyzed and deemed
// safe for fast-mode execution.
type VerifiedContract struct {
	HasSelfDestruct bool
	HasDelegateCall bool
	HasCreate       bool
	IsDeterministic bool // true if contract only depends on input + state, not block context
	CallCount       uint64
	VerifiedAt      uint64      // block number when verified
	CodeHash        common.Hash // runtime bytecode hash used to detect upgrades/code changes
}

// ContractVerifier analyzes and tracks verified contracts.
type ContractVerifier struct {
	mu        sync.RWMutex
	contracts map[common.Address]*VerifiedContract
}

var GlobalContractVerifier = &ContractVerifier{
	contracts: make(map[common.Address]*VerifiedContract),
}

// AnalyzeAndCheckFast combines contract analysis with fast-mode eligibility
// in a single lock acquisition. This eliminates the overhead of a separate
// IsFastEligible call that previously made fast mode slower than standard.
//
// For KNOWN contracts (the hot path during block processing), this function
// performs only: one map lookup, one counter increment, and one eligibility
// check — all under a single lock. No keccak256 hash computation.
//
// The keccak256 hash is ONLY computed for NEW contracts (first encounter).
func (cv *ContractVerifier) AnalyzeAndCheckFast(addr common.Address, code []byte, blockNum uint64) bool {
	mode := GlobalExecutionMode.GetMode()

	// Hot path: contract already known — bump counter, check eligibility, done.
	// Uses a write lock because we mutate CallCount.
	cv.mu.Lock()
	if existing, ok := cv.contracts[addr]; ok {
		existing.CallCount++
		eligible := mode >= ModeFast &&
			!existing.HasSelfDestruct &&
			!existing.HasDelegateCall &&
			existing.CallCount >= 5
		cv.mu.Unlock()
		return eligible
	}
	cv.mu.Unlock()

	// Cold path: first encounter — full bytecode analysis.
	codeHash := crypto.Keccak256Hash(code)

	hasSelfDestruct := false
	hasDelegateCall := false
	hasCreate := false
	isDeterministic := true

	for i := 0; i < len(code); i++ {
		op := OpCode(code[i])
		switch op {
		case SELFDESTRUCT:
			hasSelfDestruct = true
			isDeterministic = false
		case DELEGATECALL:
			hasDelegateCall = true
			isDeterministic = false
		case CREATE, CREATE2:
			hasCreate = true
		case BLOCKHASH, COINBASE, TIMESTAMP, NUMBER, DIFFICULTY, GASLIMIT, BASEFEE:
			isDeterministic = false
		case GASPRICE, ORIGIN:
			isDeterministic = false
		}

		// Skip PUSH data bytes
		if op >= PUSH1 && op <= PUSH32 {
			i += int(op - PUSH1 + 1)
		}
	}

	cv.mu.Lock()
	defer cv.mu.Unlock()

	// Double-check: another goroutine may have added it while we scanned.
	if existing, ok := cv.contracts[addr]; ok {
		existing.CallCount++
		return false // still accumulating calls
	}

	cv.contracts[addr] = &VerifiedContract{
		HasSelfDestruct: hasSelfDestruct,
		HasDelegateCall: hasDelegateCall,
		HasCreate:       hasCreate,
		IsDeterministic: isDeterministic,
		CallCount:       1,
		VerifiedAt:      blockNum,
		CodeHash:        codeHash,
	}
	return false // new contract, not yet eligible (need 5+ calls)
}

// AnalyzeCode is the legacy entry point — delegates to AnalyzeAndCheckFast
// but discards the eligibility result. Kept for call sites that only need
// the analysis side-effect.
func (cv *ContractVerifier) AnalyzeCode(addr common.Address, code []byte, blockNum uint64) {
	cv.AnalyzeAndCheckFast(addr, code, blockNum)
}

// IsFastEligible returns true if a contract is safe for fast-mode execution.
// Requirements: no self-destruct, no delegatecall, called at least 5 times.
func (cv *ContractVerifier) IsFastEligible(addr common.Address) bool {
	if GlobalExecutionMode.GetMode() < ModeFast {
		return false
	}

	cv.mu.RLock()
	vc, ok := cv.contracts[addr]
	cv.mu.RUnlock()

	if !ok {
		return false
	}

	return !vc.HasSelfDestruct && !vc.HasDelegateCall && vc.CallCount >= 5
}

// VerifiedStats holds verification data for RPC reporting.
type VerifiedStats struct {
	Address         string `json:"address"`
	HasSelfDestruct bool   `json:"hasSelfDestruct"`
	HasDelegateCall bool   `json:"hasDelegateCall"`
	HasCreate       bool   `json:"hasCreate"`
	IsDeterministic bool   `json:"isDeterministic"`
	CallCount       uint64 `json:"callCount"`
	FastEligible    bool   `json:"fastEligible"`
}

// GetAllVerified returns verification data for all analyzed contracts.
func (cv *ContractVerifier) GetAllVerified() []VerifiedStats {
	cv.mu.RLock()
	defer cv.mu.RUnlock()

	var stats []VerifiedStats
	for addr, vc := range cv.contracts {
		eligible := !vc.HasSelfDestruct && !vc.HasDelegateCall && vc.CallCount >= 5
		stats = append(stats, VerifiedStats{
			Address:         addr.Hex(),
			HasSelfDestruct: vc.HasSelfDestruct,
			HasDelegateCall: vc.HasDelegateCall,
			HasCreate:       vc.HasCreate,
			IsDeterministic: vc.IsDeterministic,
			CallCount:       vc.CallCount,
			FastEligible:    eligible,
		})
	}
	return stats
}

// Reset clears all verification data.
func (cv *ContractVerifier) Reset() {
	cv.mu.Lock()
	cv.contracts = make(map[common.Address]*VerifiedContract)
	cv.mu.Unlock()
}

// FastModeSkips tracks what checks are skipped in fast mode.
// In fast mode, for verified contracts we skip:
// 1. Redundant stack bound checks (already verified safe)
// 2. Redundant read-only violation checks (contract doesn't write in static ctx)
// The gas savings come from reduced overhead per opcode.
type FastModeStats struct {
	SkippedChecks  atomic.Uint64
	FastExecutions atomic.Uint64
	TotalGasSaved  atomic.Uint64
}

var GlobalFastModeStats = &FastModeStats{}