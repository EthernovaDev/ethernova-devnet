// scripts/04-witness-restore.js — full round-trip:
//   1. Take Archived probe contract from suite 02
//   2. Generate witness via ethernova_getStateWitness
//   3. Submit witness via 0x2F selector 0x02 (restoreState)
//   4. Verify tier flips back to Active
//   5. Test rejection paths

const sh = require("./shared");
const { ethers } = require("ethers");

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function waitForArchivedMarker(url, address, slot, maxSeconds) {
  const start = Date.now();
  let last;
  while ((Date.now() - start) / 1000 <= maxSeconds) {
    last = await sh.rpcCall(url, "ethernova_getStateTier", [address, slot]);
    if (last?.tier === "Archived" && last?.isArchived === true) return last;
    const cur = await sh.getBlockNumber(url);
    await sh.waitForBlock(url, cur + 1, Math.min(120, maxSeconds));
    await sleep(1000);
  }
  return last;
}

async function waitForRestoredConsensus(address, slot, maxSeconds) {
  const start = Date.now();
  let consensus;
  let agree;
  while ((Date.now() - start) / 1000 <= maxSeconds) {
    consensus = await sh.rpcAll("ethernova_getStateTier", [address, slot]);
    agree = sh.allAgree(consensus, ["currentBlock", "ageBlocks"]);
    if (agree.ok) return { consensus, agree };
    await sleep(5000);
  }
  return { consensus, agree };
}

async function deployFreshProbe(wallet, tracker) {
  const artifact = sh.loadArtifact("LifecycleHarness");
  const factory = new ethers.ContractFactory(artifact.abi, artifact.bytecode, wallet);
  sh.logInfo("Saved tier-probe has no archive marker; deploying a fresh witness probe...");
  const c = await factory.deploy();
  const deployReceipt = await c.deploymentTransaction().wait();
  const addr = await c.getAddress();
  const tx = await c.set(0, 12345);
  const touchReceipt = await tx.wait();
  const lastTouchBlock = Number(touchReceipt.blockNumber);
  sh.saveDeployment("tier-probe", {
    address: addr,
    deployBlock: Number(deployReceipt.blockNumber),
    lastTouchBlock,
    thresholds: {
      active: sh.CONFIG.ACTIVE_TIER_BLOCKS,
      warm: sh.CONFIG.WARM_TIER_BLOCKS,
      cold: sh.CONFIG.COLD_TIER_BLOCKS,
      buffer: sh.CONFIG.WAIT_BUFFER_BLOCKS,
    },
  });
  tracker.skip("Probe state precondition", `created fresh probe ${addr}; waiting until block ${sh.archiveTargetBlock(lastTouchBlock)} for Archived`);
  return { address: addr, lastTouchBlock };
}

async function main() {
  sh.logHeader("Suite 04 - Witness Restoration Round-Trip");

  if (sh.CONFIG.SKIP_WITNESS) {
    console.log(`  ${sh.C.yellow}[SKIP] (SKIP_WITNESS=1)${sh.C.reset}`);
    sh.appendReport({ suite: "04-witness-restore", pass: 0, fail: 0, skip: 1, total: 1, elapsed_seconds: 0, checks: [{ name: "all", status: "SKIP", detail: "SKIP_WITNESS=1" }] });
    process.exit(0);
  }

  const tracker = new sh.ResultTracker("04-witness-restore");
  const provider = sh.makeProvider(sh.CONFIG.PRIMARY_RPC);
  const wallet = sh.makeSigner(provider);

  let probe;
  try {
    probe = sh.loadDeployment("tier-probe");
  } catch (e) {
    tracker.fail("Load tier-probe", "run 02-tier-transitions first");
    sh.appendReport(tracker.finalize());
    process.exit(1);
  }

  const slot = "0x" + "00".repeat(32);
  const realExpectedValue = ethers.zeroPadValue(ethers.toBeHex(12345n), 32);

  // Verify probe is Archived. With fast devnet thresholds, this suite can self-heal:
  // if the saved tier-probe is still Active/Warm/Cold, wait just long enough
  // for it to cross ColdTierBlocks and for the lifecycle sweep to stamp it.
  let tierBefore = await sh.rpcCall(sh.CONFIG.PRIMARY_RPC, "ethernova_getStateTier", [probe.address, slot]);
  if (tierBefore.tier === "Archived" && !tierBefore.isArchived) {
    const cur = await sh.getBlockNumber(sh.CONFIG.PRIMARY_RPC);
    const target = sh.archiveTargetBlock(Number(tierBefore.lastTouched || probe.lastTouchBlock || 0));
    if (cur >= target + sh.tierBufferForSpan(sh.CONFIG.COLD_TIER_BLOCKS - sh.CONFIG.WARM_TIER_BLOCKS)) {
      probe = await deployFreshProbe(wallet, tracker);
      tierBefore = await sh.rpcCall(sh.CONFIG.PRIMARY_RPC, "ethernova_getStateTier", [probe.address, slot]);
    }
  }
  if (tierBefore.tier !== "Archived" || !tierBefore.isArchived) {
    const lastTouched = Number(tierBefore.lastTouched || probe.lastTouchBlock || 0);
    let target = sh.archiveTargetBlock(lastTouched);
    let cur = await sh.getBlockNumber(sh.CONFIG.PRIMARY_RPC);
    if (cur < target) {
      tracker.skip("Probe state precondition", `currently ${tierBefore.tier}; waiting until block ${target} for Archived`);
      await sh.waitForBlock(sh.CONFIG.PRIMARY_RPC, target, sh.CONFIG.MAX_WAIT_SECONDS);
    } else {
      target = cur + sh.tierBufferForSpan(sh.CONFIG.ACTIVE_TIER_BLOCKS);
      tracker.skip("Probe state precondition", `currently ${tierBefore.tier}; giving sweep until block ${target}`);
      await sh.waitForBlock(sh.CONFIG.PRIMARY_RPC, target, sh.CONFIG.MAX_WAIT_SECONDS);
    }
    tierBefore = await waitForArchivedMarker(sh.CONFIG.PRIMARY_RPC, probe.address, slot, sh.CONFIG.MAX_WAIT_SECONDS);
    if (tierBefore?.tier === "Archived" && !tierBefore?.isArchived) {
      probe = await deployFreshProbe(wallet, tracker);
      const freshTarget = sh.archiveTargetBlock(Number(probe.lastTouchBlock));
      await sh.waitForBlock(sh.CONFIG.PRIMARY_RPC, freshTarget, sh.CONFIG.MAX_WAIT_SECONDS);
      tierBefore = await waitForArchivedMarker(sh.CONFIG.PRIMARY_RPC, probe.address, slot, sh.CONFIG.MAX_WAIT_SECONDS);
    }
  }
  if (tierBefore.tier !== "Archived") {
    tracker.fail("Probe state precondition", `expected Archived, got ${tierBefore.tier}`);
    sh.appendReport(tracker.finalize());
    process.exit(1);
  }
  if (!tierBefore.isArchived) {
    tracker.fail("Archive marker precondition", `isArchived=false but tier=${tierBefore.tier}; last=${JSON.stringify(tierBefore)}`);
    sh.appendReport(tracker.finalize());
    process.exit(1);
  }
  tracker.pass("Probe state precondition", `Archived + marker stamped`);

  // STEP 1: Generate witness
  let witness;
  try {
    witness = await sh.rpcCall(sh.CONFIG.PRIMARY_RPC, "ethernova_getStateWitness", [probe.address, slot]);
    if (!witness || !witness.proof) {
      tracker.fail("Generate witness", `unexpected response: ${JSON.stringify(witness)}`);
      sh.appendReport(tracker.finalize());
      process.exit(1);
    }
    tracker.pass("Generate witness", `nodeCount=${witness.nodeCount} value=${witness.value} coldRoot=${witness.coldRoot.slice(0, 12)}...`);
  } catch (e) {
    tracker.fail("Generate witness", e.message);
    sh.appendReport(tracker.finalize());
    process.exit(1);
  }

  if (witness.value !== realExpectedValue) {
    tracker.fail("Witness value matches written", `expected ${realExpectedValue}, got ${witness.value}`);
  } else {
    tracker.pass("Witness value matches written", `${witness.value} == 12345`);
  }

  if (witness.storageRoot === witness.coldRoot) {
    tracker.pass("Storage root == cold root", "Phase 5 v1 non-destructive - live trie unchanged");
  } else {
    tracker.fail("Storage root == cold root", `live=${witness.storageRoot} cold=${witness.coldRoot}`);
  }

  // STEP 2: Submit witness via 0x2F selector 0x02
  const proofBytes = witness.proof;
  const proofLen = (proofBytes.length - 2) / 2;
  const calldata = ethers.concat([
    "0x02",
    ethers.zeroPadValue(probe.address, 32),
    slot,
    realExpectedValue,
    ethers.zeroPadValue(ethers.toBeHex(proofLen), 32),
    proofBytes,
  ]);

  let restoreBlock = 0;
  try {
    const tx = await wallet.sendTransaction({
      to: "0x000000000000000000000000000000000000002F",
      data: calldata,
      gasLimit: 500000n,
    });
    const receipt = await tx.wait();
    if (receipt.status === 1) {
      restoreBlock = Number(receipt.blockNumber);
      tracker.pass("Submit witness restore tx", `tx=${tx.hash} block=${receipt.blockNumber} gasUsed=${receipt.gasUsed}`);
    } else {
      tracker.fail("Submit witness restore tx", `receipt status=${receipt.status} (reverted)`);
      sh.appendReport(tracker.finalize());
      process.exit(1);
    }
  } catch (e) {
    tracker.fail("Submit witness restore tx", e.message);
    sh.appendReport(tracker.finalize());
    process.exit(1);
  }

  // STEP 3: Verify tier is Active
  const tierAfter = await sh.rpcCall(sh.CONFIG.PRIMARY_RPC, "ethernova_getStateTier", [probe.address, slot]);
  if (tierAfter.tier === "Active") {
    tracker.pass("Tier flipped to Active", `lastTouched=${tierAfter.lastTouched} (was archived)`);
  } else {
    tracker.fail("Tier flipped to Active", `tier=${tierAfter.tier} after restore - RESTORATION FAILED`);
  }

  if (tierAfter.isArchived === false) {
    tracker.pass("Archive marker cleared", "isArchived=false");
  } else {
    tracker.fail("Archive marker cleared", `isArchived=${tierAfter.isArchived}`);
  }

  // Multi-node consensus on restored state
  if (restoreBlock > 0) {
    await sh.waitForBlock(sh.CONFIG.PRIMARY_RPC, restoreBlock + 1, sh.CONFIG.MAX_WAIT_SECONDS);
  }
  const { agree: restoredAgree } = await waitForRestoredConsensus(probe.address, slot, Math.min(120, sh.CONFIG.MAX_WAIT_SECONDS));
  if (restoredAgree.ok) {
    tracker.pass("Restored state - multi-node consensus", restoredAgree.reason);
  } else {
    tracker.fail("Restored state - multi-node consensus", restoredAgree.reason);
  }

  // STEP 4: Negative test - restore non-archived contract should fail
  try {
    const tx = await wallet.sendTransaction({
      to: "0x000000000000000000000000000000000000002F",
      data: calldata,
      gasLimit: 500000n,
    });
    const receipt = await tx.wait().catch(() => null);
    if (!receipt || receipt.status === 0) {
      tracker.pass("Reject: restore non-archived account", "tx reverted as expected");
    } else {
      tracker.fail("Reject: restore non-archived account", "tx succeeded but should have reverted");
    }
  } catch (e) {
    tracker.pass("Reject: restore non-archived account", "EVM/provider rejected: " + e.message.slice(0, 100));
  }

  // STEP 5: Negative test - empty proof
  try {
    const dep = sh.loadDeployment("deployments");
    const fakeCalldata = ethers.concat([
      "0x02",
      ethers.zeroPadValue(dep.lifecycleHarness, 32),
      slot,
      "0x" + "00".repeat(32),
      ethers.zeroPadValue("0x00", 32),
      "0x",
    ]);
    const tx = await wallet.sendTransaction({
      to: "0x000000000000000000000000000000000000002F",
      data: fakeCalldata,
      gasLimit: 200000n,
    }).catch((e) => null);
    if (!tx) {
      tracker.pass("Reject: empty proof", "EVM/provider rejected at submission");
    } else {
      const receipt = await tx.wait().catch(() => null);
      if (!receipt || receipt.status === 0) {
        tracker.pass("Reject: empty proof", "tx reverted as expected");
      } else {
        tracker.fail("Reject: empty proof", "tx succeeded but should have reverted");
      }
    }
  } catch (e) {
    tracker.pass("Reject: empty proof", "rejected: " + e.message.slice(0, 100));
  }

  const result = tracker.finalize();
  sh.appendReport(result);
  console.log(`\n  Suite 04: ${result.pass} pass, ${result.fail} fail, ${result.skip} skip (${result.elapsed_seconds}s)\n`);
  process.exit(result.fail > 0 ? 1 : 0);
}

main().catch((e) => { console.error(e); process.exit(2); });
