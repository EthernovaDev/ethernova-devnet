"use strict";

// Phase 8  -  Scenario P: SDK ethers.js / Nova SDK Real Usage
//
// We test the existing CommonJS Nova SDK at devnet/nova-sdk/index.js. The
// SDK is dependency-free, so we can require it directly without a build
// step.
//
// For each spec method that the SDK exposes:
//   1. Call via SDK.
//   2. Call same method via raw JSON-RPC.
//   3. Assert results are deeply equal (modulo numeric / string canonicalisation).
//
// We also assert that SDK throws cleanly for invalid params (instead of
// returning garbage).

const path = require("path");
const fs = require("fs");
const H = require("./phase8-helpers");

const RPC = H.envString("PHASE8_RPC_URL", "http://127.0.0.1:8545");
const REPORT_DIR = H.envString("PHASE8_REPORT_DIR_RUN", path.resolve(__dirname, "..", "..", "reports", "phase8", "real-usage", "manual"));

// Resolve the SDK path: prefer the in-repo location.
const SDK_PATH = process.env.PHASE8_SDK_PATH || path.resolve(__dirname, "..", "..", "devnet", "nova-sdk");

const suite = new H.Suite("phase8-sdk-real-usage");

let sdk;
try {
  sdk = require(SDK_PATH);
} catch (err) {
  console.error(`FATAL: cannot load Nova SDK from ${SDK_PATH}: ${err.message}`);
  console.error("Set PHASE8_SDK_PATH to the directory containing index.js.");
  suite.fail("P.sdk loadable", err.message, H.SEVERITY.CRITICAL);
  H.writeJson(path.join(REPORT_DIR, "sdk-test.json"), suite.summarize());
  process.exit(1);
}

if (!sdk.NovaProvider) {
  console.error("FATAL: SDK does not export NovaProvider");
  suite.fail("P.sdk exports NovaProvider", "missing export", H.SEVERITY.CRITICAL);
  H.writeJson(path.join(REPORT_DIR, "sdk-test.json"), suite.summarize());
  process.exit(1);
}

const nova = new sdk.NovaProvider(RPC, { fallbackNamespace: false });

function deepEqualNormalized(a, b) {
  // Some go-ethereum responses normalise hex differently between two calls
  // (e.g. address case). Normalise strings to lowercase before comparing.
  if (a === null || b === null) return a === b;
  if (typeof a !== typeof b) return false;
  if (typeof a !== "object") {
    if (typeof a === "string" && typeof b === "string") {
      return a.toLowerCase() === b.toLowerCase();
    }
    return a === b;
  }
  if (Array.isArray(a) !== Array.isArray(b)) return false;
  if (Array.isArray(a)) {
    if (a.length !== b.length) return false;
    for (let i = 0; i < a.length; i++) if (!deepEqualNormalized(a[i], b[i])) return false;
    return true;
  }
  const ak = Object.keys(a).sort();
  const bk = Object.keys(b).sort();
  if (ak.length !== bk.length) return false;
  for (let i = 0; i < ak.length; i++) {
    if (ak[i] !== bk[i]) return false;
    if (!deepEqualNormalized(a[ak[i]], b[bk[i]])) return false;
  }
  return true;
}

async function sameAsRaw(name, sdkResult, method, params) {
  const raw = await H.rpcResult(RPC, method, params);
  if (!deepEqualNormalized(sdkResult, raw)) {
    throw new Error(`SDK result diverges from raw RPC for ${method}.\nSDK: ${JSON.stringify(sdkResult).slice(0, 300)}\nRAW: ${JSON.stringify(raw).slice(0, 300)}`);
  }
  return "match";
}

(async () => {
  console.log("========================================================================");
  console.log(" Phase 8 - Scenario P: Nova SDK Real Usage");
  console.log("========================================================================");
  console.log(`RPC: ${RPC}`);
  console.log(`SDK: ${SDK_PATH}`);

  await suite.step("P.sdk chainId() matches eth_chainId raw", async () => {
    const a = await nova.chainId();
    const b = await H.rpcResult(RPC, "eth_chainId", []);
    if (String(a).toLowerCase() !== String(b).toLowerCase()) {
      throw new Error(`SDK ${a} != raw ${b}`);
    }
    return a;
  });

  await suite.step("P.sdk getDomain matches raw", async () => {
    const a = await nova.getDomain(H.ZERO_ADDR);
    return await sameAsRaw("getDomain", a, "nova_getDomain", [H.ZERO_ADDR]);
  });

  await suite.step("P.sdk getCapabilities matches raw", async () => {
    const a = await nova.getCapabilities(H.ZERO_ADDR);
    return await sameAsRaw("getCapabilities", a, "nova_getCapabilities", [H.ZERO_ADDR]);
  });

  await suite.step("P.sdk getSession (zero id) matches raw", async () => {
    const a = await nova.getSession(H.ZERO_HASH);
    return await sameAsRaw("getSession", a, "nova_getSession", [H.ZERO_HASH]);
  });

  await suite.step("P.sdk getStateTier matches raw", async () => {
    const a = await nova.getStateTier(H.ZERO_ADDR, H.ZERO_HASH);
    return await sameAsRaw("getStateTier", a, "nova_getStateTier", [H.ZERO_ADDR, H.ZERO_HASH]);
  });

  await suite.step("P.sdk getStateWitness matches raw", async () => {
    const a = await nova.getStateWitness(H.ZERO_ADDR, H.ZERO_HASH);
    return await sameAsRaw("getStateWitness", a, "nova_getStateWitness", [H.ZERO_ADDR, H.ZERO_HASH]);
  });

  await suite.step("P.sdk getPendingEffects matches raw", async () => {
    const a = await nova.getPendingEffects(0, 3);
    return await sameAsRaw("getPendingEffects", a, "nova_getPendingEffects", [0, 3]);
  });

  await suite.step("P.sdk getProtocolObject (zero id) matches raw", async () => {
    const a = await nova.getProtocolObject(H.ZERO_HASH);
    const b = await H.rpcResult(RPC, "nova_getProtocolObject", [H.ZERO_HASH]);
    // Both should be null.
    if (a !== b) throw new Error(`SDK ${a} != raw ${b}`);
    return "null/null";
  });

  await suite.step("P.sdk getMailbox (zero id) matches raw", async () => {
    const a = await nova.getMailbox(H.ZERO_HASH);
    const b = await H.rpcResult(RPC, "nova_getMailbox", [H.ZERO_HASH]);
    if (a !== b) throw new Error(`SDK ${a} != raw ${b}`);
    return "null/null";
  });

  await suite.step("P.sdk getMessages (unknown mailbox) matches raw", async () => {
    const a = await nova.getMessages(H.ZERO_HASH, 0, 5);
    const b = await H.rpcResult(RPC, "nova_getMessages", [H.ZERO_HASH, 0, 5]);
    if (a !== b) throw new Error(`SDK ${a} != raw ${b}`);
    return "null/null";
  });

  await suite.step("P.sdk getContentRef (zero id) matches raw", async () => {
    const a = await nova.getContentRef(H.ZERO_HASH);
    const b = await H.rpcResult(RPC, "nova_getContentRef", [H.ZERO_HASH]);
    if (a !== b) throw new Error(`SDK ${a} != raw ${b}`);
    return "null/null";
  });

  // SDK lacks listProtocolObjects and getDeferredStats  -  confirm absence
  // (these match the audit findings, so we mark them WARN, not FAIL).
  await suite.stepWarn("P.sdk listProtocolObjects ABSENT (matches missing endpoint BUG-1)", async () => {
    if (typeof nova.listProtocolObjects === "function") {
      throw new Error("SDK now has listProtocolObjects but endpoint is missing - inconsistent");
    }
    return "SDK correctly omits missing endpoint";
  });
  await suite.stepWarn("P.sdk getDeferredStats ABSENT (matches missing endpoint BUG-2)", async () => {
    if (typeof nova.getDeferredStats === "function") {
      throw new Error("SDK now has getDeferredStats but endpoint is missing - inconsistent");
    }
    return "SDK correctly omits missing endpoint";
  });

  // Negative-path: SDK must throw clean error on JSON-RPC error.
  await suite.step("P.sdk throws on bad RPC URL", async () => {
    const bogus = new sdk.NovaProvider("http://127.0.0.1:1", { fallbackNamespace: false });
    try {
      await bogus.getDomain(H.ZERO_ADDR);
    } catch (err) {
      return `threw cleanly: ${String(err.message || err).slice(0, 80)}`;
    }
    throw new Error("expected throw on unreachable RPC, got success");
  });

  await suite.step("P.sdk throws when method-not-found and fallback disabled", async () => {
    try {
      await nova.nova("definitelyNotARealMethodXYZ", []);
    } catch (err) {
      if (err.code !== H.ERR_METHOD_NOT_FOUND) {
        throw new Error(`expected -32601, got code=${err.code}`);
      }
      return `code=${err.code}`;
    }
    throw new Error("expected throw on unknown method");
  });

  // Constructor validation: SDK must reject empty URL.
  await suite.step("P.sdk rejects empty URL", async () => {
    try {
      new sdk.NovaProvider();
    } catch (e) {
      return `threw: ${e.message}`;
    }
    throw new Error("SDK did not throw on missing URL");
  });

  suite.printFooter();
  const summary = suite.summarize();
  const out = path.join(REPORT_DIR, "sdk-test.json");
  H.writeJson(out, summary);
  console.log(`Wrote: ${out}`);

  if (summary.counts.fail > 0) process.exit(1);
  process.exit(0);
})().catch((err) => {
  console.error("Fatal:", err && err.stack ? err.stack : err);
  process.exit(1);
});
