package eth

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/vm"
)

// ResourceConfig exposes NIP-0004 Phase 10A metering metadata for SDKs,
// explorers, and external test harnesses. Phase 10A is monitoring-only.
func (api *EthernovaAPI) ResourceConfig() map[string]interface{} {
	head := api.e.blockchain.CurrentBlock().Number.Uint64()
	return map[string]interface{}{
		"phase":               10,
		"substage":            "10A",
		"mode":                "monitoring_only",
		"active":              true,
		"pricingActive":       false,
		"adaptivePricing":     false,
		"extendedTxFormat":    false,
		"currentBlock":        head,
		"dimensions":          []string{"compute", "state_read", "state_write", "protocol_ops", "proof_verify"},
		"defaultLimitFormula": "legacy gasLimit maps to compute=gasLimit, state_read=gasLimit/3, state_write=gasLimit/6, protocol_ops=gasLimit/15, proof_verify=gasLimit/30",
		"notes": []string{
			"Phase 10A does not change gas charged, receipt RLP, block headers, or state roots.",
			"Resource vectors are diagnostic and derived from deterministic EVM trace counters plus Nova precompile dispatch.",
			"Per-dimension adaptive pricing is deferred to Phase 10B/10C after devnet soak.",
		},
	}
}

// ResourcePrices returns the placeholder per-dimension price table. All prices
// are fixed at 1 in Phase 10A because pricing is not active yet.
func (api *EthernovaAPI) ResourcePrices() map[string]interface{} {
	return map[string]interface{}{
		"phase":         10,
		"substage":      "10A",
		"pricingActive": false,
		"prices": map[string]uint64{
			"compute":      1,
			"state_read":   1,
			"state_write":  1,
			"protocol_ops": 1,
			"proof_verify": 1,
		},
		"targetUtilization": "50%",
		"maxAdjustment":     "12.5%",
	}
}

// EstimateResourceLimits maps a legacy gas limit to Phase 10 resource limits.
func (api *EthernovaAPI) EstimateResourceLimits(gasLimit hexutil.Uint64) vm.ResourceVector {
	return vm.LegacyGasToResourceLimits(uint64(gasLimit))
}

// GetResourceVector returns the recent in-memory Phase 10A vector for a tx.
// It is intentionally not historical in 10A because vectors are not written
// into consensus objects. Explorers can use this for live devnet observation.
func (api *EthernovaAPI) GetResourceVector(txHash string) map[string]interface{} {
	hash := common.HexToHash(txHash)
	sample, ok := vm.GlobalResourceMonitor.GetTx(hash)
	if !ok {
		return map[string]interface{}{
			"txHash": hash.Hex(),
			"exists": false,
			"note":   "Phase 10A stores recent vectors in memory only; historical persistence is Phase 10B scope.",
		}
	}
	return map[string]interface{}{
		"txHash":      sample.TxHash.Hex(),
		"exists":      true,
		"blockNumber": sample.BlockNumber,
		"txIndex":     sample.TxIndex,
		"vector":      sample.Vector,
		"mode":        "monitoring_only",
	}
}
