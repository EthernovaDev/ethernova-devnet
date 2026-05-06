// scripts/03-warming-fee.js — verify the warming-fee surcharge formula.
//
// Strategy:
//   1. Deploy a fresh contract (Active baseline)
//   2. Measure SLOAD gas via eth_estimateGas
//   3. Use the tier-probe contract from suite 02 (Cold/Archived)
//   4. Measure same call's gas
//   5. Delta should match expected = tier_gap * 32 * fee_per_byte
//   6. Multi-node consensus on both gas estimates

const sh = require("./shared");
const { ethers } = require("ethers");

const INTRINSIC_CALL_GAS = 21000;

function applyAdaptiveGas(totalGas, adjustPct) {
  const executionGas = Math.max(0, totalGas - INTRINSIC_CALL_GAS);
  if (adjustPct < 0) {
    return totalGas - Math.floor(executionGas * Math.abs(adjustPct) / 100);
  }
  if (adjustPct > 0) {
    return totalGas + Math.floor(executionGas * adjustPct / 100);
  }
  return totalGas;
}

function invertAdaptiveGas(adjustedGas, adjustPct) {
  if (!adjustPct) return adjustedGas;
  // Gas estimates are small here. Brute force avoids off-by-one mistakes from
  // integer rounding in ApplyAdaptiveGasV2.
  for (let raw = adjustedGas; raw <= adjustedGas + 20000; raw++) {
    if (applyAdaptiveGas(raw, adjustPct) === adjustedGas) return raw;
  }
  throw new Error(`unable to invert adaptive gas adjusted=${adjustedGas} pct=${adjustPct}`);
}

async function getAdaptivePct(url) {
  try {
    const stats = await sh.rpcCall(url, "ethernova_adaptiveGasV2", []);
    return Number(stats?.lastTxClassification?.gasAdjustPercent || 0);
  } catch (_) {
    return 0;
  }
}

async function main() {
  sh.logHeader("Suite 03 - Warming Fee Surcharge");
  const tracker = new sh.ResultTracker("03-warming-fee");

  const provider = sh.makeProvider(sh.CONFIG.PRIMARY_RPC);
  const wallet = sh.makeSigner(provider);
  const artifact = sh.loadArtifact("LifecycleHarness");

  // Deploy baseline (Active)
  sh.logInfo("Deploying baseline (Active) probe...");
  const factory = new ethers.ContractFactory(artifact.abi, artifact.bytecode, wallet);
  const baseline = await factory.deploy();
  await baseline.waitForDeployment();
  const baselineAddr = await baseline.getAddress();
  const tx = await baseline.set(0, 999);
  await tx.wait();
  tracker.pass("Deploy baseline + touch", `addr=${baselineAddr}`);

  const iface = new ethers.Interface(artifact.abi);
  const baselineCalldata = iface.encodeFunctionData("read", [0]);

  // Baseline gas
  const baselineGasHex = await sh.rpcCall(sh.CONFIG.PRIMARY_RPC, "eth_estimateGas", [{
    to: baselineAddr,
    data: baselineCalldata,
  }]);
  const baselineGas = parseInt(baselineGasHex, 16);
  const baselineAdaptivePct = await getAdaptivePct(sh.CONFIG.PRIMARY_RPC);
  tracker.pass("Baseline (Active) read gas", `${baselineGas} gas`);

  // Multi-node consensus on baseline
  const baselineConsensus = await sh.rpcAll("eth_estimateGas", [{
    to: baselineAddr,
    data: baselineCalldata,
  }]);
  const baselineAgree = sh.allAgree(baselineConsensus);
  if (baselineAgree.ok) {
    tracker.pass("Baseline gas - multi-node consensus", baselineAgree.reason);
  } else {
    tracker.fail("Baseline gas - multi-node consensus", baselineAgree.reason);
  }

  // Load tier-probe (should be Archived after suite 02)
  let probe;
  try {
    probe = sh.loadDeployment("tier-probe");
  } catch (e) {
    tracker.fail("Load tier-probe", "run 02-tier-transitions first");
    sh.appendReport(tracker.finalize());
    process.exit(1);
  }

  const slot = "0x" + "00".repeat(32);
  let probeTier = await sh.rpcCall(sh.CONFIG.PRIMARY_RPC, "ethernova_getStateTier", [probe.address, slot]);
  if (probeTier.tier === "Active") {
    const lastTouched = Number(probeTier.lastTouched || probe.lastTouchBlock || 0);
    const target = sh.warmTargetBlock(lastTouched);
    const cur = await sh.getBlockNumber(sh.CONFIG.PRIMARY_RPC);
    if (cur < target) {
      tracker.skip("Probe tier precondition", `currently Active; waiting until block ${target} for Warm tier`);
      await sh.waitForBlock(sh.CONFIG.PRIMARY_RPC, target, sh.CONFIG.MAX_WAIT_SECONDS);
      probeTier = await sh.rpcCall(sh.CONFIG.PRIMARY_RPC, "ethernova_getStateTier", [probe.address, slot]);
    }
  }
  if (probeTier.tier === "Active") {
    tracker.fail("Probe tier known", `expected non-Active probe for warming-fee test, got ${JSON.stringify(probeTier)}`);
    sh.appendReport(tracker.finalize());
    process.exit(1);
  }
  tracker.pass("Probe tier known", `tier=${probeTier.tier} age=${probeTier.ageBlocks}`);

  // Probe gas
  const probeCalldata = iface.encodeFunctionData("read", [0]);
  const probeGasHex = await sh.rpcCall(sh.CONFIG.PRIMARY_RPC, "eth_estimateGas", [{
    to: probe.address,
    data: probeCalldata,
  }]);
  const probeGas = parseInt(probeGasHex, 16);
  const probeAdaptivePct = await getAdaptivePct(sh.CONFIG.PRIMARY_RPC);
  tracker.pass(`Probe (${probeTier.tier}) read gas`, `${probeGas} gas`);

  // Compute surcharge
  const delta = probeGas - baselineGas;
  const rawSurcharge = sh.expectedSurcharge(probeTier.tier, 32, sh.CONFIG.WARMING_FEE_PER_BYTE);
  const baselineRawGas = invertAdaptiveGas(baselineGas, baselineAdaptivePct);
  const expected = applyAdaptiveGas(baselineRawGas + rawSurcharge, probeAdaptivePct) - baselineGas;
  if (delta === expected) {
    tracker.pass(`Surcharge formula match`, `actual=${delta} expected=${expected} raw=${rawSurcharge} adaptive=${baselineAdaptivePct}%/${probeAdaptivePct}%`);
  } else if (Math.abs(delta - expected) <= 5) {
    tracker.pass(`Surcharge formula match (+/-5)`, `actual=${delta} expected=${expected} raw=${rawSurcharge} adaptive=${baselineAdaptivePct}%/${probeAdaptivePct}% delta=${delta - expected}`);
  } else {
    tracker.fail(`Surcharge formula match`, `actual=${delta} expected=${expected} raw=${rawSurcharge} adaptive=${baselineAdaptivePct}%/${probeAdaptivePct}%`);
  }

  // Multi-node consensus on probe gas
  const probeConsensus = await sh.rpcAll("eth_estimateGas", [{
    to: probe.address,
    data: probeCalldata,
  }]);
  const probeAgree = sh.allAgree(probeConsensus);
  if (probeAgree.ok) {
    tracker.pass(`Probe gas - multi-node consensus`, probeAgree.reason);
  } else {
    tracker.fail(`Probe gas - multi-node consensus`, probeAgree.reason);
  }

  // Verify baseline tier still Active
  const baselineTier = await sh.rpcCall(sh.CONFIG.PRIMARY_RPC, "ethernova_getStateTier", [baselineAddr, "0x" + "00".repeat(32)]);
  if (baselineTier.tier === "Active") {
    tracker.pass("Baseline tier confirmed Active", `age=${baselineTier.ageBlocks}`);
  } else {
    tracker.fail("Baseline tier confirmed Active", `got ${baselineTier.tier}`);
  }

  const result = tracker.finalize();
  sh.appendReport(result);
  console.log(`\n  Suite 03: ${result.pass} pass, ${result.fail} fail, ${result.skip} skip (${result.elapsed_seconds}s)\n`);
  process.exit(result.fail > 0 ? 1 : 0);
}

main().catch((e) => { console.error(e); process.exit(2); });
