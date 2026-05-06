// scripts/shared.js — common helpers used across all Phase 5 test scripts.
require("dotenv").config({ path: require("path").resolve(__dirname, "..", ".env") });
const { ethers } = require("ethers");
const fs = require("fs");
const path = require("path");

// ---------- env ----------

function envStr(key, def) { const v = process.env[key]; return (v === undefined || v === "") ? def : v; }
function envInt(key, def) {
  const v = process.env[key];
  if (v === undefined || v === "") return def;
  const n = parseInt(v, 10);
  if (isNaN(n)) throw new Error(`env ${key} = "${v}" not a valid integer`);
  return n;
}
function envBool(key, def) {
  const v = process.env[key];
  if (v === undefined || v === "") return def;
  return v === "1" || v.toLowerCase() === "true" || v.toLowerCase() === "yes";
}

const CONFIG = {
  PRIMARY_RPC: envStr("PRIMARY_RPC", "http://127.0.0.1:8545"),
  PRIMARY_CHAIN_ID: envInt("PRIMARY_CHAIN_ID", 121526),
  PRIVATE_KEY: envStr("PRIVATE_KEY", ""),
  CONSENSUS_NODES: envStr("CONSENSUS_NODES", "Local=http://127.0.0.1:8545,Devrpc=https://devrpc.ethnova.net"),
  ACTIVE_TIER_BLOCKS: envInt("ACTIVE_TIER_BLOCKS", 10),
  WARM_TIER_BLOCKS: envInt("WARM_TIER_BLOCKS", 25),
  COLD_TIER_BLOCKS: envInt("COLD_TIER_BLOCKS", 50),
  WARMING_FEE_PER_BYTE: envInt("WARMING_FEE_PER_BYTE", 5),
  STRESS_CONTRACTS: envInt("STRESS_CONTRACTS", 12),
  STRESS_SLOTS_PER_CONTRACT: envInt("STRESS_SLOTS_PER_CONTRACT", 2),
  STRESS_CONCURRENT_TX: envInt("STRESS_CONCURRENT_TX", 20),
  WAIT_BUFFER_BLOCKS: envInt("WAIT_BUFFER_BLOCKS", 2),
  MAX_WAIT_SECONDS: envInt("MAX_WAIT_SECONDS", 1200),
  REPORT_PATH: envStr("REPORT_PATH", "./report.json"),
  VERBOSITY: envInt("VERBOSITY", 1),
  SKIP_BRUTAL: envBool("SKIP_BRUTAL", false),
  SKIP_REGRESSION: envBool("SKIP_REGRESSION", false),
  SKIP_WITNESS: envBool("SKIP_WITNESS", false),
};

function parseConsensusNodes(s) {
  const out = [];
  for (const part of s.split(",").map((x) => x.trim()).filter(Boolean)) {
    const eq = part.indexOf("=");
    if (eq < 0) throw new Error(`Bad CONSENSUS_NODES entry "${part}" — expected Label=URL`);
    out.push({ label: part.slice(0, eq).trim(), url: part.slice(eq + 1).trim() });
  }
  if (out.length < 1) throw new Error("CONSENSUS_NODES must have at least 1 node");
  return out;
}
CONFIG.NODES = parseConsensusNodes(CONFIG.CONSENSUS_NODES);

// ---------- pretty logging ----------

const C = {
  reset: "\x1b[0m", bold: "\x1b[1m", dim: "\x1b[2m",
  red: "\x1b[31m", green: "\x1b[32m", yellow: "\x1b[33m",
  blue: "\x1b[34m", cyan: "\x1b[36m",
};

function logHeader(suite) {
  const bar = "=".repeat(72);
  console.log("");
  console.log(C.cyan + bar + C.reset);
  console.log(C.cyan + C.bold + " " + suite + C.reset);
  console.log(C.cyan + bar + C.reset);
}
function logCheck(name, status, detail) {
  const marker =
    status === "PASS" ? C.green + "[PASS]" :
    status === "FAIL" ? C.red + "[FAIL]" :
    status === "SKIP" ? C.yellow + "[SKIP]" :
    C.dim + "[ -- ]";
  console.log(`  ${marker} ${C.reset}${name}${detail ? C.dim + " - " + detail + C.reset : ""}`);
}
function logInfo(msg) { if (CONFIG.VERBOSITY >= 1) console.log(C.dim + "  " + msg + C.reset); }
function logDebug(msg) { if (CONFIG.VERBOSITY >= 2) console.log(C.dim + "  [debug] " + msg + C.reset); }

// ---------- multi-node JSON-RPC client ----------

async function rpcCall(url, method, params = [], timeoutMs = 30000) {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeoutMs);
  try {
    const res = await fetch(url, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ jsonrpc: "2.0", method, params, id: 1 }),
      signal: controller.signal,
    });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const json = await res.json();
    if (json.error) throw new Error(`RPC error: ${JSON.stringify(json.error)}`);
    return json.result;
  } finally {
    clearTimeout(timer);
  }
}

async function rpcAll(method, params = []) {
  const results = {};
  await Promise.all(
    CONFIG.NODES.map(async (node) => {
      try {
        results[node.label] = await rpcCall(node.url, method, params);
      } catch (err) {
        results[node.label] = { __error: err.message };
      }
    })
  );
  return results;
}

// ---------- block waiters ----------

async function getBlockNumber(url) {
  const hex = await rpcCall(url, "eth_blockNumber", []);
  return parseInt(hex, 16);
}

async function waitForBlock(url, target, maxSeconds) {
  const start = Date.now();
  let cur = await getBlockNumber(url);
  while (cur < target) {
    const elapsed = (Date.now() - start) / 1000;
    if (elapsed > maxSeconds) {
      throw new Error(`Timeout waiting for block ${target} (current: ${cur}, elapsed: ${elapsed.toFixed(0)}s)`);
    }
    if (CONFIG.VERBOSITY >= 1) {
      const togo = target - cur;
      process.stdout.write(`\r  ${C.dim}waiting block ${cur} -> ${target} (${togo} to go, ${elapsed.toFixed(0)}s elapsed)${C.reset}    `);
    }
    await new Promise((r) => setTimeout(r, 5000));
    cur = await getBlockNumber(url);
  }
  if (CONFIG.VERBOSITY >= 1) process.stdout.write("\r" + " ".repeat(80) + "\r");
  return cur;
}

// ---------- tier helpers ----------

const TIER_NAMES = ["Active", "Warm", "Cold", "Archived", "Expired"];

function expectedTier(ageBlocks, t) {
  if (ageBlocks <= t.ACTIVE_TIER_BLOCKS) return "Active";
  if (ageBlocks <= t.WARM_TIER_BLOCKS) return "Warm";
  if (ageBlocks <= t.COLD_TIER_BLOCKS) return "Cold";
  return "Archived";
}

function expectedSurcharge(tier, slotSize, feePerByte) {
  const gap = { Active: 0, Warm: 1, Cold: 2, Archived: 3, Expired: 3 }[tier];
  return gap * slotSize * feePerByte;
}

function computeWarmingFeeJS(tier, sizeBytes, feePerByte) {
  const gap = { Active: 0, Warm: 1, Cold: 2, Archived: 3, Expired: 3 }[tier];
  if (gap === 0 || sizeBytes === 0 || feePerByte === 0) return 0;
  return BigInt(gap) * BigInt(sizeBytes) * BigInt(feePerByte);
}

function validateLifecycleThresholds() {
  const { ACTIVE_TIER_BLOCKS: a, WARM_TIER_BLOCKS: w, COLD_TIER_BLOCKS: c } = CONFIG;
  if (!(a > 0 && w > a && c > w)) {
    throw new Error(`Invalid lifecycle thresholds: active=${a}, warm=${w}, cold=${c}. Expected 0 < active < warm < cold.`);
  }
}

function tierBufferForSpan(spanBlocks) {
  const span = Math.max(1, Number(spanBlocks) || 1);
  const safeCap = Math.max(1, Math.floor(span / 3));
  return Math.max(1, Math.min(CONFIG.WAIT_BUFFER_BLOCKS, safeCap));
}

function warmTargetBlock(lastTouchedBlock) {
  validateLifecycleThresholds();
  return Number(lastTouchedBlock) + CONFIG.ACTIVE_TIER_BLOCKS + tierBufferForSpan(CONFIG.WARM_TIER_BLOCKS - CONFIG.ACTIVE_TIER_BLOCKS);
}

function coldTargetBlock(lastTouchedBlock) {
  validateLifecycleThresholds();
  return Number(lastTouchedBlock) + CONFIG.WARM_TIER_BLOCKS + tierBufferForSpan(CONFIG.COLD_TIER_BLOCKS - CONFIG.WARM_TIER_BLOCKS);
}

function archiveTargetBlock(lastTouchedBlock) {
  validateLifecycleThresholds();
  return Number(lastTouchedBlock) + CONFIG.COLD_TIER_BLOCKS + tierBufferForSpan(CONFIG.ACTIVE_TIER_BLOCKS);
}

function lifecyclePlanSummary() {
  validateLifecycleThresholds();
  return `active=${CONFIG.ACTIVE_TIER_BLOCKS}, warm=${CONFIG.WARM_TIER_BLOCKS}, cold=${CONFIG.COLD_TIER_BLOCKS}, effectiveBuffers={warm:+${tierBufferForSpan(CONFIG.WARM_TIER_BLOCKS - CONFIG.ACTIVE_TIER_BLOCKS)}, cold:+${tierBufferForSpan(CONFIG.COLD_TIER_BLOCKS - CONFIG.WARM_TIER_BLOCKS)}, archive:+${tierBufferForSpan(CONFIG.ACTIVE_TIER_BLOCKS)}}`;
}

// ---------- consensus check ----------

function deepEqual(a, b) {
  if (a === b) return true;
  if (typeof a !== typeof b) return false;
  if (a === null || b === null) return a === b;
  if (typeof a === "object") {
    const ka = Object.keys(a), kb = Object.keys(b);
    if (ka.length !== kb.length) return false;
    for (const k of ka) if (!deepEqual(a[k], b[k])) return false;
    return true;
  }
  return false;
}

function allAgree(results, ignoreKeys = []) {
  const labels = Object.keys(results);
  if (labels.length < 2) return { ok: true, reason: "single-node (no comparison possible)" };

  const stripped = {};
  for (const l of labels) {
    let r = results[l];
    if (r && typeof r === "object" && !r.__error && !r.__skip && ignoreKeys.length > 0) {
      r = { ...r };
      for (const k of ignoreKeys) delete r[k];
    }
    stripped[l] = r;
  }

  const skipped = labels.filter((l) => stripped[l] && stripped[l].__skip);
  const comparable = labels.filter((l) => !(stripped[l] && stripped[l].__skip));
  if (comparable.length === 0) {
    return { ok: true, reason: `all nodes skipped (${skipped.length})` };
  }

  const errors = comparable.filter((l) => stripped[l] && stripped[l].__error);
  if (errors.length > 0) {
    return { ok: false, reason: `errors on: ${errors.map((l) => `${l}=${stripped[l].__error}`).join(", ")}` };
  }

  if (comparable.length < 2) {
    const suffix = skipped.length ? `; skipped ${skipped.join(",")}` : "";
    return { ok: true, reason: `single comparable node (${comparable[0]})${suffix}` };
  }

  const refLabel = comparable[0];
  const ref = stripped[refLabel];
  for (let i = 1; i < comparable.length; i++) {
    const label = comparable[i];
    if (!deepEqual(stripped[label], ref)) {
      return {
        ok: false,
        reason: `${refLabel} != ${label}: ${JSON.stringify(stripped[refLabel])} vs ${JSON.stringify(stripped[label])}`,
      };
    }
  }

  const suffix = skipped.length ? `; skipped ${skipped.join(",")}` : "";
  return { ok: true, reason: `all comparable nodes match${suffix}` };
}

// ---------- result tracking ----------

class ResultTracker {
  constructor(suiteName) {
    this.suite = suiteName;
    this.checks = [];
    this.startedAt = Date.now();
  }
  pass(name, detail) {
    this.checks.push({ name, status: "PASS", detail: detail || "" });
    logCheck(name, "PASS", detail);
  }
  fail(name, detail) {
    this.checks.push({ name, status: "FAIL", detail: detail || "" });
    logCheck(name, "FAIL", detail);
  }
  skip(name, detail) {
    this.checks.push({ name, status: "SKIP", detail: detail || "" });
    logCheck(name, "SKIP", detail);
  }
  finalize() {
    const pass = this.checks.filter((c) => c.status === "PASS").length;
    const fail = this.checks.filter((c) => c.status === "FAIL").length;
    const skip = this.checks.filter((c) => c.status === "SKIP").length;
    const elapsed = ((Date.now() - this.startedAt) / 1000).toFixed(1);
    return { suite: this.suite, pass, fail, skip, total: this.checks.length, elapsed_seconds: parseFloat(elapsed), checks: this.checks };
  }
}

function appendReport(suiteResult) {
  const reportPath = path.resolve(__dirname, "..", CONFIG.REPORT_PATH);
  let existing = { suites: [], started_at: new Date().toISOString() };
  if (fs.existsSync(reportPath)) {
    try { existing = JSON.parse(fs.readFileSync(reportPath, "utf8")); }
    catch (e) { existing = { suites: [], started_at: new Date().toISOString() }; }
  }
  existing.suites = existing.suites.filter((s) => s.suite !== suiteResult.suite);
  existing.suites.push(suiteResult);
  existing.last_updated_at = new Date().toISOString();
  fs.writeFileSync(reportPath, JSON.stringify(existing, null, 2));
}

function clearReport() {
  const reportPath = path.resolve(__dirname, "..", CONFIG.REPORT_PATH);
  fs.writeFileSync(reportPath, JSON.stringify({ suites: [], started_at: new Date().toISOString() }, null, 2));
}

// ---------- ethers helpers ----------

function makeProvider(url) {
  return new ethers.JsonRpcProvider(url, { chainId: CONFIG.PRIMARY_CHAIN_ID, name: "ethernova" });
}

function makeSigner(provider) {
  if (!CONFIG.PRIVATE_KEY || !/^0x[0-9a-fA-F]{64}$/.test(CONFIG.PRIVATE_KEY)) {
    throw new Error("PRIVATE_KEY missing/invalid in .env — set a 0x... hex key with funds.");
  }
  return new ethers.Wallet(CONFIG.PRIVATE_KEY, provider);
}

function loadDeployment(name) {
  const p = path.resolve(__dirname, "..", `.${name}.json`);
  if (!fs.existsSync(p)) throw new Error(`Deployment file not found: ${p} — run earlier suite first`);
  return JSON.parse(fs.readFileSync(p, "utf8"));
}

function saveDeployment(name, data) {
  const p = path.resolve(__dirname, "..", `.${name}.json`);
  fs.writeFileSync(p, JSON.stringify(data, null, 2));
}

function loadArtifact(name) {
  const p = path.resolve(__dirname, "..", `artifacts/contracts/${name}.sol/${name}.json`);
  if (!fs.existsSync(p)) throw new Error(`Artifact not found: ${p} — run "npx hardhat compile" first`);
  return JSON.parse(fs.readFileSync(p, "utf8"));
}

module.exports = {
  CONFIG, C,
  logHeader, logCheck, logInfo, logDebug,
  rpcCall, rpcAll,
  getBlockNumber, waitForBlock,
  expectedTier, expectedSurcharge, computeWarmingFeeJS,
  validateLifecycleThresholds, tierBufferForSpan, warmTargetBlock, coldTargetBlock, archiveTargetBlock, lifecyclePlanSummary,
  TIER_NAMES, deepEqual, allAgree,
  ResultTracker, appendReport, clearReport,
  makeProvider, makeSigner,
  loadDeployment, saveDeployment, loadArtifact,
};
