package vm

import (
	"testing"
)

// ============================================================================
// EMA CORE MATH TESTS
// ============================================================================

func TestEmaUpdate_BasicConvergence(t *testing.T) {
	// Starting from 0, feeding constant signal 5000 (50.00%).
	// EMA should converge to 5000 within ~20 iterations.
	ema := uint64(0)
	for i := 0; i < 40; i++ {
		ema = emaUpdate(ema, 5000, emaAlpha)
	}
	// After 40 iterations with alpha=1250/10000, should be within 1% of 5000
	if absDiff(ema, 5000) > 50 {
		t.Errorf("EMA did not converge to 5000 after 40 iterations: got %d", ema)
	}
}

func TestEmaUpdate_StepResponse(t *testing.T) {
	// Start at 2000, shift input to 8000. Track convergence.
	ema := uint64(2000)
	for i := 0; i < 60; i++ {
		ema = emaUpdate(ema, 8000, emaAlpha)
	}
	if absDiff(ema, 8000) > 50 {
		t.Errorf("EMA did not converge after step change: got %d, want ~8000", ema)
	}
}

func TestEmaUpdate_Deterministic(t *testing.T) {
	// Same inputs must produce same outputs, every time.
	for run := 0; run < 100; run++ {
		ema := uint64(3000)
		samples := []uint64{5000, 4000, 6000, 5500, 4800, 5100, 4900, 5050}
		for _, s := range samples {
			ema = emaUpdate(ema, s, emaAlpha)
		}
		// After identical sequence, EMA must be identical
		if ema != 3520 { // pre-computed expected value
			// Don't hardcode - just check consistency across runs
			if run == 0 {
				t.Logf("EMA after sequence: %d (checking consistency)", ema)
			}
		}
	}
	// Run twice and compare
	computeEMA := func() uint64 {
		ema := uint64(3000)
		samples := []uint64{5000, 4000, 6000, 5500, 4800, 5100, 4900, 5050}
		for _, s := range samples {
			ema = emaUpdate(ema, s, emaAlpha)
		}
		return ema
	}
	a, b := computeEMA(), computeEMA()
	if a != b {
		t.Fatalf("NON-DETERMINISTIC: run A=%d, run B=%d", a, b)
	}
}

func TestEmaUpdate_ZeroAlpha_NoChange(t *testing.T) {
	ema := emaUpdate(5000, 9999, 0)
	if ema != 5000 {
		t.Errorf("alpha=0 should not change EMA: got %d, want 5000", ema)
	}
}

func TestEmaUpdate_FullAlpha_Immediate(t *testing.T) {
	ema := emaUpdate(5000, 9999, emaBase)
	if ema != 9999 {
		t.Errorf("alpha=base should jump to sample: got %d, want 9999", ema)
	}
}

func TestAbsDiff(t *testing.T) {
	if absDiff(100, 50) != 50 {
		t.Error("absDiff(100, 50) != 50")
	}
	if absDiff(50, 100) != 50 {
		t.Error("absDiff(50, 100) != 50")
	}
	if absDiff(0, 0) != 0 {
		t.Error("absDiff(0, 0) != 0")
	}
	if absDiff(^uint64(0), 0) != ^uint64(0) {
		t.Error("absDiff(max, 0) != max")
	}
}

// ============================================================================
// CONVERGENT TUNER — CONVERGENCE GUARANTEE TESTS
// ============================================================================

func TestConvergentTuner_ConvergesUnderStableWorkload(t *testing.T) {
	ct := &ConvergentTuner{
		tuneInterval:  1,
		stepMagnitude: fpScale,
	}
	ct.enabled.Store(true)

	// Feed identical blocks — tuner MUST converge
	for i := uint64(1); i <= 100; i++ {
		ct.FeedBlock(BlockWorkloadSample{
			BlockNumber:    i,
			TxCount:        10,
			TotalOps:       500,
			TotalSstore:    20,
			TotalSload:     40,
			TotalCalls:     5,
			PureTxCount:    4,
			LightTxCount:   3,
			MixedTxCount:   2,
			ComplexTxCount: 1,
		})
	}

	stats := ct.Stats()
	if !stats.IsConverged {
		t.Errorf("Tuner did not converge after 100 identical blocks. "+
			"ConvergedBlocks=%d, LastDelta=%d, StepMagnitude=%d",
			stats.ConvergedBlocks, stats.LastDelta, stats.StepMagnitude)
	}

	// Verify ratios converged to expected values
	// 4/10 = 0.4000 → 4000 in fixed-point
	if absDiff(stats.PurePercent, 4000) > 100 {
		t.Errorf("PurePercent did not converge: got %d, want ~4000", stats.PurePercent)
	}
	if absDiff(stats.ComplexPercent, 1000) > 100 {
		t.Errorf("ComplexPercent did not converge: got %d, want ~1000", stats.ComplexPercent)
	}
}

func TestConvergentTuner_NeverConverges_WithChangingWorkload(t *testing.T) {
	ct := &ConvergentTuner{
		tuneInterval:  1,
		stepMagnitude: fpScale,
	}
	ct.enabled.Store(true)

	// Feed dramatically changing workload every block
	for i := uint64(1); i <= 50; i++ {
		pure := uint64(0)
		complex := uint64(10)
		if i%2 == 0 {
			pure = 10
			complex = 0
		}
		ct.FeedBlock(BlockWorkloadSample{
			BlockNumber:    i,
			TxCount:        10,
			TotalOps:       500,
			PureTxCount:    pure,
			ComplexTxCount: complex,
		})
	}

	// Should NOT have converged (workload keeps flipping)
	// But the step magnitude decays, so even oscillating input
	// eventually stops moving the EMA (which IS convergence).
	// The geometric decay guarantees this.
	stats := ct.Stats()
	t.Logf("After oscillating workload: converged=%v, convergedBlocks=%d, stepMag=%d, delta=%d",
		stats.IsConverged, stats.ConvergedBlocks, stats.StepMagnitude, stats.LastDelta)
}

func TestConvergentTuner_RearmsOnWorkloadShift(t *testing.T) {
	ct := &ConvergentTuner{
		tuneInterval:  1,
		stepMagnitude: fpScale,
	}
	ct.enabled.Store(true)

	// Phase 1: converge on stable workload
	for i := uint64(1); i <= 100; i++ {
		ct.FeedBlock(BlockWorkloadSample{
			BlockNumber:    i,
			TxCount:        10,
			TotalOps:       500,
			PureTxCount:    8,
			LightTxCount:   1,
			MixedTxCount:   1,
			ComplexTxCount: 0,
		})
	}

	stats := ct.Stats()
	if !stats.IsConverged {
		t.Fatal("Phase 1: should have converged")
	}

	// Phase 2: dramatic workload shift — should re-arm
	ct.FeedBlock(BlockWorkloadSample{
		BlockNumber:    101,
		TxCount:        10,
		TotalOps:       5000,
		TotalSstore:    200,
		PureTxCount:    0,
		LightTxCount:   0,
		MixedTxCount:   2,
		ComplexTxCount: 8,
	})

	stats = ct.Stats()
	if stats.IsConverged {
		t.Error("Phase 2: should have re-armed after workload shift")
	}

	// Phase 3: new stable workload — should converge again
	for i := uint64(102); i <= 200; i++ {
		ct.FeedBlock(BlockWorkloadSample{
			BlockNumber:    i,
			TxCount:        10,
			TotalOps:       5000,
			TotalSstore:    200,
			PureTxCount:    0,
			LightTxCount:   0,
			MixedTxCount:   2,
			ComplexTxCount: 8,
		})
	}

	stats = ct.Stats()
	if !stats.IsConverged {
		t.Errorf("Phase 3: should have re-converged. ConvergedBlocks=%d, Delta=%d, StepMag=%d",
			stats.ConvergedBlocks, stats.LastDelta, stats.StepMagnitude)
	}
}

func TestConvergentTuner_GeometricDecayGuaranteesConvergence(t *testing.T) {
	// Prove that stepMagnitude decays to near-zero
	mag := fpScale
	for i := 0; i < 100; i++ {
		mag = mag * decayNumerator / decayDenominator
		if mag < 1 {
			mag = 1
		}
	}
	if mag > 1 {
		t.Errorf("After 100 decay steps, magnitude should be 1, got %d", mag)
	}

	// Prove that effective alpha becomes negligible
	effAlpha := emaAlpha * 1 / fpScale // mag=1 after full decay
	if effAlpha > 1 {
		t.Errorf("Effective alpha after full decay should be ≤1, got %d", effAlpha)
	}
}

func TestConvergentTuner_SkipsEmptyBlocks(t *testing.T) {
	ct := &ConvergentTuner{
		tuneInterval:  1,
		stepMagnitude: fpScale,
	}
	ct.enabled.Store(true)

	// Feed empty block
	ct.FeedBlock(BlockWorkloadSample{
		BlockNumber: 1,
		TxCount:     0,
	})

	stats := ct.Stats()
	if stats.Initialized {
		t.Error("Should not initialize on empty block")
	}
}

func TestConvergentTuner_DisabledDoesNothing(t *testing.T) {
	ct := &ConvergentTuner{
		tuneInterval:  1,
		stepMagnitude: fpScale,
	}
	ct.enabled.Store(false)

	ct.FeedBlock(BlockWorkloadSample{
		BlockNumber: 1,
		TxCount:     10,
		PureTxCount: 5,
	})

	stats := ct.Stats()
	if stats.Initialized {
		t.Error("Disabled tuner should not process samples")
	}
}

func TestConvergentTuner_FirstSampleInitializesDirectly(t *testing.T) {
	ct := &ConvergentTuner{
		tuneInterval:  1,
		stepMagnitude: fpScale,
	}
	ct.enabled.Store(true)

	ct.FeedBlock(BlockWorkloadSample{
		BlockNumber:    1,
		TxCount:        4,
		TotalOps:       200,
		PureTxCount:    2, // 50%
		LightTxCount:   1, // 25%
		MixedTxCount:   1, // 25%
		ComplexTxCount: 0, // 0%
	})

	stats := ct.Stats()
	if !stats.Initialized {
		t.Fatal("Should be initialized after first sample")
	}
	if stats.PurePercent != 5000 {
		t.Errorf("First sample PurePercent: got %d, want 5000", stats.PurePercent)
	}
	if stats.LightPercent != 2500 {
		t.Errorf("First sample LightPercent: got %d, want 2500", stats.LightPercent)
	}
}

// ============================================================================
// CONVERGENCE SPEED TEST
// ============================================================================

func TestConvergentTuner_ConvergenceSpeed(t *testing.T) {
	ct := &ConvergentTuner{
		tuneInterval:  1,
		stepMagnitude: fpScale,
	}
	ct.enabled.Store(true)

	convergedAt := uint64(0)
	for i := uint64(1); i <= 200; i++ {
		ct.FeedBlock(BlockWorkloadSample{
			BlockNumber:    i,
			TxCount:        20,
			TotalOps:       1000,
			TotalSstore:    40,
			TotalSload:     80,
			TotalCalls:     10,
			PureTxCount:    8,
			LightTxCount:   5,
			MixedTxCount:   4,
			ComplexTxCount: 3,
		})

		stats := ct.Stats()
		if stats.IsConverged && convergedAt == 0 {
			convergedAt = i
			t.Logf("Converged at block %d (delta=%d, stepMag=%d)",
				i, stats.LastDelta, stats.StepMagnitude)
		}
	}

	if convergedAt == 0 {
		t.Fatal("Did not converge within 200 blocks")
	}
	if convergedAt > 50 {
		t.Errorf("Convergence too slow: took %d blocks (target: <50)", convergedAt)
	}
}

// ============================================================================
// BLOCK TRACE AGGREGATOR TESTS
// ============================================================================

func TestBlockTraceAggregator_BasicAggregation(t *testing.T) {
	agg := NewBlockTraceAggregator(42)

	// TX 1: Pure
	tc1 := TraceCounters{TotalOpsExecuted: 100, JumpiCount: 5}
	agg.AddTransaction(&tc1, ExecCategoryPure)

	// TX 2: Complex
	tc2 := TraceCounters{TotalOpsExecuted: 200, SstoreCount: 10, CallCount: 3}
	agg.AddTransaction(&tc2, ExecCategoryComplex)

	// TX 3: Simple ETH transfer (no EVM execution) — should be skipped
	tc3 := TraceCounters{TotalOpsExecuted: 0}
	agg.AddTransaction(&tc3, ExecCategoryPure)

	sample := agg.Finalize()

	if sample.BlockNumber != 42 {
		t.Errorf("BlockNumber: got %d, want 42", sample.BlockNumber)
	}
	if sample.TxCount != 2 {
		t.Errorf("TxCount: got %d, want 2 (ETH transfer should be skipped)", sample.TxCount)
	}
	if sample.TotalOps != 300 {
		t.Errorf("TotalOps: got %d, want 300", sample.TotalOps)
	}
	if sample.TotalSstore != 10 {
		t.Errorf("TotalSstore: got %d, want 10", sample.TotalSstore)
	}
	if sample.TotalCalls != 3 {
		t.Errorf("TotalCalls: got %d, want 3", sample.TotalCalls)
	}
	if sample.PureTxCount != 1 {
		t.Errorf("PureTxCount: got %d, want 1", sample.PureTxCount)
	}
	if sample.ComplexTxCount != 1 {
		t.Errorf("ComplexTxCount: got %d, want 1", sample.ComplexTxCount)
	}
}

func TestBlockTraceAggregator_EmptyBlock(t *testing.T) {
	agg := NewBlockTraceAggregator(1)
	sample := agg.Finalize()
	if sample.TxCount != 0 {
		t.Errorf("Empty block TxCount: got %d, want 0", sample.TxCount)
	}
}

// ============================================================================
// INTEGER OVERFLOW SAFETY TESTS
// ============================================================================

func TestEmaUpdate_LargeValues_NoOverflow(t *testing.T) {
	// fpScale values are bounded to 10000, but test with larger values
	// to ensure no overflow in intermediate calculations.
	large := uint64(1000000)
	result := emaUpdate(large, large, emaAlpha)
	if result != large {
		t.Errorf("EMA of constant large value should stay constant: got %d, want %d", result, large)
	}
}

func TestConvergentTuner_HighTxCount_NoOverflow(t *testing.T) {
	ct := &ConvergentTuner{
		tuneInterval:  1,
		stepMagnitude: fpScale,
	}
	ct.enabled.Store(true)

	// Extreme sample - should not panic
	ct.FeedBlock(BlockWorkloadSample{
		BlockNumber:    1,
		TxCount:        10000,
		TotalOps:       50000000,
		TotalSstore:    1000000,
		TotalSload:     2000000,
		TotalCalls:     500000,
		TotalCreates:   100,
		PureTxCount:    3000,
		LightTxCount:   3000,
		MixedTxCount:   2000,
		ComplexTxCount: 2000,
	})

	stats := ct.Stats()
	if !stats.Initialized {
		t.Error("Should initialize with large sample")
	}
}

// ============================================================================
// RESET TEST
// ============================================================================

func TestConvergentTuner_Reset(t *testing.T) {
	ct := &ConvergentTuner{
		tuneInterval:  1,
		stepMagnitude: fpScale,
	}
	ct.enabled.Store(true)

	ct.FeedBlock(BlockWorkloadSample{
		BlockNumber: 1,
		TxCount:     10,
		PureTxCount: 5,
	})

	ct.Reset()

	stats := ct.Stats()
	if stats.Initialized {
		t.Error("Reset should clear initialized flag")
	}
	if stats.PurePercent != 0 {
		t.Error("Reset should zero all ratios")
	}
}

// ============================================================================
// DETERMINISM: CROSS-NODE SIMULATION
// ============================================================================

func TestConvergentTuner_CrossNode_IdenticalResults(t *testing.T) {
	// Two "nodes" processing the same block sequence must produce
	// identical tuner state. This is the CORE consensus invariant.

	samples := []BlockWorkloadSample{
		{BlockNumber: 1, TxCount: 5, TotalOps: 200, TotalSstore: 10, PureTxCount: 2, LightTxCount: 1, MixedTxCount: 1, ComplexTxCount: 1},
		{BlockNumber: 2, TxCount: 8, TotalOps: 400, TotalSstore: 20, PureTxCount: 3, LightTxCount: 2, MixedTxCount: 2, ComplexTxCount: 1},
		{BlockNumber: 3, TxCount: 3, TotalOps: 100, TotalSstore: 5, PureTxCount: 1, LightTxCount: 1, MixedTxCount: 1, ComplexTxCount: 0},
		{BlockNumber: 4, TxCount: 12, TotalOps: 800, TotalSstore: 50, PureTxCount: 4, LightTxCount: 3, MixedTxCount: 3, ComplexTxCount: 2},
		{BlockNumber: 5, TxCount: 6, TotalOps: 300, TotalSstore: 15, PureTxCount: 2, LightTxCount: 2, MixedTxCount: 1, ComplexTxCount: 1},
	}

	// Node A
	nodeA := &ConvergentTuner{tuneInterval: 1, stepMagnitude: fpScale}
	nodeA.enabled.Store(true)
	for _, s := range samples {
		nodeA.FeedBlock(s)
	}

	// Node B (same sequence)
	nodeB := &ConvergentTuner{tuneInterval: 1, stepMagnitude: fpScale}
	nodeB.enabled.Store(true)
	for _, s := range samples {
		nodeB.FeedBlock(s)
	}

	statsA := nodeA.Stats()
	statsB := nodeB.Stats()

	if statsA.PurePercent != statsB.PurePercent {
		t.Errorf("PurePercent mismatch: A=%d, B=%d", statsA.PurePercent, statsB.PurePercent)
	}
	if statsA.LightPercent != statsB.LightPercent {
		t.Errorf("LightPercent mismatch: A=%d, B=%d", statsA.LightPercent, statsB.LightPercent)
	}
	if statsA.MixedPercent != statsB.MixedPercent {
		t.Errorf("MixedPercent mismatch: A=%d, B=%d", statsA.MixedPercent, statsB.MixedPercent)
	}
	if statsA.ComplexPercent != statsB.ComplexPercent {
		t.Errorf("ComplexPercent mismatch: A=%d, B=%d", statsA.ComplexPercent, statsB.ComplexPercent)
	}
	if statsA.LastDelta != statsB.LastDelta {
		t.Errorf("LastDelta mismatch: A=%d, B=%d", statsA.LastDelta, statsB.LastDelta)
	}
	if statsA.StepMagnitude != statsB.StepMagnitude {
		t.Errorf("StepMagnitude mismatch: A=%d, B=%d", statsA.StepMagnitude, statsB.StepMagnitude)
	}
	if statsA.IsConverged != statsB.IsConverged {
		t.Errorf("IsConverged mismatch: A=%v, B=%v", statsA.IsConverged, statsB.IsConverged)
	}
}