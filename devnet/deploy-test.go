package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

func main() {
	// Connect to node
	client, err := ethclient.Dial("http://192.168.1.15:8551")
	if err != nil {
		log.Fatal(err)
	}

	// Generate a throwaway key for deploying
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		log.Fatal(err)
	}
	publicKey := privateKey.Public().(*ecdsa.PublicKey)
	fromAddr := crypto.PubkeyToAddress(*publicKey)
	fmt.Printf("Deploy address: %s\n", fromAddr.Hex())

	// Send some ETH to the deploy address from a funded account
	// We'll use the geth console for this
	fmt.Printf("Fund this address first, then press enter...\n")

	// Simple contract bytecode: stores a counter, increment() and heavyMath(n)
	// Solidity equivalent:
	//   uint256 public counter;
	//   function increment() { counter++; }
	//   function heavyMath(uint n) returns (uint) { uint r=1; for(uint i=0;i<n;i++) r=r*3+i; counter++; return r; }
	
	// For now, just send transactions to generate opcode activity
	chainID := big.NewInt(121526)
	nonce := uint64(0)
	
	for i := 0; i < 100; i++ {
		tx := types.NewTransaction(
			nonce,
			common.HexToAddress("0x2222222222222222222222222222222222222222"),
			big.NewInt(1000000000), // 1 gwei
			21000,
			big.NewInt(1000000000),
			nil,
		)
		signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
		if err != nil {
			log.Fatal(err)
		}
		err = client.SendTransaction(context.Background(), signedTx)
		if err != nil {
			fmt.Printf("tx %d error: %v\n", i, err)
			break
		}
		nonce++
		if i%10 == 0 {
			fmt.Printf("Sent %d txs\n", i)
			time.Sleep(time.Second)
		}
	}
}
