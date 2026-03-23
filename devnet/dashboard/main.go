package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

var rpcURL = "http://127.0.0.1:8545"

func rpcCall(method string) (json.RawMessage, error) {
	body := fmt.Sprintf(`{"jsonrpc":"2.0","method":"%s","params":[],"id":1}`, method)
	resp, err := http.Post(rpcURL, "application/json", strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	var r struct {
		Result json.RawMessage `json:"result"`
	}
	json.Unmarshal(data, &r)
	return r.Result, nil
}

func handleAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	results := make(map[string]json.RawMessage)
	methods := []string{
		"ethernova_nodeHealth",
		"ethernova_evmProfile",
		"ethernova_adaptiveGas",
		"ethernova_executionMode",
		"ethernova_parallelStats",
		"ethernova_callCache",
		"ethernova_optimizer",
		"ethernova_autoTuner",
	}
	for _, m := range methods {
		if res, err := rpcCall(m); err == nil {
			key := strings.TrimPrefix(m, "ethernova_")
			results[key] = res
		}
	}
	json.NewEncoder(w).Encode(results)
}

func main() {
	http.HandleFunc("/api/stats", handleAPI)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, dashboardHTML)
	})
	log.Printf("Dashboard on :8081")
	log.Fatal(http.ListenAndServe(":8081", nil))
}

var dashboardHTML = `<!DOCTYPE html>
<html>
<head>
<title>Ethernova Devnet Dashboard</title>
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: -apple-system, sans-serif; background: #0b1533; color: #abc3d5; padding: 20px; }
h1 { color: #05b8f1; text-align: center; margin-bottom: 5px; }
.subtitle { text-align: center; color: #90accb; margin-bottom: 25px; font-size: 14px; }
.grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(350px, 1fr)); gap: 15px; max-width: 1200px; margin: 0 auto; }
.card { background: linear-gradient(180deg, rgba(32,47,84,0.95), rgba(30,50,97,0.95)); border: 1px solid rgba(56,68,188,0.25); border-radius: 12px; padding: 18px; }
.card h2 { color: #05b8f1; font-size: 15px; margin-bottom: 12px; border-bottom: 1px solid rgba(5,184,241,0.15); padding-bottom: 8px; }
.stat { display: flex; justify-content: space-between; padding: 5px 0; font-size: 13px; }
.stat .label { color: #90accb; }
.stat .value { color: #fff; font-weight: 600; }
.green { color: #4ade80 !important; }
.red { color: #f87171 !important; }
.yellow { color: #fbbf24 !important; }
.badge { display: inline-block; padding: 2px 8px; border-radius: 4px; font-size: 11px; font-weight: 600; }
.badge-on { background: rgba(74,222,128,0.2); color: #4ade80; }
.badge-off { background: rgba(248,113,113,0.2); color: #f87171; }
.refresh { text-align: center; margin-top: 15px; color: #90accb; font-size: 12px; }
table { width: 100%; border-collapse: collapse; font-size: 12px; margin-top: 8px; }
th { text-align: left; color: #05b8f1; padding: 4px; border-bottom: 1px solid rgba(5,184,241,0.2); }
td { padding: 4px; border-bottom: 1px solid rgba(255,255,255,0.05); }
</style>
</head>
<body>
<h1>Ethernova Devnet Dashboard</h1>
<p class="subtitle">ChainId 121526 | Adaptive EVM | Auto-refreshes every 5s</p>
<div class="grid" id="dashboard">Loading...</div>
<p class="refresh" id="lastUpdate"></p>
<script>
function badge(v) { return v ? '<span class="badge badge-on">ON</span>' : '<span class="badge badge-off">OFF</span>'; }
function fmt(n) { return n != null ? n.toLocaleString() : '0'; }

async function refresh() {
  try {
    const r = await fetch('/api/stats');
    const d = await r.json();
    const h = d.nodeHealth || {};
    const p = d.evmProfile || {};
    const g = d.adaptiveGas || {};
    const e = d.executionMode || {};
    const c = d.callCache || {};
    const o = d.optimizer || {};
    const t = d.autoTuner || {};
    const ps = d.parallelStats || {};

    let opcodeRows = '';
    if (p.topOpcodes) {
      p.topOpcodes.slice(0,10).forEach(op => {
        opcodeRows += '<tr><td>'+op.opcode+'</td><td>'+fmt(op.count)+'</td><td>'+op.percentage.toFixed(1)+'%</td></tr>';
      });
    }

    let contractRows = '';
    if (g.contracts) {
      g.contracts.forEach(c => {
        const disc = c.discountPercent > 0 ? '<span class="green">-'+c.discountPercent+'%</span>' : (c.penaltyPercent > 0 ? '<span class="red">+'+c.penaltyPercent+'%</span>' : 'normal');
        contractRows += '<tr><td>'+c.address.slice(0,10)+'...</td><td>'+fmt(c.callCount)+'</td><td>'+c.purePercent+'%</td><td>'+disc+'</td></tr>';
      });
    }

    document.getElementById('dashboard').innerHTML =
    '<div class="card"><h2>Node Health</h2>'+
      '<div class="stat"><span class="label">Version</span><span class="value">'+(h.version||'?')+'</span></div>'+
      '<div class="stat"><span class="label">Block</span><span class="value">'+fmt(h.currentBlock)+'</span></div>'+
      '<div class="stat"><span class="label">Peers</span><span class="value">'+fmt(h.peerCount)+'</span></div>'+
      '<div class="stat"><span class="label">Syncing</span><span class="value">'+(h.syncing?'<span class="yellow">Yes</span>':'<span class="green">No</span>')+'</span></div>'+
      '<div class="stat"><span class="label">Uptime</span><span class="value">'+Math.floor((h.uptimeSeconds||0)/60)+'m</span></div>'+
      '<div class="stat"><span class="label">Memory</span><span class="value">'+(h.memoryMB||0)+' MB</span></div>'+
    '</div>'+

    '<div class="card"><h2>Adaptive Gas</h2>'+
      '<div class="stat"><span class="label">Status</span><span class="value">'+badge(g.enabled)+'</span></div>'+
      '<div class="stat"><span class="label">Discount</span><span class="value green">'+fmt(g.discountPercent)+'%</span></div>'+
      '<div class="stat"><span class="label">Penalty</span><span class="value red">'+fmt(g.penaltyPercent)+'%</span></div>'+
      (contractRows ? '<table><tr><th>Contract</th><th>Calls</th><th>Pure</th><th>Effect</th></tr>'+contractRows+'</table>' : '')+
    '</div>'+

    '<div class="card"><h2>Execution Mode</h2>'+
      '<div class="stat"><span class="label">Mode</span><span class="value">'+(e.mode||'standard')+'</span></div>'+
      '<div class="stat"><span class="label">Fast Executions</span><span class="value">'+fmt(e.fastExecutions)+'</span></div>'+
      '<div class="stat"><span class="label">Skipped Checks</span><span class="value">'+fmt(e.skippedChecks)+'</span></div>'+
    '</div>'+

    '<div class="card"><h2>Call Cache</h2>'+
      '<div class="stat"><span class="label">Status</span><span class="value">'+badge(c.enabled)+'</span></div>'+
      '<div class="stat"><span class="label">Size</span><span class="value">'+fmt(c.size)+' / '+fmt(c.maxSize)+'</span></div>'+
      '<div class="stat"><span class="label">Hits</span><span class="value green">'+fmt(c.hits)+'</span></div>'+
      '<div class="stat"><span class="label">Misses</span><span class="value">'+fmt(c.misses)+'</span></div>'+
      '<div class="stat"><span class="label">Hit Rate</span><span class="value">'+(c.hitRate||0).toFixed(1)+'%</span></div>'+
    '</div>'+

    '<div class="card"><h2>Opcode Optimizer</h2>'+
      '<div class="stat"><span class="label">Status</span><span class="value">'+badge(o.enabled)+'</span></div>'+
      '<div class="stat"><span class="label">Redundant Ops</span><span class="value">'+fmt(o.redundantOps)+'</span></div>'+
      '<div class="stat"><span class="label">Gas Refunded</span><span class="value green">'+fmt(o.gasRefunded)+'</span></div>'+
      '<div class="stat"><span class="label">Patterns Found</span><span class="value">'+fmt(o.patternsFound)+'</span></div>'+
    '</div>'+

    '<div class="card"><h2>Auto-Tuner</h2>'+
      '<div class="stat"><span class="label">Status</span><span class="value">'+badge(t.enabled)+'</span></div>'+
      '<div class="stat"><span class="label">Tune Interval</span><span class="value">every '+fmt(t.tuneInterval)+' blocks</span></div>'+
      '<div class="stat"><span class="label">Last Tuned</span><span class="value">block '+fmt(t.lastTunedBlock)+'</span></div>'+
      '<div class="stat"><span class="label">Discount Range</span><span class="value">'+fmt(t.minDiscount)+'% - '+fmt(t.maxDiscount)+'%</span></div>'+
      '<div class="stat"><span class="label">Penalty Range</span><span class="value">'+fmt(t.minPenalty)+'% - '+fmt(t.maxPenalty)+'%</span></div>'+
    '</div>'+

    '<div class="card"><h2>Parallel Execution</h2>'+
      '<div class="stat"><span class="label">Status</span><span class="value">'+badge(ps.enabled)+'</span></div>'+
      '<div class="stat"><span class="label">Classified</span><span class="value">'+fmt(ps.totalClassified)+'</span></div>'+
      '<div class="stat"><span class="label">Parallel Eligible</span><span class="value green">'+fmt(ps.parallelEligible)+'</span></div>'+
      '<div class="stat"><span class="label">Sequential Only</span><span class="value">'+fmt(ps.sequentialOnly)+'</span></div>'+
      '<div class="stat"><span class="label">Merged</span><span class="value green">'+fmt(ps.mergedSuccessful)+'</span></div>'+
    '</div>'+

    '<div class="card"><h2>EVM Profiling</h2>'+
      '<div class="stat"><span class="label">Total Ops</span><span class="value">'+fmt(p.totalOps)+'</span></div>'+
      '<div class="stat"><span class="label">Total Gas</span><span class="value">'+fmt(p.totalGas)+'</span></div>'+
      (opcodeRows ? '<table><tr><th>Opcode</th><th>Count</th><th>%</th></tr>'+opcodeRows+'</table>' : '')+
    '</div>';

    document.getElementById('lastUpdate').textContent = 'Last updated: ' + new Date().toLocaleTimeString();
  } catch(e) {
    document.getElementById('dashboard').innerHTML = '<div class="card"><h2>Error</h2><p>'+e.message+'</p></div>';
  }
}
refresh();
setInterval(refresh, 5000);
</script>
</body>
</html>`
