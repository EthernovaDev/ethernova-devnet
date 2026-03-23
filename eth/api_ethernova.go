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
	DiscountPercent uint64              `json:"discountPercent"`
	PenaltyPercent  uint64              `json:"penaltyPercent"`
	Contracts       []vm.PatternStats   `json:"contracts"`
}

// AdaptiveGas returns the current adaptive gas configuration and contract patterns.
func (api *EthernovaAPI) AdaptiveGas() AdaptiveGasResult {
	return AdaptiveGasResult{
		Enabled:         vm.GlobalAdaptiveGas.Enabled.Load(),
		DiscountPercent: vm.GlobalAdaptiveGas.DiscountPercent,
		PenaltyPercent:  vm.GlobalAdaptiveGas.PenaltyPercent,
		Contracts:       vm.GlobalPatternTracker.GetAllPatterns(),
	}
}

// AdaptiveGasToggle enables or disables the adaptive gas system.
func (api *EthernovaAPI) AdaptiveGasToggle(enabled bool) bool {
	vm.GlobalAdaptiveGas.Enabled.Store(enabled)
	return vm.GlobalAdaptiveGas.Enabled.Load()
}

// AdaptiveGasSetDiscount sets the discount percentage (0-50).
func (api *EthernovaAPI) AdaptiveGasSetDiscount(percent uint64) uint64 {
	if percent > 50 {
		percent = 50
	}
	vm.GlobalAdaptiveGas.DiscountPercent = percent
	return vm.GlobalAdaptiveGas.DiscountPercent
}

// AdaptiveGasSetPenalty sets the penalty percentage for complex contracts (0-50).
func (api *EthernovaAPI) AdaptiveGasSetPenalty(percent uint64) uint64 {
	if percent > 50 {
		percent = 50
	}
	vm.GlobalAdaptiveGas.PenaltyPercent = percent
	return vm.GlobalAdaptiveGas.PenaltyPercent
}

// AdaptiveGasReset clears all pattern tracking data.
func (api *EthernovaAPI) AdaptiveGasReset() bool {
	vm.GlobalPatternTracker.Reset()
	return true
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
