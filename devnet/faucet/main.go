package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	rpcURL       = "http://127.0.0.1:8545"
	listenAddr   = ":8080"
	password     = "devnet123"
	cooldown     = 5 * time.Minute
	amount       = "0x8AC7230489E80000" // 10 ETH in wei
	lastRequests = make(map[string]time.Time)
	mu           sync.Mutex
)

type faucetRequest struct {
	Address string `json:"address"`
}

type faucetResponse struct {
	Success bool   `json:"success"`
	TxHash  string `json:"txHash,omitempty"`
	Error   string `json:"error,omitempty"`
	Amount  string `json:"amount,omitempty"`
}

func rpcCall(method, params string) (json.RawMessage, error) {
	body := fmt.Sprintf(`{"jsonrpc":"2.0","method":"%s","params":%s,"id":1}`, method, params)
	resp, err := http.Post(rpcURL, "application/json", strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	var r struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	json.Unmarshal(data, &r)
	if r.Error != nil {
		return nil, fmt.Errorf("%s", r.Error.Message)
	}
	return r.Result, nil
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

	// Rate limit
	mu.Lock()
	if last, ok := lastRequests[addr]; ok && time.Since(last) < cooldown {
		remaining := cooldown - time.Since(last)
		mu.Unlock()
		json.NewEncoder(w).Encode(faucetResponse{
			Error: fmt.Sprintf("rate limited, try again in %s", remaining.Round(time.Second)),
		})
		return
	}
	lastRequests[addr] = time.Now()
	mu.Unlock()

	// Unlock sender
	_, err := rpcCall("personal_unlockAccount", fmt.Sprintf(`["%s","%s",60]`, "0x386d7a07843e117129e8e445b9bce5961a20cff9", password))
	if err != nil {
		// Try with first account
		rpcCall("personal_unlockAccount", fmt.Sprintf(`["0x386d7a07843e117129e8e445b9bce5961a20cff9","%s",60]`, password))
	}

	// Send ETH
	params := fmt.Sprintf(`[{"from":"0x386d7a07843e117129e8e445b9bce5961a20cff9","to":"%s","value":"%s","gas":"0x5208"}]`, addr, amount)
	result, err := rpcCall("eth_sendTransaction", params)
	if err != nil {
		json.NewEncoder(w).Encode(faucetResponse{Error: err.Error()})
		return
	}

	var txHash string
	json.Unmarshal(result, &txHash)

	log.Printf("Faucet: sent 10 ETH to %s tx=%s", addr, txHash)
	json.NewEncoder(w).Encode(faucetResponse{
		Success: true,
		TxHash:  txHash,
		Amount:  "10 ETH",
	})
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	result, _ := rpcCall("eth_blockNumber", "[]")
	var block string
	json.Unmarshal(result, &block)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"network":  "ethernova-devnet",
		"chainId":  121526,
		"block":    block,
		"faucet":   "10 ETH per request",
		"cooldown": cooldown.String(),
	})
}

func main() {
	http.HandleFunc("/api/faucet", handleFaucet)
	http.HandleFunc("/api/status", handleStatus)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, faucetHTML)
	})

	log.Printf("Ethernova Devnet Faucet starting on %s", listenAddr)
	log.Fatal(http.ListenAndServe(listenAddr, nil))
}

var faucetHTML = `<!DOCTYPE html>
<html>
<head>
<title>Ethernova Devnet Faucet</title>
<style>
body { font-family: -apple-system, sans-serif; background: #0b1533; color: #abc3d5; display: flex; justify-content: center; padding-top: 80px; }
.container { max-width: 500px; width: 100%; }
h1 { color: #05b8f1; text-align: center; }
.subtitle { text-align: center; color: #90accb; margin-bottom: 30px; }
input { width: 100%; padding: 12px; border: 1px solid rgba(5,184,241,0.3); background: #202f54; color: #abc3d5; border-radius: 8px; font-size: 14px; box-sizing: border-box; }
button { width: 100%; padding: 12px; background: linear-gradient(135deg, #3844bc, #05b8f1); border: none; color: white; border-radius: 8px; cursor: pointer; font-size: 16px; margin-top: 12px; }
button:hover { opacity: 0.9; }
.result { margin-top: 20px; padding: 15px; border-radius: 8px; }
.success { background: rgba(5,184,241,0.15); border: 1px solid rgba(5,184,241,0.3); }
.error { background: rgba(255,50,50,0.15); border: 1px solid rgba(255,50,50,0.3); }
.info { text-align: center; margin-top: 30px; color: #90accb; font-size: 13px; }
</style>
</head>
<body>
<div class="container">
<h1>Ethernova Devnet Faucet</h1>
<p class="subtitle">ChainId 121526 | 10 ETH per request</p>
<input id="addr" placeholder="0x... your address" />
<button onclick="request()">Request 10 ETH</button>
<div id="result"></div>
<p class="info">Cooldown: 5 minutes per address</p>
</div>
<script>
async function request() {
  const addr = document.getElementById('addr').value;
  const res = document.getElementById('result');
  res.innerHTML = 'Sending...';
  try {
    const r = await fetch('/api/faucet', {method:'POST', headers:{'Content-Type':'application/json'}, body: JSON.stringify({address: addr})});
    const j = await r.json();
    if (j.success) {
      res.className = 'result success';
      res.innerHTML = 'Sent ' + j.amount + '<br>TX: ' + j.txHash;
    } else {
      res.className = 'result error';
      res.innerHTML = j.error;
    }
  } catch(e) { res.className = 'result error'; res.innerHTML = e.message; }
}
</script>
</body>
</html>`
