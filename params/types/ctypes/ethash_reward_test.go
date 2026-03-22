package ctypes_test

import (
	"math/big"
	"testing"

	coregeth "github.com/ethereum/go-ethereum/params/types/coregeth"
	"github.com/ethereum/go-ethereum/params/types/ctypes"
	"github.com/holiman/uint256"
)

func mustU256(dec string) *uint256.Int {
	b, ok := new(big.Int).SetString(dec, 10)
	if !ok {
		panic("invalid decimal")
	}
	u, _ := uint256.FromBig(b)
	return u
}

func TestEthashBlockRewardHalvingSchedule(t *testing.T) {
	blocksPerYear := uint64(2_102_400) // ~15s blocks
	schedule := ctypes.Uint64Uint256MapEncodesHex{
		0:                 mustU256("10000000000000000000"), // 10 NOVA
		blocksPerYear:     mustU256("5000000000000000000"),  // 5
		blocksPerYear * 2: mustU256("2500000000000000000"),  // 2.5
		blocksPerYear * 3: mustU256("1250000000000000000"),  // 1.25
		blocksPerYear * 4: mustU256("1000000000000000000"),  // floor 1
	}

	cfg := &coregeth.CoreGethChainConfig{
		ChainID:             big.NewInt(121525),
		NetworkID:           121525,
		Ethash:              new(ctypes.EthashConfig),
		BlockRewardSchedule: schedule,
	}

	tests := []struct {
		block uint64
		want  *uint256.Int
	}{
		{0, schedule[0]},
		{blocksPerYear - 1, schedule[0]},
		{blocksPerYear, schedule[blocksPerYear]},
		{blocksPerYear*2 + 1_000_000, schedule[blocksPerYear*2]},
		{blocksPerYear*3 + 10, schedule[blocksPerYear*3]},
		{blocksPerYear*5 + 1234, schedule[blocksPerYear*4]}, // after floor, stay at 1
	}

	for _, tt := range tests {
		got := ctypes.EthashBlockReward(cfg, new(big.Int).SetUint64(tt.block))
		if tt.want.Cmp(got) != 0 {
			t.Fatalf("block %d: expected %s, got %s", tt.block, tt.want.String(), got.String())
		}
	}
}
