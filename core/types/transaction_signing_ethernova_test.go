package types

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params/ethernova"
	coregeth "github.com/ethereum/go-ethereum/params/types/coregeth"
)

func TestEthernovaChainID(t *testing.T) {
	key, _ := crypto.GenerateKey()
	addr := crypto.PubkeyToAddress(key.PublicKey)

	cfg := &coregeth.CoreGethChainConfig{
		ChainID:     new(big.Int).Set(ethernova.NewChainIDBig),
		EIP155Block: big.NewInt(0),
	}

	tx := NewTransaction(0, common.HexToAddress("0x1"), big.NewInt(1), 21000, big.NewInt(1), nil)
	signed, err := SignTx(tx, NewEIP155Signer(new(big.Int).Set(ethernova.NewChainIDBig)), key)
	if err != nil {
		t.Fatalf("sign chainId tx: %v", err)
	}

	for _, block := range []*big.Int{big.NewInt(0), big.NewInt(105000)} {
		signer := MakeSigner(cfg, block, 0)
		if from, err := Sender(signer, signed); err != nil || from != addr {
			t.Fatalf("chainId signer at block %v: from=%s err=%v", block, from, err)
		}
	}
}
