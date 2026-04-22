package core

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params/ethernova"
	coregeth "github.com/ethereum/go-ethereum/params/types/coregeth"
	"github.com/ethereum/go-ethereum/triedb"
	"github.com/holiman/uint256"
)

type eip658DummyChain struct{}

func (eip658DummyChain) Engine() consensus.Engine {
	return nil
}

func (eip658DummyChain) GetHeader(common.Hash, uint64) *types.Header {
	return nil
}

func makeEIP658Receipt(t *testing.T, blockNumber, eip658Fork uint64) *types.Receipt {
	t.Helper()

	cfg := &coregeth.CoreGethChainConfig{
		NetworkID:    ethernova.NewChainID,
		ChainID:      new(big.Int).SetUint64(ethernova.NewChainID),
		EIP155Block:  big.NewInt(0),
		EIP658FBlock: new(big.Int).SetUint64(eip658Fork),
	}

	db := rawdb.NewMemoryDatabase()
	triedb := triedb.NewDatabase(db, triedb.HashDefaults)
	statedb, err := state.New(common.Hash{}, state.NewDatabaseWithNodeDB(db, triedb), nil)
	if err != nil {
		t.Fatalf("failed to create state: %v", err)
	}

	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	from := crypto.PubkeyToAddress(key.PublicKey)
	statedb.SetBalance(from, uint256.NewInt(1_000_000_000_000_000_000))

	to := common.HexToAddress("0x0000000000000000000000000000000000000001")
	tx := types.NewTransaction(0, to, big.NewInt(0), 21000, big.NewInt(1), nil)
	signed, err := types.SignTx(tx, types.NewEIP155Signer(cfg.ChainID), key)
	if err != nil {
		t.Fatalf("failed to sign tx: %v", err)
	}

	header := &types.Header{
		Number:     new(big.Int).SetUint64(blockNumber),
		Time:       1,
		GasLimit:   8_000_000,
		Difficulty: big.NewInt(1),
		Coinbase:   common.Address{},
	}

	gasPool := new(GasPool).AddGas(header.GasLimit)
	usedGas := uint64(0)
	receipt, err := ApplyTransaction(cfg, eip658DummyChain{}, &header.Coinbase, gasPool, statedb, header, signed, &usedGas, vm.Config{})
	if err != nil {
		t.Fatalf("apply transaction failed: %v", err)
	}
	return receipt
}

func TestEthernovaEIP658ReceiptStatusTransition(t *testing.T) {
	// Hardcoded fork block > 0 so we can test the "pre-fork" branch. Using
	// ethernova.EIP658ForkBlock (= 0 on devnet) would underflow fork-1 to
	// MAX uint64 and the "pre-fork" half of this test would be untestable.
	// The EIP-658 transition logic itself is fork-block-independent; any
	// non-zero value exercises the same receipt-format switch.
	const fork uint64 = 100

	pre := makeEIP658Receipt(t, fork-1, fork)
	if len(pre.PostState) == 0 {
		t.Fatalf("expected post-state before fork %d", fork)
	}

	post := makeEIP658Receipt(t, fork, fork)
	if len(post.PostState) != 0 {
		t.Fatalf("expected no post-state at/after fork %d", fork)
	}
}
