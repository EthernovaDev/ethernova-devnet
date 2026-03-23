package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"log"
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
	keyHex := os.Getenv("PRIVATE_KEY")
	if keyHex == "" {
		log.Fatal("PRIVATE_KEY env required")
	}
	keyHex = strings.TrimPrefix(keyHex, "0x")

	privateKey, err := crypto.HexToECDSA(keyHex)
	if err != nil {
		log.Fatalf("Invalid key: %v", err)
	}
	from := crypto.PubkeyToAddress(privateKey.PublicKey)

	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		log.Fatalf("RPC connect failed: %v", err)
	}
	defer client.Close()

	balance, _ := client.BalanceAt(context.Background(), from, nil)
	nova := new(big.Int).Div(balance, new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	fmt.Printf("Deployer: %s (%s NOVA)\n\n", from.Hex(), nova.String())

	binDir := os.Getenv("BIN_DIR")
	if binDir == "" {
		binDir = "/tmp"
	}

	contracts := []struct {
		name    string
		binFile string
		args    string // hex-encoded constructor args
	}{
		{"NovaToken", binDir + "/NovaToken_sol_NovaToken.bin", "00000000000000000000000000000000000000000000000000000000000f4240"},
		{"NovaNFT", binDir + "/NovaNFT_sol_NovaNFT.bin", ""},
		{"NovaMultiSig", binDir + "/NovaMultiSig_sol_NovaMultiSig.bin",
			// constructor(address[] owners, uint256 required)
			// owners = [deployer], required = 1
			"0000000000000000000000000000000000000000000000000000000000000040" +
				"0000000000000000000000000000000000000000000000000000000000000001" +
				"0000000000000000000000000000000000000000000000000000000000000001" +
				"000000000000000000000000" + strings.ToLower(from.Hex()[2:])},
	}

	for _, c := range contracts {
		fmt.Printf("=== Deploying %s ===\n", c.name)
		binHex, err := os.ReadFile(c.binFile)
		if err != nil {
			fmt.Printf("  ERROR reading %s: %v\n\n", c.binFile, err)
			continue
		}
		data, err := hex.DecodeString(strings.TrimSpace(string(binHex)) + c.args)
		if err != nil {
			fmt.Printf("  ERROR decoding bytecode: %v\n\n", err)
			continue
		}

		addr, txHash, gasUsed, err := deploy(client, privateKey, from, data)
		if err != nil {
			fmt.Printf("  ERROR: %v\n\n", err)
			continue
		}
		fmt.Printf("  Contract: %s\n", addr.Hex())
		fmt.Printf("  TX:       %s\n", txHash.Hex())
		fmt.Printf("  Gas Used: %d\n\n", gasUsed)
	}

	// Now send test transactions to generate profiling data
	fmt.Println("=== Generating test transactions ===")
	fmt.Println("Sending 50 ETH transfers to generate base data...")
	target := common.HexToAddress("0x246Cbae156Cf083F635C0E1a01586b730678f5Cb")
	oneNova := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	for i := 0; i < 50; i++ {
		nonce, _ := client.PendingNonceAt(context.Background(), from)
		gasPrice, _ := client.SuggestGasPrice(context.Background())
		tx := types.NewTransaction(nonce, target, oneNova, 21000, gasPrice, nil)
		signer := types.NewEIP155Signer(chainID)
		signed, _ := types.SignTx(tx, signer, privateKey)
		client.SendTransaction(context.Background(), signed)
		if (i+1)%25 == 0 {
			fmt.Printf("  %d/50 transfers sent\n", i+1)
			time.Sleep(2 * time.Second)
		}
	}

	fmt.Println("\nDone! Check ethernova_evmProfile and ethernova_adaptiveGas for results.")
}

func deploy(client *ethclient.Client, key *ecdsa.PrivateKey, from common.Address, data []byte) (common.Address, common.Hash, uint64, error) {
	nonce, err := client.PendingNonceAt(context.Background(), from)
	if err != nil {
		return common.Address{}, common.Hash{}, 0, err
	}
	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		gasPrice = big.NewInt(1000000000)
	}

	tx := types.NewContractCreation(nonce, big.NewInt(0), 3000000, gasPrice, data)
	signer := types.NewEIP155Signer(chainID)
	signed, err := types.SignTx(tx, signer, key)
	if err != nil {
		return common.Address{}, common.Hash{}, 0, err
	}

	if err := client.SendTransaction(context.Background(), signed); err != nil {
		return common.Address{}, common.Hash{}, 0, err
	}

	// Wait for receipt
	txHash := signed.Hash()
	for i := 0; i < 60; i++ {
		receipt, err := client.TransactionReceipt(context.Background(), txHash)
		if err == nil && receipt != nil {
			if receipt.Status == 0 {
				return common.Address{}, txHash, receipt.GasUsed, fmt.Errorf("tx reverted")
			}
			return receipt.ContractAddress, txHash, receipt.GasUsed, nil
		}
		time.Sleep(2 * time.Second)
	}
	return common.Address{}, txHash, 0, fmt.Errorf("timeout waiting for receipt")
}
