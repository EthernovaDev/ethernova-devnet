package ethernova

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestForkEnforcementDecisionMainnet(t *testing.T) {
	decision := ForkEnforcementDecision(new(big.Int).Set(NewChainIDBig), ExpectedGenesisHash)
	if decision.Block != EVMCompatibilityForkBlock {
		t.Fatalf("expected enforcement %d, got %d", EVMCompatibilityForkBlock, decision.Block)
	}
	if decision.Warning != "" {
		t.Fatalf("unexpected warning: %s", decision.Warning)
	}
}

func TestForkEnforcementDecisionMainnetGenesisMismatchWarns(t *testing.T) {
	badGenesis := common.HexToHash("0x1234")
	decision := ForkEnforcementDecision(new(big.Int).Set(NewChainIDBig), badGenesis)
	if decision.Block != EVMCompatibilityForkBlock {
		t.Fatalf("expected enforcement %d, got %d", EVMCompatibilityForkBlock, decision.Block)
	}
	if decision.Warning == "" {
		t.Fatalf("expected warning on genesis mismatch")
	}
}

func TestForkEnforcementDecisionLegacyChain(t *testing.T) {
	decision := ForkEnforcementDecision(new(big.Int).SetUint64(LegacyChainID), LegacyGenesisHash)
	if decision.Block != LegacyForkEnforcementBlock {
		t.Fatalf("expected enforcement %d, got %d", LegacyForkEnforcementBlock, decision.Block)
	}
}

func TestForkEnforcementDecisionUnknownChain(t *testing.T) {
	decision := ForkEnforcementDecision(big.NewInt(424242), common.HexToHash("0xdeadbeef"))
	if decision.Block == LegacyForkEnforcementBlock {
		t.Fatalf("unknown chain should not default to legacy enforcement %d", LegacyForkEnforcementBlock)
	}
	if decision.Block != 0 {
		t.Fatalf("expected enforcement 0 for unknown chain, got %d", decision.Block)
	}
	if decision.Warning == "" {
		t.Fatalf("expected warning for unknown chain")
	}
}
