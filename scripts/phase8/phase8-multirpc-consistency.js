"use strict";

// Phase 8  -  Scenario S: Multi-RPC Consistency
//
// If PHASE8_NODE{1,2,3}_RPC are set, compare nova_* responses on the same
// inputs across nodes and verify deterministic behaviour.
//
// Block-height drift is tolerated, but block hashes at a common height MUST
// match, and read-only Phase 8 responses for identity inputs MUST match.

const path = require("path");
const H = require("./phase8-helpers");

const REPORT_DIR = H.envString("PHASE8_REPORT_DIR_RUN", path.resolve(__dirname, "..", "..", "reports", "phase8", "real-usage", "manual"));

const NODES = [
  process.env.PHASE8_NODE1_RPC,
  process.env.PHASE8_NODE2_RPC,
  process.env.PHASE8_NODE3_RPC,
].filter(Boolean);

const suite = new H.Suite("phase8-multirpc-consistency");

async function getBlockNumber(rpc) {
  const r = await H.rpcResult(rpc, "eth_blockNumber", []);
  return parseInt(r, 16);
}
async function getChainId(rpc) {
  const r = await H.rpcResult(rpc, "eth_chainId", []);
  return parseInt(r, 16);
}
async function getBlockByNumber(rpc, n) {
  const hex = "0x" + n.toString(16);
  return await H.rpcResult(rpc, "eth_getBlockByNumber", [hex, false]);
}

function normalize(v) {
  if (v === null || v === undefined) return v;
  if (typeof v === "string") return v.toLowerCase();
  if (Array.isArray(v)) return v.map(normalize);
  if (typeof v === "object") {
    const out = {};
    for (const k of Object.keys(v).sort()) out[k] = normalize(v[k]);
    return out;
  }
  return v;
}

(async () => {
  console.log("========================================================================");
  console.log(" Phase 8 - Scenario S: Multi-RPC Consistency");
  console.log("========================================================================");
  console.log(`Nodes (${NODES.length}): ${NODES.join(", ")}`);

  if (NODES.length < 2) {
    suite.skip("S.multi-rpc consistency", `only ${NODES.length} node(s) configured; need >=2`);
    suite.printFooter();
    H.writeJson(path.join(REPORT_DIR, "multirpc-consistency.json"), suite.summarize());
    process.exit(0);
  }

  // S1: chainId same on all nodes.
  await suite.step("S.all nodes share chainId", async () => {
    const ids = await Promise.all(NODES.map(getChainId));
    const uniq = new Set(ids);
    if (uniq.size !== 1) {
      throw new Error(`chainId divergence: ${JSON.stringify(ids)}`);
    }
    return `chainId=${ids[0]}`;
  });

  // S2: block hash on a common (older) height matches.
  await suite.step("S.block hash at common height matches", async () => {
    const heights = await Promise.all(NODES.map(getBlockNumber));
    const minHeight = Math.min(...heights);
    // Pick 5 blocks back to avoid flicker on the freshest tip.
    const target = Math.max(0, minHeight - 5);
    const blocks = await Promise.all(NODES.map((n) => getBlockByNumber(n, target)));
    const hashes = blocks.map((b) => (b ? b.hash : null));
    const uniq = new Set(hashes);
    if (uniq.size !== 1) {
      throw new Error(`hash divergence at block ${target}: ${JSON.stringify(hashes)}`);
    }
    return `hash=${hashes[0]} at block ${target}`;
  });

  // S3: nova_getDomain(ZERO) identical.
  await suite.step("S.nova_getDomain(zero) identical across nodes", async () => {
    const results = await Promise.all(NODES.map((n) => H.rpcResult(n, "nova_getDomain", [H.ZERO_ADDR])));
    const norm = results.map(normalize);
    const ref = JSON.stringify(norm[0]);
    for (let i = 1; i < norm.length; i++) {
      if (JSON.stringify(norm[i]) !== ref) {
        throw new Error(`node ${i} diverges:\nref=${ref.slice(0, 200)}\ncur=${JSON.stringify(norm[i]).slice(0, 200)}`);
      }
    }
    return `${results.length} nodes agreed`;
  });

  // S4: nova_getCapabilities(ZERO) identical.
  await suite.step("S.nova_getCapabilities(zero) identical across nodes", async () => {
    const results = await Promise.all(NODES.map((n) => H.rpcResult(n, "nova_getCapabilities", [H.ZERO_ADDR])));
    const norm = results.map(normalize);
    const ref = JSON.stringify(norm[0]);
    for (let i = 1; i < norm.length; i++) {
      if (JSON.stringify(norm[i]) !== ref) {
        throw new Error(`node ${i} diverges`);
      }
    }
    return `${results.length} nodes agreed`;
  });

  // S5: nova_getSession(zero) identical (exists=false).
  await suite.step("S.nova_getSession(zero) identical (exists=false)", async () => {
    const results = await Promise.all(NODES.map((n) => H.rpcResult(n, "nova_getSession", [H.ZERO_HASH])));
    for (const r of results) {
      if (!r || r.exists !== false) throw new Error(`unexpected: ${JSON.stringify(r)}`);
    }
    return "all exists=false";
  });

  // S6: nova_deferredProcessingStats (since spec method missing)  -  only the
  // stable fields (forkBlock, queueAddress) must match. pendingCount may
  // drift slightly between near-simultaneous calls if a block is in flight.
  await suite.step("S.deferredProcessingStats stable fields agree", async () => {
    const results = await Promise.all(NODES.map((n) => H.rpcResult(n, "nova_deferredProcessingStats", [])));
    const refQA = results[0].queueAddress;
    const refFB = results[0].forkBlock;
    for (let i = 1; i < results.length; i++) {
      if (results[i].queueAddress !== refQA) throw new Error(`queueAddress drift node[${i}]`);
      if (results[i].forkBlock !== refFB) throw new Error(`forkBlock drift node[${i}]`);
    }
    return `queueAddress=${refQA} forkBlock=${refFB}`;
  });

  // S7: a known session, if provided.
  if (process.env.PHASE8_EXISTING_SESSION_ID) {
    const sid = process.env.PHASE8_EXISTING_SESSION_ID;
    await suite.step("S.nova_getSession(existing) identical across nodes", async () => {
      const results = await Promise.all(NODES.map((n) => H.rpcResult(n, "nova_getSession", [sid])));
      const norm = results.map(normalize);
      const ref = JSON.stringify(norm[0]);
      for (let i = 1; i < norm.length; i++) {
        if (JSON.stringify(norm[i]) !== ref) throw new Error(`divergence at node ${i}`);
      }
      return `${results.length} nodes agreed (status=${results[0].statusName})`;
    });
  } else {
    suite.skip("S.nova_getSession(existing)", "PHASE8_EXISTING_SESSION_ID not set");
  }

  suite.printFooter();
  const summary = suite.summarize();
  H.writeJson(path.join(REPORT_DIR, "multirpc-consistency.json"), summary);
  console.log("Wrote: " + path.join(REPORT_DIR, "multirpc-consistency.json"));

  if (summary.counts.fail > 0) process.exit(1);
  process.exit(0);
})().catch((err) => {
  console.error("Fatal:", err && err.stack ? err.stack : err);
  process.exit(1);
});
