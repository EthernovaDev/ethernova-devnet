package eth

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/vm"
)

// ResourceConfig exposes NIP-0004 Phase 10 metering metadata for SDKs,
// explorers, and external test harnesses.
func (api *EthernovaAPI) ResourceConfig() map[string]interface{} {
	head := api.e.blockchain.CurrentBlock().Number.Uint64()
	return map[string]interface{}{
		"phase":               10,
		"substage":            "10B",
		"mode":                "static_per_dimension_pricing",
		"active":              true,
		"pricingActive":       true,
		"adaptivePricing":     false,
		"extendedTxFormat":    false,
		"consensusGasChanged": false,
		"enforcement":         "quote_only_until_extended_tx_format",
		"currentBlock":        head,
		"dimensions":          []string{"compute", "state_read", "state_write", "protocol_ops", "proof_verify"},
		"defaultLimitFormula": "legacy gasLimit maps to compute=gasLimit, state_read=gasLimit/3, state_write=gasLimit/6, protocol_ops=gasLimit/15, proof_verify=gasLimit/30",
		"notes": []string{
			"Phase 10B activates deterministic per-dimension quotation while preserving legacy gas charging.",
			"Resource vectors are derived from deterministic EVM trace counters plus Nova precompile dispatch.",
			"Adaptive per-block pricing and extended transaction enforcement are deferred to Phase 10C after devnet soak.",
		},
	}
}

// ResourcePrices returns the active Phase 10B static per-dimension price table.
func (api *EthernovaAPI) ResourcePrices() map[string]interface{} {
	prices := vm.Phase10BResourcePrices()
	return map[string]interface{}{
		"phase":         10,
		"substage":      "10B",
		"pricingActive": true,
		"adaptive":      false,
		"enforcement":   "quote_only",
		"prices": map[string]uint64{
			"compute":      prices.Compute,
			"state_read":   prices.StateRead,
			"state_write":  prices.StateWrite,
			"protocol_ops": prices.ProtocolOps,
			"proof_verify": prices.ProofVerify,
		},
		"targetUtilization": "50%",
		"maxAdjustment":     "12.5%",
		"notes": []string{
			"Static devnet multipliers: compute=1, state_read=2, state_write=4, protocol_ops=1, proof_verify=3.",
			"Protocol ops stay at 1 so mailbox/chat traffic is isolated from storage-heavy workload pricing.",
		},
	}
}

// EstimateResourceLimits maps a legacy gas limit to Phase 10 resource limits.
func (api *EthernovaAPI) EstimateResourceLimits(gasLimit hexutil.Uint64) vm.ResourceVector {
	return vm.LegacyGasToResourceLimits(uint64(gasLimit))
}

// QuoteResourceFee prices a supplied resource vector with the active Phase 10B
// static table. This is an RPC quote only; consensus gas charging is unchanged.
func (api *EthernovaAPI) QuoteResourceFee(vector vm.ResourceVector) map[string]interface{} {
	prices := vm.Phase10BResourcePrices()
	return map[string]interface{}{
		"phase":               10,
		"substage":            "10B",
		"pricingActive":       true,
		"adaptive":            false,
		"consensusGasChanged": false,
		"prices":              prices,
		"vector":              vector,
		"pricedUnits":         vm.PriceResourceVector(vector, prices),
	}
}

// GetResourceVector returns the recent in-memory Phase 10 vector for a tx.
// It is intentionally not historical yet because vectors are not written into
// consensus objects. Explorers can use this for live devnet observation.
func (api *EthernovaAPI) GetResourceVector(txHash string) map[string]interface{} {
	hash := common.HexToHash(txHash)
	sample, ok := vm.GlobalResourceMonitor.GetTx(hash)
	if !ok {
		return map[string]interface{}{
			"txHash": hash.Hex(),
			"exists": false,
			"note":   "Phase 10B stores recent vectors in memory only; historical persistence is reserved for extended transaction telemetry.",
		}
	}
	prices := vm.Phase10BResourcePrices()
	return map[string]interface{}{
		"txHash":        sample.TxHash.Hex(),
		"exists":        true,
		"blockNumber":   sample.BlockNumber,
		"txIndex":       sample.TxIndex,
		"vector":        sample.Vector,
		"pricedUnits":   vm.PriceResourceVector(sample.Vector, prices),
		"mode":          "static_per_dimension_pricing",
		"pricingActive": true,
	}
}
