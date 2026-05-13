"use strict";

// Phase 8 real-usage shared helpers.
//
// Pure Node.js, no external dependencies. Uses built-in http/https/fetch so
// the same file works on Node 18+ on Windows PowerShell, macOS, and Linux.
//
// Every helper is intentionally small. The test runner relies on stable
// shapes: pass/fail/warn/skip plus per-scenario JSON. Do not change shapes
// without updating phase8-rpc-real-usage.js and the runner.

const http = require("http");
const https = require("https");
const fs = require("fs");
const path = require("path");

const SEVERITY = Object.freeze({
  CRITICAL: "critical",
  HIGH: "high",
  MEDIUM: "medium",
  LOW: "low",
  INFO: "info",
});

const ZERO_HASH = "0x0000000000000000000000000000000000000000000000000000000000000000";
const ZERO_ADDR = "0x0000000000000000000000000000000000000000";

// JSON-RPC method-not-found code per the spec.
const ERR_METHOD_NOT_FOUND = -32601;
const ERR_INVALID_PARAMS = -32602;
const ERR_PARSE_ERROR = -32700;
const ERR_INVALID_REQUEST = -32600;

let _rpcSeq = Math.floor(Math.random() * 1e6);
function nextId() {
  _rpcSeq += 1;
  return _rpcSeq;
}

function makePayload(method, params) {
  return {
    jsonrpc: "2.0",
    id: nextId(),
    method,
    params,
  };
}

// postJson posts a single JSON body and returns the parsed response or throws
// a wrapped network-style error. It NEVER throws for JSON-RPC error objects;
// those come back inside `body.error`.
async function postJson(url, payload, options = {}) {
  const timeoutMs = options.timeoutMs || 15000;
  if (typeof fetch === "function") {
    const ctrl = new AbortController();
    const tid = setTimeout(() => ctrl.abort(), timeoutMs);
    try {
      const res = await fetch(url, {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify(payload),
        signal: ctrl.signal,
      });
      const text = await res.text();
      if (!res.ok && (text === "" || text === null)) {
        throw new Error(`HTTP ${res.status} from ${url}`);
      }
      try {
        return JSON.parse(text);
      } catch (e) {
        const err = new Error(`Non-JSON response from ${url}: ${text.slice(0, 200)}`);
        err.body = text;
        err.status = res.status;
        throw err;
      }
    } finally {
      clearTimeout(tid);
    }
  }
  return new Promise((resolve, reject) => {
    let target;
    try {
      target = new URL(url);
    } catch (e) {
      reject(new Error(`Invalid RPC URL: ${url}`));
      return;
    }
    const lib = target.protocol === "https:" ? https : http;
    const req = lib.request(
      target,
      {
        method: "POST",
        headers: { "content-type": "application/json" },
        timeout: timeoutMs,
      },
      (res) => {
        let data = "";
        res.setEncoding("utf8");
        res.on("data", (chunk) => {
          data += chunk;
        });
        res.on("end", () => {
          try {
            resolve(JSON.parse(data));
          } catch (err) {
            const e = new Error(`Non-JSON response from ${url}: ${data.slice(0, 200)}`);
            e.body = data;
            reject(e);
          }
        });
      }
    );
    req.on("timeout", () => {
      req.destroy(new Error(`Timeout after ${timeoutMs}ms calling ${url}`));
    });
    req.on("error", reject);
    req.write(JSON.stringify(payload));
    req.end();
  });
}

// rpc is the canonical helper. Returns the raw JSON-RPC body so the caller
// can inspect both `result` and `error` cleanly. Never throws for
// JSON-RPC errors; only throws for transport / parsing failures.
async function rpc(rpcUrl, method, params, options = {}) {
  if (!Array.isArray(params)) {
    // JSON-RPC params must be array or object. Most go-ethereum methods
    // expect array. We pass through objects for the malformed test path.
    params = params === undefined ? [] : params;
  }
  const payload = makePayload(method, params);
  const body = await postJson(rpcUrl, payload, options);
  return body;
}

// rpcResult is sugar that throws on JSON-RPC error or returns result.
async function rpcResult(rpcUrl, method, params, options = {}) {
  const body = await rpc(rpcUrl, method, params, options);
  if (body.error) {
    const err = new Error(`${method}: ${body.error.message || "JSON-RPC error"}`);
    err.code = body.error.code;
    err.data = body.error.data;
    err.rpcMethod = method;
    err.rpcParams = params;
    throw err;
  }
  return body.result;
}

// ----------------------------------------------------------------------
// Assertion helpers
// ----------------------------------------------------------------------

function assertRpcSuccess(body, method) {
  if (!body || typeof body !== "object") {
    throw new Error(`${method}: invalid RPC body (not an object)`);
  }
  if (body.jsonrpc !== "2.0") {
    throw new Error(`${method}: missing or wrong jsonrpc version (${body.jsonrpc})`);
  }
  if (body.error) {
    throw new Error(`${method}: unexpected RPC error code=${body.error.code} message=${body.error.message}`);
  }
  if (!("result" in body)) {
    throw new Error(`${method}: response missing 'result' field`);
  }
}

function assertRpcError(body, method) {
  if (!body || typeof body !== "object") {
    throw new Error(`${method}: invalid RPC body (not an object)`);
  }
  if (!body.error) {
    throw new Error(`${method}: expected JSON-RPC error, got result=${JSON.stringify(body.result).slice(0, 200)}`);
  }
  if (typeof body.error.code !== "number") {
    throw new Error(`${method}: error.code is not a number (${body.error.code})`);
  }
  if (typeof body.error.message !== "string") {
    throw new Error(`${method}: error.message is not a string`);
  }
  return body.error;
}

const HEX_RE = /^0x[0-9a-fA-F]*$/;
const ADDR_RE = /^0x[0-9a-fA-F]{40}$/;
const HASH_RE = /^0x[0-9a-fA-F]{64}$/;

function isHex(v) {
  return typeof v === "string" && HEX_RE.test(v);
}
function isAddress(v) {
  return typeof v === "string" && ADDR_RE.test(v);
}
function isHash(v) {
  return typeof v === "string" && HASH_RE.test(v);
}

function assertHex(v, label) {
  if (!isHex(v)) {
    throw new Error(`${label || "value"} is not 0x-prefixed hex: ${JSON.stringify(v)}`);
  }
}
function assertAddress(v, label) {
  if (!isAddress(v)) {
    throw new Error(`${label || "value"} is not a 20-byte address: ${JSON.stringify(v)}`);
  }
}
function assertHash(v, label) {
  if (!isHash(v)) {
    throw new Error(`${label || "value"} is not a 32-byte hash: ${JSON.stringify(v)}`);
  }
}
function assertArray(v, label) {
  if (!Array.isArray(v)) {
    throw new Error(`${label || "value"} is not an array (got ${typeof v})`);
  }
}
function assertObject(v, label) {
  if (v === null || typeof v !== "object" || Array.isArray(v)) {
    throw new Error(`${label || "value"} is not a plain object (got ${Array.isArray(v) ? "array" : typeof v})`);
  }
}
function assertNumber(v, label) {
  if (typeof v !== "number" || !Number.isFinite(v)) {
    throw new Error(`${label || "value"} is not a finite number (got ${typeof v}: ${v})`);
  }
}
function assertString(v, label) {
  if (typeof v !== "string") {
    throw new Error(`${label || "value"} is not a string (got ${typeof v})`);
  }
}
function assertHasKey(obj, key, label) {
  assertObject(obj, label);
  if (!(key in obj)) {
    throw new Error(`${label || "object"} missing required key '${key}'`);
  }
}
function assertPaginationStable(firstPage, secondPage, label) {
  // Pagination is "stable" if successive same-offset same-limit calls return
  // the same content array (modulo head-side mutation). We accept either
  // identical contents OR a strictly longer/equal second page when content
  // has been appended (queue advances).
  if (!Array.isArray(firstPage) || !Array.isArray(secondPage)) {
    throw new Error(`${label || "pagination"}: pages must be arrays`);
  }
  if (firstPage.length === 0 && secondPage.length === 0) return true;
  if (JSON.stringify(firstPage) === JSON.stringify(secondPage)) return true;
  // If second page extends the first (prefix match), accept.
  if (secondPage.length >= firstPage.length) {
    for (let i = 0; i < firstPage.length; i++) {
      if (JSON.stringify(firstPage[i]) !== JSON.stringify(secondPage[i])) {
        throw new Error(`${label || "pagination"}: page mismatch at index ${i}`);
      }
    }
    return true;
  }
  throw new Error(`${label || "pagination"}: page shrank (${firstPage.length} -> ${secondPage.length})`);
}

// ----------------------------------------------------------------------
// Test-runner harness
// ----------------------------------------------------------------------

class Suite {
  constructor(name) {
    this.name = name;
    this.results = [];
    this.startedAt = new Date().toISOString();
  }
  pass(scenario, detail, extra) {
    const r = { scenario, status: "pass", detail: detail || "", extra: extra || null, at: new Date().toISOString() };
    this.results.push(r);
    console.log(`[PASS] ${scenario}${detail ? " - " + detail : ""}`);
    return r;
  }
  fail(scenario, detail, severity, extra) {
    const r = {
      scenario,
      status: "fail",
      detail: detail || "",
      severity: severity || SEVERITY.HIGH,
      extra: extra || null,
      at: new Date().toISOString(),
    };
    this.results.push(r);
    console.log(`[FAIL ${r.severity.toUpperCase()}] ${scenario} - ${detail}`);
    return r;
  }
  warn(scenario, detail, extra) {
    const r = { scenario, status: "warn", detail: detail || "", extra: extra || null, at: new Date().toISOString() };
    this.results.push(r);
    console.log(`[WARN] ${scenario} - ${detail}`);
    return r;
  }
  skip(scenario, detail) {
    const r = { scenario, status: "skip", detail: detail || "", at: new Date().toISOString() };
    this.results.push(r);
    console.log(`[SKIP] ${scenario} - ${detail}`);
    return r;
  }
  async step(scenario, fn, options = {}) {
    try {
      const detail = await fn();
      return this.pass(scenario, detail);
    } catch (err) {
      const sev = options.severity || SEVERITY.HIGH;
      return this.fail(scenario, err && err.message ? err.message : String(err), sev, {
        stack: err && err.stack ? err.stack.split("\n").slice(0, 6).join("\n") : null,
      });
    }
  }
  async stepWarn(scenario, fn) {
    try {
      const detail = await fn();
      return this.pass(scenario, detail);
    } catch (err) {
      return this.warn(scenario, err && err.message ? err.message : String(err));
    }
  }
  counts() {
    const c = { pass: 0, fail: 0, warn: 0, skip: 0 };
    for (const r of this.results) c[r.status]++;
    return c;
  }
  highestSeverity() {
    let sev = null;
    const order = [SEVERITY.LOW, SEVERITY.MEDIUM, SEVERITY.HIGH, SEVERITY.CRITICAL];
    for (const r of this.results) {
      if (r.status !== "fail") continue;
      const idx = order.indexOf(r.severity);
      if (idx > (sev === null ? -1 : order.indexOf(sev))) {
        sev = r.severity;
      }
    }
    return sev;
  }
  summarize() {
    const c = this.counts();
    return {
      suite: this.name,
      startedAt: this.startedAt,
      endedAt: new Date().toISOString(),
      counts: c,
      highestSeverity: this.highestSeverity(),
      results: this.results,
    };
  }
  printFooter() {
    const c = this.counts();
    console.log("------------------------------------------------------------------------");
    console.log(`RESULT: ${c.pass} pass, ${c.fail} fail, ${c.warn} warn, ${c.skip} skip`);
  }
}

// writeJson writes object as pretty JSON to path, creating parent dirs.
function writeJson(filepath, obj) {
  const dir = path.dirname(filepath);
  fs.mkdirSync(dir, { recursive: true });
  fs.writeFileSync(filepath, JSON.stringify(obj, null, 2));
}

// loadEnv reads scripts/phase8/.env if it exists and merges into process.env.
// We do not require dotenv to keep zero deps.
function loadEnv(envPath) {
  if (!envPath) {
    envPath = path.resolve(__dirname, ".env");
  }
  if (!fs.existsSync(envPath)) return false;
  const text = fs.readFileSync(envPath, "utf8");
  for (const rawLine of text.split(/\r?\n/)) {
    const line = rawLine.trim();
    if (!line || line.startsWith("#")) continue;
    const eq = line.indexOf("=");
    if (eq <= 0) continue;
    const key = line.slice(0, eq).trim();
    let val = line.slice(eq + 1).trim();
    if ((val.startsWith('"') && val.endsWith('"')) || (val.startsWith("'") && val.endsWith("'"))) {
      val = val.slice(1, -1);
    }
    if (!(key in process.env)) {
      process.env[key] = val;
    }
  }
  return true;
}

// envBool parses an env string into bool. Empty/undefined -> default.
function envBool(name, defaultValue) {
  const v = process.env[name];
  if (v === undefined || v === "") return defaultValue;
  return /^(1|true|yes|on)$/i.test(v);
}
function envNumber(name, defaultValue) {
  const v = process.env[name];
  if (v === undefined || v === "") return defaultValue;
  const n = Number(v);
  if (!Number.isFinite(n)) return defaultValue;
  return n;
}
function envString(name, defaultValue) {
  const v = process.env[name];
  if (v === undefined || v === "") return defaultValue;
  return v;
}

// requireEnv reads an env var and throws if missing/empty.
function requireEnv(name) {
  const v = process.env[name];
  if (v === undefined || v === "") {
    throw new Error(`Required env var ${name} is not set. See scripts/phase8/.env.example.`);
  }
  return v;
}

// pad32 turns a hex value into a 32-byte hash (left-pad zeros). Useful for
// constructing storage-slot keys for nova_getStateTier/nova_getStateWitness.
function pad32(hexOrNumber) {
  let h;
  if (typeof hexOrNumber === "number" || typeof hexOrNumber === "bigint") {
    h = BigInt(hexOrNumber).toString(16);
  } else {
    h = String(hexOrNumber).replace(/^0x/i, "");
  }
  if (h.length > 64) throw new Error(`pad32: value too large (${h.length} hex chars)`);
  return "0x" + h.padStart(64, "0");
}

module.exports = {
  SEVERITY,
  ZERO_HASH,
  ZERO_ADDR,
  ERR_METHOD_NOT_FOUND,
  ERR_INVALID_PARAMS,
  ERR_PARSE_ERROR,
  ERR_INVALID_REQUEST,

  postJson,
  rpc,
  rpcResult,
  makePayload,

  assertRpcSuccess,
  assertRpcError,
  assertHex,
  assertAddress,
  assertHash,
  assertArray,
  assertObject,
  assertNumber,
  assertString,
  assertHasKey,
  assertPaginationStable,
  isHex,
  isAddress,
  isHash,

  Suite,
  writeJson,
  loadEnv,
  envBool,
  envNumber,
  envString,
  requireEnv,
  pad32,
};
