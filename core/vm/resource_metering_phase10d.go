// Copyright 2026 The Ethernova Authors
//
// NIP-0004 Phase 10D — Multi-Dimensional Resource Metering: enforcement
// helpers. This file is intentionally kept SEPARATE from the
// Phase 10A/B/C surface in resource_metering.go so the quote layer and the
// consensus enforcement layer can be reasoned about independently.
//
// Importers:
//   - core/state_transition.go — calls EnforceVectorAgainstLimits after
//     transaction execution.
//   - core/state_processor.go  — calls IsResourceMeteringActive to gate
//     header field validation.
//   - miner/worker.go         — calls IsResourceMeteringActive to know
//     when to fill header.ResourceUsed / header.ResourceBasePrice.

package vm

import "github.com/ethereum/go-ethereum/params/ethernova"

// IsResourceMeteringActive reports whether the Phase 10D consensus
// enforcement is active at blockNumber. On devnet the fork block is 0,
// which means every block is post-fork. The function is a single uint64
// comparison so it is safe to call on a per-step hot path.
func IsResourceMeteringActive(blockNumber uint64) bool {
	return blockNumber >= ethernova.ResourceMeteringForkBlock
}

// IsResourceMeteringGrace reports whether the chain is inside the
// post-fork grace window where vectors are computed and recorded into the
// header but per-dimension OOR errors are NOT raised. The grace window
// lets a freshly upgraded fleet observe real usage before flipping the
// kill switch. On devnet the window is zero (hard activation).
func IsResourceMeteringGrace(blockNumber uint64) bool {
	if blockNumber < ethernova.ResourceMeteringForkBlock {
		return false
	}
	return blockNumber < ethernova.ResourceMeteringForkBlock+ethernova.ResourceMeteringTransitionGracePostFork
}

// LimitVector is a thin tuple of the five per-dimension caps that
// state_transition.go derives from a transaction. It mirrors
// types.ResourceLimits but stays in the vm package to avoid an import
// cycle when state_transition.go wants to call the enforcer.
type LimitVector struct {
	Compute     uint64
	StateRead   uint64
	StateWrite  uint64
	ProtocolOps uint64
	ProofVerify uint64
}

// EnforceVectorAgainstLimits returns the FIRST per-dimension overflow
// error encountered when comparing used against limit. The error matches
// the dimension exactly so the receipt/log layer can attribute the
// failure to the correct counter.
//
// Dimension priority for error reporting: compute, state_read,
// state_write, protocol_ops, proof_verify. This matches the canonical
// NIP-0004 §6.1 order so error logs are deterministic across nodes.
//
// A zero limit on a dimension is treated as "no limit" so this function
// can be called pre-fork with an all-zero LimitVector and never trigger.
func EnforceVectorAgainstLimits(used ResourceVector, limit LimitVector) error {
	if limit.Compute != 0 && used.Compute > limit.Compute {
		return ErrOutOfResourceCompute
	}
	if limit.StateRead != 0 && used.StateRead > limit.StateRead {
		return ErrOutOfResourceStateRead
	}
	if limit.StateWrite != 0 && used.StateWrite > limit.StateWrite {
		return ErrOutOfResourceStateWrite
	}
	if limit.ProtocolOps != 0 && used.ProtocolOps > limit.ProtocolOps {
		return ErrOutOfResourceProtocolOps
	}
	if limit.ProofVerify != 0 && used.ProofVerify > limit.ProofVerify {
		return ErrOutOfResourceProofVerify
	}
	return nil
}

// DeriveLegacyLimits returns the deterministic Phase 10D limits for a
// legacy or EIP-1559 transaction. Every dimension is set to gasLimit so
// any tx that succeeded pre-fork continues to succeed post-fork — the
// per-dimension cap is never tighter than the total gas budget.
//
// This is the SAME mapping as types.LegacyGasToResourceLimitsEnforced but
// returned as a LimitVector for direct consumption by the enforcer.
func DeriveLegacyLimits(gasLimit uint64) LimitVector {
	return LimitVector{
		Compute:     gasLimit,
		StateRead:   gasLimit,
		StateWrite:  gasLimit,
		ProtocolOps: gasLimit,
		ProofVerify: gasLimit,
	}
}
