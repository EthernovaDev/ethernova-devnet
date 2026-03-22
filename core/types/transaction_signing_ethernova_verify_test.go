package types

import (
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params/ethernova"
	coregeth "github.com/ethereum/go-ethereum/params/types/coregeth"
)

func TestVerifyChainIDGate(t *testing.T) {
	key, _ := crypto.GenerateKey()
	addr := crypto.PubkeyToAddress(key.PublicKey)

	cfg := &coregeth.CoreGethChainConfig{
		ChainID:     new(big.Int).Set(ethernova.NewChainIDBig),
		EIP155Block: big.NewInt(0),
	}

	tx := NewTransaction(0, common.HexToAddress("0x1"), big.NewInt(1), 21000, big.NewInt(1), nil)
	wrongSigned, err := SignTx(tx, NewEIP155Signer(big.NewInt(1)), key)
	if err != nil {
		t.Fatalf("sign wrong chainId tx: %v", err)
	}
	correctSigned, err := SignTx(tx, NewEIP155Signer(new(big.Int).Set(ethernova.NewChainIDBig)), key)
	if err != nil {
		t.Fatalf("sign correct chainId tx: %v", err)
	}

	signer := MakeSigner(cfg, big.NewInt(0), 0)
	if _, err := Sender(signer, wrongSigned); !errors.Is(err, ErrInvalidChainId) {
		t.Fatalf("wrong chainId: expected ErrInvalidChainId, got %v", err)
	}
	if from, err := Sender(signer, correctSigned); err != nil || from != addr {
		t.Fatalf("correct chainId: from=%s err=%v", from, err)
	}
}

// TestLegacyChainIDAccepted verifies that transactions signed with the legacy
// chain ID 77777 are accepted when the signer is configured for 121525.
// This is required so that new nodes can sync historical blocks from block 0.
func TestLegacyChainIDAccepted(t *testing.T) {
	key, _ := crypto.GenerateKey()
	addr := crypto.PubkeyToAddress(key.PublicKey)

	cfg := &coregeth.CoreGethChainConfig{
		ChainID:     new(big.Int).Set(ethernova.NewChainIDBig),
		EIP155Block: big.NewInt(0),
	}

	tx := NewTransaction(0, common.HexToAddress("0x1"), big.NewInt(1), 21000, big.NewInt(1), nil)
	legacySigned, err := SignTx(tx, NewEIP155Signer(new(big.Int).SetUint64(ethernova.LegacyChainID)), key)
	if err != nil {
		t.Fatalf("sign legacy chainId tx: %v", err)
	}

	signer := MakeSigner(cfg, big.NewInt(0), 0)
	from, err := Sender(signer, legacySigned)
	if err != nil {
		t.Fatalf("legacy chainId should be accepted, got error: %v", err)
	}
	if from != addr {
		t.Fatalf("legacy chainId: sender mismatch got=%s want=%s", from, addr)
	}
}

// TestLegacyChainIDAcceptedWithEIP1559Signer verifies the legacy chain ID
// acceptance works through the full eip1559Signer delegation chain.
// This matches the actual Ethernova genesis config where EIP1559FBlock=0.
func TestLegacyChainIDAcceptedWithEIP1559Signer(t *testing.T) {
	key, _ := crypto.GenerateKey()
	addr := crypto.PubkeyToAddress(key.PublicKey)

	eip1559Block := big.NewInt(0)
	cfg := &coregeth.CoreGethChainConfig{
		ChainID:      new(big.Int).Set(ethernova.NewChainIDBig),
		EIP155Block:  big.NewInt(0),
		EIP1559FBlock: eip1559Block,
	}

	tx := NewTransaction(0, common.HexToAddress("0x1"), big.NewInt(1), 21000, big.NewInt(1), nil)
	legacySigned, err := SignTx(tx, NewEIP155Signer(new(big.Int).SetUint64(ethernova.LegacyChainID)), key)
	if err != nil {
		t.Fatalf("sign legacy chainId tx: %v", err)
	}

	// MakeSigner with EIP1559FBlock=0 creates eip1559Signer wrapping dualSigner
	signer := MakeSigner(cfg, big.NewInt(3986), 0)
	from, err := Sender(signer, legacySigned)
	if err != nil {
		t.Fatalf("legacy chainId with eip1559 signer should be accepted, got error: %v", err)
	}
	if from != addr {
		t.Fatalf("legacy chainId with eip1559 signer: sender mismatch got=%s want=%s", from, addr)
	}

	// Also verify that new chain ID 121525 still works
	correctSigned, err := SignTx(tx, NewEIP155Signer(new(big.Int).Set(ethernova.NewChainIDBig)), key)
	if err != nil {
		t.Fatalf("sign correct chainId tx: %v", err)
	}
	from, err = Sender(signer, correctSigned)
	if err != nil {
		t.Fatalf("correct chainId with eip1559 signer: err=%v", err)
	}
	if from != addr {
		t.Fatalf("correct chainId with eip1559 signer: sender mismatch got=%s want=%s", from, addr)
	}
}
