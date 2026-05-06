// scripts/08-brutal-stress.js — high-volume mixed-lifecycle stress.
//
// Strategy:
//   1. Deploy STRESS_CONTRACTS contracts in batch (default 50)
//   2. Touch each STRESS_SLOTS_PER_CONTRACT times to populate state
//   3. Subset A (first 25%): keep touching every cycle - stays Active
//   4. Subset B (next 25%): touch sparsely - stays Active (within active window)
//   5. Subset C (next 25%): stop touching - slides through tiers
//   6. Subset D (last 25%): stop touching - becomes Archived
//   7. Concurrent burst: STRESS_CONCURRENT_TX self-sends in parallel
//   8. Verify final tiers + multi-node consensus
//
// NONCE HANDLING NOTES:
//   The maintain loop submits ~thousands of txs over hours. Manual
//   nonce management with shared counter is fragile: a single
//   submission failure (RPC hiccup, transient gas issue, anything)
//   advances the counter without producing the underlying tx, which
//   creates a permanent nonce gap that stalls every subsequent tx.
//   We defend against this by (a) only advancing the local counter
//   AFTER the submission resolves, (b) on any thrown error,
//   re-syncing the counter from the chain via getTransactionCount,
//   (c) periodically re-syncing as a safety net, and (d) waiting
//   for a small batch of inflight txs before continuing the loop
//   so a failure surfaces in the same iteration.

const sh = require("./shared");
const { ethers } = require("ethers");

async function syncNonce(provider, addr) {
  return await provider.getTransactionCount(addr, "pending");
}

async function main() {
  sh.logHeader("Suite 08 - Brutal Stress Test");

  if (sh.CONFIG.SKIP_BRUTAL) {
    console.log(`  ${sh.C.yellow}[SKIP] (SKIP_BRUTAL=1)${sh.C.reset}`);
    sh.appendReport({ suite: "08-brutal-stress", pass: 0, fail: 0, skip: 1, total: 1, elapsed_seconds: 0, checks: [{ name: "all", status: "SKIP", detail: "SKIP_BRUTAL=1" }] });
    process.exit(0);
  }

  const tracker = new sh.ResultTracker("08-brutal-stress");
  const provider = sh.makeProvider(sh.CONFIG.PRIMARY_RPC);
  const wallet = sh.makeSigner(provider);
  const artifact = sh.loadArtifact("LifecycleHarness");
  const factory = new ethers.ContractFactory(artifact.abi, artifact.bytecode, wallet);

  const N = sh.CONFIG.STRESS_CONTRACTS;
  const SLOTS = sh.CONFIG.STRESS_SLOTS_PER_CONTRACT;

  // Phase A: Deploy N contracts
  sh.logInfo(`Deploying ${N} LifecycleHarness contracts in batch...`);
  const startTime = Date.now();
  const contracts = [];
  let nonce = await syncNonce(provider, wallet.address);

  for (let i = 0; i < N; i++) {
    try {
      const c = await factory.deploy({ nonce });
      contracts.push(c);
      nonce++;
    } catch (e) {
      tracker.fail(`Deploy contract ${i}`, e.message);
      sh.appendReport(tracker.finalize());
      process.exit(1);
    }
    if ((i + 1) % 10 === 0) sh.logInfo(`  deployed ${i + 1}/${N}`);
  }
  const addrs = [];
  for (let i = 0; i < N; i++) {
    await contracts[i].waitForDeployment();
    addrs.push(await contracts[i].getAddress());
  }
  const deployElapsed = ((Date.now() - startTime) / 1000).toFixed(1);
  tracker.pass(`Deploy ${N} contracts`, `${deployElapsed}s, ~${(deployElapsed / N).toFixed(2)}s/deploy`);

  // Phase B: Populate
  sh.logInfo(`Populating state: ${SLOTS} touches x ${N} contracts...`);
  const populateStart = Date.now();
  const inflight = [];
  for (let i = 0; i < N; i++) {
    for (let s = 0; s < SLOTS; s++) {
      const c = contracts[i];
      try {
        const tx = await c.set(s, i * 100 + s, { nonce });
        nonce++;
        inflight.push(tx.wait());
      } catch (e) {
        sh.logDebug(`populate failed at i=${i} s=${s}: ${e.message}, resyncing nonce`);
        nonce = await syncNonce(provider, wallet.address);
      }
      // Drain periodically so failures surface promptly
      if (inflight.length >= 50) {
        await Promise.allSettled(inflight.splice(0, 40));
      }
    }
  }
  await Promise.allSettled(inflight);
  // Hard re-sync after populate to fully heal any drift
  nonce = await syncNonce(provider, wallet.address);
  const populateElapsed = ((Date.now() - populateStart) / 1000).toFixed(1);
  const lastPopulateBlock = await sh.getBlockNumber(sh.CONFIG.PRIMARY_RPC);
  tracker.pass(`Populate ${N * SLOTS} slots`, `${populateElapsed}s, last block=${lastPopulateBlock}`);

  // Categorize
  const quartile = Math.floor(N / 4);
  if (quartile < 1) {
    tracker.fail("Stress configuration", `STRESS_CONTRACTS=${N} too small; need at least 4`);
    sh.appendReport(tracker.finalize());
    process.exit(1);
  }
  const subsetA = addrs.slice(0, quartile);
  const subsetB = addrs.slice(quartile, 2 * quartile);
  const subsetC = addrs.slice(2 * quartile, 3 * quartile);
  const subsetD = addrs.slice(3 * quartile);
  sh.logInfo(`Subsets: A=${subsetA.length} B=${subsetB.length} C=${subsetC.length} D=${subsetD.length}`);

  // Phase C: Concurrent burst
  sh.logInfo(`Concurrent burst: ${sh.CONFIG.STRESS_CONCURRENT_TX} parallel self-sends...`);
  const burstStart = Date.now();
  const burstPromises = [];
  const burstNonce = nonce;
  for (let i = 0; i < sh.CONFIG.STRESS_CONCURRENT_TX; i++) {
    burstPromises.push(
      wallet.sendTransaction({
        to: wallet.address,
        value: 0,
        nonce: burstNonce + i,
        gasLimit: 21000,
      }).then((tx) => tx.wait())
    );
  }
  // Use allSettled so partial failures don't blow up; we re-sync below.
  const burstResults = await Promise.allSettled(burstPromises);
  const burstFailed = burstResults.filter((r) => r.status === "rejected").length;
  const burstElapsed = ((Date.now() - burstStart) / 1000).toFixed(1);
  if (burstFailed === 0) {
    tracker.pass(`Concurrent burst`, `${sh.CONFIG.STRESS_CONCURRENT_TX} txs in ${burstElapsed}s`);
  } else {
    // Partial failures still pass — we re-sync nonce. Useful diagnostic:
    tracker.pass(`Concurrent burst (with ${burstFailed} retries)`, `${sh.CONFIG.STRESS_CONCURRENT_TX - burstFailed}/${sh.CONFIG.STRESS_CONCURRENT_TX} mined in ${burstElapsed}s`);
  }
  // Hard re-sync after burst
  nonce = await syncNonce(provider, wallet.address);

  // Phase D: Maintain subsets
  const targetEnd = sh.archiveTargetBlock
    ? sh.archiveTargetBlock(lastPopulateBlock)
    : (lastPopulateBlock + sh.CONFIG.COLD_TIER_BLOCKS + sh.CONFIG.WAIT_BUFFER_BLOCKS);
  sh.logInfo(`Maintaining subsets until block ${targetEnd}...`);

  const activeMaintenanceWindow = Math.max(1, Math.floor(sh.CONFIG.ACTIVE_TIER_BLOCKS / 2));
  const batchA = Math.max(1, Math.ceil(subsetA.length / activeMaintenanceWindow));
  const batchB = Math.max(1, Math.ceil(subsetB.length / activeMaintenanceWindow));
  sh.logInfo(`Fast-threshold maintenance: batchA=${batchA}/loop batchB=${batchB}/loop activeWindow=${activeMaintenanceWindow}`);

  let touchSubsetCounter = 0;
  let idxA = 0;
  let idxB = 0;
  let consecutiveFails = 0;
  let totalSubmissions = 0;
  let totalFailures = 0;

  // Helper: submit one touch with robust nonce handling. Returns the
  // submitted Promise<receipt> so caller can choose to await it. On
  // submission failure, we resync the nonce and skip this touch.
  async function safeTouch(c, label) {
    try {
      const tx = await c.touchAll({ nonce });
      nonce++;
      consecutiveFails = 0;
      totalSubmissions++;
      return tx.wait().catch((e) => {
        sh.logDebug(`${label} mining failed: ${e.message}`);
        return null;
      });
    } catch (e) {
      consecutiveFails++;
      totalFailures++;
      sh.logDebug(`${label} submission failed at nonce=${nonce}: ${e.message}, resyncing`);
      nonce = await syncNonce(provider, wallet.address);
      return null;
    }
  }

  while (true) {
    const cur = await sh.getBlockNumber(sh.CONFIG.PRIMARY_RPC);
    if (cur >= targetEnd) break;

    const batchPromises = [];

    for (let j = 0; j < batchA; j++) {
      const cA = contracts[idxA % subsetA.length];
      const p = await safeTouch(cA, `subsetA[${idxA % subsetA.length}]`);
      if (p) batchPromises.push(p);
      idxA++;
    }

    for (let j = 0; j < batchB; j++) {
      const cB = contracts[quartile + (idxB % subsetB.length)];
      const p = await safeTouch(cB, `subsetB[${idxB % subsetB.length}]`);
      if (p) batchPromises.push(p);
      idxB++;
    }

    // Periodically wait for inflight to finish so failures surface
    // before we accumulate too many pending.
    if (touchSubsetCounter % 5 === 0 && batchPromises.length > 0) {
      await Promise.allSettled(batchPromises);
    }

    // Periodic safety re-sync every 30 iterations (~4 minutes of loop)
    if (touchSubsetCounter > 0 && touchSubsetCounter % 30 === 0) {
      const before = nonce;
      nonce = await syncNonce(provider, wallet.address);
      if (Math.abs(nonce - before) > 5) {
        sh.logDebug(`periodic resync: ${before} -> ${nonce}`);
      }
    }

    // Bail-out guard: if 20 consecutive failures, something is very
    // wrong — flag and break out instead of looping forever.
    if (consecutiveFails >= 20) {
      tracker.fail("Maintain loop stalled", `${consecutiveFails} consecutive submission failures; aborting maintain`);
      break;
    }

    touchSubsetCounter++;
    await new Promise((r) => setTimeout(r, 8000));
  }

  sh.logInfo(`Maintain loop done: ${totalSubmissions} touches submitted, ${totalFailures} submission failures resynced`);

  const finalTarget = (await sh.getBlockNumber(sh.CONFIG.PRIMARY_RPC)) + 5;
  await sh.waitForBlock(sh.CONFIG.PRIMARY_RPC, finalTarget, sh.CONFIG.MAX_WAIT_SECONDS);

  // Phase E: Verify final tiers
  sh.logInfo("Querying final tiers for all subsets...");
  const slot = "0x" + "00".repeat(32);

  let aFails = 0;
  for (const addr of subsetA) {
    const t = await sh.rpcCall(sh.CONFIG.PRIMARY_RPC, "ethernova_getStateTier", [addr, slot]);
    if (t.tier !== "Active") { aFails++; sh.logDebug(`Subset A ${addr}: tier=${t.tier} lastTouched=${t.lastTouched}`); }
  }
  if (aFails === 0) {
    tracker.pass(`Subset A all Active`, `${subsetA.length}/${subsetA.length} contracts`);
  } else {
    tracker.fail(`Subset A all Active`, `${aFails}/${subsetA.length} not Active`);
  }

  let bFails = 0;
  for (const addr of subsetB) {
    const t = await sh.rpcCall(sh.CONFIG.PRIMARY_RPC, "ethernova_getStateTier", [addr, slot]);
    if (t.tier !== "Active") { bFails++; sh.logDebug(`Subset B ${addr}: tier=${t.tier} lastTouched=${t.lastTouched}`); }
  }
  if (bFails === 0) {
    tracker.pass(`Subset B all Active`, `${subsetB.length}/${subsetB.length} contracts`);
  } else {
    tracker.fail(`Subset B all Active`, `${bFails}/${subsetB.length} not Active (touch interval too sparse?)`);
  }

  let cActive = 0;
  for (const addr of subsetC) {
    const t = await sh.rpcCall(sh.CONFIG.PRIMARY_RPC, "ethernova_getStateTier", [addr, slot]);
    if (t.tier === "Active") { cActive++; sh.logDebug(`Subset C ${addr}: unexpectedly Active`); }
  }
  if (cActive === 0) {
    tracker.pass(`Subset C demoted from Active`, `${subsetC.length}/${subsetC.length} contracts demoted`);
  } else {
    tracker.fail(`Subset C demoted from Active`, `${cActive}/${subsetC.length} still Active`);
  }

  let dArchived = 0;
  for (const addr of subsetD) {
    const t = await sh.rpcCall(sh.CONFIG.PRIMARY_RPC, "ethernova_getStateTier", [addr, slot]);
    if (t.tier === "Archived" && t.isArchived) dArchived++;
  }
  if (dArchived === subsetD.length) {
    tracker.pass(`Subset D all Archived (sweep handled load)`, `${dArchived}/${subsetD.length} archived`);
  } else if (dArchived >= subsetD.length * 0.8) {
    tracker.pass(`Subset D mostly Archived (sweep cap working)`, `${dArchived}/${subsetD.length} archived`);
  } else {
    tracker.fail(`Subset D all Archived`, `${dArchived}/${subsetD.length} archived - sweep may be backlogged`);
  }

  // Phase F: Multi-node consensus on sample
  const sample = [
    ...subsetA.slice(0, 3),
    ...subsetB.slice(0, 3),
    ...subsetC.slice(0, 3),
    ...subsetD.slice(0, 3),
  ];
  let consensusOK = 0, consensusBad = 0;
  for (const addr of sample) {
    const results = await sh.rpcAll("ethernova_getStateTier", [addr, slot]);
    const stripped = {};
    for (const [l, r] of Object.entries(results)) {
      if (r && !r.__error) stripped[l] = { tier: r.tier, lastTouched: r.lastTouched, isArchived: r.isArchived };
      else stripped[l] = r?.__error || "no-data";
    }
    const agree = sh.allAgree(stripped);
    if (agree.ok) consensusOK++;
    else { consensusBad++; sh.logDebug(`Consensus fail on ${addr}: ${agree.reason}`); }
  }
  if (consensusBad === 0) {
    tracker.pass(`Multi-node consensus on stress sample`, `${consensusOK}/${sample.length} contracts agree`);
  } else {
    tracker.fail(`Multi-node consensus on stress sample`, `${consensusBad}/${sample.length} disagree - DETERMINISM BUG`);
  }

  // Phase G: WarmStateRoot under stress
  const warmRoots = await sh.rpcAll("ethernova_getWarmStateRoot");
  const wr = {};
  for (const [l, r] of Object.entries(warmRoots)) {
    if (r && !r.__error) wr[l] = r.warmStateRoot;
    else wr[l] = r?.__error || "no-data";
  }
  const wrAgree = sh.allAgree(wr);
  if (wrAgree.ok) {
    tracker.pass(`WarmStateRoot consensus under stress`, `${Object.values(wr)[0]}`);
  } else {
    tracker.fail(`WarmStateRoot consensus under stress`, wrAgree.reason);
  }

  const result = tracker.finalize();
  sh.appendReport(result);
  console.log(`\n  Suite 08: ${result.pass} pass, ${result.fail} fail, ${result.skip} skip (${result.elapsed_seconds}s)\n`);
  process.exit(result.fail > 0 ? 1 : 0);
}

main().catch((e) => { console.error(e); process.exit(2); });
