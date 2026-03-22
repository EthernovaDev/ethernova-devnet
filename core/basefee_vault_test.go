package core

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params/types/coregeth"
	"github.com/ethereum/go-ethereum/params/types/ctypes"
	"github.com/holiman/uint256"
)

const (
	testGasLimit = uint64(21000)
)

func makeTestConfig(vault *common.Address, fromBlock *big.Int) *coregeth.CoreGethChainConfig {
	cfg := &coregeth.CoreGethChainConfig{
		NetworkID:     121525,
		ChainID:       big.NewInt(121525),
		EIP1559FBlock: big.NewInt(0),
		EIP2718FBlock: big.NewInt(0),
		Ethash:        &ctypes.EthashConfig{},
		BaseFeeVault:  vault,
		IsDevMode:     true,
	}
	if fromBlock != nil {
		cfg.BaseFeeVaultFromBlock = fromBlock
	}
	return cfg
}

type baseFeeResult struct {
	coinbaseBalance *uint256.Int
	vaultBalance    *uint256.Int
	usedGas         uint64
}

func runBaseFeeTx(t *testing.T, cfg ctypes.ChainConfigurator, baseFee, tipCap, feeCap *big.Int, blockNumber uint64, vault common.Address) baseFeeResult {
	t.Helper()

	db := state.NewDatabase(rawdb.NewMemoryDatabase())
	statedb, err := state.New(common.Hash{}, db, nil)
	if err != nil {
		t.Fatalf("failed to create state db: %v", err)
	}

	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("failed to create key: %v", err)
	}
	sender := crypto.PubkeyToAddress(key.PublicKey)
	receiver := common.HexToAddress("0x00000000000000000000000000000000000000ff")
	coinbase := common.HexToAddress("0x00000000000000000000000000000000deadbeef")

	funds := uint256.MustFromBig(big.NewInt(1_000_000_000_000_000_000))
	statedb.SetBalance(sender, funds)

	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   cfg.GetChainID(),
		Nonce:     0,
		GasFeeCap: feeCap,
		GasTipCap: tipCap,
		Gas:       testGasLimit,
		To:        &receiver,
		Value:     big.NewInt(0),
	})

	signer := types.MakeSigner(cfg, new(big.Int).SetUint64(blockNumber), 0)
	signedTx, err := types.SignTx(tx, signer, key)
	if err != nil {
		t.Fatalf("failed to sign tx: %v", err)
	}

	msg, err := TransactionToMessage(signedTx, signer, baseFee)
	if err != nil {
		t.Fatalf("failed to convert tx to message: %v", err)
	}

	gp := new(GasPool)
	gp.AddGas(testGasLimit)

	blockCtx := vm.BlockContext{
		CanTransfer: CanTransfer,
		Transfer:    Transfer,
		GetHash:     func(uint64) common.Hash { return common.Hash{} },
		Coinbase:    coinbase,
		GasLimit:    testGasLimit,
		BlockNumber: new(big.Int).SetUint64(blockNumber),
		Time:        0,
		Difficulty:  big.NewInt(0),
		BaseFee:     baseFee,
	}
	txCtx := vm.TxContext{
		Origin:   msg.From,
		GasPrice: msg.GasPrice,
	}

	evm := vm.NewEVM(blockCtx, txCtx, statedb, cfg, vm.Config{})
	res, err := ApplyMessage(evm, msg, gp)
	if err != nil {
		t.Fatalf("apply message failed: %v", err)
	}

	coinbaseBal := statedb.GetBalance(coinbase)
	vaultBal := statedb.GetBalance(vault)

	return baseFeeResult{
		coinbaseBalance: coinbaseBal,
		vaultBalance:    vaultBal,
		usedGas:         res.UsedGas,
	}
}

func TestBaseFeeVaultRedirectsBaseFee(t *testing.T) {
	vault := common.HexToAddress("0x3a38560b66205bb6a31decbcb245450b2f15d4fd")
	cfg := makeTestConfig(&vault, big.NewInt(0))
	baseFee := big.NewInt(1_000_000_000) // 1 gwei
	tip := big.NewInt(2_000_000_000)     // 2 gwei
	feeCap := new(big.Int).Add(baseFee, tip)

	result := runBaseFeeTx(t, cfg, baseFee, tip, feeCap, 1, vault)

	expectedTip := uint256.MustFromBig(new(big.Int).Mul(tip, new(big.Int).SetUint64(result.usedGas)))
	if result.coinbaseBalance.Cmp(expectedTip) != 0 {
		t.Fatalf("coinbase tip mismatch: have %s want %s", result.coinbaseBalance, expectedTip)
	}

	expectedBase := uint256.MustFromBig(new(big.Int).Mul(baseFee, new(big.Int).SetUint64(result.usedGas)))
	if result.vaultBalance.Cmp(expectedBase) != 0 {
		t.Fatalf("vault basefee mismatch: have %s want %s", result.vaultBalance, expectedBase)
	}
}

func TestBaseFeeVaultAbsentBurnsBaseFee(t *testing.T) {
	vault := common.HexToAddress("0x3a38560b66205bb6a31decbcb245450b2f15d4fd")
	cfg := makeTestConfig(nil, nil)
	baseFee := big.NewInt(1_500_000_000)
	tip := big.NewInt(1_000_000_000)
	feeCap := new(big.Int).Add(baseFee, tip)

	result := runBaseFeeTx(t, cfg, baseFee, tip, feeCap, 1, vault)

	expectedTip := uint256.MustFromBig(new(big.Int).Mul(tip, new(big.Int).SetUint64(result.usedGas)))
	if result.coinbaseBalance.Cmp(expectedTip) != 0 {
		t.Fatalf("coinbase tip mismatch without vault: have %s want %s", result.coinbaseBalance, expectedTip)
	}
	if !result.vaultBalance.IsZero() {
		t.Fatalf("vault should be empty when not configured, got %s", result.vaultBalance)
	}
}

func TestBaseFeeVaultFromBlockActivation(t *testing.T) {
	vault := common.HexToAddress("0x3a38560b66205bb6a31decbcb245450b2f15d4fd")
	fromBlock := big.NewInt(3)
	cfg := makeTestConfig(&vault, fromBlock)
	baseFee := big.NewInt(2_000_000_000)
	tip := big.NewInt(1_000_000_000)
	feeCap := new(big.Int).Add(baseFee, tip)

	before := runBaseFeeTx(t, cfg, baseFee, tip, feeCap, 2, vault)
	if !before.vaultBalance.IsZero() {
		t.Fatalf("vault should be inactive before activation block, got %s", before.vaultBalance)
	}

	after := runBaseFeeTx(t, cfg, baseFee, tip, feeCap, 3, vault)
	expectedBase := uint256.MustFromBig(new(big.Int).Mul(baseFee, new(big.Int).SetUint64(after.usedGas)))
	if after.vaultBalance.Cmp(expectedBase) != 0 {
		t.Fatalf("vault basefee mismatch after activation: have %s want %s", after.vaultBalance, expectedBase)
	}
}
