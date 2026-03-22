package runtime

import (
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params/ethernova"
	coregeth "github.com/ethereum/go-ethereum/params/types/coregeth"
)

func ethernovaForkConfig(t *testing.T, forkBlock uint64) *coregeth.CoreGethChainConfig {
	t.Helper()
	cfg := &coregeth.CoreGethChainConfig{
		NetworkID: ethernova.NewChainID,
		ChainID:   new(big.Int).SetUint64(ethernova.NewChainID),
	}
	cfg.ConstantinopleBlock = new(big.Int).SetUint64(forkBlock)
	cfg.PetersburgBlock = new(big.Int).SetUint64(forkBlock)
	cfg.IstanbulBlock = new(big.Int).SetUint64(forkBlock)
	return cfg
}

func TestEthernovaForkActivation105000(t *testing.T) {
	fork := ethernova.EVMCompatibilityForkBlock
	cfg := ethernovaForkConfig(t, fork)

	pre := big.NewInt(int64(fork - 1))
	post := big.NewInt(int64(fork))

	if cfg.IsEnabled(cfg.GetEIP145Transition, pre) {
		t.Fatalf("eip145 should be disabled before fork %d", fork)
	}
	if !cfg.IsEnabled(cfg.GetEIP145Transition, post) {
		t.Fatalf("eip145 should be enabled at fork %d", fork)
	}
	if cfg.IsEnabled(cfg.GetEIP1283DisableTransition, pre) {
		t.Fatalf("petersburg should be disabled before fork %d", fork)
	}
	if !cfg.IsEnabled(cfg.GetEIP1283DisableTransition, post) {
		t.Fatalf("petersburg should be enabled at fork %d", fork)
	}
	if cfg.IsEnabled(cfg.GetEIP1344Transition, pre) {
		t.Fatalf("eip1344 should be disabled before fork %d", fork)
	}
	if !cfg.IsEnabled(cfg.GetEIP1344Transition, post) {
		t.Fatalf("eip1344 should be enabled at fork %d", fork)
	}
}

func TestEthernovaEVMOpcodesPrePostFork(t *testing.T) {
	fork := ethernova.EVMCompatibilityForkBlock
	cfg := ethernovaForkConfig(t, fork)

	shlCode := []byte{0x60, 0x01, 0x60, 0x02, 0x1b, 0x00} // PUSH1 1, PUSH1 2, SHL, STOP
	preCfg := &Config{ChainConfig: cfg, BlockNumber: big.NewInt(int64(fork - 1))}
	if _, _, err := Execute(shlCode, nil, preCfg); err == nil {
		t.Fatal("expected SHL to be invalid before fork")
	} else {
		var invalid *vm.ErrInvalidOpCode
		if !errors.As(err, &invalid) {
			t.Fatalf("expected invalid opcode before fork, got %v", err)
		}
	}

	postCfg := &Config{ChainConfig: cfg, BlockNumber: big.NewInt(int64(fork))}
	if _, _, err := Execute(shlCode, nil, postCfg); err != nil {
		t.Fatalf("expected SHL to succeed after fork, got %v", err)
	}

	chainIDCode := []byte{0x46, 0x00} // CHAINID, STOP
	if _, _, err := Execute(chainIDCode, nil, preCfg); err == nil {
		t.Fatal("expected CHAINID to be invalid before fork")
	} else {
		var invalid *vm.ErrInvalidOpCode
		if !errors.As(err, &invalid) {
			t.Fatalf("expected invalid opcode before fork, got %v", err)
		}
	}
	if _, _, err := Execute(chainIDCode, nil, &Config{ChainConfig: cfg, BlockNumber: big.NewInt(int64(fork))}); err != nil {
		t.Fatalf("expected CHAINID to succeed after fork, got %v", err)
	}

	selfBalanceCode := []byte{0x47, 0x00} // SELFBALANCE, STOP
	if _, _, err := Execute(selfBalanceCode, nil, preCfg); err == nil {
		t.Fatal("expected SELFBALANCE to be invalid before fork")
	} else {
		var invalid *vm.ErrInvalidOpCode
		if !errors.As(err, &invalid) {
			t.Fatalf("expected invalid opcode before fork, got %v", err)
		}
	}
	if _, _, err := Execute(selfBalanceCode, nil, &Config{ChainConfig: cfg, BlockNumber: big.NewInt(int64(fork))}); err != nil {
		t.Fatalf("expected SELFBALANCE to succeed after fork, got %v", err)
	}
}
