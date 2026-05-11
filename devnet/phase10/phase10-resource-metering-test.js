#!/usr/bin/env node
"use strict";

const { NovaProvider } = require("../nova-sdk");

const RPC = process.argv[2] || "https://devrpc.ethnova.net";
let pass = 0;
let fail = 0;

function must(cond, message) {
  if (!cond) throw new Error(message);
}

async function check(name, fn) {
  try {
    const detail = await fn();
    pass++;
    console.log(`[PASS] ${name}${detail ? ` - ${detail}` : ""}`);
  } catch (err) {
    fail++;
    console.log(`[FAIL] ${name} - ${err.message}`);
  }
}

(async () => {
  console.log("========================================================================");
  console.log(" Phase 10A - Multi-Dimensional Resource Metering (Monitoring Only)");
  console.log("========================================================================");
  console.log(`RPC: ${RPC}`);

  const nova = new NovaProvider(RPC, { fallbackNamespace: false });

  await check("eth_chainId is Ethernova devnet", async () => {
    const chainId = await nova.chainId();
    must(chainId === "0x1dab6", `expected 0x1dab6, got ${chainId}`);
    return chainId;
  });

  await check("nova_resourceConfig monitoring-only", async () => {
    const cfg = await nova.resourceConfig();
    must(cfg.phase === 10, `expected phase 10, got ${cfg.phase}`);
    must(cfg.substage === "10A", `expected 10A, got ${cfg.substage}`);
    must(cfg.mode === "monitoring_only", `unexpected mode ${cfg.mode}`);
    must(cfg.pricingActive === false, "pricing must remain inactive in 10A");
    must(Array.isArray(cfg.dimensions) && cfg.dimensions.length === 5, "missing five dimensions");
    return `${cfg.substage} ${cfg.dimensions.join(",")}`;
  });

  await check("nova_resourcePrices fixed placeholders", async () => {
    const prices = await nova.resourcePrices();
    must(prices.pricingActive === false, "pricing must be inactive");
    for (const key of ["compute", "state_read", "state_write", "protocol_ops", "proof_verify"]) {
      must(prices.prices[key] === 1, `${key} price expected 1`);
    }
    return "all dimensions price=1";
  });

  await check("legacy gasLimit maps to resource vector", async () => {
    const limits = await nova.estimateResourceLimits(3000000);
    must(limits.compute === 3000000, `compute ${limits.compute}`);
    must(limits.stateRead === 1000000, `stateRead ${limits.stateRead}`);
    must(limits.stateWrite === 500000, `stateWrite ${limits.stateWrite}`);
    must(limits.protocolOps === 200000, `protocolOps ${limits.protocolOps}`);
    must(limits.proofVerify === 100000, `proofVerify ${limits.proofVerify}`);
    return JSON.stringify(limits);
  });

  await check("nova_developerTooling advertises Phase 10 methods", async () => {
    const tooling = await nova.developerTooling();
    for (const method of ["nova_resourceConfig", "nova_resourcePrices", "nova_estimateResourceLimits", "nova_getResourceVector"]) {
      must(tooling.rpcMethods.includes(method), `${method} missing`);
    }
    return `${tooling.rpcMethods.length} methods`;
  });

  console.log("------------------------------------------------------------------------");
  console.log(`RESULT: ${pass} pass, ${fail} fail`);
  if (fail) process.exit(1);
})().catch((err) => {
  console.error(err);
  process.exit(1);
});
