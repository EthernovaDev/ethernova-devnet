package vm

// ============================================================================
// ADAPTIVE GAS v2 — TRACE-BASED POST-EXECUTION ADJUSTMENT
// ============================================================================
//
// Design principles:
//   1. NEVER modify gas during EVM execution (no per-opcode changes)
//   2. Collect lightweight counters during execution (just integer increments)
//   3. After execution completes, compute adjustment from counters
//   4. Apply adjustment to gasRemaining in state_transition.go
//   5. 100% deterministic: pure integer math, no floats, no maps, no caching
//
// Why this is consensus-safe:
//   - EVM execution is IDENTICAL across all nodes (zero gas modification)
//   - TraceCounters are uint64 fields on the EVM struct (per-tx, stack-allocated)
//   - Post-execution math is pure function of counters → same input = same output
//   - No dependency on global state, caches, map iteration order, or CGO
//
// Classification model:
//   OLD: static bytecode scan → pureOps/totalOps ratio (WRONG for real contracts)
//   NEW: runtime execution trace → actual opcode counts that were EXECUTED
//
//   A NovaToken.transfer() compiles to bytecode with ~200 opcodes but only
//   EXECUTES ~50 per call. The bytecode contains function selector dispatch,
//   dead code paths, etc. Only executed opcodes matter for classification.
//
// Complexity score formula:
//   score = (SSTORE × 5) + (CALL_OPS × 3) + (SLOAD × 2) + (JUMPI × 1) + (CREATE × 10)
//
//   This weights state mutation (SSTORE) heavily, external calls (CALL/DELEGATECALL)
//   moderately, state reads (SLOAD) lightly, and branching (JUMPI) minimally.
//   CREATE/CREATE2 get the highest weight since deploying contracts is the most
//   complex operation.
//
// Gas adjustment mapping:
//   PURE (score=0, no SSTORE, no external CALL):
//     → discount up to -25%, scaled by execution size
//   LIGHT (score > 0, no SSTORE):
//     → discount up to -15%, inversely proportional to score
//   MIXED (score moderate, has SSTORE):
//     → no adjustment (0%)
//   COMPLEX (score high, heavy SSTORE/CALL):
//     → penalty up to +10%, proportional to score
//
// Scaling:
//   Uses integer linear ramp (not sigmoid — sigmoid requires float).
//   discount = (maxDiscount × (maxScore - score)) / maxScore
//   penalty  = (maxPenalty  × (score - penaltyThreshold)) / (maxScore - penaltyThreshold)
//   Both clamped to their respective bounds.

import (
	"fmt"

	"github.com/ethereum/go-ethereum/log"
)

// ============================================================================
// TRACE COUNTERS — embedded in EVM struct, per-transaction scope
// ============================================================================

// TraceCounters collects opcode execution counts during EVM execution.
// These are simple uint64 counters — no allocations, no maps, no locks.
// Each counter is incremented inline in the interpreter loop.
//
// CRITICAL: This struct must ONLY contain uint64 fields. No pointers,
// no slices, no maps. This ensures zero-allocation and deterministic behavior.
type TraceCounters struct {
	SstoreCount      uint64 // SSTORE executions (state write)
	SloadCount       uint64 // SLOAD executions (state read)
	CallCount        uint64 // CALL executions
	StaticCallCount  uint64 // STATICCALL executions
	DelegateCallCount uint64 // DELEGATECALL executions
	CallCodeCount    uint64 // CALLCODE executions
	CreateCount      uint64 // CREATE executions
	Create2Count     uint64 // CREATE2 executions
	JumpiCount       uint64 // JUMPI executions (conditional branch)
	LogCount         uint64 // LOG0-LOG4 executions
	ExtCodeCount     uint64 // EXTCODESIZE/EXTCODECOPY/EXTCODEHASH
	SelfDestructCount uint64 // SELFDESTRUCT executions
	TotalOpsExecuted uint64 // total opcodes executed
	MemoryExpansions uint64 // memory expansion events (MSTORE to new high-water)
}

// Reset zeros all counters. Called at the start of each transaction.
func (tc *TraceCounters) Reset() {
	*tc = TraceCounters{}
}

// RecordOpcode increments the appropriate counter for the given opcode.
// This is called once per opcode in the interpreter loop.
// MUST be branchless-friendly: simple switch, no allocations.
func (tc *TraceCounters) RecordOpcode(op OpCode) {
	tc.TotalOpsExecuted++

	switch op {
	case SSTORE:
		tc.SstoreCount++
	case SLOAD:
		tc.SloadCount++
	case CALL:
		tc.CallCount++
	case STATICCALL:
		tc.StaticCallCount++
	case DELEGATECALL:
		tc.DelegateCallCount++
	case CALLCODE:
		tc.CallCodeCount++
	case CREATE:
		tc.CreateCount++
	case CREATE2:
		tc.Create2Count++
	case JUMPI:
		tc.JumpiCount++
	case LOG0, LOG1, LOG2, LOG3, LOG4:
		tc.LogCount++
	case EXTCODESIZE, EXTCODECOPY, EXTCODEHASH:
		tc.ExtCodeCount++
	case SELFDESTRUCT:
		tc.SelfDestructCount++
	}
}

// ============================================================================
// EXECUTION CLASSIFICATION
// ============================================================================

// ExecutionCategory defines the runtime classification tier.
// This is based on ACTUAL executed opcodes, not static bytecode.
type ExecutionCategory uint8

const (
	ExecCategoryPure    ExecutionCategory = 0 // no state writes, no external calls
	ExecCategoryLight   ExecutionCategory = 1 // state reads only, no writes
	ExecCategoryMixed   ExecutionCategory = 2 // moderate state access
	ExecCategoryComplex ExecutionCategory = 3 // heavy state mutation / external calls
)

func (c ExecutionCategory) String() string {
	switch c {
	case ExecCategoryPure:
		return "pure"
	case ExecCategoryLight:
		return "light"
	case ExecCategoryMixed:
		return "mixed"
	case ExecCategoryComplex:
		return "complex"
	default:
		return "unknown"
	}
}

// ExecutionClassification holds the complete classification result
// from a single transaction's execution trace.
type ExecutionClassification struct {
	Category        ExecutionCategory
	ComplexityScore uint64
	GasAdjustPct    int64 // negative = discount, positive = penalty (in basis points / 100)
	Counters        TraceCounters
}

// ============================================================================
// CORE FUNCTIONS — pure, deterministic, no side effects
// ============================================================================

// ClassifyExecution determines the execution category from trace counters.
// This is a HARD classification with clear rules:
//
// Rule 1: If SSTORE > 0 → CANNOT be Pure or Light (hard gate)
// Rule 2: If external CALL (non-STATICCALL) > 0 → CANNOT be Pure
// Rule 3: If CREATE/CREATE2 > 0 → always Complex
//
// These rules are intentionally strict. A contract that writes even ONE
// storage slot is fundamentally not "pure computation".
func ClassifyExecution(tc *TraceCounters) ExecutionCategory {
	// Gate: any contract deployment → Complex
	if tc.CreateCount > 0 || tc.Create2Count > 0 {
		return ExecCategoryComplex
	}

	// Gate: any SELFDESTRUCT → Complex
	if tc.SelfDestructCount > 0 {
		return ExecCategoryComplex
	}

	hasStateWrite := tc.SstoreCount > 0
	hasExternalCall := tc.CallCount > 0 || tc.DelegateCallCount > 0 || tc.CallCodeCount > 0

	// Hard gate: ANY state write → not pure, not light
	if hasStateWrite {
		// Determine if Mixed or Complex based on score
		score := ComputeComplexityScore(tc)
		if score >= complexPenaltyThreshold {
			return ExecCategoryComplex
		}
		return ExecCategoryMixed
	}

	// No state writes below this point

	// External calls (non-STATICCALL) without state writes → Light
	// STATICCALL is read-only and doesn't disqualify from Light
	if hasExternalCall {
		return ExecCategoryLight
	}

	// No state writes, no external calls
	// May have SLOAD (state reads) and STATICCALL (read-only external)
	if tc.SloadCount > 0 || tc.StaticCallCount > 0 {
		return ExecCategoryLight
	}

	// Truly pure: no state access at all
	return ExecCategoryPure
}

// ComputeComplexityScore calculates a weighted complexity score from trace counters.
// Higher score = more complex execution.
//
// Weights:
//   SSTORE:          5 — persistent state mutation, most expensive
//   CALL/DELEGATE:   3 — external interaction, potential reentrancy
//   SLOAD:           2 — state read, I/O bound
//   JUMPI:           1 — conditional branching, control flow complexity
//   CREATE/CREATE2: 10 — contract deployment
//   SELFDESTRUCT:   10 — destructive operation
//
// The score is deterministic: same counters → same score, always.
func ComputeComplexityScore(tc *TraceCounters) uint64 {
	score := uint64(0)

	// State mutation (highest weight)
	score += tc.SstoreCount * 5

	// External calls (CALL, DELEGATECALL, CALLCODE — NOT STATICCALL)
	externalCalls := tc.CallCount + tc.DelegateCallCount + tc.CallCodeCount
	score += externalCalls * 3

	// State reads
	score += tc.SloadCount * 2

	// Branching complexity
	score += tc.JumpiCount * 1

	// Contract deployment (very heavy)
	score += (tc.CreateCount + tc.Create2Count) * 10

	// Destructive operations
	score += tc.SelfDestructCount * 10

	return score
}

// ============================================================================
// GAS ADJUSTMENT COMPUTATION
// ============================================================================

// Constants for gas adjustment computation.
// These are compile-time constants — identical across all builds.
const (
	// Maximum discount for pure contracts: 25% (2500 basis points)
	maxDiscountBps = 2500

	// Maximum discount for light contracts: 15% (1500 basis points)
	maxLightDiscountBps = 1500

	// Maximum penalty for complex contracts: 10% (1000 basis points)
	maxPenaltyBps = 1000

	// Complexity score threshold where penalty begins.
	// Below this: Mixed (no adjustment)
	// Above this: Complex (penalty applied)
	complexPenaltyThreshold = uint64(15)

	// Score at which maximum penalty is applied.
	// Linear ramp from penaltyThreshold to this value.
	maxPenaltyScore = uint64(100)

	// Minimum executed opcodes before discount is applied.
	// Prevents tiny view functions from getting huge discounts.
	minOpsForDiscount = uint64(10)

	// Opcodes at which discount reaches full scale.
	// Linear ramp from minOpsForDiscount to this value.
	fullDiscountOps = uint64(200)
)

// ComputeGasAdjustment calculates the gas adjustment in basis points (1/100 of percent).
// Returns value in PERCENT (not basis points) for backward compatibility.
//
// Negative = discount (caller pays less)
// Positive = penalty (caller pays more)
// Zero = no adjustment
//
// This function is a PURE FUNCTION of (category, score, totalOps).
// No global state, no randomness, no floating point.
func ComputeGasAdjustment(category ExecutionCategory, score uint64, totalOps uint64) int64 {
	switch category {
	case ExecCategoryPure:
		return computePureDiscount(totalOps)

	case ExecCategoryLight:
		return computeLightDiscount(score, totalOps)

	case ExecCategoryMixed:
		// No adjustment for mixed contracts
		return 0

	case ExecCategoryComplex:
		return computeComplexPenalty(score)

	default:
		return 0
	}
}

// computePureDiscount calculates discount for pure contracts.
// Pure = no SSTORE, no external CALL, no SLOAD, no STATICCALL.
//
// Scaling: Linear ramp based on execution size.
//   < minOpsForDiscount opcodes: no discount (too small to matter)
//   minOpsForDiscount → fullDiscountOps: linear ramp 0% → 25%
//   >= fullDiscountOps: full 25% discount
//
// This prevents tiny view functions (5 opcodes) from getting a 25% discount
// while substantial pure computations (hash loops, etc.) get the full benefit.
func computePureDiscount(totalOps uint64) int64 {
	if totalOps < minOpsForDiscount {
		return 0
	}

	// Linear ramp: discount scales with execution size
	if totalOps >= fullDiscountOps {
		return -25 // max discount
	}

	// Linear interpolation: (totalOps - min) / (fullDiscount - min) × maxDiscount
	// Using integer math: multiply first, then divide (avoids truncation to 0)
	opsAboveMin := totalOps - minOpsForDiscount
	rangeSize := fullDiscountOps - minOpsForDiscount

	// discount = 25 × opsAboveMin / rangeSize
	discount := int64(25 * opsAboveMin / rangeSize)
	if discount < 1 {
		discount = 1 // minimum 1% if above threshold
	}

	return -discount
}

// computeLightDiscount calculates discount for light contracts.
// Light = may have SLOAD/STATICCALL, but NO SSTORE, no external CALL.
//
// Discount is inversely proportional to complexity score:
//   score 0: full light discount (15%)
//   score → penaltyThreshold: discount decreases linearly to 0%
//
// Also scaled by execution size (same ramp as pure).
func computeLightDiscount(score uint64, totalOps uint64) int64 {
	if totalOps < minOpsForDiscount {
		return 0
	}

	// Base discount from score (inverse relationship)
	if score >= complexPenaltyThreshold {
		return 0 // score too high, no discount
	}

	// baseDiscount = 15 × (threshold - score) / threshold
	baseDiscount := int64(15 * (complexPenaltyThreshold - score) / complexPenaltyThreshold)
	if baseDiscount < 1 {
		return 0
	}

	// Scale by execution size
	if totalOps >= fullDiscountOps {
		return -baseDiscount
	}

	opsAboveMin := totalOps - minOpsForDiscount
	rangeSize := fullDiscountOps - minOpsForDiscount

	// scaled = baseDiscount × opsAboveMin / rangeSize
	scaled := baseDiscount * int64(opsAboveMin) / int64(rangeSize)
	if scaled < 1 {
		scaled = 1
	}

	return -scaled
}

// computeComplexPenalty calculates penalty for complex contracts.
// Complex = high complexity score (heavy SSTORE, CALL, etc.)
//
// Linear ramp:
//   score at penaltyThreshold: 0% penalty
//   score at maxPenaltyScore: 10% penalty (max)
//   score > maxPenaltyScore: capped at 10%
func computeComplexPenalty(score uint64) int64 {
	if score <= complexPenaltyThreshold {
		return 0
	}

	if score >= maxPenaltyScore {
		return 10 // max penalty
	}

	// Linear ramp: (score - threshold) / (maxScore - threshold) × maxPenalty
	scoreAboveThreshold := score - complexPenaltyThreshold
	rangeSize := maxPenaltyScore - complexPenaltyThreshold

	penalty := int64(10 * scoreAboveThreshold / rangeSize)
	if penalty < 1 {
		penalty = 1
	}

	return penalty
}

// ============================================================================
// GAS APPLICATION — called from state_transition.go
// ============================================================================

// ApplyAdaptiveGasV2 computes and applies the gas adjustment after execution.
// This is the ONLY entry point for modifying gas based on adaptive pricing.
//
// CONSENSUS RULE: The caller MUST gate this behind the AdaptiveGasV2ForkBlock
// check. This function does NOT check activation — it assumes the caller has
// already verified that the fork is active.
//
// Parameters:
//   tc            — trace counters collected during execution
//   gasUsed       — total gas consumed by execution
//   gasRemaining  — gas remaining after execution
//   intrinsicGas  — base intrinsic gas (not subject to adjustment)
//
// Returns:
//   newGasRemaining — adjusted gas remaining
//   adjustPct       — the adjustment percentage applied (for logging)
//   classification  — full classification result (for RPC reporting)
//
// The adjustment is applied to EXECUTION GAS only (gasUsed - intrinsicGas).
// Intrinsic gas (21000 for transfer, etc.) is never adjusted.
//
// Safety bounds:
//   - gasRemaining can INCREASE (discount) or DECREASE (penalty)
//   - gasRemaining is NEVER reduced below 0
//   - Discount is applied to execution gas only, not intrinsic
func ApplyAdaptiveGasV2(tc *TraceCounters, gasUsed, gasRemaining, intrinsicGas uint64) (uint64, int64, *ExecutionClassification) {
	// Don't adjust if no opcodes were executed (simple transfers, etc.)
	if tc.TotalOpsExecuted == 0 {
		return gasRemaining, 0, nil
	}

	// Step 1: Classify the execution
	category := ClassifyExecution(tc)

	// Step 2: Compute complexity score
	score := ComputeComplexityScore(tc)

	// Step 3: Compute gas adjustment percentage
	adjustPct := ComputeGasAdjustment(category, score, tc.TotalOpsExecuted)

	// Step 4: Apply adjustment to execution gas only
	// executionGas = gasUsed - intrinsicGas
	var executionGas uint64
	if gasUsed > intrinsicGas {
		executionGas = gasUsed - intrinsicGas
	}

	classification := &ExecutionClassification{
		Category:        category,
		ComplexityScore: score,
		GasAdjustPct:    adjustPct,
		Counters:        *tc,
	}

	if adjustPct == 0 || executionGas == 0 {
		return gasRemaining, 0, classification
	}

	if adjustPct < 0 {
		// DISCOUNT: give gas back to the caller
		// discount = executionGas × |adjustPct| / 100
		discount := executionGas * uint64(-adjustPct) / 100
		gasRemaining += discount

		log.Debug("[AdaptiveGasV2] discount applied",
			"category", category.String(),
			"score", score,
			"adjustPct", fmt.Sprintf("%+d%%", adjustPct),
			"executionGas", executionGas,
			"discount", discount,
			"newGasRemaining", gasRemaining,
			"counters", fmt.Sprintf("SSTORE=%d SLOAD=%d CALL=%d JUMPI=%d ops=%d",
				tc.SstoreCount, tc.SloadCount,
				tc.CallCount+tc.DelegateCallCount+tc.CallCodeCount,
				tc.JumpiCount, tc.TotalOpsExecuted),
		)
	} else {
		// PENALTY: consume additional gas from the caller
		// penalty = executionGas × adjustPct / 100
		penalty := executionGas * uint64(adjustPct) / 100

		// Safety: never consume more gas than remaining
		if penalty > gasRemaining {
			penalty = gasRemaining
		}
		gasRemaining -= penalty

		log.Debug("[AdaptiveGasV2] penalty applied",
			"category", category.String(),
			"score", score,
			"adjustPct", fmt.Sprintf("%+d%%", adjustPct),
			"executionGas", executionGas,
			"penalty", penalty,
			"newGasRemaining", gasRemaining,
			"counters", fmt.Sprintf("SSTORE=%d SLOAD=%d CALL=%d JUMPI=%d ops=%d",
				tc.SstoreCount, tc.SloadCount,
				tc.CallCount+tc.DelegateCallCount+tc.CallCodeCount,
				tc.JumpiCount, tc.TotalOpsExecuted),
		)
	}

	return gasRemaining, adjustPct, classification
}

// ============================================================================
// RPC REPORTING
// ============================================================================

// AdaptiveGasV2Stats holds trace-based classification data for RPC.
type AdaptiveGasV2Stats struct {
	Category        string `json:"category"`
	ComplexityScore uint64 `json:"complexityScore"`
	GasAdjustPct    int64  `json:"gasAdjustPercent"`
	SstoreCount     uint64 `json:"sstoreCount"`
	SloadCount      uint64 `json:"sloadCount"`
	CallCount       uint64 `json:"callCount"`
	StaticCallCount uint64 `json:"staticCallCount"`
	DelegateCount   uint64 `json:"delegateCallCount"`
	CreateCount     uint64 `json:"createCount"`
	JumpiCount      uint64 `json:"jumpiCount"`
	LogCount        uint64 `json:"logCount"`
	TotalOps        uint64 `json:"totalOpsExecuted"`
}

// ToStats converts an ExecutionClassification to RPC-friendly stats.
func (ec *ExecutionClassification) ToStats() AdaptiveGasV2Stats {
	return AdaptiveGasV2Stats{
		Category:        ec.Category.String(),
		ComplexityScore: ec.ComplexityScore,
		GasAdjustPct:    ec.GasAdjustPct,
		SstoreCount:     ec.Counters.SstoreCount,
		SloadCount:      ec.Counters.SloadCount,
		CallCount:       ec.Counters.CallCount,
		StaticCallCount: ec.Counters.StaticCallCount,
		DelegateCount:   ec.Counters.DelegateCallCount,
		CreateCount:     ec.Counters.CreateCount + ec.Counters.Create2Count,
		JumpiCount:      ec.Counters.JumpiCount,
		LogCount:        ec.Counters.LogCount,
		TotalOps:        ec.Counters.TotalOpsExecuted,
	}
}

// ============================================================================
// LAST TX CLASSIFICATION — for RPC introspection
// ============================================================================

// LastTxClassification stores the most recent transaction's classification.
// This is used for RPC reporting only, NOT for consensus.
// Protected by the single-threaded nature of state transitions.
var LastTxClassification *ExecutionClassification