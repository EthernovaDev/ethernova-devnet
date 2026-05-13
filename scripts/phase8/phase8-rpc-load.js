"use strict";

// Phase 8  -  Scenario O: Concurrency / Load
//
// Fire many read-only nova_* + eth_* calls in parallel. Verify:
//   - Node stays responsive (no mass timeouts).
//   - Every response is a valid JSON-RPC envelope.
//   - No "method not found" appears intermittently.
//   - Latency distribution is reasonable for a devnet (we report it, not gate it).

const path = require("path");
const H = require("./phase8-helpers");

const RPC = H.envString("PHASE8_RPC_URL", "http://127.0.0.1:8545");
const CONCURRENCY = H.envNumber("PHASE8_LOAD_CONCURRENCY", 50);
const REQUESTS = H.envNumber("PHASE8_LOAD_REQUESTS", 500);
const REPORT_DIR = H.envString("PHASE8_REPORT_DIR_RUN", path.resolve(__dirname, "..", "..", "reports", "phase8", "real-usage", "manual"));

const suite = new H.Suite("phase8-rpc-load");

const CALLS = [
  { method: "eth_blockNumber", params: [] },
  { method: "eth_chainId", params: [] },
  { method: "nova_getDomain", params: [H.ZERO_ADDR] },
  { method: "nova_getCapabilities", params: [H.ZERO_ADDR] },
  { method: "nova_deferredProcessingStats", params: [] },
  { method: "nova_getPendingEffects", params: [0, 5] },
  { method: "nova_getStateTier", params: [H.ZERO_ADDR, H.ZERO_HASH] },
];

function pickCall() {
  return CALLS[Math.floor(Math.random() * CALLS.length)];
}

async function oneRequest() {
  const c = pickCall();
  const t0 = Date.now();
  try {
    const body = await H.rpc(RPC, c.method, c.params, { timeoutMs: 10000 });
    const elapsed = Date.now() - t0;
    if (!body || typeof body !== "object" || body.jsonrpc !== "2.0") {
      return { ok: false, reason: "bad envelope", method: c.method, elapsed };
    }
    if (body.error) {
      if (body.error.code === H.ERR_METHOD_NOT_FOUND) {
        return { ok: false, reason: "method-not-found", method: c.method, elapsed };
      }
      return { ok: true, elapsed, method: c.method, code: body.error.code };
    }
    return { ok: true, elapsed, method: c.method };
  } catch (err) {
    const elapsed = Date.now() - t0;
    return { ok: false, reason: String(err.message || err), method: c.method, elapsed };
  }
}

async function pool(n, worker) {
  const results = [];
  let inflight = 0;
  let dispatched = 0;
  return new Promise((resolve) => {
    function pump() {
      while (inflight < CONCURRENCY && dispatched < n) {
        const idx = dispatched++;
        inflight++;
        worker(idx).then((r) => {
          results.push(r);
          inflight--;
          if (results.length === n) {
            resolve(results);
          } else {
            pump();
          }
        });
      }
    }
    pump();
  });
}

function percentile(arr, p) {
  if (arr.length === 0) return 0;
  const sorted = [...arr].sort((a, b) => a - b);
  const idx = Math.min(sorted.length - 1, Math.floor((p / 100) * sorted.length));
  return sorted[idx];
}

(async () => {
  console.log("========================================================================");
  console.log(" Phase 8 - Scenario O: Concurrency / Load");
  console.log("========================================================================");
  console.log(`RPC: ${RPC}`);
  console.log(`Concurrency: ${CONCURRENCY}, Requests: ${REQUESTS}`);

  // Warm-up: one call to make sure node is up.
  const warm = await H.rpc(RPC, "eth_blockNumber", [], { timeoutMs: 5000 });
  if (warm.error) {
    suite.fail("O.warmup eth_blockNumber", `error code=${warm.error.code}`, H.SEVERITY.CRITICAL);
    process.exit(1);
  }

  const start = Date.now();
  const results = await pool(REQUESTS, oneRequest);
  const elapsedMs = Date.now() - start;

  const oks = results.filter((r) => r.ok);
  const fails = results.filter((r) => !r.ok);
  const latencies = results.map((r) => r.elapsed);

  const byMethod = {};
  for (const r of results) {
    if (!byMethod[r.method]) byMethod[r.method] = { ok: 0, fail: 0 };
    byMethod[r.method][r.ok ? "ok" : "fail"]++;
  }

  const methodNotFound = fails.filter((r) => r.reason === "method-not-found").length;
  const badEnvelope = fails.filter((r) => r.reason === "bad envelope").length;
  const timeouts = fails.filter((r) => /Timeout/i.test(String(r.reason))).length;

  const stats = {
    elapsedMs,
    requests: REQUESTS,
    concurrency: CONCURRENCY,
    okCount: oks.length,
    failCount: fails.length,
    methodNotFoundCount: methodNotFound,
    badEnvelopeCount: badEnvelope,
    timeoutCount: timeouts,
    rps: Math.round((REQUESTS / elapsedMs) * 1000),
    latency: {
      avg: Math.round(latencies.reduce((a, b) => a + b, 0) / Math.max(1, latencies.length)),
      p50: percentile(latencies, 50),
      p95: percentile(latencies, 95),
      p99: percentile(latencies, 99),
      max: Math.max(...latencies, 0),
    },
    byMethod,
  };

  console.log(JSON.stringify(stats, null, 2));

  await suite.step("O.all requests returned a valid JSON-RPC envelope", async () => {
    if (badEnvelope > 0) throw new Error(`${badEnvelope} responses had invalid JSON-RPC envelope`);
    return `${oks.length}/${REQUESTS} OK envelopes`;
  });

  await suite.step("O.no method-not-found under load", async () => {
    if (methodNotFound > 0) throw new Error(`${methodNotFound} requests got -32601 (method should be registered)`);
    return "0 method-not-found";
  });

  await suite.step("O.timeout rate under 1%", async () => {
    const rate = timeouts / REQUESTS;
    if (rate > 0.01) throw new Error(`timeout rate ${(rate * 100).toFixed(2)}% > 1%`);
    return `${timeouts} timeouts (${(rate * 100).toFixed(2)}%)`;
  });

  await suite.step("O.node still responsive after load", async () => {
    const body = await H.rpc(RPC, "eth_blockNumber", [], { timeoutMs: 5000 });
    if (body.error) throw new Error(`eth_blockNumber error after load: ${body.error.message}`);
    return `block=${parseInt(body.result, 16)}`;
  });

  suite.printFooter();
  const summary = suite.summarize();
  summary.stats = stats;
  const out = path.join(REPORT_DIR, "load-test.json");
  H.writeJson(out, summary);
  console.log(`Wrote: ${out}`);

  if (summary.counts.fail > 0) process.exit(1);
  process.exit(0);
})().catch((err) => {
  console.error("Fatal:", err && err.stack ? err.stack : err);
  process.exit(1);
});
