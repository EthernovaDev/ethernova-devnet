// scripts/02-tier-transitions.js — Active -> Warm -> Cold -> Archived
//
// Strategy: deploy fresh contract, touch once, sit idle while blocks
// accumulate. At each threshold query ethernova_getStateTier across
// every consensus node and verify they all agree on expected tier.

const sh = require("./shared");
const { ethers } = require("ethers");

async function main() {
  sh.logHeader("Suite 02 - Tier Transitions (Active -> Warm -> Cold -> Archived)");
  const tracker = new sh.ResultTracker("02-tier-transitions");

  const provider = sh.makeProvider(sh.CONFIG.PRIMARY_RPC);
  const wallet = sh.makeSigner(provider);

  let artifact;
  try {
    artifact = sh.loadArtifact("LifecycleHarness");
  } catch (e) {
    tracker.fail("Load artifact", e.message);
    sh.appendReport(tracker.finalize());
    process.exit(1);
  }

  const factory = new ethers.ContractFactory(artifact.abi, artifact.bytecode, wallet);
  let probeAddr, probeDeployBlock;
  try {
    sh.logInfo("Deploying fresh LifecycleHarness for tier-transition probe...");
    const c = await factory.deploy();
    const tx = c.deploymentTransaction();
    const receipt = await tx.wait();
    probeAddr = await c.getAddress();
    probeDeployBlock = receipt.blockNumber;
    tracker.pass("Deploy probe contract", `addr=${probeAddr} block=${probeDeployBlock}`);
  } catch (e) {
    tracker.fail("Deploy probe contract", e.message);
    sh.appendReport(tracker.finalize());
    process.exit(1);
  }

  // Touch once
  let lastTouchBlock;
  try {
    const c = new ethers.Contract(probeAddr, artifact.abi, wallet);
    const tx = await c.set(0, 12345);
    const receipt = await tx.wait();
    lastTouchBlock = receipt.blockNumber;
    tracker.pass("Initial touch (set slot0=12345)", `tx=${tx.hash} block=${lastTouchBlock}`);
  } catch (e) {
    tracker.fail("Initial touch", e.message);
    sh.appendReport(tracker.finalize());
    process.exit(1);
  }

  const slot = "0x" + "00".repeat(32);

  // Initial state: Active
  const initialTier = await sh.rpcCall(sh.CONFIG.PRIMARY_RPC, "ethernova_getStateTier", [probeAddr, slot]);
  if (initialTier && initialTier.tier === "Active") {
    tracker.pass("Initial tier", `Active (lastTouched=${initialTier.lastTouched}, age=${initialTier.ageBlocks})`);
  } else {
    tracker.fail("Initial tier", `expected Active, got ${JSON.stringify(initialTier)}`);
  }

  const initial = await sh.rpcAll("ethernova_getStateTier", [probeAddr, slot]);
  const initialAgree = sh.allAgree(initial, ["currentBlock", "ageBlocks"]);
  if (initialAgree.ok) {
    tracker.pass("Initial tier - multi-node consensus", initialAgree.reason);
  } else {
    tracker.fail("Initial tier - multi-node consensus", initialAgree.reason);
  }

  // STAGE 1: Warm
  sh.logInfo(`Lifecycle plan: ${sh.lifecyclePlanSummary()}`);
  const warmTarget = sh.warmTargetBlock(lastTouchBlock);
  sh.logInfo(`Waiting for block ${warmTarget} to verify Warm tier...`);
  try {
    await sh.waitForBlock(sh.CONFIG.PRIMARY_RPC, warmTarget, sh.CONFIG.MAX_WAIT_SECONDS);
  } catch (e) {
    tracker.fail("Wait for Warm transition", e.message);
    sh.appendReport(tracker.finalize());
    process.exit(1);
  }

  const warmTier = await sh.rpcCall(sh.CONFIG.PRIMARY_RPC, "ethernova_getStateTier", [probeAddr, slot]);
  if (warmTier && warmTier.tier === "Warm") {
    tracker.pass("Tier == Warm", `lastTouched=${warmTier.lastTouched} age=${warmTier.ageBlocks}`);
  } else {
    tracker.fail("Tier == Warm", `expected Warm, got ${JSON.stringify(warmTier)}`);
  }

  const warmConsensus = await sh.rpcAll("ethernova_getStateTier", [probeAddr, slot]);
  const warmAgree = sh.allAgree(warmConsensus, ["currentBlock", "ageBlocks"]);
  if (warmAgree.ok) {
    tracker.pass("Warm tier - multi-node consensus", warmAgree.reason);
  } else {
    tracker.fail("Warm tier - multi-node consensus", warmAgree.reason);
  }

  // STAGE 2: Cold
  const coldTarget = sh.coldTargetBlock(lastTouchBlock);
  sh.logInfo(`Waiting for block ${coldTarget} to verify Cold tier...`);
  try {
    await sh.waitForBlock(sh.CONFIG.PRIMARY_RPC, coldTarget, sh.CONFIG.MAX_WAIT_SECONDS);
  } catch (e) {
    tracker.fail("Wait for Cold transition", e.message);
    sh.appendReport(tracker.finalize());
    process.exit(1);
  }

  const coldTier = await sh.rpcCall(sh.CONFIG.PRIMARY_RPC, "ethernova_getStateTier", [probeAddr, slot]);
  if (coldTier && coldTier.tier === "Cold") {
    tracker.pass("Tier == Cold", `lastTouched=${coldTier.lastTouched} age=${coldTier.ageBlocks}`);
  } else {
    tracker.fail("Tier == Cold", `expected Cold, got ${JSON.stringify(coldTier)}`);
  }

  const coldConsensus = await sh.rpcAll("ethernova_getStateTier", [probeAddr, slot]);
  const coldAgree = sh.allAgree(coldConsensus, ["currentBlock", "ageBlocks"]);
  if (coldAgree.ok) {
    tracker.pass("Cold tier - multi-node consensus", coldAgree.reason);
  } else {
    tracker.fail("Cold tier - multi-node consensus", coldAgree.reason);
  }

  // STAGE 3: Archived (sweep must run)
  const archiveTarget = sh.archiveTargetBlock(lastTouchBlock);
  sh.logInfo(`Waiting for block ${archiveTarget} to verify Archived tier (sweep must run)...`);
  try {
    await sh.waitForBlock(sh.CONFIG.PRIMARY_RPC, archiveTarget, sh.CONFIG.MAX_WAIT_SECONDS);
  } catch (e) {
    tracker.fail("Wait for Archived transition", e.message);
    sh.appendReport(tracker.finalize());
    process.exit(1);
  }

  const archTier = await sh.rpcCall(sh.CONFIG.PRIMARY_RPC, "ethernova_getStateTier", [probeAddr, slot]);
  if (archTier && archTier.tier === "Archived") {
    tracker.pass("Tier == Archived", `lastTouched=${archTier.lastTouched} age=${archTier.ageBlocks} marker=${archTier.isArchived}`);
    if (archTier.isArchived) {
      tracker.pass("Archive marker stamped", `isArchived=true (sweep ran successfully)`);
    } else {
      tracker.fail("Archive marker stamped", `tier=Archived but isArchived=false (sweep didn't run?)`);
    }
  } else {
    tracker.fail("Tier == Archived", `expected Archived, got ${JSON.stringify(archTier)}`);
  }

  const archConsensus = await sh.rpcAll("ethernova_getStateTier", [probeAddr, slot]);
  const archAgree = sh.allAgree(archConsensus, ["currentBlock", "ageBlocks"]);
  if (archAgree.ok) {
    tracker.pass("Archived tier - multi-node consensus", archAgree.reason);
  } else {
    tracker.fail("Archived tier - multi-node consensus", archAgree.reason);
  }

  sh.saveDeployment("tier-probe", {
    address: probeAddr,
    deployBlock: probeDeployBlock,
    lastTouchBlock,
    thresholds: {
      active: sh.CONFIG.ACTIVE_TIER_BLOCKS,
      warm: sh.CONFIG.WARM_TIER_BLOCKS,
      cold: sh.CONFIG.COLD_TIER_BLOCKS,
      buffer: sh.CONFIG.WAIT_BUFFER_BLOCKS,
    },
  });

  const result = tracker.finalize();
  sh.appendReport(result);
  console.log(`\n  Suite 02: ${result.pass} pass, ${result.fail} fail, ${result.skip} skip (${result.elapsed_seconds}s)\n`);
  process.exit(result.fail > 0 ? 1 : 0);
}

main().catch((e) => { console.error(e); process.exit(2); });
