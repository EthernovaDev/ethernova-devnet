package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

var chainID = big.NewInt(121526)

func main() {
	rpcURL := os.Getenv("RPC_URL")
	if rpcURL == "" {
		rpcURL = "http://127.0.0.1:28545"
	}
	keyHex := strings.TrimPrefix(os.Getenv("PRIVATE_KEY"), "0x")
	if keyHex == "" {
		fmt.Println("PRIVATE_KEY env required")
		return
	}

	key, _ := crypto.HexToECDSA(keyHex)
	from := crypto.PubkeyToAddress(key.PublicKey)
	client, _ := ethclient.Dial(rpcURL)
	defer client.Close()

	bal, _ := client.BalanceAt(context.Background(), from, nil)
	nova := new(big.Int).Div(bal, new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	fmt.Printf("Account: %s (%s NOVA)\n\n", from.Hex(), nova.String())

	token := common.HexToAddress("0xd6Dc5b3E9CEF3c4117fFd32F138717bBc0f8d91c")
	nft := common.HexToAddress("0xa407ABC46D71A56fb4fAc2Ae9CA1F599A2270C2a")
	multisig := common.HexToAddress("0x24fcDc40BFa6e8Fce87ACF50da1e69a36019083f")
	target := common.HexToAddress("0x818c1965E44A033115666F47DFF1752C656652C2")

	transferData, _ := hex.DecodeString("a9059cbb000000000000000000000000818c1965e44a033115666f47dff1752c656652c20000000000000000000000000000000000000000000000000de0b6b3a7640000")
	mintData, _ := hex.DecodeString("d0def521000000000000000000000000246cbae156cf083f635c0e1a01586b730678f5cb0000000000000000000000000000000000000000000000000000000000000040000000000000000000000000000000000000000000000000000000000000000a697066733a2f2f74657374000000000000000000000000000000000000000000")
	submitData, _ := hex.DecodeString("c6427474000000000000000000000000818c1965e44a033115666f47dff1752c656652c20000000000000000000000000000000000000000000000000de0b6b3a764000000000000000000000000000000000000000000000000000000000000000000600000000000000000000000000000000000000000000000000000000000000000")

	oneNova := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)

	fmt.Println("========================================")
	fmt.Println("  GAS BENCHMARK - Per Transaction Type")
	fmt.Println("========================================")
	fmt.Println()

	// ETH Transfer
	gasETH := sendAndMeasure(client, key, from, &target, oneNova, nil, 21000, "ETH Transfer")

	// ERC-20 Transfer
	gasToken := sendAndMeasure(client, key, from, &token, nil, transferData, 200000, "ERC-20 Transfer (NovaToken)")

	// NFT Mint
	gasNFT := sendAndMeasure(client, key, from, &nft, nil, mintData, 500000, "NFT Mint (NovaNFT)")

	// MultiSig Submit
	gasMulti := sendAndMeasure(client, key, from, &multisig, nil, submitData, 500000, "MultiSig Submit (NovaMultiSig)")

	// Precompile call - novaBatchHash
	precompileAddr := common.HexToAddress("0x0000000000000000000000000000000000000020")
	hashInput, _ := hex.DecodeString("0000000000000000000000000000000000000000000000000000000000000001")
	gasPrecompile := sendAndMeasure(client, key, from, &precompileAddr, nil, hashInput, 100000, "Precompile: novaBatchHash")

	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("  SUMMARY")
	fmt.Println("========================================")
	fmt.Printf("  ETH Transfer:      %6d gas\n", gasETH)
	fmt.Printf("  ERC-20 Transfer:   %6d gas\n", gasToken)
	fmt.Printf("  NFT Mint:          %6d gas\n", gasNFT)
	fmt.Printf("  MultiSig Submit:   %6d gas\n", gasMulti)
	fmt.Printf("  novaBatchHash:     %6d gas\n", gasPrecompile)
	fmt.Println()

	// Discount calculation
	if gasToken > 0 {
		discounted := gasToken * 75 / 100
		saved := gasToken - discounted
		fmt.Printf("  ERC-20 with 25%% discount: %d gas (saves %d gas per tx)\n", discounted, saved)
	}

	// Now do the stress test
	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("  STRESS TEST - 5000 Transactions")
	fmt.Println("========================================")

	startBlock, _ := client.BlockNumber(context.Background())
	startTime := time.Now()
	fmt.Printf("  Start: block %d at %s\n", startBlock, startTime.Format("15:04:05"))

	txCount := 0
	failCount := 0

	// 2500 ETH transfers
	fmt.Println("  Sending 2500 ETH transfers...")
	for i := 0; i < 2500; i++ {
		err := sendTx(client, key, from, &target, oneNova, nil, 21000)
		if err != nil {
			failCount++
		} else {
			txCount++
		}
		if (i+1)%500 == 0 {
			fmt.Printf("    %d/2500 (failed: %d)\n", i+1, failCount)
		}
	}

	// 1500 token transfers
	fmt.Println("  Sending 1500 token transfers...")
	for i := 0; i < 1500; i++ {
		err := sendTx(client, key, from, &token, nil, transferData, 200000)
		if err != nil {
			failCount++
		} else {
			txCount++
		}
		if (i+1)%500 == 0 {
			fmt.Printf("    %d/1500 (failed: %d)\n", i+1, failCount)
		}
	}

	// 500 NFT mints
	fmt.Println("  Sending 500 NFT mints...")
	for i := 0; i < 500; i++ {
		err := sendTx(client, key, from, &nft, nil, mintData, 500000)
		if err != nil {
			failCount++
		} else {
			txCount++
		}
		if (i+1)%250 == 0 {
			fmt.Printf("    %d/500 (failed: %d)\n", i+1, failCount)
		}
	}

	// 500 multisig submits
	fmt.Println("  Sending 500 multisig submits...")
	for i := 0; i < 500; i++ {
		err := sendTx(client, key, from, &multisig, nil, submitData, 500000)
		if err != nil {
			failCount++
		} else {
			txCount++
		}
		if (i+1)%250 == 0 {
			fmt.Printf("    %d/500 (failed: %d)\n", i+1, failCount)
		}
	}

	fmt.Printf("\n  Waiting for blocks...\n")
	time.Sleep(30 * time.Second)

	endBlock, _ := client.BlockNumber(context.Background())
	elapsed := time.Since(startTime)
	blocks := endBlock - startBlock

	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("  STRESS TEST RESULTS")
	fmt.Println("========================================")
	fmt.Printf("  Transactions sent:  %d\n", txCount)
	fmt.Printf("  Failed to send:     %d\n", failCount)
	fmt.Printf("  Total time:         %s\n", elapsed.Round(time.Second))
	fmt.Printf("  Blocks mined:       %d\n", blocks)
	if elapsed.Seconds() > 0 {
		fmt.Printf("  TPS:                %.1f\n", float64(txCount)/elapsed.Seconds())
	}
	if blocks > 0 {
		fmt.Printf("  Avg block time:     %.1fs\n", elapsed.Seconds()/float64(blocks))
		fmt.Printf("  Avg txs/block:      %.1f\n", float64(txCount)/float64(blocks))
	}
	fmt.Println()
}

func sendAndMeasure(client *ethclient.Client, key *ecdsa.PrivateKey, from common.Address, to *common.Address, value *big.Int, data []byte, gasLimit uint64, label string) uint64 {
	nonce, _ := client.PendingNonceAt(context.Background(), from)
	gasPrice, _ := client.SuggestGasPrice(context.Background())
	if value == nil {
		value = big.NewInt(0)
	}

	var tx *types.Transaction
	if to != nil {
		tx = types.NewTransaction(nonce, *to, value, gasLimit, gasPrice, data)
	}

	signer := types.NewEIP155Signer(chainID)
	signed, _ := types.SignTx(tx, signer, key)
	err := client.SendTransaction(context.Background(), signed)
	if err != nil {
		fmt.Printf("  %-30s FAILED: %v\n", label, err)
		return 0
	}

	// Wait for receipt
	for i := 0; i < 30; i++ {
		receipt, err := client.TransactionReceipt(context.Background(), signed.Hash())
		if err == nil && receipt != nil {
			status := "OK"
			if receipt.Status == 0 {
				status = "REVERTED"
			}
			fmt.Printf("  %-30s %6d gas [%s]\n", label, receipt.GasUsed, status)
			return receipt.GasUsed
		}
		time.Sleep(2 * time.Second)
	}
	fmt.Printf("  %-30s TIMEOUT\n", label)
	return 0
}

func sendTx(client *ethclient.Client, key *ecdsa.PrivateKey, from common.Address, to *common.Address, value *big.Int, data []byte, gasLimit uint64) error {
	nonce, err := client.PendingNonceAt(context.Background(), from)
	if err != nil {
		return err
	}
	gasPrice, _ := client.SuggestGasPrice(context.Background())
	if value == nil {
		value = big.NewInt(0)
	}
	tx := types.NewTransaction(nonce, *to, value, gasLimit, gasPrice, data)
	signer := types.NewEIP155Signer(chainID)
	signed, _ := types.SignTx(tx, signer, key)
	return client.SendTransaction(context.Background(), signed)
}
