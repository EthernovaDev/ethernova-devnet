package eth

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/consensus/misc"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params/ethernova"
)

// NIP-0004 Phase 10D — Multi-Dimensional Resource Metering, RPC surface.
//
// After Phase 10D the Phase 10A/B/C surface is FLIPPED from quote-only to
// consensus-enforced. Header.ResourceUsed + Header.ResourceBasePrice are
// now canonical, every full node agrees on them by definition, and the
// RPC reads price/usage straight from the chain head rather than the
// in-process pricer singleton.

// ResourceConfig exposes the active resource-metering configuration. The
// `enforcement` block reflects the current fork-block gating so SDKs and
// explorers can detect both pre-fork (advisory) and post-fork (consensus)
// states without parsing version strings.
func (api *EthernovaAPI) ResourceConfig() map[string]interface{} {
	head := api.e.blockchain.CurrentBlock().Number.Uint64()
	forkBlock := ethernova.ResourceMeteringForkBlock
	graceEnd := forkBlock + ethernova.ResourceMeteringTransitionGracePostFork
	active := head >= forkBlock
	graceActive := head >= forkBlock && head < graceEnd

	return map[string]interface{}{
		"phase":               10,
		"substage":            "10D",
		"mode":                "consensus_enforced_per_dimension",
		"active":              active,
		"pricingActive":       true,
		"adaptivePricing":     true,
		"extendedTxFormat":    true,
		"consensusGasChanged": true,
		"enforcement": map[string]interface{}{
			"forkBlock":     forkBlock,
			"graceEndBlock": graceEnd,
			"graceActive":   graceActive,
			"headerFields":  []string{"resourceUsed", "resourceBasePrice"},
			"perDimensionOOR": []string{
				"ErrOutOfResourceCompute",
				"ErrOutOfResourceStateRead",
				"ErrOutOfResourceStateWrite",
				"ErrOutOfResourceProtocolOps",
				"ErrOutOfResourceProofVerify",
			},
		},
		"currentBlock":   head,
		"resourceTxType": "0x05",
		"dimensions":     []string{"compute", "state_read", "state_write", "protocol_ops", "proof_verify"},
		"defaultLegacyMapping": map[string]string{
			"compute":      "gasLimit",
			"state_read":   "gasLimit",
			"state_write":  "gasLimit",
			"protocol_ops": "gasLimit",
			"proof_verify": "gasLimit",
			"note":         "Phase 10D maps legacy gasLimit to every dimension equally — any pre-fork-passing tx continues to pass post-fork.",
		},
		"quoteLegacyMapping": map[string]string{
			"compute":      "gasLimit",
			"state_read":   "gasLimit/3",
			"state_write":  "gasLimit/6",
			"protocol_ops": "gasLimit/15",
			"proof_verify": "gasLimit/30",
			"note":         "RPC-only quote mapping retained for backward compatibility with the Phase 10C SDK.",
		},
		"notes": []string{
			"Phase 10D activates consensus-level per-dimension metering. Header.ResourceUsed + Header.ResourceBasePrice are consensus objects.",
			"Legacy / EIP-1559 transactions continue to work unchanged; per-dimension limits are derived from gasLimit (no tightening vs total budget).",
			"Senders that want fine-grained per-dim caps can submit a ResourceTx (envelope byte 0x05).",
		},
	}
}

// resourcePriceBipsFromHead returns the canonical per-dimension price table
// for the current chain head. Post-fork it reads from header.ResourceBasePrice;
// pre-fork it falls back to the legacy in-memory pricer.
func (api *EthernovaAPI) resourcePriceBipsFromHead() vm.ResourcePriceBips {
	head := api.e.blockchain.CurrentBlock()
	if vm.IsResourceMeteringActive(head.Number.Uint64()) && head.ResourceBasePrice != nil {
		return vm.ResourcePriceBips{
			Compute:     head.ResourceBasePrice.Compute,
			StateRead:   head.ResourceBasePrice.StateRead,
			StateWrite:  head.ResourceBasePrice.StateWrite,
			ProtocolOps: head.ResourceBasePrice.ProtocolOps,
			ProofVerify: head.ResourceBasePrice.ProofVerify,
		}
	}
	return vm.GlobalAdaptiveResourcePricer.Snapshot().CurrentPriceBips
}

// headerUsageVector returns the canonical sum-of-tx vector for the current
// chain head. Post-fork it reads from header.ResourceUsed; pre-fork it
// falls back to the in-memory pricer snapshot.
func (api *EthernovaAPI) headerUsageVector() vm.ResourceVector {
	head := api.e.blockchain.CurrentBlock()
	if vm.IsResourceMeteringActive(head.Number.Uint64()) && head.ResourceUsed != nil {
		return vm.ResourceVector{
			Compute:     head.ResourceUsed.Compute,
			StateRead:   head.ResourceUsed.StateRead,
			StateWrite:  head.ResourceUsed.StateWrite,
			ProtocolOps: head.ResourceUsed.ProtocolOps,
			ProofVerify: head.ResourceUsed.ProofVerify,
		}
	}
	return vm.GlobalAdaptiveResourcePricer.Snapshot().Usage
}

// ResourcePrices returns the active per-dimension price table sourced from
// the canonical chain head (post-fork) or the legacy pricer (pre-fork).
func (api *EthernovaAPI) ResourcePrices() map[string]interface{} {
	head := api.e.blockchain.CurrentBlock()
	current := api.resourcePriceBipsFromHead()
	snap := vm.GlobalAdaptiveResourcePricer.Snapshot()
	return map[string]interface{}{
		"phase":             10,
		"substage":          "10D",
		"pricingActive":     true,
		"adaptive":          true,
		"enforcement":       "consensus",
		"source":            "header.resourceBasePrice",
		"blockNumber":       head.Number.Uint64(),
		"basePriceBips":     misc.BasePriceBips(),
		"currentPriceBips":  current,
		"lastBlock":         head.Number.Uint64(),
		"lastUsage":         api.headerUsageVector(),
		"target":            snap.Target,
		"utilizationBips":   snap.UtilizationBips,
		"maxAdjustmentBips": snap.MaxAdjustmentBips,
		"prices": map[string]uint64{
			"compute":      current.Compute,
			"state_read":   current.StateRead,
			"state_write":  current.StateWrite,
			"protocol_ops": current.ProtocolOps,
			"proof_verify": current.ProofVerify,
		},
		"targetUtilization": "50%",
		"maxAdjustment":     "12.5%",
		"notes": []string{
			"Prices are returned in basis points: 10000 = 1.00x.",
			"After Phase 10D the canonical table is the chain head's resourceBasePrice — every full node agrees on it by consensus.",
		},
	}
}

// GetResourcePrices is the nova_getResourcePrices alias requested by the
// Phase 10 spec. Identical payload to nova_resourcePrices.
func (api *EthernovaAPI) GetResourcePrices() map[string]interface{} {
	return api.ResourcePrices()
}

// GetResourceUsage is the nova_getResourceUsage alias requested by the
// Phase 10 spec. Returns the per-dimension usage carried by the canonical
// chain head plus a per-dimension utilisation snapshot.
func (api *EthernovaAPI) GetResourceUsage() map[string]interface{} {
	head := api.e.blockchain.CurrentBlock()
	usage := api.headerUsageVector()
	snap := vm.GlobalAdaptiveResourcePricer.Snapshot()
	return map[string]interface{}{
		"phase":           10,
		"substage":        "10D",
		"source":          "header.resourceUsed",
		"blockNumber":     head.Number.Uint64(),
		"gasLimit":        head.GasLimit,
		"target":          snap.Target,
		"usage":           usage,
		"utilizationBips": snap.UtilizationBips,
		"perDimension": map[string]uint64{
			"compute":      usage.Compute,
			"state_read":   usage.StateRead,
			"state_write":  usage.StateWrite,
			"protocol_ops": usage.ProtocolOps,
			"proof_verify": usage.ProofVerify,
		},
	}
}

// EstimateResourceLimits maps a legacy gas limit to a QUOTE resource vector.
// Returns the legacy quote mapping (compute=gasLimit, state_read=gasLimit/3,
// …) because that is what existing SDKs already consume. Use
// nova_estimateResourceLimitsEnforced for the consensus-enforced mapping.
func (api *EthernovaAPI) EstimateResourceLimits(gasLimit hexutil.Uint64) vm.ResourceVector {
	return vm.LegacyGasToResourceLimits(uint64(gasLimit))
}

// EstimateResourceLimitsEnforced returns the CONSENSUS-ENFORCED mapping
// (every dimension = gasLimit). Use this when sizing a ResourceTx's
// per-dim caps for a legacy-like transaction.
func (api *EthernovaAPI) EstimateResourceLimitsEnforced(gasLimit hexutil.Uint64) types.ResourceLimits {
	return types.LegacyGasToResourceLimitsEnforced(uint64(gasLimit))
}

// QuoteResourceFee prices a supplied resource vector with the active price
// table read from the canonical chain head.
func (api *EthernovaAPI) QuoteResourceFee(vector vm.ResourceVector) map[string]interface{} {
	current := api.resourcePriceBipsFromHead()
	return map[string]interface{}{
		"phase":               10,
		"substage":            "10D",
		"pricingActive":       true,
		"adaptive":            true,
		"consensusGasChanged": true,
		"priceBips":           current,
		"vector":              vector,
		"pricedUnits":         vm.PriceResourceVectorBips(vector, current),
		"source":              "header.resourceBasePrice",
	}
}

// CalcResourcePriceFor is a pure utility that exposes the canonical
// next-block adjustment formula to clients. Useful for explorers that
// want to preview the next block's price table before it is sealed.
func (api *EthernovaAPI) CalcResourcePriceFor(
	parentPrice *types.ResourceLimits,
	parentUsage *types.ResourceLimits,
	parentGasLimit hexutil.Uint64,
) types.ResourceLimits {
	return misc.CalcNextResourcePrice(parentPrice, parentUsage, uint64(parentGasLimit))
}

// ResourceCongestion returns the latest per-dimension congestion snapshot.
func (api *EthernovaAPI) ResourceCongestion() map[string]interface{} {
	snap := vm.GlobalAdaptiveResourcePricer.Snapshot()
	head := api.e.blockchain.CurrentBlock()
	headerUsage := api.headerUsageVector()
	headerPrice := api.resourcePriceBipsFromHead()
	return map[string]interface{}{
		"phase":               10,
		"substage":            "10D",
		"adaptive":            true,
		"consensusGasChanged": true,
		"snapshot":            snap,
		"headBlock":           head.Number.Uint64(),
		"headUsage":           headerUsage,
		"headBasePriceBips":   headerPrice,
		"isolation":           "each dimension adjusts only from its own usage/target ratio",
	}
}

// GetResourceVector returns the recent in-memory vector for a tx.
// Historical persistence is the Phase 10E follow-up; until then use
// eth_getBlockByNumber to fetch the aggregate header.resourceUsed.
func (api *EthernovaAPI) GetResourceVector(txHash string) map[string]interface{} {
	hash := common.HexToHash(txHash)
	sample, ok := vm.GlobalResourceMonitor.GetTx(hash)
	if !ok {
		return map[string]interface{}{
			"txHash": hash.Hex(),
			"exists": false,
			"note":   "Phase 10D stores recent per-tx vectors in memory (cap 2048). Use eth_getBlockByNumber.resourceUsed for historical aggregate usage.",
		}
	}
	current := api.resourcePriceBipsFromHead()
	return map[string]interface{}{
		"txHash":        sample.TxHash.Hex(),
		"exists":        true,
		"blockNumber":   sample.BlockNumber,
		"txIndex":       sample.TxIndex,
		"vector":        sample.Vector,
		"pricedUnits":   vm.PriceResourceVectorBips(sample.Vector, current),
		"priceBips":     current,
		"mode":          "consensus_enforced_per_dimension",
		"pricingActive": true,
	}
}
