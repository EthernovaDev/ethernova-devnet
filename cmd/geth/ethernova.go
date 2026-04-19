package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/params/ethernova"
	"github.com/ethereum/go-ethereum/params/types/genesisT"
	"github.com/urfave/cli/v2"
)

const (
	ethernovaGenesisFilename = "genesis-121525-alloc.json"
	ethernovaLockFilename    = "ethernova.lock.json"
)

const wrongGenesisMessage = "WRONG GENESIS IN DATADIR. This build requires genesis " + ethernova.ExpectedGenesisHashHex + ". Delete ./data and restart."

type ethernovaPaths struct {
	Cwd     string
	DataDir string
	LogDir  string
}

type ethernovaGenesisInfo struct {
	Paths               ethernovaPaths
	GenesisPath         string
	GenesisSource       string
	GenesisSHA256       string
	GenesisHash         common.Hash
	ExpectedGenesisHash common.Hash
	ChainID             uint64
	NetworkID           uint64
	Genesis             *genesisT.Genesis
}

type ethernovaLock struct {
	Version             string          `json:"version"`
	ChainID             uint64          `json:"chainId"`
	NetworkID           uint64          `json:"networkId"`
	ExpectedGenesisHash string          `json:"expectedGenesisHash"`
	GenesisSHA256       string          `json:"genesisSha256"`
	ForkStatus          *forkLockStatus `json:"forkStatus,omitempty"`
}

type forkLockStatus struct {
	EVMCompatApplied bool   `json:"evmCompatApplied"`
	EVMCompatBlock   uint64 `json:"evmCompatBlock"`
	EIP658Applied    bool   `json:"eip658Applied"`
	EIP658Block      uint64 `json:"eip658Block"`
	MegaForkApplied  bool   `json:"megaForkApplied"`
	MegaForkBlock    uint64 `json:"megaForkBlock"`
	LastCheckedAt    string `json:"lastCheckedAt"`
}

var ethernovaCachedGenesis *ethernovaGenesisInfo

func applyEthernovaPathDefaults(ctx *cli.Context) (*ethernovaPaths, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve cwd: %w", err)
	}

	dataDir := ctx.String(utils.DataDirFlag.Name)
	if !ctx.IsSet(utils.DataDirFlag.Name) || dataDir == "" {
		dataDir = filepath.Join(cwd, "data")
		if err := ctx.Set(utils.DataDirFlag.Name, dataDir); err != nil {
			return nil, fmt.Errorf("failed to set datadir: %w", err)
		}
	}

	logDir := filepath.Join(cwd, "logs")
	if !ctx.IsSet("log.file") {
		logFile := filepath.Join(logDir, "ethernova.log")
		if err := ctx.Set("log.file", logFile); err != nil {
			return nil, fmt.Errorf("failed to set log.file: %w", err)
		}
	}

	return &ethernovaPaths{
		Cwd:     cwd,
		DataDir: dataDir,
		LogDir:  logDir,
	}, nil
}

func applyEthernovaDefaults(ctx *cli.Context) (*ethernovaPaths, error) {
	paths, err := applyEthernovaPathDefaults(ctx)
	if err != nil {
		return nil, err
	}
	if ctx.IsSet(utils.NetworkIdFlag.Name) {
		if ctx.Uint64(utils.NetworkIdFlag.Name) != ethernova.NewChainID {
			return nil, fmt.Errorf("networkId mismatch: have %d want %d", ctx.Uint64(utils.NetworkIdFlag.Name), ethernova.NewChainID)
		}
	} else {
		if err := ctx.Set(utils.NetworkIdFlag.Name, strconv.FormatUint(ethernova.NewChainID, 10)); err != nil {
			return nil, fmt.Errorf("failed to set networkid: %w", err)
		}
	}
	return paths, nil
}

func applyEthernovaOneClickDefaults(ctx *cli.Context) (uint64, error) {
	if len(os.Args) > 1 {
		return 0, nil
	}
	apiList := "eth,net,web3,debug,txpool,ethernova"

	if !ctx.IsSet(utils.HTTPEnabledFlag.Name) {
		if err := ctx.Set(utils.HTTPEnabledFlag.Name, "true"); err != nil {
			return 0, err
		}
	}
	if !ctx.IsSet(utils.HTTPListenAddrFlag.Name) {
		if err := ctx.Set(utils.HTTPListenAddrFlag.Name, "127.0.0.1"); err != nil {
			return 0, err
		}
	}
	if !ctx.IsSet(utils.HTTPPortFlag.Name) {
		if err := ctx.Set(utils.HTTPPortFlag.Name, "8545"); err != nil {
			return 0, err
		}
	}
	if !ctx.IsSet(utils.HTTPApiFlag.Name) {
		if err := ctx.Set(utils.HTTPApiFlag.Name, apiList); err != nil {
			return 0, err
		}
	}

	if !ctx.IsSet(utils.WSEnabledFlag.Name) {
		if err := ctx.Set(utils.WSEnabledFlag.Name, "true"); err != nil {
			return 0, err
		}
	}
	if !ctx.IsSet(utils.WSListenAddrFlag.Name) {
		if err := ctx.Set(utils.WSListenAddrFlag.Name, "127.0.0.1"); err != nil {
			return 0, err
		}
	}
	if !ctx.IsSet(utils.WSPortFlag.Name) {
		if err := ctx.Set(utils.WSPortFlag.Name, "8546"); err != nil {
			return 0, err
		}
	}
	if !ctx.IsSet(utils.WSApiFlag.Name) {
		if err := ctx.Set(utils.WSApiFlag.Name, apiList); err != nil {
			return 0, err
		}
	}

	var chosen uint64
	if !ctx.IsSet(utils.ListenPortFlag.Name) {
		port, err := pickAvailableP2PPort(30303, 64)
		if err != nil {
			return 0, err
		}
		chosen = port
		if err := ctx.Set(utils.ListenPortFlag.Name, strconv.FormatUint(port, 10)); err != nil {
			return 0, err
		}
	}
	if chosen != 0 && !ctx.IsSet(utils.DiscoveryPortFlag.Name) {
		if err := ctx.Set(utils.DiscoveryPortFlag.Name, strconv.FormatUint(chosen, 10)); err != nil {
			return 0, err
		}
	}
	return chosen, nil
}

// applyEthernovaPeerDefaults adds the Ethernova public peers to the P2P config
// as static + trusted nodes so a bare-flags invocation (e.g. Windows
// double-click) auto-dials the devnet without discovery.
func applyEthernovaPeerDefaults(cfg *p2p.Config) {
	for _, url := range ethernova.DefaultPublicPeers {
		n, err := enode.Parse(enode.ValidSchemes, url)
		if err != nil {
			log.Warn("Ignoring invalid ethernova default peer", "err", err, "url", url)
			continue
		}
		if !containsEnode(cfg.StaticNodes, n) {
			cfg.StaticNodes = append(cfg.StaticNodes, n)
		}
		if !containsEnode(cfg.TrustedNodes, n) {
			cfg.TrustedNodes = append(cfg.TrustedNodes, n)
		}
	}
}

func containsEnode(list []*enode.Node, target *enode.Node) bool {
	for _, n := range list {
		if n.ID() == target.ID() {
			return true
		}
	}
	return false
}

func pickAvailableP2PPort(start uint64, attempts uint64) (uint64, error) {
	for i := uint64(0); i < attempts; i++ {
		port := start + i
		if port > 65535 {
			break
		}
		if portAvailable(port) {
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available p2p port found starting at %d", start)
}

func portAvailable(port uint64) bool {
	addr := fmt.Sprintf(":%d", port)
	tcpListener, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	tcpListener.Close()
	udpListener, err := net.ListenPacket("udp", addr)
	if err != nil {
		return false
	}
	udpListener.Close()
	return true
}

func loadEthernovaGenesis(ctx *cli.Context) (*ethernovaGenesisInfo, error) {
	if ethernovaCachedGenesis != nil {
		return ethernovaCachedGenesis, nil
	}
	paths, err := applyEthernovaDefaults(ctx)
	if err != nil {
		return nil, err
	}

	genesisPath, source := resolveGenesisPath(paths.Cwd)
	genesisJSON, err := readGenesisJSON(genesisPath, source)
	if err != nil {
		return nil, err
	}
	sha := sha256.Sum256(genesisJSON)

	genesis := new(genesisT.Genesis)
	if err := genesis.UnmarshalJSON(genesisJSON); err != nil {
		return nil, fmt.Errorf("invalid genesis json: %w", err)
	}
	if genesis.Config == nil {
		return nil, errors.New("genesis config is missing")
	}

	genesisHash := core.GenesisToBlock(genesis, nil).Hash()
	chainID := genesis.Config.GetChainID()
	if chainID == nil {
		return nil, errors.New("genesis chainId is missing")
	}
	networkID := chainID.Uint64()
	if n := genesis.GetNetworkID(); n != nil {
		networkID = *n
	}

	info := &ethernovaGenesisInfo{
		Paths:               *paths,
		GenesisPath:         genesisPath,
		GenesisSource:       source,
		GenesisSHA256:       hex.EncodeToString(sha[:]),
		GenesisHash:         genesisHash,
		ExpectedGenesisHash: ethernova.ExpectedGenesisHash,
		ChainID:             chainID.Uint64(),
		NetworkID:           networkID,
		Genesis:             genesis,
	}
	ethernovaCachedGenesis = info
	return info, nil
}

func applyEthernovaGenesisConfig(ctx *cli.Context, cfg *ethconfig.Config) (*ethernovaGenesisInfo, error) {
	info, err := loadEthernovaGenesis(ctx)
	if err != nil {
		return nil, err
	}
	cfg.Genesis = info.Genesis
	cfg.NetworkId = info.NetworkID
	return info, nil
}

func resolveGenesisPath(cwd string) (string, string) {
	cwdPath := filepath.Join(cwd, ethernovaGenesisFilename)
	if fileExists(cwdPath) {
		return cwdPath, "cwd"
	}
	if exe, err := os.Executable(); err == nil {
		exePath := filepath.Join(filepath.Dir(exe), ethernovaGenesisFilename)
		if fileExists(exePath) {
			return exePath, "exe"
		}
	}
	return "", "embedded"
}

func readGenesisJSON(path string, source string) ([]byte, error) {
	if source == "embedded" {
		return ethernova.EmbeddedGenesisJSON(), nil
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read genesis file: %w", err)
	}
	return payload, nil
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func genesisPathUsed(info *ethernovaGenesisInfo) string {
	if info.GenesisPath == "" {
		return "embedded"
	}
	return info.GenesisPath
}

func printEthernovaStartup(info *ethernovaGenesisInfo) {
	fmt.Println("==========================================================")
	fmt.Println("  ETHERNOVA NODE v" + params.Version)
	fmt.Println("  PoW (Ethash) | Chain ID: 121525 | Network ID: 121525")
	fmt.Println("==========================================================")
	fmt.Println()
	fmt.Printf("  Datadir:    %s\n", info.Paths.DataDir)
	fmt.Printf("  Genesis:    %s\n", genesisPathUsed(info))
	fmt.Printf("  Hash:       %s\n", info.GenesisHash.Hex())
	fmt.Println()
	fmt.Println("  Fork Schedule:")
	fmt.Printf("    Constantinople/Petersburg/Istanbul .. block %s\n", ethernova.FormatBlockWithCommas(ethernova.EVMCompatibilityForkBlock))
	fmt.Printf("    EIP-658 (Receipt Status) ........... block %s\n", ethernova.FormatBlockWithCommas(ethernova.EIP658ForkBlock))
	fmt.Printf("    MegaFork (Historical EVM) .......... block %s\n", ethernova.FormatBlockWithCommas(ethernova.MegaForkBlock))
	fmt.Printf("    Legacy Chain Enforcement ........... block %s\n", ethernova.FormatBlockWithCommas(ethernova.LegacyForkEnforcementBlock))
	fmt.Println()
	fmt.Println("  RPC: ethernova_forkStatus, ethernova_chainConfig, ethernova_nodeHealth")
	fmt.Println("==========================================================")
}
func validateEthernovaGenesisInfo(info *ethernovaGenesisInfo) error {
	if info.ChainID != ethernova.NewChainID {
		return fmt.Errorf("chainId mismatch: have %d want %d", info.ChainID, ethernova.NewChainID)
	}
	if info.NetworkID != ethernova.NewChainID {
		return fmt.Errorf("networkId mismatch: have %d want %d", info.NetworkID, ethernova.NewChainID)
	}
	if info.GenesisSource != "embedded" {
		expectedSHA := ethernova.EmbeddedGenesisSHA256Hex()
		if !strings.EqualFold(info.GenesisSHA256, expectedSHA) {
			return fmt.Errorf("genesis file sha256 mismatch: have %s want %s", info.GenesisSHA256, expectedSHA)
		}
	}
	if info.GenesisHash != info.ExpectedGenesisHash {
		return errors.New(wrongGenesisMessage)
	}
	return nil
}

func ensureEthernovaGenesis(ctx *cli.Context, info *ethernovaGenesisInfo) (common.Hash, error) {
	if err := validateEthernovaGenesisInfo(info); err != nil {
		return common.Hash{}, err
	}
	if err := os.MkdirAll(info.Paths.DataDir, 0o755); err != nil {
		return common.Hash{}, fmt.Errorf("failed to create datadir: %w", err)
	}

	lockPath := filepath.Join(info.Paths.DataDir, ethernovaLockFilename)
	lockExists, err := checkEthernovaLock(lockPath, info)
	if err != nil {
		return common.Hash{}, err
	}

	chainPath, ancientPath := resolveChainPaths(info.Paths.DataDir, ctx.String(utils.AncientFlag.Name))
	db, err := rawdb.Open(rawdb.OpenOptions{
		Type:              ctx.String(utils.DBEngineFlag.Name),
		Directory:         chainPath,
		AncientsDirectory: ancientPath,
		Cache:             0,
		Handles:           0,
		ReadOnly:          false,
	})
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	triedb := utils.MakeTrieDatabase(ctx, db, ctx.Bool(utils.CachePreimagesFlag.Name), false, info.Genesis.IsVerkle())
	defer triedb.Close()

	var overrides core.ChainOverrides
	if ctx.IsSet(utils.OverrideCancun.Name) {
		v := ctx.Uint64(utils.OverrideCancun.Name)
		overrides.OverrideCancun = &v
	}
	if ctx.IsSet(utils.OverrideVerkle.Name) {
		v := ctx.Uint64(utils.OverrideVerkle.Name)
		overrides.OverrideVerkle = &v
	}

	_, hash, err := core.SetupGenesisBlockWithOverride(db, triedb, info.Genesis, &overrides)
	if err != nil {
		var mismatch *genesisT.GenesisMismatchError
		if errors.As(err, &mismatch) {
			return common.Hash{}, errors.New(wrongGenesisMessage)
		}
		return common.Hash{}, err
	}
	if hash != info.ExpectedGenesisHash {
		return common.Hash{}, errors.New(wrongGenesisMessage)
	}

	if !lockExists {
		if err := writeEthernovaLock(lockPath, info); err != nil {
			return common.Hash{}, err
		}
	}
	return hash, nil
}

func resolveChainPaths(datadir, ancient string) (string, string) {
	cfg := node.DefaultConfig
	cfg.Name = databaseIdentifier
	cfg.DataDir = datadir
	chainPath := cfg.ResolvePath("chaindata")
	if ancient == "" {
		return chainPath, filepath.Join(chainPath, "ancient")
	}
	if filepath.IsAbs(ancient) {
		return chainPath, ancient
	}
	return chainPath, cfg.ResolvePath(ancient)
}

func checkEthernovaLock(path string, info *ethernovaGenesisInfo) (bool, error) {
	if !fileExists(path) {
		return false, nil
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		return true, fmt.Errorf("failed to read lock file: %w", err)
	}
	var lock ethernovaLock
	if err := json.Unmarshal(payload, &lock); err != nil {
		return true, fmt.Errorf("failed to parse lock file: %w", err)
	}
	if lock.ChainID != info.ChainID || lock.NetworkID != info.NetworkID {
		return true, errors.New(wrongGenesisMessage)
	}
	if !strings.EqualFold(lock.ExpectedGenesisHash, info.ExpectedGenesisHash.Hex()) {
		return true, errors.New(wrongGenesisMessage)
	}
	if !strings.EqualFold(lock.GenesisSHA256, info.GenesisSHA256) {
		return true, errors.New(wrongGenesisMessage)
	}
	return true, nil
}

func writeEthernovaLock(path string, info *ethernovaGenesisInfo) error {
	lock := ethernovaLock{
		Version:             "v" + params.Version,
		ChainID:             info.ChainID,
		NetworkID:           info.NetworkID,
		ExpectedGenesisHash: info.ExpectedGenesisHash.Hex(),
		GenesisSHA256:       info.GenesisSHA256,
		ForkStatus: &forkLockStatus{
			EVMCompatApplied: true,
			EVMCompatBlock:   ethernova.EVMCompatibilityForkBlock,
			EIP658Applied:    true,
			EIP658Block:      ethernova.EIP658ForkBlock,
			MegaForkApplied:  true,
			MegaForkBlock:    ethernova.MegaForkBlock,
			LastCheckedAt:    time.Now().UTC().Format(time.RFC3339),
		},
	}
	payload, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal lock file: %w", err)
	}
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		return fmt.Errorf("failed to write lock file: %w", err)
	}
	log.Info("Ethernova lock written", "path", path)
	return nil
}
