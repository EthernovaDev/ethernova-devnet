package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

var (
	rpcURL     = envOr("FAUCET_RPC_URL", "http://127.0.0.1:28545")
	listenAddr = envOr("FAUCET_LISTEN", ":18088")
	privateKey *ecdsa.PrivateKey
	fromAddr   common.Address
	chainID    = big.NewInt(121526)
	amount     = new(big.Int).Mul(big.NewInt(100000), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)) // 100000 NOVA
	cooldown   = 5 * time.Minute

	lastByAddr = make(map[string]time.Time)
	lastByIP   = make(map[string]time.Time)
	mu         sync.Mutex
	nonceMu    sync.Mutex
)

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	keyHex := os.Getenv("FAUCET_PRIVATE_KEY")
	if keyHex == "" {
		log.Fatal("FAUCET_PRIVATE_KEY environment variable required")
	}
	keyHex = strings.TrimPrefix(keyHex, "0x")

	var err error
	privateKey, err = crypto.HexToECDSA(keyHex)
	if err != nil {
		log.Fatalf("Invalid private key: %v", err)
	}
	fromAddr = crypto.PubkeyToAddress(privateKey.PublicKey)
	log.Printf("Faucet address: %s", fromAddr.Hex())

	// Check balance
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		log.Fatalf("Cannot connect to RPC: %v", err)
	}
	balance, err := client.BalanceAt(context.Background(), fromAddr, nil)
	if err != nil {
		log.Printf("Warning: cannot check balance: %v", err)
	} else {
		nova := new(big.Int).Div(balance, new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
		log.Printf("Faucet balance: %s NOVA", nova.String())
	}
	client.Close()

	http.HandleFunc("/api/faucet", handleFaucet)
	http.HandleFunc("/api/status", handleStatus)
	http.HandleFunc("/", handleUI)

	log.Printf("Ethernova DEVNET Faucet starting on %s -> %s", listenAddr, rpcURL)
	log.Fatal(http.ListenAndServe(listenAddr, nil))
}

type faucetRequest struct {
	Address string `json:"address"`
}

type faucetResponse struct {
	Success bool   `json:"success"`
	TxHash  string `json:"txHash,omitempty"`
	Error   string `json:"error,omitempty"`
	Amount  string `json:"amount,omitempty"`
}

func getClientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return strings.Split(fwd, ",")[0]
	}
	if real := r.Header.Get("X-Real-IP"); real != "" {
		return real
	}
	return strings.Split(r.RemoteAddr, ":")[0]
}

func handleFaucet(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		return
	}
	if r.Method != "POST" {
		json.NewEncoder(w).Encode(faucetResponse{Error: "POST only"})
		return
	}

	var req faucetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(faucetResponse{Error: "invalid JSON"})
		return
	}

	addr := strings.ToLower(strings.TrimSpace(req.Address))
	if len(addr) != 42 || !strings.HasPrefix(addr, "0x") {
		json.NewEncoder(w).Encode(faucetResponse{Error: "invalid address"})
		return
	}

	ip := getClientIP(r)

	// Rate limit by address AND IP
	mu.Lock()
	if last, ok := lastByAddr[addr]; ok && time.Since(last) < cooldown {
		remaining := cooldown - time.Since(last)
		mu.Unlock()
		json.NewEncoder(w).Encode(faucetResponse{
			Error: fmt.Sprintf("address rate limited, try again in %s", remaining.Round(time.Second)),
		})
		return
	}
	if last, ok := lastByIP[ip]; ok && time.Since(last) < cooldown {
		remaining := cooldown - time.Since(last)
		mu.Unlock()
		json.NewEncoder(w).Encode(faucetResponse{
			Error: fmt.Sprintf("IP rate limited, try again in %s", remaining.Round(time.Second)),
		})
		return
	}
	lastByAddr[addr] = time.Now()
	lastByIP[ip] = time.Now()
	mu.Unlock()

	// Send transaction
	txHash, err := sendNova(common.HexToAddress(addr))
	if err != nil {
		// Undo rate limit on failure
		mu.Lock()
		delete(lastByAddr, addr)
		delete(lastByIP, ip)
		mu.Unlock()
		json.NewEncoder(w).Encode(faucetResponse{Error: err.Error()})
		return
	}

	log.Printf("Sent 100000 NOVA to %s from %s tx=%s ip=%s", addr, fromAddr.Hex(), txHash, ip)
	json.NewEncoder(w).Encode(faucetResponse{
		Success: true,
		TxHash:  txHash,
		Amount:  "100000 NOVA",
	})
}

func sendNova(to common.Address) (string, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return "", fmt.Errorf("RPC connection failed")
	}
	defer client.Close()

	nonceMu.Lock()
	defer nonceMu.Unlock()

	nonce, err := client.PendingNonceAt(context.Background(), fromAddr)
	if err != nil {
		return "", fmt.Errorf("cannot get nonce")
	}

	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		gasPrice = big.NewInt(1000000000) // 1 gwei default
	}

	tx := types.NewTransaction(nonce, to, amount, 21000, gasPrice, nil)

	signer := types.NewEIP155Signer(chainID)
	signedTx, err := types.SignTx(tx, signer, privateKey)
	if err != nil {
		return "", fmt.Errorf("signing failed")
	}

	if err := client.SendTransaction(context.Background(), signedTx); err != nil {
		return "", fmt.Errorf("tx failed: %v", err)
	}

	return signedTx.Hash().Hex(), nil
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "RPC unavailable"})
		return
	}
	defer client.Close()

	block, _ := client.BlockNumber(context.Background())
	balance, _ := client.BalanceAt(context.Background(), fromAddr, nil)
	var balNova string
	if balance != nil {
		nova := new(big.Int).Div(balance, new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
		balNova = nova.String() + " NOVA"
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"network":       "Ethernova Devnet",
		"chainId":       121526,
		"block":         block,
		"faucetAddress": fromAddr.Hex(),
		"faucetBalance": balNova,
		"amount":        "100000 NOVA",
		"cooldown":      cooldown.String(),
		"explorer":      "https://devexplorer.ethnova.net",
		"rpc":           "https://devrpc.ethnova.net",
	})
}

func handleUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, faucetHTML)
}

var faucetHTML = `<!DOCTYPE html>
<html>
<head>
<title>Ethernova Devnet Faucet</title>
<meta name="viewport" content="width=device-width, initial-scale=1">
<style>
* { box-sizing: border-box; margin: 0; padding: 0; }
body {
  font-family: 'Inter', -apple-system, sans-serif;
  background: #0b1020;
  color: #eaf0ff;
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 60px 20px;
}
.badge {
  display: inline-block;
  background: rgba(37,230,255,0.15);
  border: 1px solid rgba(37,230,255,0.3);
  color: #25e6ff;
  padding: 4px 14px;
  border-radius: 20px;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 1px;
  text-transform: uppercase;
  margin-bottom: 16px;
}
h1 {
  font-size: 36px;
  font-weight: 700;
  background: linear-gradient(135deg, #7c5cff, #25e6ff);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  margin-bottom: 8px;
}
.subtitle {
  color: #a8b4d9;
  margin-bottom: 40px;
  font-size: 16px;
}
.card {
  background: #111a33;
  border: 1px solid rgba(124,92,255,0.2);
  border-radius: 16px;
  padding: 32px;
  max-width: 480px;
  width: 100%;
}
input {
  width: 100%;
  padding: 14px 16px;
  border: 1px solid rgba(124,92,255,0.3);
  background: #0b1020;
  color: #eaf0ff;
  border-radius: 10px;
  font-size: 15px;
  outline: none;
  transition: border 0.2s;
}
input:focus { border-color: #7c5cff; }
input::placeholder { color: #4a5580; }
button {
  width: 100%;
  padding: 14px;
  background: linear-gradient(135deg, #7c5cff, #25e6ff);
  border: none;
  color: white;
  border-radius: 10px;
  cursor: pointer;
  font-size: 16px;
  font-weight: 600;
  margin-top: 16px;
  transition: opacity 0.2s;
}
button:hover { opacity: 0.9; }
button:disabled { opacity: 0.5; cursor: not-allowed; }
.result {
  margin-top: 20px;
  padding: 16px;
  border-radius: 10px;
  font-size: 14px;
  word-break: break-all;
}
.success {
  background: rgba(37,230,255,0.1);
  border: 1px solid rgba(37,230,255,0.25);
  color: #25e6ff;
}
.error {
  background: rgba(255,80,80,0.1);
  border: 1px solid rgba(255,80,80,0.25);
  color: #ff5050;
}
.info {
  margin-top: 24px;
  text-align: center;
  color: #4a5580;
  font-size: 13px;
}
.info a { color: #7c5cff; text-decoration: none; }
.info a:hover { color: #25e6ff; }
.stats {
  display: flex;
  gap: 16px;
  margin-bottom: 24px;
}
.stat {
  flex: 1;
  background: rgba(124,92,255,0.08);
  border-radius: 10px;
  padding: 12px;
  text-align: center;
}
.stat-value { font-size: 18px; font-weight: 700; color: #25e6ff; }
.stat-label { font-size: 11px; color: #4a5580; margin-top: 4px; }
</style>
</head>
<body>
<span class="badge">Devnet Testnet</span>
<h1>Ethernova Faucet</h1>
<p class="subtitle">Get free NOVA tokens for testing on the devnet</p>

<div class="card">
  <div class="stats">
    <div class="stat">
      <div class="stat-value">100000 NOVA</div>
      <div class="stat-label">Per Request</div>
    </div>
    <div class="stat">
      <div class="stat-value">121526</div>
      <div class="stat-label">Chain ID</div>
    </div>
    <div class="stat">
      <div class="stat-value" id="block">-</div>
      <div class="stat-label">Block</div>
    </div>
  </div>

  <input id="addr" placeholder="0x... paste your wallet address" />
  <button id="btn" onclick="request()">Request 100000 NOVA</button>
  <div id="result"></div>

  <div class="info">
    <p>Cooldown: 5 minutes per address / IP</p>
    <p style="margin-top:8px">
      <a href="https://devexplorer.ethnova.net" target="_blank">Explorer</a> &middot;
      <a href="https://devrpc.ethnova.net" target="_blank">RPC</a> &middot;
      <a href="https://github.com/EthernovaDev/ethernova-devnet" target="_blank">GitHub</a>
    </p>
    <p style="margin-top:12px;color:#2a3355">These are devnet tokens with no real value</p>
  </div>
</div>

<script>
async function request() {
  const addr = document.getElementById('addr').value.trim();
  const res = document.getElementById('result');
  const btn = document.getElementById('btn');
  if (!addr) { res.className='result error'; res.innerHTML='Enter an address'; return; }
  btn.disabled = true;
  btn.textContent = 'Sending...';
  res.innerHTML = '';
  try {
    const r = await fetch('/api/faucet', {method:'POST', headers:{'Content-Type':'application/json'}, body: JSON.stringify({address: addr})});
    const j = await r.json();
    if (j.success) {
      res.className = 'result success';
      res.innerHTML = 'Sent ' + j.amount + '<br><a href="https://devexplorer.ethnova.net/tx/' + j.txHash + '" target="_blank" style="color:#7c5cff">' + j.txHash.substring(0,20) + '...</a>';
    } else {
      res.className = 'result error';
      res.innerHTML = j.error;
    }
  } catch(e) { res.className = 'result error'; res.innerHTML = 'Network error'; }
  btn.disabled = false;
  btn.textContent = 'Request 100000 NOVA';
}
// Load block number
fetch('/api/status').then(r=>r.json()).then(j=>{
  document.getElementById('block').textContent = j.block || '-';
}).catch(()=>{});
</script>
</body>
</html>`
