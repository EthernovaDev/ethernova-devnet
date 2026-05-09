"use strict";

const { NovaProvider, buildDomainInitcode, domainRuntimeBytecode, ZERO_HASH } = require("../nova-sdk");

const RPC = process.argv[2] || process.env.RPC_URL || "https://devrpc.ethnova.net";
const ZERO_ADDR = "0x0000000000000000000000000000000000000000";

let pass = 0;
let fail = 0;
let warn = 0;

function log(level, name, detail) {
  const suffix = detail === undefined ? "" : ` - ${detail}`;
  console.log(`[${level}] ${name}${suffix}`);
}

async function check(name, fn) {
  try {
    const detail = await fn();
    pass += 1;
    log("PASS", name, detail);
  } catch (err) {
    fail += 1;
    log("FAIL", name, err && err.message ? err.message : String(err));
  }
}

async function optional(name, fn) {
  try {
    const detail = await fn();
    pass += 1;
    log("PASS", name, detail);
  } catch (err) {
    warn += 1;
    log("WARN", name, err && err.message ? err.message : String(err));
  }
}

function must(condition, message) {
  if (!condition) {
    throw new Error(message);
  }
}

(async () => {
  console.log("========================================================================");
  console.log(" Phase 8 - Nova RPC Namespace & Developer Tooling");
  console.log("========================================================================");
  console.log(`RPC: ${RPC}`);

  const nova = new NovaProvider(RPC, { fallbackNamespace: false });

  await check("eth_chainId is Ethernova devnet", async () => {
    const chainId = await nova.chainId();
    must(chainId === "0x1dab6", `expected 0x1dab6, got ${chainId}`);
    return chainId;
  });

  await check("nova_developerTooling", async () => {
    const tooling = await nova.developerTooling();
    must(tooling.canonicalNamespace === "nova", "canonical namespace is not nova");
    must(tooling.rpcMethods.includes("nova_getDomain"), "nova_getDomain missing");
    return `${tooling.rpcMethods.length} methods`;
  });

  await check("nova_getDomain for EOA", async () => {
    const domain = await nova.getDomain(ZERO_ADDR);
    must(domain.domain === 0, `expected Domain 0, got ${domain.domain}`);
    must(domain.canCallNovaPrecompiles === true, "EOA should keep direct Nova precompile access");
    return domain.domainName;
  });

  await check("nova_getCapabilities for EOA", async () => {
    const caps = await nova.getCapabilities(ZERO_ADDR);
    must(caps.capabilities.includes("sessionArbiter"), "sessionArbiter capability missing");
    must(caps.precompileRequirements.some((p) => p.address === "0x2D"), "0x2D gate missing");
    return `${caps.capabilityMask} ${caps.capabilities.join(",")}`;
  });

  await check("nova_getSession empty result", async () => {
    const session = await nova.getSession(ZERO_HASH);
    must(session.exists === false, "zero session should not exist");
    return "exists=false";
  });

  await check("nova_sessionConfig active", async () => {
    const cfg = await nova.sessionConfig();
    must(cfg.active === true, "Session fork should be active on devnet");
    must(cfg.precompile === "0x2D", "wrong session precompile");
    return `fork=${cfg.forkBlock} maxStateBytes=${cfg.maxStateBytes}`;
  });

  await check("nova_getPendingEffects", async () => {
    const pending = await nova.getPendingEffects(0, 2);
    must(typeof pending.pending === "number", "pending count missing");
    return `pending=${pending.pending}`;
  });

  await check("nova_getProtocolObjectTier empty object", async () => {
    const tier = await nova.getProtocolObjectTier(ZERO_HASH);
    must(tier.exists === false, "zero protocol object should not exist");
    return `tier=${tier.tier}`;
  });

  await check("nova_getStateTier", async () => {
    const tier = await nova.getStateTier(ZERO_ADDR, ZERO_HASH);
    must(typeof tier.tier === "string", "tier missing");
    return `tier=${tier.tier} age=${tier.ageBlocks}`;
  });

  await optional("nova_getStateWitness archive method", async () => {
    const witness = await nova.getStateWitness(ZERO_ADDR, ZERO_HASH);
    must(typeof witness.proof === "string", "proof missing");
    return `nodes=${witness.nodeCount}`;
  });

  await check("SDK Domain 1 runtime helper", async () => {
    const runtime = domainRuntimeBytecode(1, "0x60006000f3");
    must(runtime.startsWith("0xef01"), "Domain 1 prefix missing");
    return runtime;
  });

  await check("SDK Domain 2 initcode helper", async () => {
    const initcode = buildDomainInitcode(2, "0x60006000f3");
    must(initcode.includes("ef02"), "Domain 2 prefix missing from initcode");
    return `bytes=${(initcode.length - 2) / 2}`;
  });

  console.log("------------------------------------------------------------------------");
  console.log(`RESULT: ${pass} pass, ${fail} fail, ${warn} warn`);
  if (fail > 0) {
    process.exit(1);
  }
})().catch((err) => {
  console.error(err);
  process.exit(1);
});
