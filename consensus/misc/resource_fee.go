// Copyright 2026 The Ethernova Authors
//
// NIP-0004 Phase 10D — Multi-Dimensional Resource Metering.
//
// CalcNextResourcePrice is the consensus-critical formula that computes
// next-block per-dimension base prices from the parent header. It MUST
// be deterministic and pure: no global state, no float arithmetic, no
// timezone/locale/random dependency. Every full node will evaluate this
// function on the same inputs and arrive at the same outputs, by
// definition.
//
// The formula mirrors EIP-1559 baseFee adjustment but applied
// independently per dimension:
//
//	target_i      = parentGasLimit / 2
//	used_i        = parentUsage[i]
//	delta_i       = parentPrice[i] * |used_i - target_i| / target_i
//	delta_i       = delta_i * MaxAdjustmentBips / PriceUnitBips
//	if delta_i == 0: delta_i = 1
//	if used_i > target_i: new_i = saturating_add(parentPrice[i], delta_i)
//	else:                 new_i = parentPrice[i] - delta_i (floored at base)
//	new_i = clamp(new_i, base[i], base[i] * MaxPriceMultiplier)
//
// Notation: bips = basis points (10_000 = 1.00x).
//
// Hard invariants:
//   - The function is total: every uint64 input is accepted, every output
//     is a well-formed ResourceLimits.
//   - No panic on zero parent usage (treated as "below target" → decay).
//   - No panic on zero parent gas limit (returns base price unchanged).
//   - Saturating arithmetic everywhere — the per-dimension price never
//     overflows uint64.
//   - Lower bound = corresponding entry in Phase10CBasePriceBips. Upper
//     bound = base * resourceMaxPriceMultiplier (16x).

package misc

import (
	"math"

	"github.com/ethereum/go-ethereum/core/types"
)

// Tunable constants — kept package-local to make it obvious that consensus
// is computed from these exact numbers. Any change is a hard fork.
const (
	// resourcePriceUnitBips defines the unit denominator for all
	// per-dimension prices. 10_000 means 1.00x.
	resourcePriceUnitBips uint64 = 10_000
	// resourceMaxAdjustmentBips bounds the per-block movement to 12.5%.
	// Mirrors EIP-1559's BaseFeeMaxChangeDenominator=8 (1/8 = 12.5%).
	resourceMaxAdjustmentBips uint64 = 1_250
	// resourceMaxPriceMultiplier caps the absolute movement of any
	// dimension to 16x its genesis base price.
	resourceMaxPriceMultiplier uint64 = 16
)

// ResourceTargetForBlockGas returns the per-dimension utilization target.
// For a block gas limit G, each dimension targets G/2 of usage per block;
// above target prices rise, below target prices fall. Mirrors EIP-1559
// elasticity of 2.
func ResourceTargetForBlockGas(blockGasLimit uint64) uint64 {
	return blockGasLimit / 2
}

// BasePriceBips returns the genesis / floor price table. These are the
// SAME values exposed by core/vm.Phase10CBasePriceBips so the consensus
// path and the RPC quote path agree on the floor.
func BasePriceBips() types.ResourceLimits {
	return types.ResourceLimits{
		Compute:     10_000,
		StateRead:   20_000,
		StateWrite:  40_000,
		ProtocolOps: 10_000,
		ProofVerify: 30_000,
	}
}

// MaxPriceBips returns the per-dimension cap (16x base).
func MaxPriceBips() types.ResourceLimits {
	base := BasePriceBips()
	return types.ResourceLimits{
		Compute:     base.Compute * resourceMaxPriceMultiplier,
		StateRead:   base.StateRead * resourceMaxPriceMultiplier,
		StateWrite:  base.StateWrite * resourceMaxPriceMultiplier,
		ProtocolOps: base.ProtocolOps * resourceMaxPriceMultiplier,
		ProofVerify: base.ProofVerify * resourceMaxPriceMultiplier,
	}
}

// CalcNextResourcePrice is the pure deterministic transition function.
//
// Inputs:
//   - parentPrice: parent header's ResourceBasePrice. If nil OR all zero
//     this is the activation block — the function returns BasePriceBips()
//     unchanged.
//   - parentUsage: parent header's ResourceUsed. nil is treated as zero
//     usage (decay toward base).
//   - parentGasLimit: parent header's GasLimit. Zero is treated as no
//     change (returns parentPrice).
//
// Output: the canonical per-dimension price for the CURRENT block (i.e.
// the block whose parent is the input). Every full node computes the same
// table for the same block.
func CalcNextResourcePrice(
	parentPrice *types.ResourceLimits,
	parentUsage *types.ResourceLimits,
	parentGasLimit uint64,
) types.ResourceLimits {
	base := BasePriceBips()
	maxP := MaxPriceBips()

	// Phase 10D activation block: parent header has no resource fields
	// yet. Use the base table.
	if parentPrice == nil || parentPrice.IsZero() {
		return base
	}
	// Defensive: parent header missing usage means we treat usage = 0.
	usage := types.ResourceLimits{}
	if parentUsage != nil {
		usage = *parentUsage
	}
	if parentGasLimit == 0 {
		return *parentPrice
	}
	target := ResourceTargetForBlockGas(parentGasLimit)

	return types.ResourceLimits{
		Compute:     adjustOne(parentPrice.Compute, usage.Compute, target, base.Compute, maxP.Compute),
		StateRead:   adjustOne(parentPrice.StateRead, usage.StateRead, target, base.StateRead, maxP.StateRead),
		StateWrite:  adjustOne(parentPrice.StateWrite, usage.StateWrite, target, base.StateWrite, maxP.StateWrite),
		ProtocolOps: adjustOne(parentPrice.ProtocolOps, usage.ProtocolOps, target, base.ProtocolOps, maxP.ProtocolOps),
		ProofVerify: adjustOne(parentPrice.ProofVerify, usage.ProofVerify, target, base.ProofVerify, maxP.ProofVerify),
	}
}

// adjustOne implements the per-dimension EIP-1559-style adjustment.
// Identical numeric semantics to core/vm.adjustResourcePriceBips so the
// quote layer and consensus layer never disagree.
func adjustOne(current, used, target, base, max uint64) uint64 {
	if target == 0 {
		return current
	}
	if used == target {
		return clampPrice(current, base, max)
	}
	diff := absDiff(used, target)
	// delta = current * diff / target (saturating mul, then div)
	delta := saturatingMul(current, diff) / target
	// delta = delta * MaxAdjustmentBips / PriceUnitBips
	delta = saturatingMul(delta, resourceMaxAdjustmentBips) / resourcePriceUnitBips
	if delta == 0 {
		delta = 1
	}
	var next uint64
	if used > target {
		next = saturatingAdd(current, delta)
	} else {
		if delta >= current {
			next = base
		} else {
			next = current - delta
		}
	}
	return clampPrice(next, base, max)
}

func clampPrice(v, lo, hi uint64) uint64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func absDiff(a, b uint64) uint64 {
	if a > b {
		return a - b
	}
	return b - a
}

func saturatingAdd(a, b uint64) uint64 {
	if math.MaxUint64-a < b {
		return math.MaxUint64
	}
	return a + b
}

func saturatingMul(a, b uint64) uint64 {
	if a == 0 || b == 0 {
		return 0
	}
	if a > math.MaxUint64/b {
		return math.MaxUint64
	}
	return a * b
}
