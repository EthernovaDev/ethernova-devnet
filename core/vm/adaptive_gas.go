package vm

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

// AdaptiveGasConfig controls the LEGACY v1 adaptive-gas pattern tracker.
//
// IMPORTANT — NOT CONSENSUS RELEVANT.
// The real consensus path is Adaptive Gas V2 (ApplyAdaptiveGasV2 in
// adaptive_gas_v2.go), gated exclusively on ethernova.AdaptiveGasV2ForkBlock.
// It does NOT consult this struct at all. The fields here drive the legacy
// PatternTracker / StaticClassifier used only for RPC reporting and the
// opcode profiler — every gas-returning helper that reads Enabled is
// reporting-only.
//
// The Enabled flag stays atomic.Bool only to protect concurrent RPC
// readers. The RPC toggles (AdaptiveGasToggle, AdaptiveGasSetDiscount,
// AdaptiveGasSetPenalty) are no-ops by design. Do NOT reintroduce a
// mutation path here — if the v2 consensus path ever consults this flag
// again, divergent Enabled values across nodes would immediately split
// consensus.
type AdaptiveGasConfig struct {
	Enabled         atomic.Bool
	DiscountPercent uint64 // legacy monitoring only; not consensus-visible
	PenaltyPercent  uint64 // legacy monitoring only; not consensus-visible
}

var GlobalAdaptiveGas = &AdaptiveGasConfig{}

func init() {
	// Enable the monitoring tracker at startup so RPC reports real data.
	// This flag has no effect on consensus — see struct comment above.
	GlobalAdaptiveGas.Enabled.Store(true)
	GlobalAdaptiveGas.DiscountPercent = 25
	GlobalAdaptiveGas.PenaltyPercent = 10
}

// ============================================================================
// STATIC BYTECODE CLASSIFICATION (deterministic, consensus-safe)
// ============================================================================

// ContractClassification holds the deterministic classification result
// derived purely from static bytecode analysis. This MUST be identical
// across all nodes because it depends only on the deployed bytecode.
type ContractClassification struct {
	PureScore       uint64 // 0-100: percentage of weighted pure operations
	StorageOps      uint64 // count of SSTORE + SLOAD opcodes in bytecode
	ExternalCallOps uint64 // count of CALL/DELEGATECALL/STATICCALL/CALLCODE
	LogOps          uint64 // count of LOG0-LOG4
	CreateOps       uint64 // count of CREATE/CREATE2
	TotalOpcodes    uint64 // total opcodes in bytecode
	Category        ContractCategory
	GasAdjustment   int64 // negative = discount, positive = penalty (in percent)
}

// ContractCategory defines the classification tier.
type ContractCategory uint8

const (
	CategoryPure         ContractCategory = 0 // pure arithmetic/memory only
	CategoryLightState   ContractCategory = 1 // mostly pure, minimal state access
	CategoryMixed        ContractCategory = 2 // moderate state access
	CategoryStorageHeavy ContractCategory = 3 // heavy state mutation
)

func (c ContractCategory) String() string {
	switch c {
	case CategoryPure:
		return "pure"
	case CategoryLightState:
		return "light_state"
	case CategoryMixed:
		return "mixed"
	case CategoryStorageHeavy:
		return "storage_heavy"
	default:
		return "unknown"
	}
}

// StaticClassifier performs deterministic bytecode-level classification.
// Results are cached per contract address and never change for the same bytecode.
type StaticClassifier struct {
	mu              sync.RWMutex
	classifications map[common.Address]*ContractClassification
}

var GlobalStaticClassifier = &StaticClassifier{
	classifications: make(map[common.Address]*ContractClassification),
}

// Classify performs static bytecode analysis and returns a deterministic
// classification. This function is the ONLY source of truth for gas adjustment.
// It analyzes raw bytecode (no runtime state dependency).
func (sc *StaticClassifier) Classify(addr common.Address, code []byte) *ContractClassification {
	// Check cache first
	sc.mu.RLock()
	if existing, ok := sc.classifications[addr]; ok {
		sc.mu.RUnlock()
		return existing
	}
	sc.mu.RUnlock()

	// Perform static analysis on bytecode
	classification := classifyBytecode(code)

	// Cache the result
	sc.mu.Lock()
	// Double-check after acquiring write lock
	if existing, ok := sc.classifications[addr]; ok {
		sc.mu.Unlock()
		return existing
	}
	sc.classifications[addr] = classification
	sc.mu.Unlock()

	log.Debug("[AdaptiveGas] classified contract",
		"contract", addr.Hex(),
		"pureScore", classification.PureScore,
		"storageOps", classification.StorageOps,
		"callOps", classification.ExternalCallOps,
		"logOps", classification.LogOps,
		"category", classification.Category.String(),
		"gasAdjustment", fmt.Sprintf("%+d%%", classification.GasAdjustment),
	)

	return classification
}

// classifyBytecode performs deterministic static analysis of EVM bytecode.
// It uses WEIGHTED scoring: state-mutating opcodes have higher negative weight.
//
// Weights rationale:
//   - SSTORE: weight 10 (expensive, mutates persistent state)
//   - SLOAD:  weight 3  (cheaper but accesses external state, not pure)
//   - CALL/DELEGATECALL: weight 8 (external interaction, potential reentrancy)
//   - STATICCALL: weight 2 (read-only external, but still cross-contract)
//   - LOG*:   weight 2  (event emission, state-adjacent)
//   - CREATE*: weight 15 (deploy new contract)
//   - Pure opcodes: weight 1 (baseline)
func classifyBytecode(code []byte) *ContractClassification {
	var (
		totalOpcodes    uint64
		storageOps      uint64 // SLOAD + SSTORE
		externalCallOps uint64 // CALL, DELEGATECALL, STATICCALL, CALLCODE
		logOps          uint64 // LOG0-LOG4
		createOps       uint64 // CREATE, CREATE2

		weightedPure  uint64
		weightedTotal uint64
	)

	for i := 0; i < len(code); i++ {
		op := OpCode(code[i])
		totalOpcodes++

		weight := opcodeWeight(op)
		weightedTotal += weight

		if isPureOpcode(op) {
			weightedPure += weight
		} else {
			// Track specific non-pure categories
			switch op {
			case SSTORE, SLOAD:
				storageOps++
			case CALL, DELEGATECALL, STATICCALL, CALLCODE:
				externalCallOps++
			case LOG0, LOG1, LOG2, LOG3, LOG4:
				logOps++
			case CREATE, CREATE2:
				createOps++
			}
		}

		// Skip PUSH data bytes (critical: must not count data as opcodes)
		if op >= PUSH1 && op <= PUSH32 {
			i += int(op - PUSH1 + 1)
		}
	}

	// Calculate weighted pure score (0-100)
	var pureScore uint64
	if weightedTotal > 0 {
		pureScore = (weightedPure * 100) / weightedTotal
	}

	// Determine category and gas adjustment
	category, adjustment := determineCategory(pureScore, storageOps, externalCallOps, createOps, totalOpcodes)

	return &ContractClassification{
		PureScore:       pureScore,
		StorageOps:      storageOps,
		ExternalCallOps: externalCallOps,
		LogOps:          logOps,
		CreateOps:       createOps,
		TotalOpcodes:    totalOpcodes,
		Category:        category,
		GasAdjustment:   adjustment,
	}
}

// determineCategory assigns a category and gas adjustment based on scores.
// This uses a tiered system with hard thresholds for determinism.
//
// Thresholds:
//   Pure:          pureScore >= 90 AND storageOps == 0 AND callOps == 0
//   Light state:   pureScore >= 75 AND storageOps/total < 5%
//   Mixed:         pureScore >= 40
//   Storage heavy: pureScore < 40 OR storageOps/total >= 10%
func determineCategory(pureScore, storageOps, callOps, createOps, totalOpcodes uint64) (ContractCategory, int64) {
	if totalOpcodes == 0 {
		return CategoryPure, 0
	}

	storageRatio := (storageOps * 100) / totalOpcodes
	hasCreate := createOps > 0

	// Category: PURE — no state access at all
	if pureScore >= 90 && storageOps == 0 && callOps == 0 && !hasCreate {
		// Full discount scaled by pureScore
		// 90 → 22%, 95 → 23%, 100 → 25%
		discount := int64(GlobalAdaptiveGas.DiscountPercent) * int64(pureScore) / 100
		return CategoryPure, -discount
	}

	// Category: LIGHT STATE — mostly pure with minimal state reads
	if pureScore >= 75 && storageRatio < 5 && !hasCreate {
		// Partial discount: 5-15%
		discount := int64(GlobalAdaptiveGas.DiscountPercent) * int64(pureScore-50) / 100
		if discount < 5 {
			discount = 5
		}
		return CategoryLightState, -discount
	}

	// Category: STORAGE HEAVY — lots of state mutation
	if pureScore < 40 || storageRatio >= 10 || hasCreate {
		// Full penalty
		return CategoryStorageHeavy, int64(GlobalAdaptiveGas.PenaltyPercent)
	}

	// Category: MIXED — moderate state access, no adjustment
	return CategoryMixed, 0
}

// GetClassification returns the cached classification for a contract.
func (sc *StaticClassifier) GetClassification(addr common.Address) *ContractClassification {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.classifications[addr]
}

// GetGasAdjustment returns the gas adjustment for a contract.
// Returns 0 if adaptive gas is disabled or contract is not classified.
// Negative = discount, Positive = penalty.
func (sc *StaticClassifier) GetGasAdjustment(addr common.Address) int64 {
	if !GlobalAdaptiveGas.Enabled.Load() {
		return 0
	}

	sc.mu.RLock()
	c, ok := sc.classifications[addr]
	sc.mu.RUnlock()

	if !ok {
		return 0
	}

	return c.GasAdjustment
}

// ApplyGasAdjustment adjusts gas cost based on contract classification.
// This is the ONLY function that should modify gas for adaptive pricing.
// It is deterministic: same bytecode → same classification → same adjustment.
func (sc *StaticClassifier) ApplyGasAdjustment(addr common.Address, baseCost uint64) uint64 {
	adj := sc.GetGasAdjustment(addr)
	if adj == 0 {
		return baseCost
	}

	if adj < 0 {
		// Discount: reduce gas
		discount := (baseCost * uint64(-adj)) / 100
		if discount >= baseCost {
			return 1 // never reduce to 0
		}
		return baseCost - discount
	}

	// Penalty: increase gas
	penalty := (baseCost * uint64(adj)) / 100
	return baseCost + penalty
}

// Reset clears all classification data.
func (sc *StaticClassifier) Reset() {
	sc.mu.Lock()
	sc.classifications = make(map[common.Address]*ContractClassification)
	sc.mu.Unlock()
}

// GetAllClassifications returns all classifications for RPC reporting.
func (sc *StaticClassifier) GetAllClassifications() []ClassificationStats {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	var stats []ClassificationStats
	for addr, c := range sc.classifications {
		stats = append(stats, ClassificationStats{
			Address:       addr.Hex(),
			PureScore:     c.PureScore,
			StorageOps:    c.StorageOps,
			CallOps:       c.ExternalCallOps,
			LogOps:        c.LogOps,
			CreateOps:     c.CreateOps,
			TotalOpcodes:  c.TotalOpcodes,
			Category:      c.Category.String(),
			GasAdjustment: c.GasAdjustment,
		})
	}
	return stats
}

// ClassificationStats for RPC reporting.
type ClassificationStats struct {
	Address       string `json:"address"`
	PureScore     uint64 `json:"pureScore"`
	StorageOps    uint64 `json:"storageOps"`
	CallOps       uint64 `json:"callOps"`
	LogOps        uint64 `json:"logOps"`
	CreateOps     uint64 `json:"createOps"`
	TotalOpcodes  uint64 `json:"totalOpcodes"`
	Category      string `json:"category"`
	GasAdjustment int64  `json:"gasAdjustmentPercent"`
}

// ============================================================================
// LEGACY RUNTIME TRACKER (kept for backward-compatible RPC, NOT used for gas)
// ============================================================================

// ContractPattern tracks execution patterns for a contract address.
// NOTE: This is kept for RPC monitoring only. Gas adjustments use
// StaticClassifier which is deterministic.
type ContractPattern struct {
	mu           sync.Mutex
	callCount    uint64
	lastOpcodes  [8]OpCode // last 8 opcodes executed
	patternScore uint64    // higher = more predictable
	totalOps     uint64
	pureOps      uint64 // opcodes that don't touch storage/external state
}

// PatternTracker tracks patterns across all contracts.
// NOTE: For monitoring/RPC only. NOT used for consensus-critical gas calculation.
type PatternTracker struct {
	mu        sync.RWMutex
	contracts map[common.Address]*ContractPattern
}

var GlobalPatternTracker = &PatternTracker{
	contracts: make(map[common.Address]*ContractPattern),
}

// RecordOp records an opcode execution for pattern analysis (monitoring only).
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

	// Track if this is a "pure" opcode (using corrected classification)
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

// GetDiscount is DEPRECATED — delegates to StaticClassifier for backward compat.
func (pt *PatternTracker) GetDiscount(addr common.Address) uint64 {
	if !GlobalAdaptiveGas.Enabled.Load() {
		return 0
	}
	adj := GlobalStaticClassifier.GetGasAdjustment(addr)
	if adj < 0 {
		return uint64(-adj)
	}
	return 0
}

// GetPenalty is DEPRECATED — delegates to StaticClassifier for backward compat.
func (pt *PatternTracker) GetPenalty(addr common.Address) uint64 {
	if !GlobalAdaptiveGas.Enabled.Load() {
		return 0
	}
	adj := GlobalStaticClassifier.GetGasAdjustment(addr)
	if adj > 0 {
		return uint64(adj)
	}
	return 0
}

// PatternStats holds pattern analysis for RPC reporting.
type PatternStats struct {
	Address         string `json:"address"`
	CallCount       uint64 `json:"callCount"`
	TotalOps        uint64 `json:"totalOps"`
	PureOps         uint64 `json:"pureOps"`
	PurePercent     uint64 `json:"purePercent"`
	PatternScore    uint64 `json:"patternScore"`
	Discount        uint64 `json:"discountPercent"`
	Penalty         uint64 `json:"penaltyPercent"`
	StaticCategory  string `json:"staticCategory"`
	StaticPureScore uint64 `json:"staticPureScore"`
	GasAdjustment   int64  `json:"gasAdjustmentPercent"`
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

		// Get static classification data
		var staticCategory string
		var staticPureScore uint64
		var gasAdj int64
		if c := GlobalStaticClassifier.GetClassification(addr); c != nil {
			staticCategory = c.Category.String()
			staticPureScore = c.PureScore
			gasAdj = c.GasAdjustment
		}

		var discount, penalty uint64
		if gasAdj < 0 {
			discount = uint64(-gasAdj)
		} else if gasAdj > 0 {
			penalty = uint64(gasAdj)
		}

		stats = append(stats, PatternStats{
			Address:         addr.Hex(),
			CallCount:       cp.callCount,
			TotalOps:        cp.totalOps,
			PureOps:         cp.pureOps,
			PurePercent:     purePercent,
			PatternScore:    cp.patternScore,
			Discount:        discount,
			Penalty:         penalty,
			StaticCategory:  staticCategory,
			StaticPureScore: staticPureScore,
			GasAdjustment:   gasAdj,
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

// ============================================================================
// OPCODE CLASSIFICATION (FIXED)
// ============================================================================

// opcodeWeight returns the weight for scoring. State-mutating opcodes have
// higher weight so they disproportionately reduce the pure score.
func opcodeWeight(op OpCode) uint64 {
	switch op {
	case SSTORE:
		return 10 // heavy state mutation
	case SLOAD:
		return 3 // state read (NOT pure, but lighter than write)
	case CALL, DELEGATECALL, CALLCODE:
		return 8 // external call, potential reentrancy
	case STATICCALL:
		return 2 // read-only external call
	case CREATE, CREATE2:
		return 15 // deploy new contract
	case LOG0, LOG1, LOG2, LOG3, LOG4:
		return 2 // event emission
	case SELFDESTRUCT:
		return 20 // destructive
	default:
		return 1 // all pure opcodes
	}
}

// isPureOpcode returns true if the opcode does not access external/persistent state.
//
// CRITICAL FIX (v1.1.0):
//   BUG 1: SLOAD was previously classified as pure. SLOAD reads from persistent
//          storage (account trie), which is external state. A DEX contract doing
//          heavy SLOAD (reading reserves, balances, etc.) was scored ~98% pure.
//          SLOAD is now correctly classified as NON-pure.
//
//   BUG 2: Only PUSH0-PUSH4 were listed as pure. PUSH5 through PUSH32 were
//          missing, causing them to fall to default:false (non-pure). This
//          deflated pure scores for ALL contracts since PUSH opcodes are among
//          the most frequently occurring opcodes in any EVM bytecode.
//          All PUSH variants are now correctly classified as pure.
//
// Pure opcodes: arithmetic, comparison, bitwise, stack manipulation,
//               memory (volatile), control flow, calldata, code introspection,
//               hash, return/revert. These depend only on the current
//               execution frame's stack/memory and calldata.
//
// NON-pure opcodes: SLOAD, SSTORE, CALL, DELEGATECALL, STATICCALL,
//                   CALLCODE, CREATE, CREATE2, LOG*, SELFDESTRUCT,
//                   BALANCE, EXTCODECOPY, EXTCODESIZE, EXTCODEHASH.
//                   These access persistent state or interact with other accounts.
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
	// Stack — ALL PUSH variants (FIXED: was missing PUSH5-PUSH32)
	case POP, PUSH0:
		return true
	case PUSH1, PUSH2, PUSH3, PUSH4, PUSH5, PUSH6, PUSH7, PUSH8:
		return true
	case PUSH9, PUSH10, PUSH11, PUSH12, PUSH13, PUSH14, PUSH15, PUSH16:
		return true
	case PUSH17, PUSH18, PUSH19, PUSH20, PUSH21, PUSH22, PUSH23, PUSH24:
		return true
	case PUSH25, PUSH26, PUSH27, PUSH28, PUSH29, PUSH30, PUSH31, PUSH32:
		return true
	// DUP
	case DUP1, DUP2, DUP3, DUP4, DUP5, DUP6, DUP7, DUP8:
		return true
	case DUP9, DUP10, DUP11, DUP12, DUP13, DUP14, DUP15, DUP16:
		return true
	// SWAP
	case SWAP1, SWAP2, SWAP3, SWAP4, SWAP5, SWAP6, SWAP7, SWAP8:
		return true
	case SWAP9, SWAP10, SWAP11, SWAP12, SWAP13, SWAP14, SWAP15, SWAP16:
		return true
	// Memory (volatile, per-execution frame, not persisted)
	case MLOAD, MSTORE, MSTORE8, MSIZE:
		return true
	// Control flow
	case JUMP, JUMPI, JUMPDEST, PC, GAS:
		return true
	// Calldata & code introspection (read-only, frame-local)
	case CALLDATALOAD, CALLDATASIZE, CALLDATACOPY:
		return true
	case CODESIZE, CODECOPY:
		return true
	case RETURNDATASIZE, RETURNDATACOPY:
		return true
	// Hash
	case KECCAK256:
		return true
	// Return / halt
	case RETURN, REVERT, STOP:
		return true
	// Transaction context (read-only, deterministic within a tx)
	case CALLVALUE, CALLER, ADDRESS, ORIGIN, GASPRICE:
		return true
	// Block context (read-only within a block)
	case BLOCKHASH, COINBASE, TIMESTAMP, NUMBER, DIFFICULTY, GASLIMIT, CHAINID, BASEFEE:
		return true

	default:
		return false
	}
}
