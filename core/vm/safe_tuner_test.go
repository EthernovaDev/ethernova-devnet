package vm

import (
	"testing"
)

// ============================================================================
// ApplyScaleFactor tests
// ============================================================================

func TestApplyScaleFactor_FullScale(t *testing.T) {
	// scale=10000 (100%) → returns adjustedRemaining unchanged
	result := ApplyScaleFactor(1000, 800, 10000) // penalty
	if result != 800 {
		t.Errorf("full scale penalty: got %d, want 800", result)
	}
	result = ApplyScaleFactor(1000, 1250, 10000) // discount
	if result != 1250 {
		t.Errorf("full scale discount: got %d, want 1250", result)
	}
}

func TestApplyScaleFactor_ZeroScale(t *testing.T) {
	// scale=0 → returns originalRemaining (no adjustment)
	result := ApplyScaleFactor(1000, 800, 0)
	if result != 1000 {
		t.Errorf("zero scale: got %d, want 1000", result)
	}
}

func TestApplyScaleFactor_HalfScale_Penalty(t *testing.T) {
	// original=1000, adjusted=800 → delta=200, half=100
	// result = 1000 - 100 = 900
	result := ApplyScaleFactor(1000, 800, 5000)
	if result != 900 {
		t.Errorf("half scale penalty: got %d, want 900", result)
	}
}

func TestApplyScaleFactor_HalfScale_Discount(t *testing.T) {
	// original=1000, adjusted=1250 → delta=250, half=125
	// result = 1000 + 125 = 1125
	result := ApplyScaleFactor(1000, 1250, 5000)
	if result != 1125 {
		t.Errorf("half scale discount: got %d, want 1125", result)
	}
}

func TestApplyScaleFactor_NoChange(t *testing.T) {
	// original == adjusted → result is same regardless of scale
	result := ApplyScaleFactor(1000, 1000, 5000)
	if result != 1000 {
		t.Errorf("no change: got %d, want 1000", result)
	}
}

func TestApplyScaleFactor_Deterministic(t *testing.T) {
	for i := 0; i < 1000; i++ {
		a := ApplyScaleFactor(50000, 45000, 7500)
		b := ApplyScaleFactor(50000, 45000, 7500)
		if a != b {
			t.Fatalf("NON-DETERMINISTIC at run %d: a=%d b=%d", i, a, b)
		}
	}
}

// ============================================================================
// SafeTuner emergency reduction tests
// ============================================================================

func TestSafeTuner_EmergencyOnFailure(t *testing.T) {
	st := &SafeTuner{scaleFactor: 10000}
	st.enabled.Store(true)

	// Block with 2 failures out of 10 txs
	st.UpdateAfterBlock(BlockSafetySample{
		BlockNumber:   1,
		BlockGasLimit: 30000000,
		TotalTxCount:  10,
		FailedTxCount: 2,
	})

	if st.scaleFactor != 10000-emergencyReductionST {
		t.Errorf("after failure: scaleFactor=%d, want %d", st.scaleFactor, 10000-emergencyReductionST)
	}
	if !st.cautiousMode {
		t.Error("should be in cautious mode after failure")
	}
	if st.totalEmergencies != 1 {
		t.Errorf("totalEmergencies=%d, want 1", st.totalEmergencies)
	}
}

func TestSafeTuner_SevereFailure_DoubleReduction(t *testing.T) {
	st := &SafeTuner{scaleFactor: 10000}
	st.enabled.Store(true)

	// >20% failure rate: 3 out of 10
	st.UpdateAfterBlock(BlockSafetySample{
		BlockNumber:   1,
		BlockGasLimit: 30000000,
		TotalTxCount:  10,
		FailedTxCount: 3,
	})

	// Should get double emergency reduction (capped at maxScaleChangePerBlock)
	expected := uint64(10000 - maxScaleChangePerBlockST)
	if emergencyReductionST*2 < maxScaleChangePerBlockST {
		expected = 10000 - emergencyReductionST*2
	}
	if st.scaleFactor != expected {
		t.Errorf("severe failure: scaleFactor=%d, want %d", st.scaleFactor, expected)
	}
}

// ============================================================================
// SafeTuner stress reduction tests
// ============================================================================

func TestSafeTuner_StressReduction(t *testing.T) {
	st := &SafeTuner{scaleFactor: 10000}
	st.enabled.Store(true)

	// Penalty gas = 10% of block gas limit (> 8% threshold)
	st.UpdateAfterBlock(BlockSafetySample{
		BlockNumber:     1,
		BlockGasLimit:   30000000,
		TotalPenaltyGas: 3000000, // 10%
		TotalTxCount:    50,
	})

	if st.scaleFactor != 10000-stressReductionST {
		t.Errorf("stress: scaleFactor=%d, want %d", st.scaleFactor, 10000-stressReductionST)
	}
}

func TestSafeTuner_NoStress_BelowThreshold(t *testing.T) {
	st := &SafeTuner{scaleFactor: 10000}
	st.enabled.Store(true)

	// Penalty gas = 5% of block gas limit (< 8% threshold)
	st.UpdateAfterBlock(BlockSafetySample{
		BlockNumber:     1,
		BlockGasLimit:   30000000,
		TotalPenaltyGas: 1500000, // 5%
		TotalTxCount:    50,
	})

	// Should not reduce (goes to consecutive safe tracking)
	if st.scaleFactor != 10000 {
		t.Errorf("below threshold: scaleFactor=%d, want 10000", st.scaleFactor)
	}
}

// ============================================================================
// SafeTuner recovery tests
// ============================================================================

func TestSafeTuner_GradualRecovery(t *testing.T) {
	st := &SafeTuner{scaleFactor: 8000} // start reduced
	st.enabled.Store(true)

	// Feed safe blocks
	for i := uint64(1); i <= 20; i++ {
		st.UpdateAfterBlock(BlockSafetySample{
			BlockNumber:   i,
			BlockGasLimit: 30000000,
			TotalTxCount:  10,
		})
	}

	// After minSafeBlocksBeforeRecovery blocks, recovery starts
	// Block 5 onward: consecutiveSafe >= 5, recovery triggers
	// 20 - 4 = 16 recovery blocks × 100 = 1600
	expected := uint64(8000 + 16*normalRecoveryST)
	if expected > 10000 {
		expected = 10000
	}
	if st.scaleFactor != expected {
		t.Errorf("after recovery: scaleFactor=%d, want %d", st.scaleFactor, expected)
	}
}

func TestSafeTuner_CautiousRecovery(t *testing.T) {
	st := &SafeTuner{
		scaleFactor:  6000,
		cautiousMode: true,
		cautiousEndBlock: 100,
	}
	st.enabled.Store(true)

	// Feed safe blocks during cautious period
	for i := uint64(1); i <= 20; i++ {
		st.UpdateAfterBlock(BlockSafetySample{
			BlockNumber:   i, // still < cautiousEndBlock=100
			BlockGasLimit: 30000000,
			TotalTxCount:  10,
		})
	}

	// 16 recovery blocks × 50 (cautious) = 800
	expected := uint64(6000 + 16*cautiousRecoveryST)
	if st.scaleFactor != expected {
		t.Errorf("cautious recovery: scaleFactor=%d, want %d", st.scaleFactor, expected)
	}
}

func TestSafeTuner_CautiousModeExpires(t *testing.T) {
	st := &SafeTuner{
		scaleFactor:      8000,
		cautiousMode:     true,
		cautiousEndBlock: 10,
	}
	st.enabled.Store(true)

	// Feed blocks past the cautious end
	for i := uint64(1); i <= 20; i++ {
		st.UpdateAfterBlock(BlockSafetySample{
			BlockNumber:   i,
			BlockGasLimit: 30000000,
			TotalTxCount:  10,
		})
	}

	if st.cautiousMode {
		t.Error("cautious mode should have expired after block 10")
	}
}

// ============================================================================
// SafeTuner bounds tests
// ============================================================================

func TestSafeTuner_NeverExceedsMax(t *testing.T) {
	st := &SafeTuner{scaleFactor: 9990}
	st.enabled.Store(true)

	for i := uint64(1); i <= 100; i++ {
		st.UpdateAfterBlock(BlockSafetySample{
			BlockNumber:   i,
			BlockGasLimit: 30000000,
			TotalTxCount:  10,
		})
	}

	if st.scaleFactor > maxScaleFactorST {
		t.Errorf("scaleFactor %d exceeds max %d", st.scaleFactor, maxScaleFactorST)
	}
}

func TestSafeTuner_NeverBelowZero(t *testing.T) {
	st := &SafeTuner{scaleFactor: 500}
	st.enabled.Store(true)

	// Massive failure
	st.UpdateAfterBlock(BlockSafetySample{
		BlockNumber:   1,
		BlockGasLimit: 30000000,
		TotalTxCount:  10,
		FailedTxCount: 10,
	})

	if st.scaleFactor > maxScaleFactorST {
		t.Error("scaleFactor underflowed")
	}
	t.Logf("scaleFactor after massive failure from 500: %d", st.scaleFactor)
}

// ============================================================================
// SafeTuner full cycle test
// ============================================================================

func TestSafeTuner_FullCycle_FailureRecoveryStabilization(t *testing.T) {
	st := &SafeTuner{scaleFactor: 10000}
	st.enabled.Store(true)

	// Phase 1: Failure detected → emergency reduction
	st.UpdateAfterBlock(BlockSafetySample{
		BlockNumber:   1,
		BlockGasLimit: 30000000,
		TotalTxCount:  50,
		FailedTxCount: 5,
	})
	if st.scaleFactor >= 10000 {
		t.Fatal("Phase 1: should have reduced scaleFactor")
	}
	phase1Scale := st.scaleFactor
	t.Logf("Phase 1 (failure): scaleFactor=%d", phase1Scale)

	// Phase 2: Stable blocks → gradual recovery
	for i := uint64(2); i <= 100; i++ {
		st.UpdateAfterBlock(BlockSafetySample{
			BlockNumber:   i,
			BlockGasLimit: 30000000,
			TotalTxCount:  50,
		})
	}
	if st.scaleFactor <= phase1Scale {
		t.Fatal("Phase 2: should have recovered")
	}
	t.Logf("Phase 2 (recovered): scaleFactor=%d", st.scaleFactor)

	// Phase 3: Scale should reach max (or near max)
	for i := uint64(101); i <= 300; i++ {
		st.UpdateAfterBlock(BlockSafetySample{
			BlockNumber:   i,
			BlockGasLimit: 30000000,
			TotalTxCount:  50,
		})
	}
	if st.scaleFactor != maxScaleFactorST {
		t.Errorf("Phase 3: should be at max, got %d", st.scaleFactor)
	}
	t.Logf("Phase 3 (stabilized): scaleFactor=%d", st.scaleFactor)
}

// ============================================================================
// Cross-node determinism test
// ============================================================================

func TestSafeTuner_CrossNode_Identical(t *testing.T) {
	samples := []BlockSafetySample{
		{BlockNumber: 1, BlockGasLimit: 30000000, TotalTxCount: 20, TotalPenaltyGas: 500000},
		{BlockNumber: 2, BlockGasLimit: 30000000, TotalTxCount: 30, FailedTxCount: 2},
		{BlockNumber: 3, BlockGasLimit: 30000000, TotalTxCount: 25},
		{BlockNumber: 4, BlockGasLimit: 30000000, TotalTxCount: 15, TotalPenaltyGas: 3000000},
		{BlockNumber: 5, BlockGasLimit: 30000000, TotalTxCount: 10},
	}

	// Node A
	nodeA := &SafeTuner{scaleFactor: 10000}
	nodeA.enabled.Store(true)
	for _, s := range samples {
		nodeA.UpdateAfterBlock(s)
	}

	// Node B
	nodeB := &SafeTuner{scaleFactor: 10000}
	nodeB.enabled.Store(true)
	for _, s := range samples {
		nodeB.UpdateAfterBlock(s)
	}

	statsA := nodeA.Stats()
	statsB := nodeB.Stats()

	if statsA.ScaleFactor != statsB.ScaleFactor {
		t.Errorf("ScaleFactor mismatch: A=%d B=%d", statsA.ScaleFactor, statsB.ScaleFactor)
	}
	if statsA.CautiousMode != statsB.CautiousMode {
		t.Errorf("CautiousMode mismatch: A=%v B=%v", statsA.CautiousMode, statsB.CautiousMode)
	}
	if statsA.TotalEmergencies != statsB.TotalEmergencies {
		t.Errorf("TotalEmergencies mismatch: A=%d B=%d", statsA.TotalEmergencies, statsB.TotalEmergencies)
	}
}

// ============================================================================
// RecordGasSafety aggregator tests
// ============================================================================

func TestBlockTraceAggregator_RecordGasSafety_Penalty(t *testing.T) {
	agg := NewBlockTraceAggregator(1)

	// TX with penalty: gasRemaining reduced from 100k to 60k
	agg.RecordGasSafety(100000, 60000, 500000, 5, false)

	if agg.totalPenaltyGas != 40000 {
		t.Errorf("totalPenaltyGas=%d, want 40000", agg.totalPenaltyGas)
	}
	if agg.minGasHeadroom != 60000 {
		t.Errorf("minGasHeadroom=%d, want 60000", agg.minGasHeadroom)
	}
}

func TestBlockTraceAggregator_RecordGasSafety_Discount(t *testing.T) {
	agg := NewBlockTraceAggregator(1)

	// TX with discount: gasRemaining increased from 100k to 125k
	agg.RecordGasSafety(100000, 125000, 500000, -25, false)

	if agg.totalDiscountGas != 25000 {
		t.Errorf("totalDiscountGas=%d, want 25000", agg.totalDiscountGas)
	}
}

func TestBlockTraceAggregator_RecordGasSafety_MinHeadroom(t *testing.T) {
	agg := NewBlockTraceAggregator(1)

	agg.RecordGasSafety(100000, 60000, 500000, 5, false)  // headroom=60k
	agg.RecordGasSafety(200000, 10000, 500000, 8, false)  // headroom=10k (lower)
	agg.RecordGasSafety(50000, 30000, 500000, 3, false)   // headroom=30k

	if agg.minGasHeadroom != 10000 {
		t.Errorf("minGasHeadroom=%d, want 10000 (the minimum)", agg.minGasHeadroom)
	}
}

func TestBlockTraceAggregator_RecordGasSafety_FailureCount(t *testing.T) {
	agg := NewBlockTraceAggregator(1)

	agg.RecordGasSafety(100000, 60000, 500000, 5, false)
	agg.RecordGasSafety(100000, 60000, 500000, 5, true)  // failed
	agg.RecordGasSafety(100000, 60000, 500000, 5, true)  // failed

	if agg.failedTxCount != 2 {
		t.Errorf("failedTxCount=%d, want 2", agg.failedTxCount)
	}
}

func TestBlockTraceAggregator_FinalizeSafety(t *testing.T) {
	agg := NewBlockTraceAggregator(42)

	// Add a regular tx via AddTransaction
	tc := TraceCounters{TotalOpsExecuted: 100, SstoreCount: 5}
	agg.AddTransaction(&tc, ExecCategoryComplex)

	// Record safety for it
	agg.RecordGasSafety(200000, 180000, 500000, 5, false)

	sample := agg.FinalizeSafety(30000000)

	if sample.BlockNumber != 42 {
		t.Errorf("BlockNumber=%d, want 42", sample.BlockNumber)
	}
	if sample.BlockGasLimit != 30000000 {
		t.Errorf("BlockGasLimit=%d, want 30000000", sample.BlockGasLimit)
	}
	if sample.TotalPenaltyGas != 20000 {
		t.Errorf("TotalPenaltyGas=%d, want 20000", sample.TotalPenaltyGas)
	}
	if sample.AdjustedTxCount != 1 {
		t.Errorf("AdjustedTxCount=%d, want 1", sample.AdjustedTxCount)
	}
}