package eth

import (
	"runtime"
	"time"

	"github.com/ethereum/go-ethereum/core"
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
	Version       string  `json:"version"`
	Network       string  `json:"network"`
	CurrentBlock  uint64  `json:"currentBlock"`
	HighestBlock  uint64  `json:"highestBlock"`
	PeerCount     int     `json:"peerCount"`
	Syncing       bool    `json:"syncing"`
	SyncProgress  float64 `json:"syncProgress"`
	UptimeSeconds int64   `json:"uptimeSeconds"`
	MemoryMB      uint64  `json:"memoryMB"`
	DualSignerFallbacks int64 `json:"dualSignerFallbacks"`
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
	Enabled       bool              `json:"enabled"`
	TotalOps      uint64            `json:"totalOps"`
	TotalGas      uint64            `json:"totalGas"`
	TopOpcodes    []vm.OpcodeStats  `json:"topOpcodes"`
	TopContracts  []vm.ContractStats `json:"topContracts"`
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
	Enabled         bool                `json:"enabled"`
	Version         string              `json:"version"`
	ForkBlock       uint64              `json:"forkBlock"`
	DiscountPercent uint64              `json:"maxDiscountPercent"`
	PenaltyPercent  uint64              `json:"maxPenaltyPercent"`
	Contracts       []vm.PatternStats   `json:"contracts"`
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
	Enabled         bool                       `json:"enabled"`
	ConsensusRule   bool                       `json:"consensusRule"`
	Version         string                     `json:"version"`
	ForkBlock       uint64                     `json:"forkBlock"`
	DiscountPercent uint64                     `json:"maxDiscountPercent"`
	PenaltyPercent  uint64                     `json:"maxPenaltyPercent"`
	LastTx          *vm.AdaptiveGasV2Stats     `json:"lastTxClassification,omitempty"`
	LegacyContracts []vm.PatternStats          `json:"legacyContracts,omitempty"`
	StaticClasses   []vm.ClassificationStats   `json:"staticClassifications,omitempty"`
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
	adjustPct := vm.ComputeGasAdjustment(category, score, totalOps)

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
	Mode            string              `json:"mode"`
	FastExecutions  uint64              `json:"fastExecutions"`
	SkippedChecks   uint64              `json:"skippedChecks"`
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

// AutoTuner returns the auto-tuner status.
func (api *EthernovaAPI) AutoTuner() vm.AutoTunerStats {
	return vm.GlobalAutoTuner.Stats()
}

// AutoTunerToggle enables or disables auto-tuning.
func (api *EthernovaAPI) AutoTunerToggle(enabled bool) bool {
	vm.GlobalAutoTuner.SetEnabled(enabled)
	return vm.GlobalAutoTuner.IsEnabled()
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