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
  console.log(" Phase 10B - Multi-Dimensional Resource Pricing (Static Quote)");
  console.log("========================================================================");
  console.log(`RPC: ${RPC}`);

  const nova = new NovaProvider(RPC, { fallbackNamespace: false });

  await check("eth_chainId is Ethernova devnet", async () => {
    const chainId = await nova.chainId();
    must(chainId === "0x1dab6", `expected 0x1dab6, got ${chainId}`);
    return chainId;
  });

  await check("nova_resourceConfig static pricing", async () => {
    const cfg = await nova.resourceConfig();
    must(cfg.phase === 10, `expected phase 10, got ${cfg.phase}`);
    must(cfg.substage === "10B", `expected 10B, got ${cfg.substage}`);
    must(cfg.mode === "static_per_dimension_pricing", `unexpected mode ${cfg.mode}`);
    must(cfg.pricingActive === true, "pricing must be active in 10B");
    must(cfg.consensusGasChanged === false, "10B must not change consensus gas");
    must(Array.isArray(cfg.dimensions) && cfg.dimensions.length === 5, "missing five dimensions");
    return `${cfg.substage} ${cfg.dimensions.join(",")}`;
  });

  await check("nova_resourcePrices static multipliers", async () => {
    const prices = await nova.resourcePrices();
    must(prices.pricingActive === true, "pricing must be active");
    must(prices.adaptive === false, "adaptive pricing remains 10C scope");
    must(prices.prices.compute === 1, `compute ${prices.prices.compute}`);
    must(prices.prices.state_read === 2, `state_read ${prices.prices.state_read}`);
    must(prices.prices.state_write === 4, `state_write ${prices.prices.state_write}`);
    must(prices.prices.protocol_ops === 1, `protocol_ops ${prices.prices.protocol_ops}`);
    must(prices.prices.proof_verify === 3, `proof_verify ${prices.prices.proof_verify}`);
    return "compute=1 state_read=2 state_write=4 protocol_ops=1 proof_verify=3";
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

  await check("nova_quoteResourceFee applies per-dimension prices", async () => {
    const quote = await nova.quoteResourceFee({
      compute: 1000,
      stateRead: 100,
      stateWrite: 50,
      protocolOps: 20,
      proofVerify: 10,
    });
    must(quote.substage === "10B", `substage ${quote.substage}`);
    must(quote.pricingActive === true, "pricingActive expected true");
    must(quote.consensusGasChanged === false, "consensus gas must remain unchanged");
    must(quote.pricedUnits.compute === 1000, `compute ${quote.pricedUnits.compute}`);
    must(quote.pricedUnits.stateRead === 200, `stateRead ${quote.pricedUnits.stateRead}`);
    must(quote.pricedUnits.stateWrite === 200, `stateWrite ${quote.pricedUnits.stateWrite}`);
    must(quote.pricedUnits.protocolOps === 20, `protocolOps ${quote.pricedUnits.protocolOps}`);
    must(quote.pricedUnits.proofVerify === 30, `proofVerify ${quote.pricedUnits.proofVerify}`);
    must(quote.pricedUnits.total === 1450, `total ${quote.pricedUnits.total}`);
    return `total=${quote.pricedUnits.total}`;
  });

  await check("nova_developerTooling advertises Phase 10 methods", async () => {
    const tooling = await nova.developerTooling();
    for (const method of ["nova_resourceConfig", "nova_resourcePrices", "nova_estimateResourceLimits", "nova_quoteResourceFee", "nova_getResourceVector"]) {
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
