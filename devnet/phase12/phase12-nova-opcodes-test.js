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
  console.log(" Phase 12 - Nova Opcode Bridge");
  console.log("========================================================================");
  console.log(`RPC: ${RPC}`);

  const nova = new NovaProvider(RPC, { fallbackNamespace: false });

  await check("eth_chainId is Ethernova devnet", async () => {
    const chainId = await nova.chainId();
    must(chainId === "0x1dab6", `expected 0x1dab6, got ${chainId}`);
    return chainId;
  });

  await check("nova_opcodeConfig exposes 0xD0-0xD8 bridge", async () => {
    const cfg = await nova.opcodeConfig();
    must(cfg.phase === 12, `phase ${cfg.phase}`);
    must(cfg.active === true, "Phase 12 must be active on devnet");
    must(cfg.opcodeRange === "0xD0-0xD8", `range ${cfg.opcodeRange}`);
    const map = Object.fromEntries(cfg.opcodes.map((o) => [o.name, o.opcode]));
    const expected = { MSEND: "0xD0", MRECV: "0xD1", MPEEK: "0xD2", MCOUNT: "0xD3", CREF: "0xD4", CVERIFY: "0xD5", SOPEN: "0xD6", SCOMMIT: "0xD7", SCLOSE: "0xD8" };
    for (const [name, opcode] of Object.entries(expected)) {
      must(map[name] === opcode, `${name}=${map[name]} expected ${opcode}`);
    }
    must(String(cfg.legacyDraftRangeAvoided).includes("0xF6-0xFE"), "missing collision note");
    return `${cfg.opcodes.length} opcodes`;
  });

  await check("opcode bridges target existing precompile selectors", async () => {
    const cfg = await nova.opcodeConfig();
    const bridge = Object.fromEntries(cfg.opcodes.map((o) => [o.name, o.bridge]));
    must(bridge.MSEND.includes("0x35 selector 0x01"), `MSEND bridge ${bridge.MSEND}`);
    must(bridge.CREF.includes("0x2B selector 0x01"), `CREF bridge ${bridge.CREF}`);
    must(bridge.SOPEN.includes("0x2D selector 0x01"), `SOPEN bridge ${bridge.SOPEN}`);
    return "mailbox/content/session bridges OK";
  });

  await check("nova_developerTooling advertises opcode config", async () => {
    const tooling = await nova.developerTooling();
    must(tooling.rpcMethods.includes("nova_opcodeConfig"), "nova_opcodeConfig missing");
    return `${tooling.rpcMethods.length} methods`;
  });

  console.log("------------------------------------------------------------------------");
  console.log(`RESULT: ${pass} pass, ${fail} fail`);
  if (fail) process.exit(1);
})().catch((err) => {
  console.error(err);
  process.exit(1);
});
