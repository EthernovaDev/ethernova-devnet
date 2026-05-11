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
		"substage":            "10C",
		"mode":                "adaptive_per_dimension_pricing",
		"active":              true,
		"pricingActive":       true,
		"adaptivePricing":     true,
		"extendedTxFormat":    false,
		"consensusGasChanged": false,
		"enforcement":         "quote_only_until_extended_tx_format",
		"currentBlock":        head,
		"dimensions":          []string{"compute", "state_read", "state_write", "protocol_ops", "proof_verify"},
		"defaultLimitFormula": "legacy gasLimit maps to compute=gasLimit, state_read=gasLimit/3, state_write=gasLimit/6, protocol_ops=gasLimit/15, proof_verify=gasLimit/30",
		"notes": []string{
			"Phase 10C activates adaptive per-dimension quotation while preserving legacy gas charging.",
			"Resource vectors are derived from deterministic EVM trace counters plus Nova precompile dispatch.",
			"Each dimension moves independently, so compute/storage congestion does not raise protocol_ops pricing.",
			"Extended transaction enforcement remains deferred until the quote layer has completed devnet soak.",
		},
	}
}

// ResourcePrices returns the active Phase 10C adaptive per-dimension price table.
func (api *EthernovaAPI) ResourcePrices() map[string]interface{} {
	snap := vm.GlobalAdaptiveResourcePricer.Snapshot()
	return map[string]interface{}{
		"phase":             10,
		"substage":          "10C",
		"pricingActive":     true,
		"adaptive":          true,
		"enforcement":       "quote_only",
		"basePriceBips":     snap.BasePriceBips,
		"currentPriceBips":  snap.CurrentPriceBips,
		"lastBlock":         snap.BlockNumber,
		"lastUsage":         snap.Usage,
		"target":            snap.Target,
		"utilizationBips":   snap.UtilizationBips,
		"maxAdjustmentBips": snap.MaxAdjustmentBips,
		"prices": map[string]uint64{
			"compute":      snap.CurrentPriceBips.Compute,
			"state_read":   snap.CurrentPriceBips.StateRead,
			"state_write":  snap.CurrentPriceBips.StateWrite,
			"protocol_ops": snap.CurrentPriceBips.ProtocolOps,
			"proof_verify": snap.CurrentPriceBips.ProofVerify,
		},
		"targetUtilization": "50%",
		"maxAdjustment":     "12.5%",
		"notes": []string{
			"Prices are returned in basis points: 10000 = 1.00x.",
			"Protocol ops have their own independent controller so mailbox/chat quotes are isolated from compute/state congestion.",
		},
	}
}

// EstimateResourceLimits maps a legacy gas limit to Phase 10 resource limits.
func (api *EthernovaAPI) EstimateResourceLimits(gasLimit hexutil.Uint64) vm.ResourceVector {
	return vm.LegacyGasToResourceLimits(uint64(gasLimit))
}

// QuoteResourceFee prices a supplied resource vector with the active Phase 10C
// adaptive table. This is an RPC quote only; consensus gas charging is unchanged.
func (api *EthernovaAPI) QuoteResourceFee(vector vm.ResourceVector) map[string]interface{} {
	snap := vm.GlobalAdaptiveResourcePricer.Snapshot()
	return map[string]interface{}{
		"phase":               10,
		"substage":            "10C",
		"pricingActive":       true,
		"adaptive":            true,
		"consensusGasChanged": false,
		"priceBips":           snap.CurrentPriceBips,
		"vector":              vector,
		"pricedUnits":         vm.PriceResourceVectorBips(vector, snap.CurrentPriceBips),
	}
}

// ResourceCongestion returns the latest Phase 10C per-dimension congestion
// controller snapshot. It is intentionally operational telemetry, not a
// consensus object.
func (api *EthernovaAPI) ResourceCongestion() map[string]interface{} {
	snap := vm.GlobalAdaptiveResourcePricer.Snapshot()
	return map[string]interface{}{
		"phase":               10,
		"substage":            "10C",
		"adaptive":            true,
		"consensusGasChanged": false,
		"snapshot":            snap,
		"isolation":           "each dimension adjusts only from its own usage/target ratio",
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
			"note":   "Phase 10C stores recent vectors in memory only; historical persistence is reserved for extended transaction telemetry.",
		}
	}
	snap := vm.GlobalAdaptiveResourcePricer.Snapshot()
	return map[string]interface{}{
		"txHash":        sample.TxHash.Hex(),
		"exists":        true,
		"blockNumber":   sample.BlockNumber,
		"txIndex":       sample.TxIndex,
		"vector":        sample.Vector,
		"pricedUnits":   vm.PriceResourceVectorBips(sample.Vector, snap.CurrentPriceBips),
		"priceBips":     snap.CurrentPriceBips,
		"mode":          "adaptive_per_dimension_pricing",
		"pricingActive": true,
	}
}
