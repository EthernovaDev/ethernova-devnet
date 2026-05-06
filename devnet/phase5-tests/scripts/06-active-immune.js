// scripts/06-active-immune.js — verify continuously-touched contract
// stays Active throughout full Cold-tier window.

const sh = require("./shared");
const { ethers } = require("ethers");

async function main() {
  sh.logHeader("Suite 06 - Active Contract Never Demoted (Longevity)");
  const tracker = new sh.ResultTracker("06-active-immune");

  const provider = sh.makeProvider(sh.CONFIG.PRIMARY_RPC);
  const wallet = sh.makeSigner(provider);
  const artifact = sh.loadArtifact("LifecycleHarness");

  sh.logInfo("Deploying continuously-touched contract...");
  const factory = new ethers.ContractFactory(artifact.abi, artifact.bytecode, wallet);
  const c = await factory.deploy();
  await c.waitForDeployment();
  const addr = await c.getAddress();
  tracker.pass("Deploy active contract", `addr=${addr}`);

  let lastTouchedBlock;
  try {
    const tx = await c.touchAll();
    const receipt = await tx.wait();
    lastTouchedBlock = receipt.blockNumber;
    tracker.pass("Initial touchAll", `block=${lastTouchedBlock}`);
  } catch (e) {
    tracker.fail("Initial touchAll", e.message);
    sh.appendReport(tracker.finalize());
    process.exit(1);
  }

  // Touch every ACTIVE/2 blocks (stay well within active window)
  const touchInterval = Math.max(1, Math.floor(sh.CONFIG.ACTIVE_TIER_BLOCKS / 2));
  const totalDuration = sh.CONFIG.COLD_TIER_BLOCKS + sh.tierBufferForSpan(sh.CONFIG.ACTIVE_TIER_BLOCKS);
  const targetEnd = lastTouchedBlock + totalDuration;
  sh.logInfo(`Will touch every ${touchInterval} blocks for ${totalDuration} blocks total`);
  sh.logInfo(`Start block: ${lastTouchedBlock}, target end: ${targetEnd}`);

  const slot = "0x" + "00".repeat(32);
  let touchCount = 0;
  let allActive = true;

  while (true) {
    const cur = await sh.getBlockNumber(sh.CONFIG.PRIMARY_RPC);
    if (cur >= targetEnd) break;

    const nextTouch = lastTouchedBlock + touchInterval;
    if (cur < nextTouch) {
      try {
        await sh.waitForBlock(sh.CONFIG.PRIMARY_RPC, nextTouch, sh.CONFIG.MAX_WAIT_SECONDS);
      } catch (e) {
        tracker.fail(`Wait for touch ${touchCount + 1}`, e.message);
        break;
      }
    }

    try {
      const tx = await c.touchAll();
      const receipt = await tx.wait();
      lastTouchedBlock = receipt.blockNumber;
      touchCount++;
    } catch (e) {
      tracker.fail(`Touch #${touchCount + 1}`, e.message);
      allActive = false;
      break;
    }

    const tier = await sh.rpcCall(sh.CONFIG.PRIMARY_RPC, "ethernova_getStateTier", [addr, slot]);
    if (tier.tier !== "Active") {
      tracker.fail(`Tier check at touch #${touchCount}`, `tier=${tier.tier} block=${lastTouchedBlock} (BREAKS Active immunity)`);
      allActive = false;
      break;
    }
    sh.logInfo(`  touch #${touchCount} at block ${lastTouchedBlock}: tier=${tier.tier} age=${tier.ageBlocks}`);
  }

  if (allActive) {
    tracker.pass("Active never demoted across full window", `${touchCount} touches over ${totalDuration} blocks; tier always Active`);

    const finalConsensus = await sh.rpcAll("ethernova_getStateTier", [addr, slot]);
    const finalAgree = sh.allAgree(finalConsensus, ["currentBlock", "ageBlocks"]);
    if (finalAgree.ok) {
      tracker.pass("Final tier - multi-node consensus", finalAgree.reason);
    } else {
      tracker.fail("Final tier - multi-node consensus", finalAgree.reason);
    }
  }

  const result = tracker.finalize();
  sh.appendReport(result);
  console.log(`\n  Suite 06: ${result.pass} pass, ${result.fail} fail, ${result.skip} skip (${result.elapsed_seconds}s)\n`);
  process.exit(result.fail > 0 ? 1 : 0);
}

main().catch((e) => { console.error(e); process.exit(2); });
