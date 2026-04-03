package vm

// ============================================================================
// SAFE TUNER — ADAPTIVE GAS SAFETY ENVELOPE
// ============================================================================
//
// ROOT CAUSE OF TX FAILURES:
//
//   state_transition.go line 265: st.gp.SubGas(st.msg.GasLimit)
//     → each tx reserves its FULL gasLimit from the block GasPool
//
//   state_transition.go line 646: st.gp.AddGas(st.gasRemaining)
//     → after execution, REMAINING gas is returned to the pool
//
//   Net pool consumption per tx = gasLimit - gasRemaining
//
//   When adaptive gas applies a PENALTY, it REDUCES gasRemaining.
//   Less gas is returned to the pool. The pool depletes faster.
//
//   Example with 30M block gas limit, 60 complex txs:
//     Without penalty: each tx returns 100k → pool consumption = 400k/tx
//     With 10% penalty: each tx returns 60k → pool consumption = 440k/tx
//     Extra pool drain: 40k × 60 = 2.4M gas
//     Late txs call SubGas → ErrGasLimitReached → tx not included
//
//   This is why 11 out of 88 txs failed: the first ~77 txs consumed
//   enough extra gas through penalties to exhaust the pool.
//
// SOLUTION:
//
//   A scaleFactor (0-10000, where 10000 = 100%) is applied to all
//   adaptive gas adjustments AFTER ApplyAdaptiveGasV2 computes them
//   but BEFORE the result is written to st.gasRemaining.
//
//   The scaleFactor is updated at block boundaries based on three
//   deterministic safety signals:
//
//   1. TX FAILURE COUNT: If any tx in a block failed, immediately
//      reduce scaleFactor (the penalty is too aggressive).
//
//   2. GAS POOL STRESS: If cumulative penalty gas exceeds 8% of the
//      block gas limit, reduce scaleFactor (approaching danger zone).
//
//   3. HEADROOM FLOOR: If any tx's post-adjustment gasRemaining is
//      dangerously low relative to block gas limit, reduce scaleFactor.
//
//   Recovery is gradual: +1% per block in normal mode, +0.5% in
//   cautious mode (after emergency). This ensures the system doesn't
//   immediately return to an unsafe state.
//
// CONSENSUS SAFETY:
//
//   scaleFactor is deterministic:
//     block N execution traces → BlockSafetySample → safety rules → scaleFactor
//
//   All nodes processing the same chain produce identical scaleFactor
//   values because the inputs (execution traces) and rules (integer
//   math with compile-time constants) are identical.
//
//   The scale factor is applied via ApplyScaleFactor(), which is
//   pure integer interpolation between original and adjusted gas.
//   No floats, no maps, no randomness.
//
// INTEGRATION:
//
//   state_transition.go:
//     gasBeforeAdjust = st.gasRemaining
//     newRemaining = ApplyAdaptiveGasV2(...)
//     newRemaining = ApplyScaleFactor(gasBeforeAdjust, newRemaining, scaleFactor)
//     st.gasRemaining = newRemaining
//
//   state_processor.go:
//     after tx loop: GlobalSafeTuner.UpdateAfterBlock(safetySample)
//
// ============================================================================

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// ============================================================================
// SAFETY CONSTANTS — compile-time, deterministic
// ============================================================================

const (
	// Scale factor range: 0 = adjustments fully disabled, 10000 = 100%
	maxScaleFactorST uint64 = 10000

	// Initial scale: start at 100% (full adjustment = current behavior)
	initialScaleFactorST uint64 = 10000

	// Emergency reduction: when tx failures detected, drop by 20%
	emergencyReductionST uint64 = 2000

	// Stress reduction: when gas pool stress > threshold, drop by 5%
	stressReductionST uint64 = 500

	// Headroom reduction: when headroom is critically low, drop by 3%
	headroomReductionST uint64 = 300

	// Normal recovery: when stable, increase by 1% per block
	normalRecoveryST uint64 = 100

	// Cautious recovery: after emergency, increase by 0.5% per block
	cautiousRecoveryST uint64 = 50

	// Cautious period: stay in cautious mode for 50 blocks after emergency
	cautiousPeriodST uint64 = 50

	// Minimum consecutive safe blocks before recovery starts
	minSafeBlocksBeforeRecovery uint64 = 5

	// Gas pool stress threshold: if cumulative penalty gas exceeds this
	// fraction of block gas limit, the system is draining the pool too fast.
	// 800 / 10000 = 8.0% of block gas limit
	poolStressThresholdST uint64 = 800

	// Headroom floor: if minimum gasRemaining / initialGas across any tx
	// falls below this ratio, tx is dangerously close to gas exhaustion.
	// 100 / 10000 = 1.0%
	headroomFloorST uint64 = 100

	// Maximum scale change per block (prevents sudden jumps)
	maxScaleChangePerBlockST uint64 = 2000
)

// ============================================================================
// BLOCK SAFETY SAMPLE — deterministic per-block metrics
// ============================================================================

// BlockSafetySample holds gas pool safety metrics collected during
// block processing. Produced by the extended BlockTraceAggregator.
type BlockSafetySample struct {
	BlockNumber      uint64
	BlockGasLimit    uint64
	TotalPenaltyGas  uint64 // cumulative gas consumed by adaptive penalties
	TotalDiscountGas uint64 // cumulative gas added back by adaptive discounts
	MinGasHeadroom   uint64 // minimum gasRemaining across all adjusted txs
	MaxInitialGas    uint64 // maximum initialGas (gasLimit) across all txs
	FailedTxCount    uint64 // txs with execution errors (vmerr != nil)
	TotalTxCount     uint64 // total txs processed in block
	AdjustedTxCount  uint64 // txs that received non-zero adaptive adjustment
}

// ============================================================================
// SAFE TUNER
// ============================================================================

// SafeTuner implements the safety envelope around adaptive gas adjustments.
// It controls a scaleFactor that modulates the magnitude of all adjustments.
type SafeTuner struct {
	enabled atomic.Bool

	mu sync.RWMutex

	// Core state: the scale factor (0 to 10000)
	scaleFactor uint64

	// Cautious mode tracking
	cautiousMode     bool
	cautiousEndBlock uint64

	// Convergence tracking
	consecutiveSafe uint64 // consecutive blocks without safety issues
	lastBlockNumber uint64

	// Lifetime stats
	totalEmergencies  uint64
	totalStressEvents uint64
	totalReductions   uint64 // how many times scaleFactor was reduced
}

// GlobalSafeTuner is the singleton instance.
var GlobalSafeTuner = &SafeTuner{
	scaleFactor: initialScaleFactorST,
}

func init() {
	// Enabled by default. At scale=10000, behavior is identical to
	// having no safety envelope (transparent pass-through).
	GlobalSafeTuner.enabled.Store(true)
}

// SetEnabled enables or disables the safe tuner.
func (st *SafeTuner) SetEnabled(v bool) {
	st.enabled.Store(v)
}

// IsEnabled returns whether the safe tuner is active.
func (st *SafeTuner) IsEnabled() bool {
	return st.enabled.Load()
}

// GetScaleFactor returns the current scale factor.
// Thread-safe — called from state_transition.go during block processing.
func (st *SafeTuner) GetScaleFactor() uint64 {
	st.mu.RLock()
	sf := st.scaleFactor
	st.mu.RUnlock()
	return sf
}

// ============================================================================
// CORE SAFETY LOGIC — UpdateAfterBlock
// ============================================================================

// UpdateAfterBlock processes one block's safety metrics and updates the
// scaleFactor for the NEXT block's adaptive gas adjustments.
//
// Must be called exactly once per block, after all transactions are processed,
// in block-number order, from the block processing pipeline.
//
// The update implements a three-tier safety envelope:
//
//   TIER 1 — EMERGENCY (tx failures detected):
//     Immediately reduce scaleFactor by 20%.
//     Enter cautious mode for 50 blocks.
//     Rationale: failures are the worst outcome. Aggressive reduction
//     prevents further failures in the next block.
//
//   TIER 2 — STRESS (gas pool depletion high, no failures yet):
//     Reduce scaleFactor by 5%.
//     Rationale: pool is being drained too fast. Preventive reduction
//     before failures actually occur.
//
//   TIER 3 — HEADROOM (any tx near gas exhaustion):
//     Reduce scaleFactor by 3%.
//     Rationale: at least one tx barely survived. One more penalty
//     increase could push it over the edge.
//
//   RECOVERY (all tiers clear):
//     After 5 consecutive safe blocks, increase scaleFactor by 1%/block
//     (0.5%/block in cautious mode).
//     Rationale: conditions have stabilized, safe to re-engage.
//
// All math is integer-only. Deterministic. Consensus-safe.
func (st *SafeTuner) UpdateAfterBlock(sample BlockSafetySample) {
	if !st.enabled.Load() {
		return
	}

	if sample.TotalTxCount == 0 {
		return // empty block, nothing to learn from
	}

	st.mu.Lock()
	defer st.mu.Unlock()

	st.lastBlockNumber = sample.BlockNumber

	// ── TIER 1: Emergency — tx failures detected ──
	if sample.FailedTxCount > 0 {
		reduction := emergencyReductionST

		// Scale by severity: >20% failure rate gets double reduction
		failureRatio := sample.FailedTxCount * maxScaleFactorST / sample.TotalTxCount
		if failureRatio > 2000 {
			reduction = emergencyReductionST * 2
		}
		if reduction > maxScaleChangePerBlockST {
			reduction = maxScaleChangePerBlockST
		}

		st.scaleFactor = safeSub(st.scaleFactor, reduction)
		st.cautiousMode = true
		st.cautiousEndBlock = sample.BlockNumber + cautiousPeriodST
		st.consecutiveSafe = 0
		st.totalEmergencies++
		st.totalReductions++
		return
	}

	// ── TIER 2: Stress — gas pool depletion approaching danger ──
	if sample.BlockGasLimit > 0 && sample.TotalPenaltyGas > 0 {
		depletionRatio := sample.TotalPenaltyGas * maxScaleFactorST / sample.BlockGasLimit
		if depletionRatio > poolStressThresholdST {
			st.scaleFactor = safeSub(st.scaleFactor, stressReductionST)
			st.consecutiveSafe = 0
			st.totalStressEvents++
			st.totalReductions++
			return
		}
	}

	// ── TIER 3: Headroom — any tx dangerously close to zero gas ──
	if sample.MaxInitialGas > 0 && sample.AdjustedTxCount > 0 {
		headroomRatio := sample.MinGasHeadroom * maxScaleFactorST / sample.MaxInitialGas
		if headroomRatio < headroomFloorST {
			st.scaleFactor = safeSub(st.scaleFactor, headroomReductionST)
			st.consecutiveSafe = 0
			st.totalReductions++
			return
		}
	}

	// ── RECOVERY: All clear, gradually restore scaleFactor ──
	st.consecutiveSafe++

	// Exit cautious mode after period elapses
	if st.cautiousMode && sample.BlockNumber >= st.cautiousEndBlock {
		st.cautiousMode = false
	}

	// Wait for stability before recovering
	if st.consecutiveSafe < minSafeBlocksBeforeRecovery {
		return
	}

	// Already at max — nothing to recover
	if st.scaleFactor >= maxScaleFactorST {
		return
	}

	recovery := normalRecoveryST
	if st.cautiousMode {
		recovery = cautiousRecoveryST
	}

	st.scaleFactor = safeAdd(st.scaleFactor, recovery, maxScaleFactorST)
}

// Reset restores the safe tuner to initial state.
func (st *SafeTuner) Reset() {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.scaleFactor = initialScaleFactorST
	st.cautiousMode = false
	st.cautiousEndBlock = 0
	st.lastBlockNumber = 0
	st.consecutiveSafe = 0
	st.totalEmergencies = 0
	st.totalStressEvents = 0
	st.totalReductions = 0
}

// ============================================================================
// SCALE APPLICATION — called from state_transition.go
// ============================================================================

// ApplyScaleFactor interpolates between the pre-adjustment and post-adjustment
// gasRemaining based on the current safety scale factor.
//
//   scale = 10000 (100%): returns adjustedRemaining (full adjustment applied)
//   scale = 5000  (50%):  returns midpoint (half the adjustment applied)
//   scale = 0     (0%):   returns originalRemaining (no adjustment applied)
//
// This is a pure function. Integer-only. Deterministic.
//
// The interpolation handles both directions:
//   - Discount (adjusted > original): scales the gas ADDED back
//   - Penalty  (adjusted < original): scales the gas CONSUMED
//
// This is the ONLY point where adaptive gas magnitude is controlled.
// ApplyAdaptiveGasV2 computes the raw adjustment unchanged.
// ApplyScaleFactor then modulates how much of that adjustment takes effect.
func ApplyScaleFactor(originalRemaining, adjustedRemaining, scale uint64) uint64 {
	if scale >= maxScaleFactorST {
		return adjustedRemaining // 100% → full adjustment
	}
	if scale == 0 {
		return originalRemaining // 0% → no adjustment
	}

	if adjustedRemaining >= originalRemaining {
		// Discount direction: adjusted > original (more gas returned)
		delta := adjustedRemaining - originalRemaining
		scaledDelta := delta * scale / maxScaleFactorST
		return originalRemaining + scaledDelta
	}

	// Penalty direction: adjusted < original (less gas returned)
	delta := originalRemaining - adjustedRemaining
	scaledDelta := delta * scale / maxScaleFactorST
	return safeSub(originalRemaining, scaledDelta)
}

// ============================================================================
// RPC REPORTING
// ============================================================================

// SafeTunerStats holds the safety envelope state for RPC reporting.
type SafeTunerStats struct {
	Enabled           bool   `json:"enabled"`
	ScaleFactor       uint64 `json:"scaleFactor"`       // 0-10000
	ScalePercent      string `json:"scalePercent"`       // human-readable
	CautiousMode      bool   `json:"cautiousMode"`
	CautiousEndBlock  uint64 `json:"cautiousEndBlock,omitempty"`
	ConsecutiveSafe   uint64 `json:"consecutiveSafeBlocks"`
	TotalEmergencies  uint64 `json:"totalEmergencies"`
	TotalStressEvents uint64 `json:"totalStressEvents"`
	TotalReductions   uint64 `json:"totalReductions"`
	LastBlockNumber   uint64 `json:"lastBlockNumber"`
}

// Stats returns the safe tuner state for RPC.
func (st *SafeTuner) Stats() SafeTunerStats {
	st.mu.RLock()
	defer st.mu.RUnlock()

	whole := st.scaleFactor / 100
	frac := st.scaleFactor % 100
	var pct string
	if frac == 0 {
		pct = fmt.Sprintf("%d%%", whole)
	} else {
		pct = fmt.Sprintf("%d.%02d%%", whole, frac)
	}

	return SafeTunerStats{
		Enabled:           st.enabled.Load(),
		ScaleFactor:       st.scaleFactor,
		ScalePercent:      pct,
		CautiousMode:      st.cautiousMode,
		CautiousEndBlock:  st.cautiousEndBlock,
		ConsecutiveSafe:   st.consecutiveSafe,
		TotalEmergencies:  st.totalEmergencies,
		TotalStressEvents: st.totalStressEvents,
		TotalReductions:   st.totalReductions,
		LastBlockNumber:   st.lastBlockNumber,
	}
}

// ============================================================================
// HELPERS
// ============================================================================

func safeSub(a, b uint64) uint64 {
	if b > a {
		return 0
	}
	return a - b
}

func safeAdd(a, b, max uint64) uint64 {
	r := a + b
	if r > max || r < a { // overflow
		return max
	}
	return r
}

// MaxScaleFactorValue returns the maximum scale factor constant (10000 = 100%).
// Used by state_transition.go to check if scaling is needed.
func MaxScaleFactorValue() uint64 {
	return maxScaleFactorST
}