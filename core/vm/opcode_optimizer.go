package vm

import (
	"sync"
	"sync/atomic"
)

// OpcodeOptimizer detects redundant opcode sequences and tracks optimization
// opportunities. It identifies patterns like:
//   - PUSH0 POP (push then immediately pop = no-op)
//   - DUP1 POP (duplicate then pop = no-op)
//   - PUSH1 X PUSH1 X (same value pushed twice, could use DUP1)
//   - ISZERO ISZERO (double negation = no-op)
//   - NOT NOT (double bitwise not = no-op)
//
// In fast mode, these redundant sequences are detected and the gas for
// the redundant opcodes is refunded.
type OpcodeOptimizer struct {
	enabled atomic.Bool

	// Sliding window of recent opcodes for pattern detection
	mu      sync.Mutex
	windows map[string]*opcodeWindow // per-contract windows

	// Stats
	redundantOps   atomic.Uint64
	gasRefunded    atomic.Uint64
	patternsFound  atomic.Uint64
}

type opcodeWindow struct {
	ops  [4]OpCode // last 4 opcodes
	vals [4]uint64 // associated values (for PUSH)
	pos  int
}

var GlobalOpcodeOptimizer = &OpcodeOptimizer{
	windows: make(map[string]*opcodeWindow),
}

func init() {
	GlobalOpcodeOptimizer.enabled.Store(false)
}

// SetEnabled enables or disables the optimizer.
func (oo *OpcodeOptimizer) SetEnabled(v bool) {
	oo.enabled.Store(v)
}

// IsEnabled returns whether optimization is active.
func (oo *OpcodeOptimizer) IsEnabled() bool {
	return oo.enabled.Load()
}

// RecordAndCheck records an opcode and checks if it's part of a redundant
// sequence. Returns the gas that should be refunded (0 if no optimization).
func (oo *OpcodeOptimizer) RecordAndCheck(contractAddr string, op OpCode, pushVal uint64) uint64 {
	if !oo.enabled.Load() {
		return 0
	}

	oo.mu.Lock()
	w, ok := oo.windows[contractAddr]
	if !ok {
		w = &opcodeWindow{}
		oo.windows[contractAddr] = w
	}

	// Shift window
	prev := w.ops[0]
	prevVal := w.vals[0]
	copy(w.ops[1:], w.ops[:3])
	copy(w.vals[1:], w.vals[:3])
	w.ops[0] = op
	w.vals[0] = pushVal
	w.pos++
	oo.mu.Unlock()

	if w.pos < 2 {
		return 0
	}

	var refund uint64

	// Pattern: PUSH POP → no-op (refund both: 3+2=5)
	if op == POP && (prev >= PUSH0 && prev <= PUSH32) {
		refund = 5
	}

	// Pattern: DUP1 POP → no-op (refund both: 3+2=5)
	if op == POP && (prev >= DUP1 && prev <= DUP16) {
		refund = 5
	}

	// Pattern: ISZERO ISZERO → no-op (refund both: 3+3=6)
	if op == ISZERO && prev == ISZERO {
		refund = 6
	}

	// Pattern: NOT NOT → no-op (refund both: 3+3=6)
	if op == NOT && prev == NOT {
		refund = 6
	}

	// Pattern: SWAP1 SWAP1 → no-op (refund both: 3+3=6)
	if op == SWAP1 && prev == SWAP1 {
		refund = 6
	}

	// Pattern: same PUSH twice in a row → could use DUP1 (save 1 gas)
	if op >= PUSH1 && op <= PUSH32 && prev == op && pushVal == prevVal {
		refund = 1 // minor: could have used DUP1 instead
	}

	if refund > 0 {
		oo.redundantOps.Add(1)
		oo.gasRefunded.Add(refund)
		oo.patternsFound.Add(1)
	}

	return refund
}

// Reset clears all optimizer state.
func (oo *OpcodeOptimizer) Reset() {
	oo.mu.Lock()
	oo.windows = make(map[string]*opcodeWindow)
	oo.mu.Unlock()
	oo.redundantOps.Store(0)
	oo.gasRefunded.Store(0)
	oo.patternsFound.Store(0)
}

// OptimizerStats holds optimizer statistics for RPC reporting.
type OptimizerStats struct {
	Enabled       bool   `json:"enabled"`
	RedundantOps  uint64 `json:"redundantOps"`
	GasRefunded   uint64 `json:"gasRefunded"`
	PatternsFound uint64 `json:"patternsFound"`
}

// Stats returns current optimizer statistics.
func (oo *OpcodeOptimizer) Stats() OptimizerStats {
	return OptimizerStats{
		Enabled:       oo.enabled.Load(),
		RedundantOps:  oo.redundantOps.Load(),
		GasRefunded:   oo.gasRefunded.Load(),
		PatternsFound: oo.patternsFound.Load(),
	}
}

// AutoTuner adjusts adaptive gas percentages based on real network data.
// It runs periodically and analyzes profiling data to find optimal values.
type AutoTuner struct {
	enabled       atomic.Bool
	lastBlock     atomic.Uint64
	tuneInterval  uint64 // tune every N blocks
	minDiscount   uint64
	maxDiscount   uint64
	minPenalty    uint64
	maxPenalty    uint64
}

var GlobalAutoTuner = &AutoTuner{
	tuneInterval: 100, // tune every 100 blocks
	minDiscount:  5,
	maxDiscount:  40,
	minPenalty:   5,
	maxPenalty:   25,
}

func init() {
	GlobalAutoTuner.enabled.Store(false)
}

// SetEnabled enables or disables auto-tuning.
func (at *AutoTuner) SetEnabled(v bool) {
	at.enabled.Store(v)
}

// IsEnabled returns whether auto-tuning is active.
func (at *AutoTuner) IsEnabled() bool {
	return at.enabled.Load()
}

// MaybeTune checks if it's time to auto-tune and adjusts percentages.
// Should be called once per block.
//
// SAFETY NOTE (v1.1.0): Auto-tuning of DiscountPercent/PenaltyPercent is
// DISABLED because it depends on GlobalProfiler data which differs across
// nodes (each node may have processed different transactions in different
// order). Changing consensus-critical gas parameters from non-deterministic
// data causes state root mismatches. The auto-tuner now only logs metrics.
func (at *AutoTuner) MaybeTune(blockNum uint64) {
	if !at.enabled.Load() {
		return
	}

	lastTuned := at.lastBlock.Load()
	if blockNum-lastTuned < at.tuneInterval {
		return
	}
	at.lastBlock.Store(blockNum)

	// Analyze current profiling data (for metrics/logging only)
	totalOps := GlobalProfiler.TotalOps()
	if totalOps < 1000 {
		return // not enough data
	}

	// Get snapshot of opcode usage
	stats := GlobalProfiler.Snapshot()
	if len(stats) == 0 {
		return
	}

	// Calculate network-wide pure ratio (informational only)
	var pureOps, totalCounted uint64
	for _, s := range stats {
		op := opcodeFromString(s.Opcode)
		if op != 0xFF {
			totalCounted += s.Count
			if isPureOpcode(op) {
				pureOps += s.Count
			}
		}
	}

	// DO NOT modify GlobalAdaptiveGas.DiscountPercent or PenaltyPercent
	// from runtime data — this breaks consensus determinism.
	// Gas adjustments are now handled by StaticClassifier using bytecode analysis.
	_ = pureOps
	_ = totalCounted
}

// AutoTunerStats holds auto-tuner status for RPC reporting.
type AutoTunerStats struct {
	Enabled        bool   `json:"enabled"`
	TuneInterval   uint64 `json:"tuneInterval"`
	LastTunedBlock uint64 `json:"lastTunedBlock"`
	CurrentDiscount uint64 `json:"currentDiscount"`
	CurrentPenalty  uint64 `json:"currentPenalty"`
	MinDiscount    uint64 `json:"minDiscount"`
	MaxDiscount    uint64 `json:"maxDiscount"`
	MinPenalty     uint64 `json:"minPenalty"`
	MaxPenalty     uint64 `json:"maxPenalty"`
}

// Stats returns current auto-tuner status.
func (at *AutoTuner) Stats() AutoTunerStats {
	return AutoTunerStats{
		Enabled:         at.enabled.Load(),
		TuneInterval:    at.tuneInterval,
		LastTunedBlock:  at.lastBlock.Load(),
		CurrentDiscount: GlobalAdaptiveGas.DiscountPercent,
		CurrentPenalty:  GlobalAdaptiveGas.PenaltyPercent,
		MinDiscount:     at.minDiscount,
		MaxDiscount:     at.maxDiscount,
		MinPenalty:      at.minPenalty,
		MaxPenalty:      at.maxPenalty,
	}
}

// opcodeFromString converts an opcode name back to OpCode.
// Returns 0xFF if not found.
func opcodeFromString(name string) OpCode {
	for i := 0; i < 256; i++ {
		if OpCode(i).String() == name {
			return OpCode(i)
		}
	}
	return 0xFF
}
