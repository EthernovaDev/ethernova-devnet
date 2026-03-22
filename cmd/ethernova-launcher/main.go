package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultHTTPPort = 8545
	defaultWSPort   = 8546
	fallbackHTTP    = 8547
	fallbackWS      = 8548

	mainnetNetworkID = "121525"
	genesisHashExp   = "0xc3812eb81498965a3f9ff3e73d2f423934e6d440578d4f4fbb6623cc61c453d9"
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
	fmt.Println("EthernovaNode launcher (Windows, portable)")

	exePath, err := os.Executable()
	checkFatal(err, "resolve executable path")
	exeDir := filepath.Dir(exePath)
	checkFatal(os.Chdir(exeDir), "chdir to executable directory")

	paths := struct {
		bin       string
		genesis   string
		dataDir   string
		logsDir   string
		initLog   string
		initErr   string
		nodeLog   string
		nodeErr   string
		ipcPath   string
		pidPath   string
		httpPort  int
		wsPort    int
		networkID string
	}{
		bin:       filepath.Join(exeDir, "ethernova.exe"),
		genesis:   filepath.Join(exeDir, "genesis-mainnet.json"),
		dataDir:   filepath.Join(exeDir, "data-mainnet"),
		logsDir:   filepath.Join(exeDir, "logs"),
		initLog:   filepath.Join(exeDir, "logs", "init.log"),
		initErr:   filepath.Join(exeDir, "logs", "init.err.log"),
		nodeLog:   filepath.Join(exeDir, "logs", "mainnet-node.log"),
		nodeErr:   filepath.Join(exeDir, "logs", "mainnet-node.err.log"),
		ipcPath:   filepath.Join(exeDir, "data-mainnet", "ethernova.ipc"),
		pidPath:   filepath.Join(exeDir, "logs", "mainnet-node.pid"),
		httpPort:  defaultHTTPPort,
		wsPort:    defaultWSPort,
		networkID: mainnetNetworkID,
	}

	printlnPath("Binary", paths.bin)
	printlnPath("Genesis", paths.genesis)
	printlnPath("DataDir", paths.dataDir)
	printlnPath("LogsDir", paths.logsDir)

	ensureFile(paths.bin, "ethernova.exe not found next to launcher")
	ensureFile(paths.genesis, "genesis-mainnet.json not found next to launcher")
	ensureDir(paths.logsDir)
	ensureDir(paths.dataDir)

	initNeeded := needsInit(paths.dataDir)
	fmt.Printf("Init needed: %v\n", initNeeded)

	if initNeeded {
		runInit(paths.bin, paths.dataDir, paths.genesis, paths.initLog, paths.initErr)
	}

	paths.httpPort, paths.wsPort = choosePorts(defaultHTTPPort, defaultWSPort, fallbackHTTP, fallbackWS)
	fmt.Printf("HTTP RPC: http://127.0.0.1:%d\n", paths.httpPort)
	fmt.Printf("WS RPC:   ws://127.0.0.1:%d\n", paths.wsPort)

	proc := startNode(paths)
	fmt.Printf("Node started (pid=%d). Logs: %s / %s\n", proc.Process.Pid, paths.nodeLog, paths.nodeErr)

	checkRPC(paths.httpPort)

	fmt.Println("Press Enter to stop the node...")
	_, _ = fmt.Scanln()

	stopNode(proc)
	fmt.Println("Node stopped. Bye.")
}

func printlnPath(label, path string) {
	fmt.Printf("%s: %s\n", label, path)
}

func ensureFile(path, msg string) {
	st, err := os.Stat(path)
	if err != nil || st.IsDir() {
		fatalf("%s (%s)", msg, path)
	}
}

func ensureDir(path string) {
	if err := os.MkdirAll(path, 0o755); err != nil {
		fatalf("cannot create directory %s: %v", path, err)
	}
}

func needsInit(dataDir string) bool {
	paths := []string{
		filepath.Join(dataDir, "geth", "chaindata"),
		filepath.Join(dataDir, "geth", "LOCK"),
		filepath.Join(dataDir, "geth", "ancient", "chain"),
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return false
		}
	}
	return true
}

func runInit(binary, dataDir, genesis, logOut, logErr string) {
	fmt.Println("Initializing genesis...")
	outFile, err := os.Create(logOut)
	checkFatal(err, "open init log")
	defer outFile.Close()
	errFile, err := os.Create(logErr)
	checkFatal(err, "open init err log")
	defer errFile.Close()

	cmd := exec.Command(binary, "--datadir", dataDir, "init", genesis)
	cmd.Stdout = outFile
	cmd.Stderr = errFile
	if err := cmd.Run(); err != nil {
		fatalf("genesis init failed (see %s / %s): %v", logOut, logErr, err)
	}
	fmt.Println("Genesis init OK.")
}

func choosePorts(primaryHTTP, primaryWS, fallbackHTTP, fallbackWS int) (int, int) {
	if portFree(primaryHTTP) && portFree(primaryWS) {
		return primaryHTTP, primaryWS
	}
	fmt.Printf("Port %d or %d busy, falling back to %d/%d\n", primaryHTTP, primaryWS, fallbackHTTP, fallbackWS)
	if !portFree(fallbackHTTP) || !portFree(fallbackWS) {
		fatalf("fallback ports %d/%d are busy; stop other services or choose a free port", fallbackHTTP, fallbackWS)
	}
	return fallbackHTTP, fallbackWS
}

func portFree(port int) bool {
	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	_ = l.Close()
	return true
}

func startNode(paths struct {
	bin       string
	genesis   string
	dataDir   string
	logsDir   string
	initLog   string
	initErr   string
	nodeLog   string
	nodeErr   string
	ipcPath   string
	pidPath   string
	httpPort  int
	wsPort    int
	networkID string
}) *exec.Cmd {
	outFile, err := os.Create(paths.nodeLog)
	checkFatal(err, "open node log")
	errFile, err := os.Create(paths.nodeErr)
	checkFatal(err, "open node err log")

	args := []string{
		"--datadir", paths.dataDir,
		"--networkid", paths.networkID,
		"--port", "30303",
		"--http", "--http.addr", "127.0.0.1", "--http.port", fmt.Sprintf("%d", paths.httpPort),
		"--http.api", "eth,net,web3,txpool",
		"--ws", "--ws.addr", "127.0.0.1", "--ws.port", fmt.Sprintf("%d", paths.wsPort),
		"--ws.api", "eth,net,web3,txpool",
		"--ipcpath", paths.ipcPath,
		"--authrpc.addr", "127.0.0.1", "--authrpc.port", "8551",
		"--verbosity", "3",
	}

	cmd := exec.Command(paths.bin, args...)
	cmd.Stdout = outFile
	cmd.Stderr = errFile

	if err := cmd.Start(); err != nil {
		fatalf("failed to start node: %v", err)
	}

	// write pid file best-effort
	_ = os.WriteFile(paths.pidPath, []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0o644)
	return cmd
}

func stopNode(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		fmt.Printf("interrupt failed, killing: %v\n", err)
		_ = cmd.Process.Kill()
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-time.After(5 * time.Second):
		fmt.Println("node did not exit in time, killing...")
		_ = cmd.Process.Kill()
	case err := <-done:
		if err != nil {
			fmt.Printf("node exited with error: %v\n", err)
		}
	}
}

func checkRPC(port int) {
	time.Sleep(3 * time.Second)
	url := fmt.Sprintf("http://127.0.0.1:%d", port)
	chainID, err := rpcChainID(url)
	if err != nil {
		fmt.Printf("WARN: RPC not reachable yet (%v). Check logs.\n", err)
		return
	}
	if strings.EqualFold(chainID, "0x1dab5") {
		fmt.Println("RPC OK (chainId=121525)")
	} else {
		fmt.Printf("WARN: RPC chainId unexpected: %s\n", chainID)
	}
	hash, err := rpcBlock0Hash(url)
	if err == nil && strings.EqualFold(hash, genesisHashExp) {
		fmt.Println("Genesis hash matches expected.")
	} else if err == nil {
		fmt.Printf("WARN: Genesis hash differs: %s (expected %s)\n", hash, genesisHashExp)
	}
}

func rpcChainID(url string) (string, error) {
	resp, err := doRPC(url, "eth_chainId", nil)
	if err != nil {
		return "", err
	}
	return string(resp.Result), nil
}

func rpcBlock0Hash(url string) (string, error) {
	params := []interface{}{"0x0", false}
	resp, err := doRPC(url, "eth_getBlockByNumber", params)
	if err != nil {
		return "", err
	}
	if len(resp.Result) == 0 {
		return "", errors.New("empty result")
	}
	var parsed struct {
		Hash string `json:"hash"`
	}
	if err := json.Unmarshal(resp.Result, &parsed); err != nil {
		return "", err
	}
	return parsed.Hash, nil
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

func checkFatal(err error, context string) {
	if err != nil {
		fatalf("%s: %v", context, err)
	}
}

func fatalf(format string, a ...interface{}) {
	fmt.Printf("ERROR: "+format+"\n", a...)
	os.Exit(1)
}
