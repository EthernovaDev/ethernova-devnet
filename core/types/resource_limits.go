// Copyright 2026 The Ethernova Authors
//
// This file is part of the Ethernova devnet CoreGeth fork.
//
// NIP-0004 Phase 10D — Multi-Dimensional Resource Metering: shared types.
//
// ResourceLimits is the on-wire 5-tuple representation used by:
//   - core/types/tx_resource.go (ResourceTx limit field)
//   - core/types/block.go      (Header.ResourceUsed / Header.ResourceBasePrice)
//   - core/state_transition.go (per-dim enforcement)
//   - consensus/misc           (deterministic base price adjustment)
//   - eth/api_ethernova_phase10.go (RPC marshalling)
//
// The struct intentionally has a stable field ORDER for RLP: any change
// here is a hard fork because Header RLP would diverge.

package types

import (
	"encoding/json"
	"errors"
	"io"
	"math"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rlp"
)

// ResourceLimits encodes a 5-dimensional resource vector. Fields appear in
// canonical NIP-0004 §6.1 order. The struct is reused for three roles:
//
//  1. As a TX-level LIMIT on a ResourceTx — declared by the sender, enforced
//     during state transition.
//  2. As a HEADER-LEVEL aggregate (Header.ResourceUsed) — sum of all
//     per-tx vectors in the block.
//  3. As a HEADER-LEVEL price table (Header.ResourceBasePrice) — current
//     per-dimension base price expressed in basis points (10_000 = 1.00x).
//
// All fields are uint64 so RLP encoding is stable across architectures.
type ResourceLimits struct {
	Compute     uint64 `json:"compute"`
	StateRead   uint64 `json:"stateRead"`
	StateWrite  uint64 `json:"stateWrite"`
	ProtocolOps uint64 `json:"protocolOps"`
	ProofVerify uint64 `json:"proofVerify"`
}

// IsZero reports whether all five dimensions are zero. Used by the header
// validator to special-case the genesis row (no parent header to derive from).
func (r ResourceLimits) IsZero() bool {
	return r.Compute == 0 &&
		r.StateRead == 0 &&
		r.StateWrite == 0 &&
		r.ProtocolOps == 0 &&
		r.ProofVerify == 0
}

// Equal performs an exact 5-way comparison. Used by state_processor.go to
// validate that a block's claimed Header.ResourceUsed matches the recomputed
// sum of tx vectors.
func (r ResourceLimits) Equal(other ResourceLimits) bool {
	return r.Compute == other.Compute &&
		r.StateRead == other.StateRead &&
		r.StateWrite == other.StateWrite &&
		r.ProtocolOps == other.ProtocolOps &&
		r.ProofVerify == other.ProofVerify
}

// Add returns the saturating sum of two limits. Used to accumulate per-tx
// usage into the block-level Header.ResourceUsed.
func (r ResourceLimits) Add(other ResourceLimits) ResourceLimits {
	return ResourceLimits{
		Compute:     resourceSaturatingAddU64(r.Compute, other.Compute),
		StateRead:   resourceSaturatingAddU64(r.StateRead, other.StateRead),
		StateWrite:  resourceSaturatingAddU64(r.StateWrite, other.StateWrite),
		ProtocolOps: resourceSaturatingAddU64(r.ProtocolOps, other.ProtocolOps),
		ProofVerify: resourceSaturatingAddU64(r.ProofVerify, other.ProofVerify),
	}
}

// LegacyGasMapping returns the deterministic mapping from a single-dim
// gasLimit to a 5-dim resource vector for backward compatibility.
//
// IMPORTANT: every dimension is set equal to gasLimit. This means a legacy
// transaction that succeeded BEFORE the Phase 10D fork (because its gasUsed
// fit inside gasLimit) will ALWAYS succeed AFTER the fork too — no
// per-dimension limit is tighter than the total gasLimit.
//
// The asymmetric ratios documented in the Phase 10C quote layer
// (compute=gasLimit, state_read=gasLimit/3, state_write=gasLimit/6, ...)
// remain available as a separate quote helper in core/vm/resource_metering.go
// (LegacyGasToResourceLimits). They are NOT used by the Phase 10D enforcer.
func LegacyGasToResourceLimitsEnforced(gasLimit uint64) ResourceLimits {
	return ResourceLimits{
		Compute:     gasLimit,
		StateRead:   gasLimit,
		StateWrite:  gasLimit,
		ProtocolOps: gasLimit,
		ProofVerify: gasLimit,
	}
}

// resourceSaturatingAddU64 is a local copy of the saturating-add helper.
// We intentionally duplicate it here so this file has zero dependency on
// core/vm (which would create an import cycle: core/types -> core/vm ->
// core/types).
func resourceSaturatingAddU64(a, b uint64) uint64 {
	if math.MaxUint64-a < b {
		return math.MaxUint64
	}
	return a + b
}

// ---------------------------------------------------------------------------
// RLP — fixed 5-uint64 list, stable across platforms.
// ---------------------------------------------------------------------------

// resourceLimitsRLP is the on-wire shape. It is a tuple of five uint64s in
// CANONICAL order: compute, stateRead, stateWrite, protocolOps, proofVerify.
// Changing the order is a hard fork.
type resourceLimitsRLP struct {
	Compute     uint64
	StateRead   uint64
	StateWrite  uint64
	ProtocolOps uint64
	ProofVerify uint64
}

// EncodeRLP serializes the limits as a 5-element list. The fields are
// emitted in the canonical NIP-0004 order; rlpgen would produce identical
// bytes if it ran on this struct, but we implement it manually so the
// canonical order is locked into source.
func (r ResourceLimits) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, resourceLimitsRLP{
		Compute:     r.Compute,
		StateRead:   r.StateRead,
		StateWrite:  r.StateWrite,
		ProtocolOps: r.ProtocolOps,
		ProofVerify: r.ProofVerify,
	})
}

// DecodeRLP reads a 5-element list back into the limits. A list with the
// wrong number of elements is a hard error: header decoders that already
// know a ResourceLimits field is present will reject the block.
func (r *ResourceLimits) DecodeRLP(s *rlp.Stream) error {
	var raw resourceLimitsRLP
	if err := s.Decode(&raw); err != nil {
		return err
	}
	r.Compute = raw.Compute
	r.StateRead = raw.StateRead
	r.StateWrite = raw.StateWrite
	r.ProtocolOps = raw.ProtocolOps
	r.ProofVerify = raw.ProofVerify
	return nil
}

// ---------------------------------------------------------------------------
// JSON — hex-encoded uint64s so the wire shape matches eth_getBlockByNumber.
// ---------------------------------------------------------------------------

type resourceLimitsJSON struct {
	Compute     hexutil.Uint64 `json:"compute"`
	StateRead   hexutil.Uint64 `json:"stateRead"`
	StateWrite  hexutil.Uint64 `json:"stateWrite"`
	ProtocolOps hexutil.Uint64 `json:"protocolOps"`
	ProofVerify hexutil.Uint64 `json:"proofVerify"`
}

// MarshalJSON renders the limits as a JSON object of hex-prefixed uint64s.
// The shape matches existing block-header fields like gasLimit.
func (r ResourceLimits) MarshalJSON() ([]byte, error) {
	return json.Marshal(resourceLimitsJSON{
		Compute:     hexutil.Uint64(r.Compute),
		StateRead:   hexutil.Uint64(r.StateRead),
		StateWrite:  hexutil.Uint64(r.StateWrite),
		ProtocolOps: hexutil.Uint64(r.ProtocolOps),
		ProofVerify: hexutil.Uint64(r.ProofVerify),
	})
}

// UnmarshalJSON accepts the canonical JSON object shape. Missing fields are
// treated as zero. Extra fields are accepted (forward-compat).
func (r *ResourceLimits) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		*r = ResourceLimits{}
		return nil
	}
	var raw resourceLimitsJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	r.Compute = uint64(raw.Compute)
	r.StateRead = uint64(raw.StateRead)
	r.StateWrite = uint64(raw.StateWrite)
	r.ProtocolOps = uint64(raw.ProtocolOps)
	r.ProofVerify = uint64(raw.ProofVerify)
	return nil
}

// ErrResourceLimitsNil is returned when the validator finds a nil
// ResourceLimits pointer where Phase 10D requires it to be present (e.g.
// post-fork header.ResourceUsed).
var ErrResourceLimitsNil = errors.New("ResourceLimits: nil where required after Phase 10D fork")
