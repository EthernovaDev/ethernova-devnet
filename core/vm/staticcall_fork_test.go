package vm

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/params/types/coregeth"
)

func TestStaticcallFork(t *testing.T) {
	const forkBlock = 70000
	cfg := &coregeth.CoreGethChainConfig{
		ChainID:      big.NewInt(121525),
		NetworkID:    121525,
		EIP214FBlock: big.NewInt(forkBlock),
	}

	preFork := big.NewInt(forkBlock - 1)
	postFork := big.NewInt(forkBlock)

	jtPre, err := LookupInstructionSet(cfg, preFork, nil)
	if err != nil {
		t.Fatalf("pre-fork lookup failed: %v", err)
	}
	if jtPre[STATICCALL].HasCost() {
		t.Fatalf("expected STATICCALL disabled before fork %d", forkBlock)
	}

	jtPost, err := LookupInstructionSet(cfg, postFork, nil)
	if err != nil {
		t.Fatalf("post-fork lookup failed: %v", err)
	}
	if !jtPost[STATICCALL].HasCost() {
		t.Fatalf("expected STATICCALL enabled at fork %d", forkBlock)
	}
}
