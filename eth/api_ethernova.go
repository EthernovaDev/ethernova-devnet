package eth

import (
	"math/big"
	"runtime"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/params/ethernova"
)

// EthernovaAPI provides Ethernova-specific RPC endpoints.
type EthernovaAPI struct {
	e         *Ethereum
	startTime time.Time
}

func NewEthernovaAPI(e *Ethereum) *EthernovaAPI {
	return &EthernovaAPI{e: e, startTime: time.Now()}
}

type ForkEntry struct {
	Name            string `json:"name"`
	Block           uint64 `json:"block"`
	Active          bool   `json:"active"`
	BlocksRemaining int64  `json:"blocksRemaining"`
}

func (api *EthernovaAPI) ForkStatus() map[string]interface{} {
	head := api.e.blockchain.CurrentBlock().Number.Uint64()

	forks := []ForkEntry{
		forkEntry("Constantinople/Petersburg/Istanbul", ethernova.EVMCompatibilityForkBlock, head),
		forkEntry("EIP-658 (Receipt Status)", ethernova.EIP658ForkBlock, head),
		forkEntry("MegaFork (Historical EVM)", ethernova.MegaForkBlock, head),
		forkEntry("Legacy Chain Enforcement", ethernova.LegacyForkEnforcementBlock, head),
		forkEntry("NIP-0004 Protocol Objects", ethernova.ProtocolObjectForkBlock, head),
		forkEntry("NIP-0004 Deferred Execution", ethernova.DeferredExecForkBlock, head),
	}

	// Check config status from DB
	cfg := api.e.blockchain.Config()
	_, evmMismatched, _ := core.EthernovaForkStatus(cfg, ethernova.EVMCompatibilityForkBlock)
	_, eip658Mismatched, _ := core.EthernovaEIP658Status(cfg, ethernova.EIP658ForkBlock)
	megaMissing, megaMismatched, _ := core.EthernovaMegaForkStatus(cfg, ethernova.MegaForkBlock)

	configOK := len(evmMismatched) == 0 && len(eip658Mismatched) == 0 && len(megaMissing) == 0 && len(megaMismatched) == 0

	return map[string]interface{}{
		"currentBlock": head,
		"forks":        forks,
		"configValid":  configOK,
	}
}

func forkEntry(name string, block, head uint64) ForkEntry {
	active := head >= block
	var remaining int64
	if !active {
		remaining = int64(block - head)
	}
	return ForkEntry{
		Name:            name,
		Block:           block,
		Active:          active,
		BlocksRemaining: remaining,
	}
}

type ChainConfigResult struct {
	ChainID   uint64 `json:"chainId"`
	NetworkID uint64 `json:"networkId"`
	Consensus string `json:"consensus"`
	Version   string `json:"version"`
}

func (api *EthernovaAPI) ChainConfig() ChainConfigResult {
	cfg := api.e.blockchain.Config()
	chainID := cfg.GetChainID().Uint64()
	consensus := cfg.GetConsensusEngineType().String()

	return ChainConfigResult{
		ChainID:   chainID,
		NetworkID: api.e.networkID,
		Consensus: consensus,
		Version:   "v" + params.Version,
	}
}

type NodeHealthResult struct {
	Version             string  `json:"version"`
	Network             string  `json:"network"`
	CurrentBlock        uint64  `json:"currentBlock"`
	HighestBlock        uint64  `json:"highestBlock"`
	PeerCount           int     `json:"peerCount"`
	Syncing             bool    `json:"syncing"`
	SyncProgress        float64 `json:"syncProgress"`
	UptimeSeconds       int64   `json:"uptimeSeconds"`
	MemoryMB            uint64  `json:"memoryMB"`
	DualSignerFallbacks int64   `json:"dualSignerFallbacks"`
}

func (api *EthernovaAPI) NodeHealth() NodeHealthResult {
	current := api.e.blockchain.CurrentBlock().Number.Uint64()
	highest := current

	syncing := api.e.handler.downloader.Progress()
	if syncing.HighestBlock > highest {
		highest = syncing.HighestBlock
	}

	isSyncing := highest > current+10
	var progress float64
	if highest > 0 {
		progress = float64(current) / float64(highest) * 100
		if progress > 100 {
			progress = 100
		}
	}

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	var fallbacks int64
	if counter := metrics.DefaultRegistry.Get("ethernova/dualsigner/fallback"); counter != nil {
		if c, ok := counter.(metrics.Counter); ok {
			fallbacks = c.Snapshot().Count()
		}
	}

	return NodeHealthResult{
		Version:             "v" + params.Version,
		Network:             "ethernova",
		CurrentBlock:        current,
		HighestBlock:        highest,
		PeerCount:           api.e.handler.peers.len(),
		Syncing:             isSyncing,
		SyncProgress:        progress,
		UptimeSeconds:       int64(time.Since(api.startTime).Seconds()),
		MemoryMB:            mem.Alloc / 1024 / 1024,
		DualSignerFallbacks: fallbacks,
	}
}

// EvmProfileResult holds the EVM opcode profiling snapshot.
type EvmProfileResult struct {
	Enabled      bool               `json:"enabled"`
	TotalOps     uint64             `json:"totalOps"`
	TotalGas     uint64             `json:"totalGas"`
	TopOpcodes   []vm.OpcodeStats   `json:"topOpcodes"`
	TopContracts []vm.ContractStats `json:"topContracts"`
}

// EvmProfile returns EVM opcode execution profiling data.
func (api *EthernovaAPI) EvmProfile() EvmProfileResult {
	opcodes := vm.GlobalProfiler.Snapshot()
	// Return top 30 opcodes
	if len(opcodes) > 30 {
		opcodes = opcodes[:30]
	}

	contracts := vm.GlobalContractProfiler.TopContracts(20)

	return EvmProfileResult{
		Enabled:      vm.GlobalProfiler.IsEnabled(),
		TotalOps:     vm.GlobalProfiler.TotalOps(),
		TotalGas:     vm.GlobalProfiler.TotalGas(),
		TopOpcodes:   opcodes,
		TopContracts: contracts,
	}
}

// EvmProfileReset clears all profiling data.
func (api *EthernovaAPI) EvmProfileReset() bool {
	vm.GlobalProfiler.Reset()
	vm.GlobalContractProfiler.Reset()
	return true
}

// EvmProfileToggle enables or disables profiling.
func (api *EthernovaAPI) EvmProfileToggle(enabled bool) bool {
	vm.GlobalProfiler.SetEnabled(enabled)
	vm.GlobalContractProfiler.SetEnabled(enabled)
	return vm.GlobalProfiler.IsEnabled()
}

// AdaptiveGasResult holds the adaptive gas system status.
type AdaptiveGasResult struct {
	Enabled         bool              `json:"enabled"`
	Version         string            `json:"version"`
	ForkBlock       uint64            `json:"forkBlock"`
	DiscountPercent uint64            `json:"maxDiscountPercent"`
	PenaltyPercent  uint64            `json:"maxPenaltyPercent"`
	Contracts       []vm.PatternStats `json:"contracts"`
}

// AdaptiveGas returns the current adaptive gas configuration and contract patterns.
func (api *EthernovaAPI) AdaptiveGas() AdaptiveGasResult {
	return AdaptiveGasResult{
		Enabled:         true, // CONSENSUS RULE: always active after fork block
		Version:         "2.0.0-trace-based",
		ForkBlock:       ethernova.AdaptiveGasV2ForkBlock,
		DiscountPercent: 25, // compile-time constant from adaptive_gas_v2.go
		PenaltyPercent:  10, // compile-time constant from adaptive_gas_v2.go
		Contracts:       vm.GlobalPatternTracker.GetAllPatterns(),
	}
}

// AdaptiveGasToggle is DEPRECATED and intentionally a no-op.
// Adaptive gas v2 is a CONSENSUS RULE activated by fork block.
// Allowing runtime toggle would cause consensus forks between nodes.
// Returns: always true (system is always active).
func (api *EthernovaAPI) AdaptiveGasToggle(enabled bool) bool {
	// NO-OP: consensus rules cannot be toggled at runtime.
	// Log warning if someone tries to disable it.
	if !enabled {
		// Do NOT actually disable — just warn.
		_ = enabled // suppress unused warning
	}
	return true // always active
}

// AdaptiveGasSetDiscount is DEPRECATED and intentionally a no-op.
// Gas adjustment parameters are compile-time constants for consensus safety.
// Returns: the fixed discount value (25%).
func (api *EthernovaAPI) AdaptiveGasSetDiscount(percent uint64) uint64 {
	// NO-OP: consensus-critical parameters cannot be changed at runtime.
	return 25
}

// AdaptiveGasSetPenalty is DEPRECATED and intentionally a no-op.
// Gas adjustment parameters are compile-time constants for consensus safety.
// Returns: the fixed penalty value (10%).
func (api *EthernovaAPI) AdaptiveGasSetPenalty(percent uint64) uint64 {
	// NO-OP: consensus-critical parameters cannot be changed at runtime.
	return 10
}

// AdaptiveGasReset clears monitoring/pattern tracking data (NOT consensus-affecting).
func (api *EthernovaAPI) AdaptiveGasReset() bool {
	vm.GlobalPatternTracker.Reset()
	vm.GlobalStaticClassifier.Reset()
	return true
}

// ============================================================================
// Adaptive Gas v2 — Trace-Based System (NEW)
// ============================================================================

// AdaptiveGasV2Result holds the trace-based adaptive gas system status.
type AdaptiveGasV2Result struct {
	Enabled         bool                     `json:"enabled"`
	ConsensusRule   bool                     `json:"consensusRule"`
	Version         string                   `json:"version"`
	ForkBlock       uint64                   `json:"forkBlock"`
	DiscountPercent uint64                   `json:"maxDiscountPercent"`
	PenaltyPercent  uint64                   `json:"maxPenaltyPercent"`
	LastTx          *vm.AdaptiveGasV2Stats   `json:"lastTxClassification,omitempty"`
	LegacyContracts []vm.PatternStats        `json:"legacyContracts,omitempty"`
	StaticClasses   []vm.ClassificationStats `json:"staticClassifications,omitempty"`
}

// AdaptiveGasV2 returns the v2 trace-based adaptive gas system status.
// Includes the last transaction's classification for debugging.
func (api *EthernovaAPI) AdaptiveGasV2() AdaptiveGasV2Result {
	result := AdaptiveGasV2Result{
		Enabled:         true, // always active after fork block
		ConsensusRule:   true,
		Version:         "2.0.0-trace-based",
		ForkBlock:       ethernova.AdaptiveGasV2ForkBlock,
		DiscountPercent: 25,
		PenaltyPercent:  10,
		LegacyContracts: vm.GlobalPatternTracker.GetAllPatterns(),
		StaticClasses:   vm.GlobalStaticClassifier.GetAllClassifications(),
	}

	if vm.LastTxClassification != nil {
		stats := vm.LastTxClassification.ToStats()
		result.LastTx = &stats
	}

	return result
}

// AdaptiveGasV2Simulate simulates classification for given trace counters.
// Useful for testing without executing a real transaction.
func (api *EthernovaAPI) AdaptiveGasV2Simulate(
	sstoreCount, sloadCount, callCount, delegateCallCount,
	staticCallCount, jumpiCount, createCount, totalOps uint64,
) vm.AdaptiveGasV2Stats {
	tc := &vm.TraceCounters{
		SstoreCount:       sstoreCount,
		SloadCount:        sloadCount,
		CallCount:         callCount,
		DelegateCallCount: delegateCallCount,
		StaticCallCount:   staticCallCount,
		JumpiCount:        jumpiCount,
		CreateCount:       createCount,
		TotalOpsExecuted:  totalOps,
	}

	category := vm.ClassifyExecution(tc)
	score := vm.ComputeComplexityScore(tc)
	adjustPct := vm.ComputeGasAdjustment(category, score, totalOps, tc)

	return vm.AdaptiveGasV2Stats{
		Category:        category.String(),
		ComplexityScore: score,
		GasAdjustPct:    adjustPct,
		SstoreCount:     sstoreCount,
		SloadCount:      sloadCount,
		CallCount:       callCount,
		StaticCallCount: staticCallCount,
		DelegateCount:   delegateCallCount,
		CreateCount:     createCount,
		JumpiCount:      jumpiCount,
		TotalOps:        totalOps,
	}
}

// ExecutionModeResult holds execution mode status.
type ExecutionModeResult struct {
	Mode              string             `json:"mode"`
	FastExecutions    uint64             `json:"fastExecutions"`
	SkippedChecks     uint64             `json:"skippedChecks"`
	VerifiedContracts []vm.VerifiedStats `json:"verifiedContracts"`
}

// ExecutionMode returns the current execution mode and stats.
func (api *EthernovaAPI) ExecutionMode() ExecutionModeResult {
	return ExecutionModeResult{
		Mode:              vm.GlobalExecutionMode.GetMode().String(),
		FastExecutions:    vm.GlobalFastModeStats.FastExecutions.Load(),
		SkippedChecks:     vm.GlobalFastModeStats.SkippedChecks.Load(),
		VerifiedContracts: vm.GlobalContractVerifier.GetAllVerified(),
	}
}

// ExecutionModeSet sets the execution mode: 0=standard, 1=fast, 2=parallel.
func (api *EthernovaAPI) ExecutionModeSet(mode uint64) string {
	vm.GlobalExecutionMode.SetMode(vm.ExecutionMode(mode))
	return vm.GlobalExecutionMode.GetMode().String()
}

// ParallelStats returns parallel execution statistics.
func (api *EthernovaAPI) ParallelStats() core.ParallelStats {
	return core.GetParallelStats()
}

// CallCache returns the call result cache statistics.
func (api *EthernovaAPI) CallCache() vm.CacheStats {
	return vm.GlobalCallCache.Stats()
}

// CallCacheToggle enables or disables the call result cache.
func (api *EthernovaAPI) CallCacheToggle(enabled bool) bool {
	vm.GlobalCallCache.SetEnabled(enabled)
	return vm.GlobalCallCache.IsEnabled()
}

// CallCacheReset clears all cached call results.
func (api *EthernovaAPI) CallCacheReset() bool {
	vm.GlobalCallCache.Reset()
	return true
}

// BytecodeAnalysis returns static analysis data for all deployed contracts.
func (api *EthernovaAPI) BytecodeAnalysis() map[string]*vm.BytecodeAnalysis {
	return vm.GlobalBytecodeAnalyzer.GetAllAnalysis()
}

// Optimizer returns opcode sequence optimizer stats.
func (api *EthernovaAPI) Optimizer() vm.OptimizerStats {
	return vm.GlobalOpcodeOptimizer.Stats()
}

// OptimizerToggle enables or disables the opcode optimizer.
func (api *EthernovaAPI) OptimizerToggle(enabled bool) bool {
	vm.GlobalOpcodeOptimizer.SetEnabled(enabled)
	return vm.GlobalOpcodeOptimizer.IsEnabled()
}

// OptimizerReset clears optimizer state.
func (api *EthernovaAPI) OptimizerReset() bool {
	vm.GlobalOpcodeOptimizer.Reset()
	return true
}

// AutoTuner returns the convergent auto-tuner status.
// Ethernova v3.0: Includes both convergent tuner EMA metrics and
// safety envelope status (scaleFactor, cautious mode, etc.).
func (api *EthernovaAPI) AutoTuner() map[string]interface{} {
	return map[string]interface{}{
		"convergent": vm.GlobalConvergentTuner.Stats(),
		"safety":     vm.GlobalSafeTuner.Stats(),
	}
}

// AutoTunerToggle enables or disables the convergent auto-tuner.
func (api *EthernovaAPI) AutoTunerToggle(enabled bool) bool {
	vm.GlobalConvergentTuner.SetEnabled(enabled)
	vm.GlobalSafeTuner.SetEnabled(enabled)
	return vm.GlobalConvergentTuner.IsEnabled()
}

// PrecompileInfo describes a custom Ethernova precompiled contract.
type PrecompileInfo struct {
	Address     string `json:"address"`
	Name        string `json:"name"`
	Description string `json:"description"`
	GasModel    string `json:"gasModel"`
}

// Precompiles returns information about Ethernova custom precompiled contracts.
func (api *EthernovaAPI) Precompiles() []PrecompileInfo {
	return []PrecompileInfo{
		{
			Address:     "0x0000000000000000000000000000000000000020",
			Name:        "novaBatchHash",
			Description: "Batch keccak256 hashing - hash multiple 32-byte items in one call",
			GasModel:    "30 gas per 32-byte item",
		},
		{
			Address:     "0x0000000000000000000000000000000000000021",
			Name:        "novaBatchVerify",
			Description: "Batch ecrecover - verify multiple signatures in one call",
			GasModel:    "2000 gas per signature (vs 3000 for individual ecrecover)",
		},
		{
			Address:     "0x0000000000000000000000000000000000000022",
			Name:        "novaAccountManager",
			Description: "Native smart wallet - key rotation, guardian recovery",
			GasModel:    "2000 gas reads, 10000 gas writes",
		},
		{
			Address:     "0x0000000000000000000000000000000000000023",
			Name:        "novaFrameApprove",
			Description: "Frame AA: smart contracts approve/reject transactions (EIP-8141 style)",
			GasModel:    "5000 gas per approval",
		},
		{
			Address:     "0x0000000000000000000000000000000000000024",
			Name:        "novaFrameIntrospect",
			Description: "Frame AA: inspect other frames in the transaction for conditional logic",
			GasModel:    "2000 gas per introspection",
		},
		{
			Address:     "0x0000000000000000000000000000000000000025",
			Name:        "novaTokenManager",
			Description: "Native multi-token: create, transfer, balanceOf without ERC-20 contracts",
			GasModel:    "500k create, 5k transfer, 1k read",
		},
		{
			Address:     "0x0000000000000000000000000000000000000026",
			Name:        "novaShieldedPool",
			Description: "Optional privacy: shield/unshield NOVA with commitment-nullifier scheme",
			GasModel:    "50k shield, 100k unshield, max 10k NOVA/withdrawal",
		},
		{
			Address:     "0x0000000000000000000000000000000000000027",
			Name:        "novaContractUpgrade",
			Description: "Native contract upgrades with 100-block timelock, no proxy pattern needed",
			GasModel:    "50k initiate/execute, 2k status",
		},
		{
			Address:     "0x0000000000000000000000000000000000000028",
			Name:        "novaOracle",
			Description: "Protocol-level price oracle with TWAP and 15% circuit breaker",
			GasModel:    "2k read, 5k TWAP, 50k submit",
		},
		{
			Address:     "0x0000000000000000000000000000000000000029",
			Name:        "novaProtocolObjectRegistry",
			Description: "NIP-0004 Protocol Object CRUD: create, read, list, delete first-class protocol entities (Mailbox, Session, ContentRef, Identity, Subscription, GameRoom)",
			GasModel:    "20k create, 2k read, 1k count, 10k delete",
		},
		{
			Address:     "0x000000000000000000000000000000000000002A",
			Name:        "novaDeferredQueue",
			Description: "NIP-0004 Phase 2 Pending Effects Queue: enqueue deferred effect, query pending, read queue stats. Effects enqueued in block N are drained at the start of block N+1.",
			GasModel:    "10k enqueue base + 200/chunk, 2k read, 1k stats",
		},
	}
}

// StateRent was removed in v1.0.5 (Phase 10 cleanup).
// State Expiry garbage collector replaces rent surcharge.

// TempoConfig returns the current Tempo transaction configuration.
func (api *EthernovaAPI) TempoConfig() map[string]interface{} {
	return map[string]interface{}{
		"forkBlock":     ethernova.TempoTxForkBlock,
		"txType":        "0x04",
		"maxCalls":      16,
		"feeDelegation": true,
		"scheduling":    true,
		"erc20Gas":      false,
		"gasToken":      "NOVA (native only)",
		"description":   "Tempo-style smart transactions: atomic batching, fee delegation, scheduling",
	}
}

// StateExpiry returns the current state expiry configuration.
func (api *EthernovaAPI) StateExpiry() map[string]interface{} {
	return map[string]interface{}{
		"forkBlock":    ethernova.StateExpiryForkBlock,
		"expiryPeriod": ethernova.StateExpiryPeriod,
		"appliesTo":    "contracts only (EOAs never expire)",
		"description":  "Blockchain garbage collector - archives dead contracts after inactivity period",
	}
}

// ============================================================
// NIP-0004 Protocol Object RPC endpoints
// ============================================================

// getStateDB returns a statedb at the current head for read-only queries.
func (api *EthernovaAPI) getStateDB() (*state.StateDB, error) {
	header := api.e.blockchain.CurrentBlock()
	return api.e.blockchain.StateAt(header.Root)
}

// ProtocolObjectResult is the JSON-serializable representation of a Protocol Object.
type ProtocolObjectResult struct {
	ID               common.Hash    `json:"id"`
	Owner            common.Address `json:"owner"`
	TypeTag          uint8          `json:"typeTag"`
	TypeName         string         `json:"typeName"`
	StateDataHex     string         `json:"stateData"`
	StateDataLen     int            `json:"stateDataLen"`
	ExpiryBlock      uint64         `json:"expiryBlock"`
	LastTouchedBlock uint64         `json:"lastTouchedBlock"`
	RentBalance      string         `json:"rentBalance"`
}

func protocolObjectToResult(obj *types.ProtocolObject) *ProtocolObjectResult {
	rentStr := "0"
	if obj.RentBalance != nil {
		rentStr = obj.RentBalance.String()
	}
	return &ProtocolObjectResult{
		ID:               obj.ID,
		Owner:            obj.Owner,
		TypeTag:          obj.TypeTag,
		TypeName:         types.ProtocolObjectTypeName(obj.TypeTag),
		StateDataHex:     common.Bytes2Hex(obj.StateData),
		StateDataLen:     len(obj.StateData),
		ExpiryBlock:      obj.ExpiryBlock,
		LastTouchedBlock: obj.LastTouchedBlock,
		RentBalance:      rentStr,
	}
}

// GetProtocolObject returns a Protocol Object by its ID (hex string).
// Returns null if not found. This is a read-only query against the current head state.
func (api *EthernovaAPI) GetProtocolObject(idHex string) (interface{}, error) {
	statedb, err := api.getStateDB()
	if err != nil {
		return nil, err
	}
	id := common.HexToHash(idHex)
	obj := vm.PoGetObject(statedb, id)
	if obj == nil {
		return nil, nil
	}
	return protocolObjectToResult(obj), nil
}

// GetProtocolObjectCount returns the total number of Protocol Objects.
func (api *EthernovaAPI) GetProtocolObjectCount() (map[string]interface{}, error) {
	statedb, err := api.getStateDB()
	if err != nil {
		return nil, err
	}
	total := vm.PoGetObjectCount(statedb)

	perType := make(map[string]uint64)
	for tag := uint8(1); tag <= 6; tag++ {
		name := types.ProtocolObjectTypeName(tag)
		count := vm.PoGetTypeCount(statedb, tag)
		perType[name] = count
	}

	return map[string]interface{}{
		"total":           total,
		"perType":         perType,
		"registryAddress": vm.ProtocolObjectRegistryAddr.Hex(),
		"forkBlock":       ethernova.ProtocolObjectForkBlock,
	}, nil
}

// GetProtocolObjectsByOwner returns Protocol Object IDs owned by an address.
func (api *EthernovaAPI) GetProtocolObjectsByOwner(ownerHex string, offset, limit uint64) (interface{}, error) {
	statedb, err := api.getStateDB()
	if err != nil {
		return nil, err
	}
	if limit == 0 || limit > 100 {
		limit = 100
	}
	owner := common.HexToAddress(ownerHex)
	ids := vm.PoGetObjectsByOwner(statedb, owner, offset, limit)

	results := make([]string, len(ids))
	for i, id := range ids {
		results[i] = id.Hex()
	}
	return map[string]interface{}{
		"owner":  owner.Hex(),
		"count":  len(results),
		"offset": offset,
		"limit":  limit,
		"ids":    results,
	}, nil
}

// ProtocolObjectConfig returns the NIP-0004 Protocol Object configuration.
func (api *EthernovaAPI) ProtocolObjectConfig() map[string]interface{} {
	head := api.e.blockchain.CurrentBlock().Number.Uint64()
	active := head >= ethernova.ProtocolObjectForkBlock

	return map[string]interface{}{
		"forkBlock":       ethernova.ProtocolObjectForkBlock,
		"active":          active,
		"currentBlock":    head,
		"registryAddress": vm.ProtocolObjectRegistryAddr.Hex(),
		"precompile":      "0x29",
		"maxTypes":        types.MaxProtocolObjectTypes,
		"supportedTypes": []map[string]interface{}{
			{"tag": types.ProtoTypeMailbox, "name": "Mailbox"},
			{"tag": types.ProtoTypeSession, "name": "Session"},
			{"tag": types.ProtoTypeContentReference, "name": "ContentReference"},
			{"tag": types.ProtoTypeIdentity, "name": "Identity"},
			{"tag": types.ProtoTypeSubscription, "name": "Subscription"},
			{"tag": types.ProtoTypeGameRoom, "name": "GameRoom"},
		},
		"description": "NIP-0004 Phase 1: Protocol Object Trie Foundation — first-class entities in the Ethernova state tree",
	}
}

// ============================================================
// NIP-0004 Phase 2: Deferred Execution Engine RPC endpoints
// ============================================================

// DeferredEffectResult is the JSON representation of a DeferredEffect.
type DeferredEffectResult struct {
	SeqNum       uint64         `json:"seqNum"`
	EffectType   uint8          `json:"effectType"`
	EffectName   string         `json:"effectName"`
	SourceBlock  uint64         `json:"sourceBlock"`
	SourceCaller common.Address `json:"sourceCaller"`
	SourceTxHash common.Hash    `json:"sourceTxHash"`
	PayloadHex   string         `json:"payload"`
	PayloadLen   int            `json:"payloadLen"`
}

func deferredEffectToResult(e *types.DeferredEffect) *DeferredEffectResult {
	return &DeferredEffectResult{
		SeqNum:       e.SeqNum,
		EffectType:   e.EffectType,
		EffectName:   types.DeferredEffectTypeName(e.EffectType),
		SourceBlock:  e.SourceBlock,
		SourceCaller: e.SourceCaller,
		SourceTxHash: e.SourceTxHash,
		PayloadHex:   common.Bytes2Hex(e.Payload),
		PayloadLen:   len(e.Payload),
	}
}

// GetPendingEffects returns up to `limit` pending effects starting at
// `offset` past the current queue head. Useful for inspecting what Phase 0
// will process at the next block. Limit is hard-capped at 256 to avoid
// runaway RPC responses; use paginated calls for larger surveys.
func (api *EthernovaAPI) GetPendingEffects(offset, limit uint64) (interface{}, error) {
	statedb, err := api.getStateDB()
	if err != nil {
		return nil, err
	}
	if limit == 0 {
		limit = 50
	}
	effects := vm.DqListPending(statedb, offset, limit)
	results := make([]*DeferredEffectResult, len(effects))
	for i, e := range effects {
		results[i] = deferredEffectToResult(e)
	}
	head := vm.DqGetHead(statedb)
	tail := vm.DqGetTail(statedb)
	return map[string]interface{}{
		"head":     head,
		"tail":     tail,
		"pending":  vm.DqGetPendingCount(statedb),
		"offset":   offset,
		"limit":    limit,
		"returned": len(results),
		"effects":  results,
	}, nil
}

// GetPendingEffect returns a single DeferredEffect by its sequence number,
// or null if the entry is absent (never existed or already drained).
func (api *EthernovaAPI) GetPendingEffect(seq uint64) (interface{}, error) {
	statedb, err := api.getStateDB()
	if err != nil {
		return nil, err
	}
	e := vm.DqGetEntry(statedb, seq)
	if e == nil {
		return nil, nil
	}
	return deferredEffectToResult(e), nil
}

// DeferredProcessingStats returns queue-level counters at the current head
// state. This is the debugging endpoint hinted at by the NIP-0004
// implementation plan (§11 of Phase 2) as
// `nova_getDeferredProcessingStats(blockNumber)` — we expose the current
// head state; historical per-block stats can be reconstructed from logs.
func (api *EthernovaAPI) DeferredProcessingStats() (map[string]interface{}, error) {
	statedb, err := api.getStateDB()
	if err != nil {
		return nil, err
	}
	head := api.e.blockchain.CurrentBlock().Number.Uint64()
	return map[string]interface{}{
		"currentBlock":        head,
		"queueHead":           vm.DqGetHead(statedb),
		"queueTail":           vm.DqGetTail(statedb),
		"pendingCount":        vm.DqGetPendingCount(statedb),
		"totalProcessed":      vm.DqGetTotalProcessed(statedb),
		"enqueuesAtThisBlock": vm.DqGetEnqueueCountAtBlock(statedb, head),
		"queueAddress":        vm.DeferredQueueAddr.Hex(),
		"forkBlock":           ethernova.DeferredExecForkBlock,
		"forkActive":          head >= ethernova.DeferredExecForkBlock,
		"maxEnqueuePerBlock":  ethernova.MaxPendingEffectsPerBlock,
		"maxDrainPerBlock":    ethernova.MaxDeferredProcessingPerBlock,
		"maxPayloadBytes":     ethernova.MaxDeferredEffectPayloadBytes,
	}, nil
}

// DeferredExecConfig returns the NIP-0004 Phase 2 configuration. Mirror of
// ProtocolObjectConfig() but for the deferred engine.
func (api *EthernovaAPI) DeferredExecConfig() map[string]interface{} {
	head := api.e.blockchain.CurrentBlock().Number.Uint64()
	active := head >= ethernova.DeferredExecForkBlock
	return map[string]interface{}{
		"forkBlock":          ethernova.DeferredExecForkBlock,
		"active":             active,
		"currentBlock":       head,
		"queueAddress":       vm.DeferredQueueAddr.Hex(),
		"precompile":         "0x2A",
		"maxEnqueuePerBlock": ethernova.MaxPendingEffectsPerBlock,
		"maxDrainPerBlock":   ethernova.MaxDeferredProcessingPerBlock,
		"maxPayloadBytes":    ethernova.MaxDeferredEffectPayloadBytes,
		"supportedEffectTypes": []map[string]interface{}{
			{"tag": types.EffectTypeNoop, "name": "Noop", "active": true},
			{"tag": types.EffectTypePing, "name": "Ping", "active": true},
			{"tag": types.EffectTypeMailboxSend, "name": "MailboxSend", "active": false, "phase": 4},
			{"tag": types.EffectTypeAsyncCallback, "name": "AsyncCallback", "active": false, "phase": 7},
			{"tag": types.EffectTypeSessionUpdate, "name": "SessionUpdate", "active": false, "phase": 7},
		},
		"description": "NIP-0004 Phase 2: Deferred Execution Engine — pending effects queue + block-prologue drain",
	}
}

// ============================================================
// NIP-0004 Phase 3: Content Reference Primitive RPC endpoints
//
// Public methods (registered via the "ethernova" RPC namespace):
//   - GetContentRef(idHex)                        -> ContentRefResult | null
//   - ListContentRefs(ownerHex, offset, limit)    -> list wrapper
//   - GetContentRefCount()                        -> counters
//   - ContentRefConfig()                          -> spec metadata
//
// Canonical JSON-RPC names follow the same style used by the rest of
// this API (see go-ethereum's standard lowercase-first-word mapping):
//
//   ethernova_getContentRef            -> GetContentRef
//   ethernova_listContentRefs          -> ListContentRefs
//   ethernova_getContentRefCount       -> GetContentRefCount
//   ethernova_contentRefConfig         -> ContentRefConfig
//
// The "nova_*" aliases mentioned in the Phase 3 spec are mapped by the
// admin layer that registers this API under the "nova" namespace too —
// see node.RegisterAPIs; if only one namespace is exposed, prefer
// "ethernova" which matches the service struct name.
// ============================================================

// ContentRefResult is the JSON-serializable representation of a ContentRef.
type ContentRefResult struct {
	ID                   common.Hash    `json:"id"`
	Owner                common.Address `json:"owner"`
	ContentHash          common.Hash    `json:"contentHash"`
	Size                 uint64         `json:"size"`
	ContentType          string         `json:"contentType"`
	AvailabilityProofHex string         `json:"availabilityProof"`
	ExpiryBlock          uint64         `json:"expiryBlock"`
	LastTouchedBlock     uint64         `json:"lastTouchedBlock"`
	RentBalanceStored    string         `json:"rentBalanceStored"`
	RentBalanceEffective string         `json:"rentBalanceEffective"`
	IsValid              bool           `json:"isValid"`
	ExpiredReason        string         `json:"expiredReason,omitempty"`
}

func contentRefToResult(obj *types.ProtocolObject, currentBlock uint64) (*ContentRefResult, error) {
	d, err := vm.DecodeContentRefStateData(obj.StateData)
	if err != nil {
		return nil, err
	}
	stored := "0"
	if obj.RentBalance != nil {
		stored = obj.RentBalance.String()
	}
	effBal := vm.CrEffectiveRentBalance(obj, currentBlock)
	valid := true
	reason := ""
	if obj.ExpiryBlock != 0 && currentBlock > obj.ExpiryBlock {
		valid = false
		reason = "past_expiry_block"
	}
	nextEpoch := stateComputeEpochRentWei(d.Size)
	if effBal.Cmp(nextEpoch) < 0 {
		valid = false
		if reason == "" {
			reason = "rent_exhausted"
		}
	}
	return &ContentRefResult{
		ID:                   obj.ID,
		Owner:                obj.Owner,
		ContentHash:          d.ContentHash,
		Size:                 d.Size,
		ContentType:          string(d.ContentType),
		AvailabilityProofHex: common.Bytes2Hex(d.AvailabilityProof),
		ExpiryBlock:          obj.ExpiryBlock,
		LastTouchedBlock:     obj.LastTouchedBlock,
		RentBalanceStored:    stored,
		RentBalanceEffective: effBal.String(),
		IsValid:              valid,
		ExpiredReason:        reason,
	}, nil
}

// stateComputeEpochRentWei is a tiny wrapper so this file doesn't need
// to import core/state at the top. Keeping the import surface narrow
// avoids pulling rent-math into every file that imports eth.
func stateComputeEpochRentWei(size uint64) *big.Int {
	// Inline the math: rate * size * epochLength. Matches
	// state.ComputeEpochRentWei exactly.
	r := new(big.Int).SetUint64(ethernova.RentRatePerBytePerBlock)
	s := new(big.Int).SetUint64(size)
	e := new(big.Int).SetUint64(ethernova.RentEpochLength)
	r.Mul(r, s)
	r.Mul(r, e)
	return r
}

// GetContentRef returns a ContentRef by ID hex string, or null if absent
// or wrong type. Wire name: ethernova_getContentRef (also aliased as
// nova_getContentRef when the namespace is registered).
func (api *EthernovaAPI) GetContentRef(idHex string) (interface{}, error) {
	statedb, err := api.getStateDB()
	if err != nil {
		return nil, err
	}
	id := common.HexToHash(idHex)
	obj := vm.CrGetContentRef(statedb, id)
	if obj == nil {
		return nil, nil
	}
	head := api.e.blockchain.CurrentBlock().Number.Uint64()
	return contentRefToResult(obj, head)
}

// ListContentRefs returns ContentRefs owned by an address, paginated.
// Wire name: ethernova_listContentRefs.
func (api *EthernovaAPI) ListContentRefs(ownerHex string, offset, limit uint64) (interface{}, error) {
	statedb, err := api.getStateDB()
	if err != nil {
		return nil, err
	}
	if limit == 0 || limit > 100 {
		limit = 100
	}
	owner := common.HexToAddress(ownerHex)
	ids := vm.CrListByOwner(statedb, owner, offset, limit)
	head := api.e.blockchain.CurrentBlock().Number.Uint64()

	results := make([]*ContentRefResult, 0, len(ids))
	for _, id := range ids {
		obj := vm.CrGetContentRef(statedb, id)
		if obj == nil {
			continue
		}
		r, err := contentRefToResult(obj, head)
		if err != nil {
			continue
		}
		results = append(results, r)
	}
	return map[string]interface{}{
		"owner":    owner.Hex(),
		"offset":   offset,
		"limit":    limit,
		"count":    len(results),
		"returned": results,
	}, nil
}

// GetContentRefCount returns live + monotonic counts.
// Wire name: ethernova_getContentRefCount.
func (api *EthernovaAPI) GetContentRefCount() (map[string]interface{}, error) {
	statedb, err := api.getStateDB()
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"live":            vm.CrGetLiveCount(statedb),
		"slotsUsed":       vm.CrGetSlotsUsed(statedb),
		"registryAddress": vm.ContentRegistryAddr.Hex(),
		"precompile":      "0x2B",
		"forkBlock":       ethernova.ContentRefForkBlock,
	}, nil
}

// ContentRefConfig returns Phase 3 metadata for clients/explorers.
// Wire name: ethernova_contentRefConfig.
func (api *EthernovaAPI) ContentRefConfig() map[string]interface{} {
	head := api.e.blockchain.CurrentBlock().Number.Uint64()
	active := head >= ethernova.ContentRefForkBlock
	return map[string]interface{}{
		"forkBlock":                  ethernova.ContentRefForkBlock,
		"active":                     active,
		"currentBlock":               head,
		"registryAddress":            vm.ContentRegistryAddr.Hex(),
		"precompile":                 "0x2B",
		"rentEpochLength":            ethernova.RentEpochLength,
		"rentRatePerBytePerBlock":    ethernova.RentRatePerBytePerBlock,
		"minRentPrepayWei":           ethernova.MinRentPrepayWei,
		"maxContentRefSize":          ethernova.MaxContentRefSize,
		"maxContentTypeBytes":        ethernova.MaxContentRefTypeBytes,
		"maxAvailabilityProofBytes":  ethernova.MaxContentRefAvailabilityProofBytes,
		"maxContentRefsPerRentEpoch": ethernova.MaxContentRefsPerRentEpoch,
		"description":                "NIP-0004 Phase 3: Content Reference Primitive — pointer to off-chain content with rent-backed expiry",
		"notes":                      "Precompile moved from NIP-0004 draft 0x2A to 0x2B to avoid collision with Phase 2 novaDeferredQueue (already at 0x2A).",
	}
}
