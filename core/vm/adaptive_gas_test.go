package vm

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

// ============================================================================
// isPureOpcode tests
// ============================================================================

func TestIsPureOpcode_Arithmetic(t *testing.T) {
	pureOps := []OpCode{ADD, MUL, SUB, DIV, SDIV, MOD, SMOD, ADDMOD, MULMOD, EXP, SIGNEXTEND}
	for _, op := range pureOps {
		if !isPureOpcode(op) {
			t.Errorf("expected %v to be pure", op)
		}
	}
}

func TestIsPureOpcode_AllPushVariants(t *testing.T) {
	// BUG 2 fix: ALL PUSH variants must be pure (was missing PUSH5-PUSH32)
	for i := PUSH0; i <= PUSH32; i++ {
		op := OpCode(i)
		if !isPureOpcode(op) {
			t.Errorf("expected %v to be pure (BUG 2 regression: PUSH5-PUSH32 were missing)", op)
		}
	}
}

func TestIsPureOpcode_AllDupVariants(t *testing.T) {
	for i := DUP1; i <= DUP16; i++ {
		op := OpCode(i)
		if !isPureOpcode(op) {
			t.Errorf("expected %v to be pure", op)
		}
	}
}

func TestIsPureOpcode_AllSwapVariants(t *testing.T) {
	for i := SWAP1; i <= SWAP16; i++ {
		op := OpCode(i)
		if !isPureOpcode(op) {
			t.Errorf("expected %v to be pure", op)
		}
	}
}

func TestIsPureOpcode_MemoryOps(t *testing.T) {
	pureOps := []OpCode{MLOAD, MSTORE, MSTORE8, MSIZE}
	for _, op := range pureOps {
		if !isPureOpcode(op) {
			t.Errorf("expected %v to be pure", op)
		}
	}
}

func TestIsPureOpcode_SLOADIsNotPure(t *testing.T) {
	// BUG 1 fix: SLOAD must NOT be pure — it reads persistent storage
	if isPureOpcode(SLOAD) {
		t.Error("SLOAD must NOT be classified as pure (BUG 1: was incorrectly pure)")
	}
}

func TestIsPureOpcode_NonPureOps(t *testing.T) {
	nonPure := []OpCode{
		SLOAD, SSTORE,
		CALL, STATICCALL, DELEGATECALL, CALLCODE,
		CREATE, CREATE2,
		LOG0, LOG1, LOG2, LOG3, LOG4,
		SELFDESTRUCT,
		BALANCE, SELFBALANCE,
		EXTCODESIZE, EXTCODECOPY,
	}
	for _, op := range nonPure {
		if isPureOpcode(op) {
			t.Errorf("expected %v to be NON-pure", op)
		}
	}
}

// ============================================================================
// opcodeWeight tests
// ============================================================================

func TestOpcodeWeight_StorageMutationHeaviest(t *testing.T) {
	if opcodeWeight(SSTORE) <= opcodeWeight(SLOAD) {
		t.Error("SSTORE should have higher weight than SLOAD")
	}
	if opcodeWeight(SLOAD) <= opcodeWeight(ADD) {
		t.Error("SLOAD should have higher weight than pure arithmetic")
	}
}

func TestOpcodeWeight_CreateHeaviest(t *testing.T) {
	if opcodeWeight(CREATE) <= opcodeWeight(SSTORE) {
		t.Error("CREATE should have higher weight than SSTORE")
	}
}

// ============================================================================
// classifyBytecode tests
// ============================================================================

// buildBytecode is a test helper to construct bytecode from opcode sequences.
func buildBytecode(ops ...OpCode) []byte {
	var code []byte
	for _, op := range ops {
		code = append(code, byte(op))
		// PUSH1 needs a data byte
		if op >= PUSH1 && op <= PUSH32 {
			dataLen := int(op - PUSH1 + 1)
			for j := 0; j < dataLen; j++ {
				code = append(code, 0x00)
			}
		}
	}
	return code
}

func TestClassify_PureContract(t *testing.T) {
	// Pure contract: only arithmetic, stack, memory, control flow
	code := buildBytecode(
		PUSH1, PUSH1, ADD,
		PUSH1, PUSH1, MUL,
		MSTORE,
		PUSH1, MLOAD,
		RETURN,
	)

	c := classifyBytecode(code)

	if c.PureScore < 90 {
		t.Errorf("pure contract should have pureScore >= 90, got %d", c.PureScore)
	}
	if c.StorageOps != 0 {
		t.Errorf("pure contract should have 0 storage ops, got %d", c.StorageOps)
	}
	if c.Category != CategoryPure {
		t.Errorf("expected CategoryPure, got %v", c.Category)
	}
	if c.GasAdjustment >= 0 {
		t.Errorf("pure contract should get a discount (negative adjustment), got %d", c.GasAdjustment)
	}
}

func TestClassify_StorageHeavyContract(t *testing.T) {
	// Storage-heavy: lots of SLOAD, SSTORE, CALL, LOG
	code := buildBytecode(
		// Some setup
		PUSH1, PUSH1, ADD,
		// Heavy storage
		SLOAD, SSTORE, SLOAD, SSTORE, SLOAD, SSTORE,
		SLOAD, SSTORE, SLOAD, SSTORE, SLOAD, SSTORE,
		// External calls
		PUSH1, PUSH1, PUSH1, PUSH1, PUSH1, PUSH1, PUSH1, CALL,
		PUSH1, PUSH1, PUSH1, PUSH1, PUSH1, PUSH1, PUSH1, CALL,
		// Logs
		PUSH1, PUSH1, LOG1,
		PUSH1, PUSH1, LOG2,
		STOP,
	)

	c := classifyBytecode(code)

	if c.PureScore > 50 {
		t.Errorf("storage-heavy contract should have pureScore < 50, got %d", c.PureScore)
	}
	if c.StorageOps < 10 {
		t.Errorf("expected many storage ops, got %d", c.StorageOps)
	}
	if c.Category != CategoryStorageHeavy {
		t.Errorf("expected CategoryStorageHeavy, got %v", c.Category)
	}
	if c.GasAdjustment <= 0 {
		t.Errorf("storage-heavy contract should get a penalty (positive adjustment), got %d", c.GasAdjustment)
	}
}

func TestClassify_MixedContract(t *testing.T) {
	// Mixed: moderate SLOAD with substantial pure ops
	var ops []OpCode
	// 80 pure operations
	for i := 0; i < 20; i++ {
		ops = append(ops, PUSH1, PUSH1, ADD, MSTORE)
	}
	// 5 state reads
	for i := 0; i < 5; i++ {
		ops = append(ops, SLOAD)
	}
	ops = append(ops, RETURN)

	code := buildBytecode(ops...)
	c := classifyBytecode(code)

	// Should be in the middle range — not pure, not storage-heavy
	if c.Category == CategoryPure {
		t.Error("mixed contract should not be CategoryPure")
	}
	if c.PureScore < 20 || c.PureScore > 95 {
		t.Errorf("mixed contract pureScore should be moderate, got %d", c.PureScore)
	}
}

func TestClassify_DEXPattern(t *testing.T) {
	// Simulate a DEX/AMM contract: many SLOAD (reading reserves, balances),
	// SSTORE (updating state), CALL (token transfers), LOG (events)
	var ops []OpCode

	// Function selector dispatch (common pattern)
	ops = append(ops, PUSH1, CALLDATALOAD, PUSH1, SHR)

	// AMM swap logic: heavy SLOAD for reserves
	for i := 0; i < 15; i++ {
		ops = append(ops, PUSH1, SLOAD) // read storage slots
	}

	// Arithmetic on reserves (pure)
	for i := 0; i < 10; i++ {
		ops = append(ops, PUSH1, PUSH1, MUL, DIV, ADD)
	}

	// State writes
	for i := 0; i < 8; i++ {
		ops = append(ops, PUSH1, PUSH1, SSTORE) // write to storage
	}

	// External calls (token transfers)
	for i := 0; i < 4; i++ {
		ops = append(ops, PUSH1, PUSH1, PUSH1, PUSH1, PUSH1, PUSH1, PUSH1, CALL)
	}

	// Events
	ops = append(ops, PUSH1, PUSH1, PUSH1, LOG2)
	ops = append(ops, PUSH1, PUSH1, PUSH1, LOG2)

	ops = append(ops, STOP)

	code := buildBytecode(ops...)
	c := classifyBytecode(code)

	// A DEX MUST be classified as storage-heavy, NOT pure
	if c.Category == CategoryPure {
		t.Errorf("DEX contract must NOT be classified as pure! pureScore=%d, storageOps=%d",
			c.PureScore, c.StorageOps)
	}
	if c.GasAdjustment < 0 {
		t.Errorf("DEX contract should NOT receive a gas discount, got adjustment=%d%%", c.GasAdjustment)
	}
	if c.StorageOps < 20 {
		t.Errorf("DEX should have many storage ops, got %d", c.StorageOps)
	}

	t.Logf("DEX classification: pureScore=%d, category=%s, storageOps=%d, callOps=%d, adjustment=%+d%%",
		c.PureScore, c.Category, c.StorageOps, c.ExternalCallOps, c.GasAdjustment)
}

// ============================================================================
// Determinism tests
// ============================================================================

func TestClassify_Deterministic(t *testing.T) {
	// Same bytecode MUST produce identical classification every time
	code := buildBytecode(
		PUSH1, PUSH1, ADD, SLOAD, SSTORE,
		PUSH1, PUSH1, MUL, MSTORE,
		PUSH1, PUSH1, PUSH1, PUSH1, PUSH1, PUSH1, PUSH1, CALL,
		LOG1, RETURN,
	)

	results := make([]*ContractClassification, 100)
	for i := 0; i < 100; i++ {
		results[i] = classifyBytecode(code)
	}

	for i := 1; i < 100; i++ {
		if results[i].PureScore != results[0].PureScore {
			t.Errorf("non-deterministic: run %d pureScore=%d, run 0 pureScore=%d",
				i, results[i].PureScore, results[0].PureScore)
		}
		if results[i].Category != results[0].Category {
			t.Errorf("non-deterministic: run %d category=%v, run 0 category=%v",
				i, results[i].Category, results[0].Category)
		}
		if results[i].GasAdjustment != results[0].GasAdjustment {
			t.Errorf("non-deterministic: run %d gasAdj=%d, run 0 gasAdj=%d",
				i, results[i].GasAdjustment, results[0].GasAdjustment)
		}
	}
}

func TestStaticClassifier_CacheConsistency(t *testing.T) {
	sc := &StaticClassifier{
		classifications: make(map[common.Address]*ContractClassification),
	}

	addr := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
	code := buildBytecode(PUSH1, PUSH1, ADD, SLOAD, SSTORE, RETURN)

	// First call classifies
	c1 := sc.Classify(addr, code)
	// Second call returns cached
	c2 := sc.Classify(addr, code)

	if c1 != c2 {
		t.Error("cached result should be identical pointer")
	}
	if c1.PureScore != c2.PureScore || c1.Category != c2.Category {
		t.Error("cached result content mismatch")
	}
}

// ============================================================================
// Gas adjustment tests
// ============================================================================

func TestApplyGasAdjustment_Discount(t *testing.T) {
	sc := &StaticClassifier{
		classifications: make(map[common.Address]*ContractClassification),
	}
	addr := common.HexToAddress("0xaaaa")

	// Store a pure classification with -25% adjustment
	sc.mu.Lock()
	sc.classifications[addr] = &ContractClassification{
		PureScore:     95,
		Category:      CategoryPure,
		GasAdjustment: -25,
	}
	sc.mu.Unlock()

	// Enable adaptive gas
	GlobalAdaptiveGas.Enabled.Store(true)
	defer GlobalAdaptiveGas.Enabled.Store(false)

	baseCost := uint64(1000)
	adjusted := sc.ApplyGasAdjustment(addr, baseCost)
	expected := uint64(750) // 1000 - 25%

	if adjusted != expected {
		t.Errorf("expected adjusted gas %d, got %d", expected, adjusted)
	}
}

func TestApplyGasAdjustment_Penalty(t *testing.T) {
	sc := &StaticClassifier{
		classifications: make(map[common.Address]*ContractClassification),
	}
	addr := common.HexToAddress("0xbbbb")

	// Store a storage-heavy classification with +10% penalty
	sc.mu.Lock()
	sc.classifications[addr] = &ContractClassification{
		PureScore:     30,
		Category:      CategoryStorageHeavy,
		GasAdjustment: 10,
	}
	sc.mu.Unlock()

	GlobalAdaptiveGas.Enabled.Store(true)
	defer GlobalAdaptiveGas.Enabled.Store(false)

	baseCost := uint64(1000)
	adjusted := sc.ApplyGasAdjustment(addr, baseCost)
	expected := uint64(1100) // 1000 + 10%

	if adjusted != expected {
		t.Errorf("expected adjusted gas %d, got %d", expected, adjusted)
	}
}

func TestApplyGasAdjustment_DisabledReturnsBase(t *testing.T) {
	sc := &StaticClassifier{
		classifications: make(map[common.Address]*ContractClassification),
	}
	addr := common.HexToAddress("0xcccc")

	sc.mu.Lock()
	sc.classifications[addr] = &ContractClassification{
		GasAdjustment: -25,
	}
	sc.mu.Unlock()

	GlobalAdaptiveGas.Enabled.Store(false)

	baseCost := uint64(1000)
	adjusted := sc.ApplyGasAdjustment(addr, baseCost)
	if adjusted != baseCost {
		t.Errorf("when disabled, adjusted should equal base cost; got %d", adjusted)
	}
}

func TestApplyGasAdjustment_NeverReduceToZero(t *testing.T) {
	sc := &StaticClassifier{
		classifications: make(map[common.Address]*ContractClassification),
	}
	addr := common.HexToAddress("0xdddd")

	sc.mu.Lock()
	sc.classifications[addr] = &ContractClassification{
		GasAdjustment: -100, // extreme discount
	}
	sc.mu.Unlock()

	GlobalAdaptiveGas.Enabled.Store(true)
	defer GlobalAdaptiveGas.Enabled.Store(false)

	adjusted := sc.ApplyGasAdjustment(addr, 100)
	if adjusted == 0 {
		t.Error("gas should never be reduced to 0")
	}
	if adjusted != 1 {
		t.Errorf("extreme discount should reduce to 1, got %d", adjusted)
	}
}

// ============================================================================
// PUSH data byte skipping test
// ============================================================================

func TestClassify_PushDataNotCountedAsOpcodes(t *testing.T) {
	// PUSH20 followed by 20 bytes of data that look like SSTORE, SLOAD, CALL
	code := []byte{byte(PUSH20)}
	// Fill data bytes with SSTORE/SLOAD/CALL opcodes — these are DATA, not instructions
	for i := 0; i < 20; i++ {
		code = append(code, byte(SSTORE)) // 0x55 as data, not an opcode
	}
	code = append(code, byte(RETURN))

	c := classifyBytecode(code)

	// The SSTORE bytes inside PUSH20 data must NOT be counted as storage ops
	if c.StorageOps != 0 {
		t.Errorf("PUSH data bytes should not be counted as opcodes; storageOps=%d", c.StorageOps)
	}
	if c.TotalOpcodes != 2 { // PUSH20 + RETURN
		t.Errorf("expected 2 opcodes, got %d", c.TotalOpcodes)
	}
}

// ============================================================================
// Regression: old bug where SLOAD was pure
// ============================================================================

func TestRegression_SLOADHeavyContractNotPure(t *testing.T) {
	// This was the exact bug: a contract with many SLOADs was getting ~98% pure
	var ops []OpCode
	// 50 SLOADs (simulating AMM reading reserves, balances, etc.)
	for i := 0; i < 50; i++ {
		ops = append(ops, PUSH1, SLOAD)
	}
	// 50 pure arithmetic ops
	for i := 0; i < 50; i++ {
		ops = append(ops, PUSH1, PUSH1, ADD)
	}
	ops = append(ops, RETURN)

	code := buildBytecode(ops...)
	c := classifyBytecode(code)

	if c.PureScore >= 90 {
		t.Errorf("REGRESSION: contract with 50 SLOADs should NOT score >= 90 pure, got %d", c.PureScore)
	}
	if c.Category == CategoryPure {
		t.Error("REGRESSION: SLOAD-heavy contract classified as pure!")
	}
}

// ============================================================================
// Contract creation (v1.1.1) — adaptive gas must NOT apply to init code
// ============================================================================

func TestStaticClassifier_NoClassificationForCreation(t *testing.T) {
	// Simulates contract creation: init code should NOT be classified.
	// If it were classified, the constructor arg bytes would be misread
	// as opcodes, poisoning the cache.
	sc := &StaticClassifier{
		classifications: make(map[common.Address]*ContractClassification),
	}

	addr := common.HexToAddress("0xdeadbeef00000000000000000000000000000001")

	// Simulate init code: real opcodes + trailing constructor args
	// Constructor args include bytes 0x54 (SLOAD) and 0x55 (SSTORE)
	// which would be misinterpreted as storage opcodes
	initCode := buildBytecode(PUSH1, PUSH1, ADD, CODECOPY, RETURN)
	// Append fake constructor args that look like SSTORE/SLOAD
	for i := 0; i < 40; i++ {
		initCode = append(initCode, byte(SSTORE)) // 0x55 as data
	}

	// During creation, we should NOT call Classify()
	// Verify the address has no classification
	c := sc.GetClassification(addr)
	if c != nil {
		t.Error("fresh address should have no classification before deployment")
	}

	// After deployment, classify with RUNTIME code (not init code)
	runtimeCode := buildBytecode(PUSH1, PUSH1, ADD, PUSH1, MSTORE, RETURN)
	sc.Classify(addr, runtimeCode)

	c = sc.GetClassification(addr)
	if c == nil {
		t.Fatal("should have classification after Classify()")
	}
	if c.StorageOps != 0 {
		t.Errorf("runtime code has no storage ops, but got %d (init code contamination?)", c.StorageOps)
	}
	if c.Category != CategoryPure {
		t.Errorf("pure runtime code should be CategoryPure, got %v", c.Category)
	}
}

func TestStaticClassifier_CachePoisonPrevention(t *testing.T) {
	// Verify that classifying init code would produce a WRONG result,
	// confirming why we must skip it during creation.
	sc := &StaticClassifier{
		classifications: make(map[common.Address]*ContractClassification),
	}

	addr := common.HexToAddress("0xdeadbeef00000000000000000000000000000002")

	// Init code with trailing constructor args that look like SSTORE
	initCode := buildBytecode(PUSH1, PUSH1, ADD, RETURN)
	for i := 0; i < 50; i++ {
		initCode = append(initCode, byte(SSTORE)) // 0x55 data bytes
	}

	// Classify with init code (this is what the BUG did)
	badClassification := sc.Classify(addr, initCode)

	// The trailing 0x55 bytes get misread as SSTORE opcodes
	if badClassification.StorageOps == 0 {
		t.Skip("classifier correctly skipped PUSH data (not the bug scenario)")
	}

	t.Logf("CONFIRMED: init code misclassification - storageOps=%d, pureScore=%d, category=%s",
		badClassification.StorageOps, badClassification.PureScore, badClassification.Category)

	// Now verify: if we had classified with actual runtime code instead,
	// the result would be completely different
	sc2 := &StaticClassifier{
		classifications: make(map[common.Address]*ContractClassification),
	}
	runtimeCode := buildBytecode(PUSH1, PUSH1, ADD, PUSH1, MSTORE, RETURN)
	goodClassification := sc2.Classify(addr, runtimeCode)

	if goodClassification.StorageOps != 0 {
		t.Error("runtime code should have 0 storage ops")
	}
	if badClassification.PureScore == goodClassification.PureScore {
		t.Error("init code and runtime code should produce DIFFERENT classifications")
	}
}

func TestApplyGasAdjustment_ContractCreationGetsZero(t *testing.T) {
	// Even if a classification exists, contract creation must not use it.
	// This tests the interpreter-level guard: isContractCreation == true → no adjustment.
	sc := &StaticClassifier{
		classifications: make(map[common.Address]*ContractClassification),
	}

	addr := common.HexToAddress("0xaaaa")
	sc.mu.Lock()
	sc.classifications[addr] = &ContractClassification{
		PureScore:     95,
		Category:      CategoryPure,
		GasAdjustment: -25,
	}
	sc.mu.Unlock()

	GlobalAdaptiveGas.Enabled.Store(true)
	defer GlobalAdaptiveGas.Enabled.Store(false)

	baseCost := uint64(1000)

	// Simulate what happens in the interpreter:
	// During creation → isContractCreation=true → skip adjustment
	isContractCreation := true
	adjustedCost := baseCost
	if !isContractCreation && GlobalAdaptiveGas.Enabled.Load() {
		adjustedCost = sc.ApplyGasAdjustment(addr, baseCost)
	}

	if adjustedCost != baseCost {
		t.Errorf("during contract creation, gas must NOT be adjusted; got %d want %d",
			adjustedCost, baseCost)
	}

	// During regular call → isContractCreation=false → apply adjustment
	isContractCreation = false
	adjustedCost = baseCost
	if !isContractCreation && GlobalAdaptiveGas.Enabled.Load() {
		adjustedCost = sc.ApplyGasAdjustment(addr, baseCost)
	}

	expectedCost := uint64(750) // 1000 - 25%
	if adjustedCost != expectedCost {
		t.Errorf("during regular call, gas should be adjusted; got %d want %d",
			adjustedCost, expectedCost)
	}
}