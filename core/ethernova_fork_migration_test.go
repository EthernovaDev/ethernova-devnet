package core

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/params/ethernova"
	"github.com/ethereum/go-ethereum/params/types/coregeth"
	"github.com/ethereum/go-ethereum/params/types/ctypes"
)

// TestMegaForkAppliedWhenEIP658Present is a regression test for a bug where
// the MegaFork (block 118200) was never applied if EIP658 was already present
// in the chain config. The root cause was an early return in the EIP658
// section of ethernovaPatchConfigIfNeeded:
//
//	if !missing {          // EIP658 already configured
//	    return updated, nil // ← skipped MegaFork entirely
//	}
//
// This test creates a config where Constantinople/Petersburg/Istanbul AND
// EIP658 are already set (simulating a node upgraded to v1.2.7+), but
// MegaFork fields (eip2FBlock, eip7FBlock, eip150Block, etc.) are nil.
// The patch function MUST still apply the MegaFork fields.
func TestMegaForkAppliedWhenEIP658Present(t *testing.T) {
	// Use hardcoded fork blocks > 0 so the "head < fork" patch path is
	// reachable. The devnet globals all default to 0 (genesis activation),
	// which would make this test's scenario impossible (head can't be
	// less than 0). The patch logic itself is identical regardless of
	// which specific fork block values are used.
	const (
		forkBlock   uint64 = 105000
		eip658Block uint64 = 110500
		megaBlock   uint64 = 118200
	)

	cfg := &coregeth.CoreGethChainConfig{
		ChainID:             big.NewInt(int64(ethernova.NewChainID)),
		NetworkID:           ethernova.NewChainID,
		Ethash:              &ctypes.EthashConfig{},
		ConstantinopleBlock: new(big.Int).SetUint64(forkBlock),
		PetersburgBlock:     new(big.Int).SetUint64(forkBlock),
		IstanbulBlock:       new(big.Int).SetUint64(forkBlock),
		EIP658FBlock:        new(big.Int).SetUint64(eip658Block),
		// MegaFork fields intentionally nil — this is the scenario the bug triggers
	}

	// Head is past the mega fork block, but before that would trigger
	// "UPGRADE REQUIRED". We use head < megaBlock so the patch is allowed.
	head := megaBlock - 1

	updated, err := ethernovaPatchConfigIfNeededForForks(cfg, head, forkBlock, eip658Block, megaBlock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !updated {
		t.Fatal("expected config to be updated (MegaFork fields applied), but got updated=false")
	}

	// Verify all MegaFork fields were set
	megaFields := map[string]*big.Int{
		"EIP2FBlock":    cfg.EIP2FBlock,
		"EIP7FBlock":    cfg.EIP7FBlock,
		"EIP150Block":   cfg.EIP150Block,
		"EIP160FBlock":  cfg.EIP160FBlock,
		"EIP161FBlock":  cfg.EIP161FBlock,
		"EIP170FBlock":  cfg.EIP170FBlock,
		"EIP100FBlock":  cfg.EIP100FBlock,
		"EIP140FBlock":  cfg.EIP140FBlock,
		"EIP198FBlock":  cfg.EIP198FBlock,
		"EIP211FBlock":  cfg.EIP211FBlock,
		"EIP212FBlock":  cfg.EIP212FBlock,
		"EIP213FBlock":  cfg.EIP213FBlock,
		"EIP214FBlock":  cfg.EIP214FBlock,
		"EIP1706FBlock": cfg.EIP1706FBlock,
	}
	for name, val := range megaFields {
		if val == nil {
			t.Errorf("MegaFork field %s is nil after patch — was not applied", name)
		} else if val.Uint64() != megaBlock {
			t.Errorf("MegaFork field %s = %d, want %d", name, val.Uint64(), megaBlock)
		}
	}
}

// TestAllForksAppliedFromScratch verifies that when all fork fields are nil,
// a single call to ethernovaPatchConfigIfNeededForForks applies all three
// stages: Constantinople/Petersburg/Istanbul, EIP658, and MegaFork.
func TestAllForksAppliedFromScratch(t *testing.T) {
	// Hardcoded fork blocks — see TestMegaForkAppliedWhenEIP658Present for
	// why we don't use the global ethernova.* constants here.
	const (
		forkBlock   uint64 = 105000
		eip658Block uint64 = 110500
		megaBlock   uint64 = 118200
	)

	cfg := &coregeth.CoreGethChainConfig{
		ChainID:   big.NewInt(int64(ethernova.NewChainID)),
		NetworkID: ethernova.NewChainID,
		Ethash:    &ctypes.EthashConfig{},
		// Everything nil
	}

	head := uint64(0) // before any fork

	updated, err := ethernovaPatchConfigIfNeededForForks(cfg, head, forkBlock, eip658Block, megaBlock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !updated {
		t.Fatal("expected config to be updated from scratch")
	}

	// Constantinople/Petersburg/Istanbul
	if cfg.ConstantinopleBlock == nil || cfg.ConstantinopleBlock.Uint64() != forkBlock {
		t.Errorf("ConstantinopleBlock not set correctly")
	}
	if cfg.PetersburgBlock == nil || cfg.PetersburgBlock.Uint64() != forkBlock {
		t.Errorf("PetersburgBlock not set correctly")
	}
	if cfg.IstanbulBlock == nil || cfg.IstanbulBlock.Uint64() != forkBlock {
		t.Errorf("IstanbulBlock not set correctly")
	}

	// EIP658
	if cfg.EIP658FBlock == nil || cfg.EIP658FBlock.Uint64() != eip658Block {
		t.Errorf("EIP658FBlock not set correctly")
	}

	// MegaFork
	if cfg.EIP2FBlock == nil || cfg.EIP2FBlock.Uint64() != megaBlock {
		t.Errorf("EIP2FBlock not set correctly")
	}
	if cfg.EIP150Block == nil || cfg.EIP150Block.Uint64() != megaBlock {
		t.Errorf("EIP150Block not set correctly")
	}
	if cfg.EIP214FBlock == nil || cfg.EIP214FBlock.Uint64() != megaBlock {
		t.Errorf("EIP214FBlock not set correctly")
	}
}

// TestNoUpdateWhenAllForksPresent verifies that if all forks are already
// configured correctly, no update is reported.
func TestNoUpdateWhenAllForksPresent(t *testing.T) {
	forkBlock := ethernova.EVMCompatibilityForkBlock
	eip658Block := ethernova.EIP658ForkBlock
	megaBlock := ethernova.MegaForkBlock

	cfg := &coregeth.CoreGethChainConfig{
		ChainID:             big.NewInt(int64(ethernova.NewChainID)),
		NetworkID:           ethernova.NewChainID,
		Ethash:              &ctypes.EthashConfig{},
		ConstantinopleBlock: new(big.Int).SetUint64(forkBlock),
		PetersburgBlock:     new(big.Int).SetUint64(forkBlock),
		IstanbulBlock:       new(big.Int).SetUint64(forkBlock),
		EIP658FBlock:        new(big.Int).SetUint64(eip658Block),
		EIP2FBlock:          new(big.Int).SetUint64(megaBlock),
		EIP7FBlock:          new(big.Int).SetUint64(megaBlock),
		EIP150Block:         new(big.Int).SetUint64(megaBlock),
		EIP160FBlock:        new(big.Int).SetUint64(megaBlock),
		EIP161FBlock:        new(big.Int).SetUint64(megaBlock),
		EIP170FBlock:        new(big.Int).SetUint64(megaBlock),
		EIP100FBlock:        new(big.Int).SetUint64(megaBlock),
		EIP140FBlock:        new(big.Int).SetUint64(megaBlock),
		EIP198FBlock:        new(big.Int).SetUint64(megaBlock),
		EIP211FBlock:        new(big.Int).SetUint64(megaBlock),
		EIP212FBlock:        new(big.Int).SetUint64(megaBlock),
		EIP213FBlock:        new(big.Int).SetUint64(megaBlock),
		EIP214FBlock:        new(big.Int).SetUint64(megaBlock),
		EIP1706FBlock:       new(big.Int).SetUint64(megaBlock),
	}

	head := megaBlock + 1000

	updated, err := ethernovaPatchConfigIfNeeded(cfg, head)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated {
		t.Fatal("expected no update when all forks are already present")
	}
}
