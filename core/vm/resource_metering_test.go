package vm

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestLegacyGasToResourceLimits(t *testing.T) {
	got := LegacyGasToResourceLimits(3_000_000)
	want := ResourceVector{
		Compute:     3_000_000,
		StateRead:   1_000_000,
		StateWrite:  500_000,
		ProtocolOps: 200_000,
		ProofVerify: 100_000,
	}
	if got != want {
		t.Fatalf("unexpected resource limits: got %+v want %+v", got, want)
	}
}

func TestResourceVectorFromExecution(t *testing.T) {
	tc := &TraceCounters{
		TotalOpsExecuted: 10,
		SloadCount:       2,
		SstoreCount:      1,
		ExtCodeCount:     1,
		CreateCount:      1,
	}
	precompile := ResourceVector{ProtocolOps: 5000, ProofVerify: 7000}
	got := ResourceVectorFromExecution(tc, precompile, 80_000, 21_000)
	want := ResourceVector{
		Compute:     59_000,
		StateRead:   4_900,
		StateWrite:  52_000,
		ProtocolOps: 5_000,
		ProofVerify: 7_000,
	}
	if got != want {
		t.Fatalf("unexpected resource vector: got %+v want %+v", got, want)
	}
}

func TestResourceMeterPrecompileClassification(t *testing.T) {
	var meter ResourceMeter
	meter.RecordPrecompile(common.BytesToAddress([]byte{0x29}), []byte{0x01}, 20_000)
	meter.RecordPrecompile(common.BytesToAddress([]byte{0x2f}), []byte{0x01}, 5_000)
	meter.RecordPrecompile(common.BytesToAddress([]byte{0x2d}), []byte{0x02}, 25_000)
	got := meter.Vector()
	if got.ProtocolOps != 20_000 {
		t.Fatalf("protocol_ops mismatch: got %d", got.ProtocolOps)
	}
	if got.ProofVerify != 30_000 {
		t.Fatalf("proof_verify mismatch: got %d", got.ProofVerify)
	}
}
