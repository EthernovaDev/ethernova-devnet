package main

import (
	"fmt"

	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/params/ethernova"
	"github.com/urfave/cli/v2"
)

var sanitycheckCommand = &cli.Command{
	Name:  "sanitycheck",
	Usage: "Verify Ethernova genesis/datadir without starting networking",
	Flags: []cli.Flag{
		utils.DataDirFlag,
		utils.DBEngineFlag,
		utils.AncientFlag,
		utils.CachePreimagesFlag,
		utils.OverrideCancun,
		utils.OverrideVerkle,
		utils.NetworkIdFlag,
	},
	Action: sanitycheck,
}

var printGenesisCommand = &cli.Command{
	Name:   "print-genesis",
	Usage:  "Print expected genesis and embedded genesis SHA256",
	Action: printGenesis,
}

func sanitycheck(ctx *cli.Context) error {
	info, err := loadEthernovaGenesis(ctx)
	if err != nil {
		fmt.Printf("FAIL: %v\n", err)
		return cli.Exit(err.Error(), 1)
	}
	printEthernovaStartup(info)
	fmt.Printf("genesis_sha256=%s\n", info.GenesisSHA256)

	if _, err := ensureEthernovaGenesis(ctx, info); err != nil {
		fmt.Printf("FAIL: %v\n", err)
		return cli.Exit(err.Error(), 1)
	}
	fmt.Println("PASS")
	return nil
}

var validateConfigCommand = &cli.Command{
	Name:  "validate-config",
	Usage: "Validate chain configuration from database without starting the node",
	Flags: []cli.Flag{
		utils.DataDirFlag,
		utils.DBEngineFlag,
		utils.AncientFlag,
		utils.CachePreimagesFlag,
		utils.NetworkIdFlag,
	},
	Action: validateConfig,
}

func validateConfig(ctx *cli.Context) error {
	info, err := loadEthernovaGenesis(ctx)
	if err != nil {
		fmt.Printf("FAIL: %v\n", err)
		return cli.Exit(err.Error(), 1)
	}
	if err := validateEthernovaGenesisInfo(info); err != nil {
		fmt.Printf("FAIL genesis: %v\n", err)
		return cli.Exit(err.Error(), 1)
	}
	fmt.Println("genesis: OK")

	chainPath, ancientPath := resolveChainPaths(info.Paths.DataDir, ctx.String(utils.AncientFlag.Name))
	db, err := rawdb.Open(rawdb.OpenOptions{
		Type:              ctx.String(utils.DBEngineFlag.Name),
		Directory:         chainPath,
		AncientsDirectory: ancientPath,
		Cache:             0,
		Handles:           0,
		ReadOnly:          true,
	})
	if err != nil {
		fmt.Printf("FAIL db: %v\n", err)
		return cli.Exit(err.Error(), 1)
	}
	defer db.Close()

	triedb := utils.MakeTrieDatabase(ctx, db, false, false, false)
	defer triedb.Close()

	stored, _, setupErr := core.SetupGenesisBlock(db, triedb, info.Genesis)
	if setupErr != nil {
		fmt.Printf("WARN setup: %v\n", setupErr)
	}

	if stored == nil {
		fmt.Println("WARN: no stored chain config found (fresh database?)")
		fmt.Println("PASS (genesis valid, no stored config to check)")
		return nil
	}

	fmt.Printf("chain_id:   %d\n", stored.GetChainID())
	fmt.Printf("consensus:  %s\n", stored.GetConsensusEngineType())

	forkBlock := ethernova.EVMCompatibilityForkBlock
	missing, mismatched, _ := core.EthernovaForkStatus(stored, forkBlock)
	if len(mismatched) > 0 {
		fmt.Printf("evm-compat: MISMATCH (%v)\n", mismatched)
	} else if missing {
		fmt.Printf("evm-compat: MISSING (block %d)\n", forkBlock)
	} else {
		fmt.Printf("evm-compat: OK (block %d)\n", forkBlock)
	}

	eip658Block := ethernova.EIP658ForkBlock
	missing658, mismatched658, _ := core.EthernovaEIP658Status(stored, eip658Block)
	if len(mismatched658) > 0 {
		fmt.Printf("eip658:     MISMATCH (%v)\n", mismatched658)
	} else if missing658 {
		fmt.Printf("eip658:     MISSING (block %d)\n", eip658Block)
	} else {
		fmt.Printf("eip658:     OK (block %d)\n", eip658Block)
	}

	megaBlock := ethernova.MegaForkBlock
	missingMega, mismatchedMega, _ := core.EthernovaMegaForkStatus(stored, megaBlock)
	if len(mismatchedMega) > 0 {
		fmt.Printf("mega-fork:  MISMATCH (%v)\n", mismatchedMega)
	} else if len(missingMega) > 0 {
		fmt.Printf("mega-fork:  MISSING %d fields (block %d)\n", len(missingMega), megaBlock)
	} else {
		fmt.Printf("mega-fork:  OK (block %d)\n", megaBlock)
	}

	fmt.Println("PASS")
	return nil
}

func printGenesis(ctx *cli.Context) error {
	genesis := ethernova.MustGenesis()
	chainID := genesis.Config.GetChainID()
	var chainIDValue uint64
	if chainID != nil {
		chainIDValue = chainID.Uint64()
	}
	networkID := chainIDValue
	if n := genesis.GetNetworkID(); n != nil {
		networkID = *n
	}

	fmt.Printf("expected_genesis_hash=%s\n", ethernova.ExpectedGenesisHashHex)
	fmt.Printf("chain_id=%d\n", chainIDValue)
	fmt.Printf("network_id=%d\n", networkID)
	fmt.Printf("embedded_genesis_sha256=%s\n", ethernova.EmbeddedGenesisSHA256Hex())
	return nil
}
