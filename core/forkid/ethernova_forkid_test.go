package forkid

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/params/ethernova"
	coregeth "github.com/ethereum/go-ethereum/params/types/coregeth"
	"github.com/ethereum/go-ethereum/params/types/genesisT"
)

func TestEthernovaForkIDChangesWithForkBlock(t *testing.T) {
	baseCfg := &coregeth.CoreGethChainConfig{
		NetworkID: ethernova.NewChainID,
		ChainID:   new(big.Int).SetUint64(ethernova.NewChainID),
	}
	genesis := &genesisT.Genesis{
		Config:     baseCfg,
		GasLimit:   1,
		Difficulty: big.NewInt(1),
		Alloc:      genesisT.GenesisAlloc{},
	}
	genesisBlock := core.GenesisToBlock(genesis, nil)

	idNoFork := NewID(baseCfg, genesisBlock, 0, 0)

	forkCfg := &coregeth.CoreGethChainConfig{
		NetworkID:           ethernova.NewChainID,
		ChainID:             new(big.Int).SetUint64(ethernova.NewChainID),
		ConstantinopleBlock: new(big.Int).SetUint64(ethernova.EVMCompatibilityForkBlock),
		PetersburgBlock:     new(big.Int).SetUint64(ethernova.EVMCompatibilityForkBlock),
		IstanbulBlock:       new(big.Int).SetUint64(ethernova.EVMCompatibilityForkBlock),
	}
	idFork := NewID(forkCfg, genesisBlock, 0, 0)

	if idFork == idNoFork {
		t.Fatalf("forkid should change when fork block is configured")
	}
	if idFork.Next != ethernova.EVMCompatibilityForkBlock {
		t.Fatalf("unexpected forkid next: have %d want %d", idFork.Next, ethernova.EVMCompatibilityForkBlock)
	}
}

func TestEthernovaForkIDIncludesEIP658(t *testing.T) {
	baseCfg := &coregeth.CoreGethChainConfig{
		NetworkID:           ethernova.NewChainID,
		ChainID:             new(big.Int).SetUint64(ethernova.NewChainID),
		ConstantinopleBlock: new(big.Int).SetUint64(ethernova.EVMCompatibilityForkBlock),
		PetersburgBlock:     new(big.Int).SetUint64(ethernova.EVMCompatibilityForkBlock),
		IstanbulBlock:       new(big.Int).SetUint64(ethernova.EVMCompatibilityForkBlock),
	}
	genesis := &genesisT.Genesis{
		Config:     baseCfg,
		GasLimit:   1,
		Difficulty: big.NewInt(1),
		Alloc:      genesisT.GenesisAlloc{},
	}
	genesisBlock := core.GenesisToBlock(genesis, nil)

	cfgWith658 := &coregeth.CoreGethChainConfig{
		NetworkID:           ethernova.NewChainID,
		ChainID:             new(big.Int).SetUint64(ethernova.NewChainID),
		ConstantinopleBlock: new(big.Int).SetUint64(ethernova.EVMCompatibilityForkBlock),
		PetersburgBlock:     new(big.Int).SetUint64(ethernova.EVMCompatibilityForkBlock),
		IstanbulBlock:       new(big.Int).SetUint64(ethernova.EVMCompatibilityForkBlock),
		EIP658FBlock:        new(big.Int).SetUint64(ethernova.EIP658ForkBlock),
		EIP2FBlock:          new(big.Int).SetUint64(ethernova.MegaForkBlock),
		EIP7FBlock:          new(big.Int).SetUint64(ethernova.MegaForkBlock),
		EIP150Block:         new(big.Int).SetUint64(ethernova.MegaForkBlock),
		EIP160FBlock:        new(big.Int).SetUint64(ethernova.MegaForkBlock),
		EIP161FBlock:        new(big.Int).SetUint64(ethernova.MegaForkBlock),
		EIP170FBlock:        new(big.Int).SetUint64(ethernova.MegaForkBlock),
		EIP100FBlock:        new(big.Int).SetUint64(ethernova.MegaForkBlock),
		EIP140FBlock:        new(big.Int).SetUint64(ethernova.MegaForkBlock),
		EIP198FBlock:        new(big.Int).SetUint64(ethernova.MegaForkBlock),
		EIP211FBlock:        new(big.Int).SetUint64(ethernova.MegaForkBlock),
		EIP212FBlock:        new(big.Int).SetUint64(ethernova.MegaForkBlock),
		EIP213FBlock:        new(big.Int).SetUint64(ethernova.MegaForkBlock),
		EIP214FBlock:        new(big.Int).SetUint64(ethernova.MegaForkBlock),
		EIP1706FBlock:       new(big.Int).SetUint64(ethernova.MegaForkBlock),
	}

	idBase := NewID(baseCfg, genesisBlock, 0, 0)
	idWith := NewID(cfgWith658, genesisBlock, 0, 0)
	if idWith != idBase {
		t.Fatalf("forkid should be identical before the first fork")
	}
	if idWith.Next != ethernova.EVMCompatibilityForkBlock {
		t.Fatalf("unexpected forkid next: have %d want %d", idWith.Next, ethernova.EVMCompatibilityForkBlock)
	}

	idPostCompat := NewID(cfgWith658, genesisBlock, ethernova.EVMCompatibilityForkBlock, 0)
	if idPostCompat.Next != ethernova.EIP658ForkBlock {
		t.Fatalf("unexpected forkid next after 105000: have %d want %d", idPostCompat.Next, ethernova.EIP658ForkBlock)
	}

	idPost658 := NewID(cfgWith658, genesisBlock, ethernova.EIP658ForkBlock, 0)
	if idPost658.Next != ethernova.MegaForkBlock {
		t.Fatalf("unexpected forkid next after EIP-658: have %d want %d", idPost658.Next, ethernova.MegaForkBlock)
	}
}
