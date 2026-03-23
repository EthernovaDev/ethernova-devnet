package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

const (
	defaultHTTPPort = 8545
	defaultWSPort   = 8546
	fallbackHTTP    = 8547
	fallbackWS      = 8548

	devnetNetworkID  = "121526"
	devnetChainIDHex = "0x1dab6"
	genesisHashExp   = "0x2b6206a40fd6cf3c9afcb410eff7811c0eef0f8dbd3bac4f39547ffe9f0ec050"

	bootstrapEnode = "enode://6d6f8341c08058a8f966d4e0d75e1cf7009bbe8647741e105e5ef2edd929baf3157292dcb31a1e1bd6cbb9161fe7bfde8e15539bef801ace55950e2e23f92a88@192.168.1.15:30301?discport=0"
)

type rpcReq struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

type rpcResp struct {
	Result json.RawMessage   `json:"result"`
	Error  *rpcErrorResponse `json:"error"`
}

type rpcErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func main() {
	fmt.Println("==========================================================")
	fmt.Println("  ETHERNOVA DEVNET NODE - One Click Launcher")
	fmt.Println("  Chain ID: 121526 | Network: Devnet | Consensus: Ethash")
	fmt.Println("==========================================================")
	fmt.Println()

	exePath, err := os.Executable()
	if err != nil {
		fatalWait("Cannot resolve executable path: %v", err)
	}
	exeDir := filepath.Dir(exePath)
	if err := os.Chdir(exeDir); err != nil {
		fatalWait("Cannot change to executable directory: %v", err)
	}

	// Find the geth binary - try multiple names
	gethBin := findGethBinary(exeDir)
	if gethBin == "" {
		fatalWait("Cannot find geth binary.\nPlace one of these next to this launcher:\n  - geth.exe (Windows) / geth (Linux)\n  - geth-windows-amd64.exe\n  - ethernova-devnet.exe\n\nDirectory: %s", exeDir)
	}
	fmt.Printf("  Geth binary: %s\n", gethBin)

	// Setup directories
	dataDir := filepath.Join(exeDir, "devnet-data")
	logsDir := filepath.Join(exeDir, "logs")
	ensureDir(dataDir)
	ensureDir(logsDir)
	fmt.Printf("  Data dir:    %s\n", dataDir)
	fmt.Printf("  Logs dir:    %s\n", logsDir)
	fmt.Println()

	// Choose ports
	httpPort, wsPort := choosePorts(defaultHTTPPort, defaultWSPort, fallbackHTTP, fallbackWS)
	fmt.Printf("  HTTP RPC:    http://127.0.0.1:%d\n", httpPort)
	fmt.Printf("  WS RPC:      ws://127.0.0.1:%d\n", wsPort)
	fmt.Println()

	// Start the node - geth handles genesis init automatically with embedded genesis
	fmt.Println("Starting Ethernova Devnet node...")
	nodeLog := filepath.Join(logsDir, "devnet-node.log")
	nodeErr := filepath.Join(logsDir, "devnet-node.err.log")

	outFile, err := os.Create(nodeLog)
	if err != nil {
		fatalWait("Cannot create log file: %v", err)
	}
	errFile, err := os.Create(nodeErr)
	if err != nil {
		fatalWait("Cannot create error log file: %v", err)
	}

	apiList := "eth,net,web3,txpool,admin,ethernova"
	args := []string{
		"--datadir", dataDir,
		"--networkid", devnetNetworkID,
		"--port", "30303",
		"--http", "--http.addr", "0.0.0.0", "--http.port", fmt.Sprintf("%d", httpPort),
		"--http.api", apiList,
		"--http.corsdomain", "*",
		"--http.vhosts", "*",
		"--ws", "--ws.addr", "0.0.0.0", "--ws.port", fmt.Sprintf("%d", wsPort),
		"--ws.api", apiList,
		"--ws.origins", "*",
		"--nodiscover",
		"--verbosity", "3",
	}

	cmd := exec.Command(gethBin, args...)
	cmd.Stdout = outFile
	cmd.Stderr = errFile

	if err := cmd.Start(); err != nil {
		fatalWait("Failed to start node: %v", err)
	}
	fmt.Printf("  Node PID:    %d\n", cmd.Process.Pid)
	fmt.Println()

	// Wait for RPC to be ready and verify
	fmt.Println("Waiting for node to start...")
	rpcURL := fmt.Sprintf("http://127.0.0.1:%d", httpPort)
	if waitForRPC(rpcURL, 15*time.Second) {
		fmt.Println("  [OK] RPC is responding")
		verifyChainID(rpcURL)
		// Auto-connect to bootstrap peer
		connectBootstrap(rpcURL)
	} else {
		fmt.Println("  [WARN] RPC not responding yet - check logs for errors")
		fmt.Printf("         Log: %s\n", nodeErr)
	}

	fmt.Println()
	fmt.Println("==========================================================")
	fmt.Printf("  Node is running! RPC: http://127.0.0.1:%d\n", httpPort)
	fmt.Println()
	fmt.Println("  MetaMask Setup:")
	fmt.Println("    Network Name:  Ethernova Devnet")
	fmt.Printf("    RPC URL:       http://127.0.0.1:%d\n", httpPort)
	fmt.Println("    Chain ID:      121526")
	fmt.Println("    Symbol:        NOVA")
	fmt.Println("==========================================================")
	fmt.Println()
	fmt.Println("Press Ctrl+C or close this window to stop the node.")
	fmt.Println()

	// Wait for interrupt signal or process exit
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	doneCh := make(chan error, 1)
	go func() { doneCh <- cmd.Wait() }()

	select {
	case sig := <-sigCh:
		fmt.Printf("\nReceived %s, stopping node...\n", sig)
		stopNode(cmd)
	case err := <-doneCh:
		if err != nil {
			fmt.Printf("\nNode exited with error: %v\n", err)
			fmt.Printf("Check logs: %s\n", nodeErr)
		} else {
			fmt.Println("\nNode exited.")
		}
	}

	fmt.Println()
	pauseBeforeExit()
}

func findGethBinary(dir string) string {
	var candidates []string
	if runtime.GOOS == "windows" {
		candidates = []string{
			"geth.exe",
			"geth-windows-amd64.exe",
			"ethernova-devnet.exe",
			"ethernova.exe",
		}
	} else {
		candidates = []string{
			"geth",
			"geth-linux-amd64",
			"ethernova-devnet",
			"ethernova",
		}
	}

	for _, name := range candidates {
		path := filepath.Join(dir, name)
		// Don't match ourselves
		if exe, err := os.Executable(); err == nil {
			if abs1, err1 := filepath.Abs(path); err1 == nil {
				if abs2, err2 := filepath.Abs(exe); err2 == nil {
					if strings.EqualFold(abs1, abs2) {
						continue
					}
				}
			}
		}
		if st, err := os.Stat(path); err == nil && !st.IsDir() {
			return path
		}
	}
	return ""
}

func ensureDir(path string) {
	if err := os.MkdirAll(path, 0o755); err != nil {
		fatalWait("Cannot create directory %s: %v", path, err)
	}
}

func choosePorts(primaryHTTP, primaryWS, fallbackHTTP, fallbackWS int) (int, int) {
	if portFree(primaryHTTP) && portFree(primaryWS) {
		return primaryHTTP, primaryWS
	}
	fmt.Printf("  Ports %d/%d busy, trying %d/%d...\n", primaryHTTP, primaryWS, fallbackHTTP, fallbackWS)
	if portFree(fallbackHTTP) && portFree(fallbackWS) {
		return fallbackHTTP, fallbackWS
	}
	fatalWait("No free ports available for RPC. Stop other services or try later.")
	return 0, 0
}

func portFree(port int) bool {
	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	_ = l.Close()
	return true
}

func waitForRPC(url string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		_, err := doRPC(url, "net_version", nil)
		if err == nil {
			return true
		}
		time.Sleep(1 * time.Second)
	}
	return false
}

func verifyChainID(url string) {
	resp, err := doRPC(url, "eth_chainId", nil)
	if err != nil {
		fmt.Printf("  [WARN] Cannot verify chainId: %v\n", err)
		return
	}
	chainID := strings.Trim(string(resp.Result), "\"")
	if strings.EqualFold(chainID, devnetChainIDHex) {
		fmt.Println("  [OK] Chain ID: 121526 (devnet)")
	} else {
		fmt.Printf("  [WARN] Unexpected chainId: %s (expected %s)\n", chainID, devnetChainIDHex)
	}
}

func connectBootstrap(url string) {
	fmt.Println("  Connecting to devnet bootstrap peer...")
	params := []interface{}{bootstrapEnode}
	_, err := doRPC(url, "admin_addPeer", params)
	if err != nil {
		fmt.Printf("  [WARN] Cannot add bootstrap peer: %v\n", err)
		return
	}
	// Wait a moment and check peer count
	time.Sleep(3 * time.Second)
	resp, err := doRPC(url, "net_peerCount", nil)
	if err == nil {
		peerCount := strings.Trim(string(resp.Result), "\"")
		fmt.Printf("  [OK] Connected peers: %s\n", peerCount)
	}
}

func stopNode(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		fmt.Println("Sending kill signal...")
		_ = cmd.Process.Kill()
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-time.After(10 * time.Second):
		fmt.Println("Node did not exit in time, killing...")
		_ = cmd.Process.Kill()
	case err := <-done:
		if err != nil {
			fmt.Printf("Node exited: %v\n", err)
		} else {
			fmt.Println("Node stopped cleanly.")
		}
	}
}

func doRPC(url, method string, params []interface{}) (*rpcResp, error) {
	req := rpcReq{
		JSONRPC: "2.0",
		ID:      1,
		Method:  method,
		Params:  params,
	}
	b, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpClient := &http.Client{Timeout: 5 * time.Second}
	r, err := httpClient.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	var resp rpcResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("rpc error: %s", resp.Error.Message)
	}
	return &resp, nil
}

func fatalWait(format string, a ...interface{}) {
	fmt.Printf("\nERROR: "+format+"\n", a...)
	fmt.Println()
	pauseBeforeExit()
	os.Exit(1)
}

func pauseBeforeExit() {
	fmt.Println("Press Enter to exit...")
	buf := make([]byte, 1)
	os.Stdin.Read(buf)
}
