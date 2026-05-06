// scripts/07-multinode-consensus.js — comprehensive consensus check.
//
// Runs a battery of state queries across all configured nodes and
// verifies they all agree. If any single check disagrees, Phase 5
// has a determinism bug somewhere.

const sh = require("./shared");

async function main() {
  sh.logHeader("Suite 07 - Multi-Node Consensus");

  if (sh.CONFIG.NODES.length < 2) {
    console.log(`  ${sh.C.yellow}[SKIP] (need 2+ nodes for consensus check, have ${sh.CONFIG.NODES.length})${sh.C.reset}`);
    sh.appendReport({ suite: "07-multinode-consensus", pass: 0, fail: 0, skip: 1, total: 1, elapsed_seconds: 0, checks: [{ name: "all", status: "SKIP", detail: "single-node" }] });
    process.exit(0);
  }

  const tracker = new sh.ResultTracker("07-multinode-consensus");

  // Wait for nodes to converge
  sh.logInfo("Waiting for nodes to converge on same head block...");
  for (let i = 0; i < 30; i++) {
    const heads = {};
    for (const node of sh.CONFIG.NODES) {
      try {
        heads[node.label] = await sh.getBlockNumber(node.url);
      } catch (e) {
        heads[node.label] = null;
      }
    }
    const valid = Object.values(heads).filter((h) => h !== null);
    if (valid.length >= 2) {
      const min = Math.min(...valid);
      const max = Math.max(...valid);
      if (max - min <= 2) {
        sh.logInfo(`Converged: ${JSON.stringify(heads)}`);
        break;
      }
    }
    await new Promise((r) => setTimeout(r, 3000));
  }

  const head = await sh.getBlockNumber(sh.CONFIG.PRIMARY_RPC);
  const checkBlock = "0x" + (head - 5).toString(16);
  sh.logInfo(`Using check block ${head - 5} (head=${head})`);

  // 1. Block hash
  const blockResults = await sh.rpcAll("eth_getBlockByNumber", [checkBlock, false]);
  const blockHashes = {};
  for (const [label, result] of Object.entries(blockResults)) {
    if (result && !result.__error && result.hash) blockHashes[label] = result.hash;
    else blockHashes[label] = result?.__error || "no-hash";
  }
  const hashAgree = sh.allAgree(blockHashes);
  if (hashAgree.ok) {
    tracker.pass("Block hash at head-5", `hash=${Object.values(blockHashes)[0]}`);
  } else {
    tracker.fail("Block hash at head-5 - CONSENSUS SPLIT", hashAgree.reason);
  }

  // 2. State root
  const stateRoots = {};
  for (const [label, result] of Object.entries(blockResults)) {
    if (result && !result.__error && result.stateRoot) stateRoots[label] = result.stateRoot;
    else stateRoots[label] = result?.__error || "no-stateRoot";
  }
  const stateRootAgree = sh.allAgree(stateRoots);
  if (stateRootAgree.ok) {
    tracker.pass("State root at head-5", `root=${Object.values(stateRoots)[0]}`);
  } else {
    tracker.fail("State root at head-5 - CONSENSUS SPLIT", stateRootAgree.reason);
  }

  // 3. WarmStateRoot
  const warmRootResults = await sh.rpcAll("ethernova_getWarmStateRoot");
  const warmRoots = {};
  for (const [label, result] of Object.entries(warmRootResults)) {
    if (result && !result.__error && result.warmStateRoot) warmRoots[label] = result.warmStateRoot;
    else warmRoots[label] = result?.__error || "no-warmStateRoot";
  }
  const warmAgree = sh.allAgree(warmRoots);
  if (warmAgree.ok) {
    tracker.pass("WarmStateRoot consensus", `root=${Object.values(warmRoots)[0]}`);
  } else {
    tracker.fail("WarmStateRoot consensus - INDEX DIVERGENCE", warmAgree.reason);
  }

  // 4. Lifecycle config
  const cfgResults = await sh.rpcAll("ethernova_stateLifecycleConfig");
  const cfgs = {};
  for (const [label, result] of Object.entries(cfgResults)) {
    if (result && !result.__error) cfgs[label] = {
      activeTierBlocks: result.activeTierBlocks,
      warmTierBlocks: result.warmTierBlocks,
      coldTierBlocks: result.coldTierBlocks,
      warmingFeePerByte: result.warmingFeePerByte,
      forkBlock: result.forkBlock,
    };
    else cfgs[label] = result?.__error || "no-config";
  }
  const cfgAgree = sh.allAgree(cfgs);
  if (cfgAgree.ok) {
    tracker.pass("Lifecycle config consensus", JSON.stringify(Object.values(cfgs)[0]));
  } else {
    tracker.fail("Lifecycle config consensus", cfgAgree.reason);
  }

  // 5. Tier of probe contracts
  const probeAddresses = [];
  try {
    const dep = sh.loadDeployment("deployments");
    if (dep.lifecycleHarness) probeAddresses.push({ name: "lifecycleHarness", addr: dep.lifecycleHarness });
    if (dep.witnessProbe) probeAddresses.push({ name: "witnessProbe", addr: dep.witnessProbe });
    if (dep.regressionToken) probeAddresses.push({ name: "regressionToken", addr: dep.regressionToken });
  } catch (e) {
    tracker.skip("Probe tier consensus", "no deployments yet");
  }
  try {
    const tp = sh.loadDeployment("tier-probe");
    probeAddresses.push({ name: "tier-probe", addr: tp.address });
  } catch (e) { /* optional */ }

  for (const probe of probeAddresses) {
    const slot = "0x" + "00".repeat(32);
    const tierResults = await sh.rpcAll("ethernova_getStateTier", [probe.addr, slot]);
    const stripped = {};
    for (const [label, result] of Object.entries(tierResults)) {
      if (result && !result.__error) {
        stripped[label] = { tier: result.tier, lastTouched: result.lastTouched, isArchived: result.isArchived };
      } else {
        stripped[label] = result?.__error || "no-tier";
      }
    }
    const agree = sh.allAgree(stripped);
    if (agree.ok) {
      tracker.pass(`Tier consensus: ${probe.name}`, `tier=${stripped[Object.keys(stripped)[0]].tier}`);
    } else {
      tracker.fail(`Tier consensus: ${probe.name}`, agree.reason);
    }
  }

  // 6. Storage roots
  for (const probe of probeAddresses.slice(0, 2)) {
    const slot = "0x" + "00".repeat(32);
    const proofResults = await sh.rpcAll("eth_getProof", [probe.addr, [slot], "latest"]);
    const storageRoots = {};
    for (const [label, result] of Object.entries(proofResults)) {
      if (result && !result.__error && result.storageHash) storageRoots[label] = result.storageHash;
      else storageRoots[label] = result?.__error || "no-storageHash";
    }
    const srAgree = sh.allAgree(storageRoots);
    if (srAgree.ok) {
      tracker.pass(`Storage root consensus: ${probe.name}`, `root=${Object.values(storageRoots)[0]}`);
    } else {
      tracker.fail(`Storage root consensus: ${probe.name}`, srAgree.reason);
    }
  }

  const result = tracker.finalize();
  sh.appendReport(result);
  console.log(`\n  Suite 07: ${result.pass} pass, ${result.fail} fail, ${result.skip} skip (${result.elapsed_seconds}s)\n`);
  process.exit(result.fail > 0 ? 1 : 0);
}

main().catch((e) => { console.error(e); process.exit(2); });
