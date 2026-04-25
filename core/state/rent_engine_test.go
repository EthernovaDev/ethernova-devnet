// Ethernova: Rent Engine unit tests (NIP-0004 Phase 3)
//
// These tests validate the deterministic integer math behind per-epoch
// rent deduction. They do NOT touch state — rent_engine.go is pure
// arithmetic by design. If any of these fail, Phase 3 will produce
// divergent state roots across nodes, so treat regressions as release-
// blocking.

package state

import (
	"math/big"
	"testing"
)

func TestIsRentEpochBoundary(t *testing.T) {
	tests := []struct {
		name  string
		block uint64
		epoch uint64
		want  bool
	}{
		{"block 0 is never a boundary", 0, 10000, false},
		{"block 10000 with epoch 10000 is a boundary", 10000, 10000, true},
		{"block 10001 with epoch 10000 is not", 10001, 10000, false},
		{"block 20000 with epoch 10000 is a boundary", 20000, 10000, true},
		{"epoch 0 degenerate case", 100, 0, false},
		{"block 1 with epoch 1 is a boundary", 1, 1, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsRentEpochBoundary(tc.block, tc.epoch)
			if got != tc.want {
				t.Errorf("IsRentEpochBoundary(%d, %d) = %v, want %v",
					tc.block, tc.epoch, got, tc.want)
			}
		})
	}
}

func TestEpochsElapsed(t *testing.T) {
	tests := []struct {
		name                         string
		fromBlock, toBlock, epochLen uint64
		want                         uint64
	}{
		{"no elapsed when to == from", 10, 10, 10, 0},
		{"one epoch crossed (0 → 10)", 0, 10, 10, 1},
		{"two epochs crossed (0 → 20)", 0, 20, 10, 2},
		{"two epochs across offset (5 → 25)", 5, 25, 10, 2},
		{"no boundary touched (10 → 11)", 10, 11, 10, 0},
		{"one boundary after last touch (10 → 20)", 10, 20, 10, 1},
		{"inverted range is zero", 20, 5, 10, 0},
		{"epoch 0 is zero (safe)", 0, 100, 0, 0},
		{"realistic devnet values (9999 → 10005)", 9999, 10005, 10000, 1},
		{"realistic devnet values (10000 → 10001)", 10000, 10001, 10000, 0},
		{"large gap (0 → 100000)", 0, 100000, 10000, 10},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := EpochsElapsed(tc.fromBlock, tc.toBlock, tc.epochLen)
			if got != tc.want {
				t.Errorf("EpochsElapsed(%d, %d, %d) = %d, want %d",
					tc.fromBlock, tc.toBlock, tc.epochLen, got, tc.want)
			}
		})
	}
}

func TestComputeEpochRentWei(t *testing.T) {
	tests := []struct {
		name                string
		rate, size, epochLen uint64
		want                string // decimal string because values can exceed uint64
	}{
		{"zero size → zero rent", 1, 0, 10000, "0"},
		{"zero rate → zero rent", 0, 1024, 10000, "0"},
		{"1 byte at rate 1 for 10000 blocks", 1, 1, 10000, "10000"},
		{"1024 bytes at rate 1 for 10000 blocks", 1, 1024, 10000, "10240000"},
		{"rate 100 × 1MB × 10000 blocks", 100, 1 << 20, 10000, "1048576000000"},
		{"MaxContentRefSize × rate 1 × 10000", 1, 1 << 32, 10000, "42949672960000"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ComputeEpochRentWei(tc.rate, tc.size, tc.epochLen)
			want, _ := new(big.Int).SetString(tc.want, 10)
			if got.Cmp(want) != 0 {
				t.Errorf("ComputeEpochRentWei(%d, %d, %d) = %s, want %s",
					tc.rate, tc.size, tc.epochLen, got.String(), want.String())
			}
		})
	}
}

func TestComputeAccruedRentWei(t *testing.T) {
	// At rate=1, size=1024, epoch=10000:
	//   per-epoch cost = 10_240_000 wei
	//
	// From block 0 to block 30000 (3 full epoch boundaries) → 30_720_000 wei.
	rate, size, epoch := uint64(1), uint64(1024), uint64(10000)
	got := ComputeAccruedRentWei(rate, size, epoch, 0, 30000)
	want := new(big.Int).SetUint64(30720000)
	if got.Cmp(want) != 0 {
		t.Errorf("ComputeAccruedRentWei = %s, want %s", got.String(), want.String())
	}

	// Mid-range: from 15000 (no boundary between 10000 and 20000 already
	// counted) → 25000 should count only boundary 20000 = 1 epoch.
	got2 := ComputeAccruedRentWei(rate, size, epoch, 15000, 25000)
	want2 := new(big.Int).SetUint64(10240000)
	if got2.Cmp(want2) != 0 {
		t.Errorf("mid-range accrued = %s, want %s", got2.String(), want2.String())
	}

	// No boundaries → zero.
	got3 := ComputeAccruedRentWei(rate, size, epoch, 10001, 10999)
	if got3.Sign() != 0 {
		t.Errorf("no-boundary accrued should be zero, got %s", got3.String())
	}
}

// TestDeterminismGoldenValues pins a handful of computed values so any
// accidental formula change (e.g. someone swaps multiplication order or
// introduces a rounding step) surfaces immediately in CI.
func TestDeterminismGoldenValues(t *testing.T) {
	cases := []struct {
		rate, size, epoch uint64
		want              string
	}{
		{1, 1, 10000, "10000"},
		{1, 1024, 10000, "10240000"},
		{7, 123456, 10000, "8641920000"},
		{1, 1 << 16, 10000, "655360000"},
	}
	for i, tc := range cases {
		got := ComputeEpochRentWei(tc.rate, tc.size, tc.epoch).String()
		if got != tc.want {
			t.Fatalf("case %d: ComputeEpochRentWei(%d, %d, %d) = %s, want %s",
				i, tc.rate, tc.size, tc.epoch, got, tc.want)
		}
	}
}
