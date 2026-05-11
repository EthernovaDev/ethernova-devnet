package vm

import (
	"math"
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

func TestPhase10BResourcePricing(t *testing.T) {
	vector := ResourceVector{
		Compute:     1000,
		StateRead:   100,
		StateWrite:  50,
		ProtocolOps: 20,
		ProofVerify: 10,
	}
	got := PriceResourceVector(vector, Phase10BResourcePrices())
	want := ResourceCharge{
		Compute:     1000,
		StateRead:   200,
		StateWrite:  200,
		ProtocolOps: 20,
		ProofVerify: 30,
		Total:       1450,
	}
	if got != want {
		t.Fatalf("unexpected Phase 10B charge: got %+v want %+v", got, want)
	}
}

func TestPriceResourceVectorSaturates(t *testing.T) {
	got := PriceResourceVector(
		ResourceVector{Compute: math.MaxUint64, StateRead: 2},
		ResourcePrices{Compute: 2, StateRead: math.MaxUint64},
	)
	if got.Compute != math.MaxUint64 {
		t.Fatalf("compute charge should saturate, got %d", got.Compute)
	}
	if got.Total != math.MaxUint64 {
		t.Fatalf("total charge should saturate, got %d", got.Total)
	}
}

func TestPriceResourceVectorBips(t *testing.T) {
	vector := ResourceVector{
		Compute:     1000,
		StateRead:   100,
		StateWrite:  50,
		ProtocolOps: 20,
		ProofVerify: 10,
	}
	got := PriceResourceVectorBips(vector, Phase10CBasePriceBips())
	if got.Total != 1450 {
		t.Fatalf("unexpected bips quote total: got %d want 1450 (%+v)", got.Total, got)
	}
}

func TestAdaptiveResourcePricingIsolation(t *testing.T) {
	pricer := NewAdaptiveResourcePricer()
	target := ResourceTargetForBlockGas(30_000_000)
	base := Phase10CBasePriceBips()
	usage := target
	usage.Compute = target.Compute * 2

	pricer.RecordBlock(10, 30_000_000, usage)
	snap := pricer.Snapshot()
	if snap.CurrentPriceBips.Compute <= base.Compute {
		t.Fatalf("compute price should increase under compute congestion: got %d base %d", snap.CurrentPriceBips.Compute, base.Compute)
	}
	if snap.CurrentPriceBips.ProtocolOps != base.ProtocolOps {
		t.Fatalf("protocol_ops price should not move from compute congestion: got %d base %d", snap.CurrentPriceBips.ProtocolOps, base.ProtocolOps)
	}
	if snap.CurrentPriceBips.StateWrite != base.StateWrite {
		t.Fatalf("state_write price should remain isolated: got %d base %d", snap.CurrentPriceBips.StateWrite, base.StateWrite)
	}
}

func TestAdaptiveResourcePricingClampsToBase(t *testing.T) {
	pricer := NewAdaptiveResourcePricer()
	pricer.RecordBlock(1, 30_000_000, ResourceVector{})
	snap := pricer.Snapshot()
	base := Phase10CBasePriceBips()
	if snap.CurrentPriceBips != base {
		t.Fatalf("empty blocks should not decay below base: got %+v want %+v", snap.CurrentPriceBips, base)
	}
}
