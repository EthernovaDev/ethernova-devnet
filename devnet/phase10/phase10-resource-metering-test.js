#!/usr/bin/env node
"use strict";

const { NovaProvider } = require("../nova-sdk");

const RPC = process.argv[2] || "https://devrpc.ethnova.net";
let pass = 0;
let fail = 0;

function must(cond, message) {
  if (!cond) throw new Error(message);
}

function priceWithBips(amount, bips) {
  return Number((BigInt(amount) * BigInt(bips) + 9999n) / 10000n);
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
  console.log(" Phase 10C - Adaptive Multi-Dimensional Resource Pricing");
  console.log("========================================================================");
  console.log(`RPC: ${RPC}`);

  const nova = new NovaProvider(RPC, { fallbackNamespace: false });

  await check("eth_chainId is Ethernova devnet", async () => {
    const chainId = await nova.chainId();
    must(chainId === "0x1dab6", `expected 0x1dab6, got ${chainId}`);
    return chainId;
  });

  await check("nova_resourceConfig adaptive pricing", async () => {
    const cfg = await nova.resourceConfig();
    must(cfg.phase === 10, `expected phase 10, got ${cfg.phase}`);
    must(cfg.substage === "10C", `expected 10C, got ${cfg.substage}`);
    must(cfg.mode === "adaptive_per_dimension_pricing", `unexpected mode ${cfg.mode}`);
    must(cfg.pricingActive === true, "pricing must be active in 10C");
    must(cfg.adaptivePricing === true, "adaptivePricing must be true in 10C");
    must(cfg.consensusGasChanged === false, "10C quote layer must not change consensus gas");
    must(Array.isArray(cfg.dimensions) && cfg.dimensions.length === 5, "missing five dimensions");
    return `${cfg.substage} ${cfg.dimensions.join(",")}`;
  });

  await check("nova_resourcePrices adaptive bips", async () => {
    const prices = await nova.resourcePrices();
    must(prices.pricingActive === true, "pricing must be active");
    must(prices.adaptive === true, "adaptive pricing expected");
    must(prices.basePriceBips.compute === 10000, `base compute ${prices.basePriceBips.compute}`);
    must(prices.basePriceBips.stateRead === 20000, `base stateRead ${prices.basePriceBips.stateRead}`);
    must(prices.basePriceBips.stateWrite === 40000, `base stateWrite ${prices.basePriceBips.stateWrite}`);
    must(prices.basePriceBips.protocolOps === 10000, `base protocolOps ${prices.basePriceBips.protocolOps}`);
    must(prices.basePriceBips.proofVerify === 30000, `base proofVerify ${prices.basePriceBips.proofVerify}`);
    for (const key of ["compute", "stateRead", "stateWrite", "protocolOps", "proofVerify"]) {
      must(prices.currentPriceBips[key] >= prices.basePriceBips[key], `${key} below base`);
    }
    return `block=${prices.lastBlock} computeBips=${prices.currentPriceBips.compute} protocolOpsBips=${prices.currentPriceBips.protocolOps}`;
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

  await check("nova_quoteResourceFee applies adaptive prices", async () => {
    const quote = await nova.quoteResourceFee({
      compute: 1000,
      stateRead: 100,
      stateWrite: 50,
      protocolOps: 20,
      proofVerify: 10,
    });
    must(quote.substage === "10C", `substage ${quote.substage}`);
    must(quote.pricingActive === true, "pricingActive expected true");
    must(quote.adaptive === true, "adaptive expected true");
    must(quote.consensusGasChanged === false, "consensus gas must remain unchanged");
    must(quote.pricedUnits.compute === priceWithBips(1000, quote.priceBips.compute), `compute ${quote.pricedUnits.compute}`);
    must(quote.pricedUnits.stateRead === priceWithBips(100, quote.priceBips.stateRead), `stateRead ${quote.pricedUnits.stateRead}`);
    must(quote.pricedUnits.stateWrite === priceWithBips(50, quote.priceBips.stateWrite), `stateWrite ${quote.pricedUnits.stateWrite}`);
    must(quote.pricedUnits.protocolOps === priceWithBips(20, quote.priceBips.protocolOps), `protocolOps ${quote.pricedUnits.protocolOps}`);
    must(quote.pricedUnits.proofVerify === priceWithBips(10, quote.priceBips.proofVerify), `proofVerify ${quote.pricedUnits.proofVerify}`);
    const expectedTotal = quote.pricedUnits.compute + quote.pricedUnits.stateRead + quote.pricedUnits.stateWrite + quote.pricedUnits.protocolOps + quote.pricedUnits.proofVerify;
    must(quote.pricedUnits.total === expectedTotal, `total ${quote.pricedUnits.total}`);
    return `total=${quote.pricedUnits.total}`;
  });

  await check("nova_resourceCongestion exposes isolated controller", async () => {
    const congestion = await nova.resourceCongestion();
    must(congestion.substage === "10C", `substage ${congestion.substage}`);
    must(congestion.adaptive === true, "adaptive expected true");
    must(congestion.consensusGasChanged === false, "consensus gas must remain unchanged");
    must(congestion.snapshot && congestion.snapshot.currentPriceBips, "missing snapshot prices");
    must(String(congestion.isolation || "").includes("each dimension"), "missing isolation note");
    return `block=${congestion.snapshot.blockNumber}`;
  });

  await check("nova_developerTooling advertises Phase 10 methods", async () => {
    const tooling = await nova.developerTooling();
    for (const method of ["nova_resourceConfig", "nova_resourcePrices", "nova_estimateResourceLimits", "nova_quoteResourceFee", "nova_resourceCongestion", "nova_getResourceVector"]) {
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
