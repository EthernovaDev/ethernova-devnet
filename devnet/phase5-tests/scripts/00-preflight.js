// scripts/00-preflight.js — verify env before any test runs.
const sh = require("./shared");

async function main() {
  sh.logHeader("Suite 00 - Preflight");
  sh.clearReport(); // fresh run starts here
  const tracker = new sh.ResultTracker("00-preflight");

  try {
    sh.validateLifecycleThresholds();
    tracker.pass("Lifecycle threshold order", sh.lifecyclePlanSummary());
  } catch (e) {
    tracker.fail("Lifecycle threshold order", e.message);
  }

  // 1. Reachability
  const heads = {};
  for (const node of sh.CONFIG.NODES) {
    try {
      const head = await sh.getBlockNumber(node.url);
      heads[node.label] = head;
      tracker.pass(`Reachable: ${node.label}`, `head=${head}`);
    } catch (e) {
      heads[node.label] = -1;
      tracker.fail(`Reachable: ${node.label}`, e.message);
    }
  }

  // 2. Chain ID
  const chainResults = await sh.rpcAll("eth_chainId");
  for (const [label, result] of Object.entries(chainResults)) {
    if (result && result.__error) {
      tracker.fail(`ChainID: ${label}`, result.__error);
    } else {
      const cid = parseInt(result, 16);
      if (cid === sh.CONFIG.PRIMARY_CHAIN_ID) {
        tracker.pass(`ChainID: ${label}`, `${cid}`);
      } else {
        tracker.fail(`ChainID: ${label}`, `expected ${sh.CONFIG.PRIMARY_CHAIN_ID}, got ${cid}`);
      }
    }
  }

  // 3. Head sync
  const validHeads = Object.values(heads).filter((h) => h > 0);
  if (validHeads.length >= 2) {
    const min = Math.min(...validHeads);
    const max = Math.max(...validHeads);
    if (max - min <= 5) {
      tracker.pass("Head sync", `min=${min} max=${max} (delta=${max - min})`);
    } else {
      tracker.fail("Head sync", `min=${min} max=${max} (delta=${max - min} > 5)`);
    }
  } else {
    tracker.skip("Head sync", "need 2+ nodes");
  }

  // 4. Phase 5 fork active
  const cfgResults = await sh.rpcAll("ethernova_stateLifecycleConfig");
  for (const [label, result] of Object.entries(cfgResults)) {
    if (result && result.__error) {
      tracker.fail(`Phase 5 fork: ${label}`, result.__error);
      continue;
    }
    if (!result || typeof result !== "object") {
      tracker.fail(`Phase 5 fork: ${label}`, "no config returned");
      continue;
    }
    const head = heads[label];
    const fb = result.forkBlock || 0;
    if (head >= fb) {
      tracker.pass(`Phase 5 fork: ${label}`, `forkBlock=${fb} head=${head} active=true`);
    } else {
      tracker.fail(`Phase 5 fork: ${label}`, `forkBlock=${fb} head=${head} not yet active`);
    }
  }

  // 5. Threshold consistency
  for (const [label, result] of Object.entries(cfgResults)) {
    if (!result || result.__error) continue;
    const ok =
      result.activeTierBlocks == sh.CONFIG.ACTIVE_TIER_BLOCKS &&
      result.warmTierBlocks == sh.CONFIG.WARM_TIER_BLOCKS &&
      result.coldTierBlocks == sh.CONFIG.COLD_TIER_BLOCKS &&
      result.warmingFeePerByte == sh.CONFIG.WARMING_FEE_PER_BYTE;
    if (ok) {
      tracker.pass(`Threshold match: ${label}`, `active=${result.activeTierBlocks} warm=${result.warmTierBlocks} cold=${result.coldTierBlocks} fee=${result.warmingFeePerByte}`);
    } else {
      tracker.fail(`Threshold match: ${label}`,
        `node=${JSON.stringify({a: result.activeTierBlocks, w: result.warmTierBlocks, c: result.coldTierBlocks, f: result.warmingFeePerByte})}, env active=${sh.CONFIG.ACTIVE_TIER_BLOCKS} warm=${sh.CONFIG.WARM_TIER_BLOCKS} cold=${sh.CONFIG.COLD_TIER_BLOCKS} fee=${sh.CONFIG.WARMING_FEE_PER_BYTE}`);
    }
  }

  // 6. Precompile 0x2F responds
  const probeAddr = "0x0000000000000000000000000000000000000000";
  const probeInput = "0x03" + "00".repeat(12) + probeAddr.slice(2);
  for (const node of sh.CONFIG.NODES) {
    try {
      const out = await sh.rpcCall(node.url, "eth_call", [{
        to: "0x000000000000000000000000000000000000002F",
        data: probeInput,
      }, "latest"]);
      if (out && out.length === 66) {
        tracker.pass(`Precompile 0x2F: ${node.label}`, `selector 0x03 returned ${out}`);
      } else {
        tracker.fail(`Precompile 0x2F: ${node.label}`, `unexpected output: ${out}`);
      }
    } catch (e) {
      tracker.fail(`Precompile 0x2F: ${node.label}`, e.message);
    }
  }

  // 7. Signer balance
  if (!sh.CONFIG.PRIVATE_KEY || !/^0x[0-9a-fA-F]{64}$/.test(sh.CONFIG.PRIVATE_KEY)) {
    tracker.fail("Signer", "PRIVATE_KEY missing or invalid in .env");
  } else {
    try {
      const provider = sh.makeProvider(sh.CONFIG.PRIMARY_RPC);
      const wallet = sh.makeSigner(provider);
      const bal = await provider.getBalance(wallet.address);
      const balEth = Number(bal) / 1e18;
      if (balEth >= 1) {
        tracker.pass("Signer balance", `${wallet.address} has ${balEth.toFixed(4)} NOVA`);
      } else {
        tracker.fail("Signer balance", `${wallet.address} has only ${balEth.toFixed(4)} NOVA (need >= 1, ideally 5+)`);
      }
    } catch (e) {
      tracker.fail("Signer balance", e.message);
    }
  }

  const result = tracker.finalize();
  sh.appendReport(result);
  console.log(`\n  Suite 00: ${result.pass} pass, ${result.fail} fail, ${result.skip} skip (${result.elapsed_seconds}s)\n`);
  process.exit(result.fail > 0 ? 1 : 0);
}

main().catch((e) => { console.error(e); process.exit(2); });
