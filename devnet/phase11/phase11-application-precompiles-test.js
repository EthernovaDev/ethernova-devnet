#!/usr/bin/env node
"use strict";

const { NovaProvider, PRECOMPILES } = require("../nova-sdk");

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
  console.log(" Phase 11 - Application-Layer Precompiles");
  console.log("========================================================================");
  console.log(`RPC: ${RPC}`);

  const nova = new NovaProvider(RPC, { fallbackNamespace: false });

  await check("eth_chainId is Ethernova devnet", async () => {
    const chainId = await nova.chainId();
    must(chainId === "0x1dab6", `expected 0x1dab6, got ${chainId}`);
    return chainId;
  });

  await check("nova_applicationPrecompiles advertises Phase 11", async () => {
    const cfg = await nova.applicationPrecompiles();
    must(cfg.phase === 11, `expected phase 11, got ${cfg.phase}`);
    must(cfg.active === true, "Phase 11 must be active on devnet");
    const names = cfg.precompiles.map((p) => p.name).sort();
    for (const name of ["novaAsyncCallback", "novaIdentityAttestation", "novaSocialGraph", "novaContentManifest", "novaGameState", "novaComputeBounty"]) {
      must(names.includes(name), `${name} missing`);
    }
    const addrs = Object.fromEntries(cfg.precompiles.map((p) => [p.name, p.address.toLowerCase()]));
    must(addrs.novaIdentityAttestation === "0x31", "identity must be 0x31, not old 0x2C");
    must(addrs.novaSocialGraph === "0x32", "social must be 0x32, not old 0x2B");
    must(addrs.novaComputeBounty === "0x36", "compute bounty must be 0x36, leaving 0x35 for mailbox ops");
    return `${cfg.precompiles.length} precompiles`;
  });

  await check("SDK precompile map includes collision-safe slots", async () => {
    must(PRECOMPILES.identityAttestation.endsWith("31"), `identity ${PRECOMPILES.identityAttestation}`);
    must(PRECOMPILES.socialGraph.endsWith("32"), `social ${PRECOMPILES.socialGraph}`);
    must(PRECOMPILES.mailboxOps.endsWith("35"), `mailboxOps ${PRECOMPILES.mailboxOps}`);
    must(PRECOMPILES.computeBounty.endsWith("36"), `computeBounty ${PRECOMPILES.computeBounty}`);
    return "0x31/0x32/0x35/0x36 OK";
  });

  await check("nova_getCapabilities exposes applicationPrecompiles gate", async () => {
    const caps = await nova.getCapabilities("0x0000000000000000000000000000000000000000");
    const appGate = caps.precompileRequirements.find((g) => g.address === "0x30");
    must(appGate, "0x30 gate missing");
    must(appGate.capability === "applicationPrecompiles", `0x30 capability ${appGate.capability}`);
    must(caps.capabilities.includes("applicationPrecompiles"), "EOA tooling capability missing applicationPrecompiles");
    return appGate.mask;
  });

  await check("nova_developerTooling advertises Phase 11/12 methods", async () => {
    const tooling = await nova.developerTooling();
    must(tooling.phase === 12, `tooling phase ${tooling.phase}`);
    for (const method of ["nova_applicationPrecompiles", "nova_opcodeConfig"]) {
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
