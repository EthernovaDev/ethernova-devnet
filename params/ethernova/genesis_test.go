package ethernova

import "testing"

func TestEthernovaMegaForkEIP150Transition(t *testing.T) {
	genesis := MustGenesis()
	tr := genesis.Config.GetEIP150Transition()
	if tr == nil {
		t.Fatalf("expected eip150 transition to be set for chainId 121525")
	}
	if *tr != MegaForkBlock {
		t.Fatalf("unexpected eip150 transition: have %d want %d", *tr, MegaForkBlock)
	}
}
