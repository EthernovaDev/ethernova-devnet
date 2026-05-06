// scripts/diag-phase5.js — call after running suite 03 / making SLOAD calls
// to see which surcharge branch is hit. Pinpoints why surcharge=0.
//
// Usage:
//   node scripts/diag-phase5.js
//
// Expected output for HEALTHY node (Archived probe):
//   called > 0, global_db_hit > 0, tier_archived > 0, surcharge_applied > 0
//
// Diagnostic table for unhealthy outputs:
//   called == 0           -> precompile not registered or instrumentation
//                            not in binary; rebuild geth, restart
//   block_number_nil > 0  -> simulation passes nil BlockNumber to EVM —
//                            uncommon but possible in some override paths
//   pre_fork > 0          -> StateLifecycleForkBlock is set above current
//                            block; check params/ethernova/forks.go
//   global_db_miss > 0
//   AND type_assert_fail > 0   -> StateDB is wrapped during simulation;
//                                 vm.SetLifecycleDB was not called at
//                                 startup (or backend.go change didn't
//                                 take effect — check rebuild/restart)
//   tier_active > 0       -> engine reads index but sees account as
//                            Active; the LevelDB surcharge engine reads
//                            different prefix than what hook writes.
//                            Check rawdb prefix bytes are consistent.
//   tier_archived > 0
//   AND surcharge_zero > 0    -> ComputeWarmingFee returns 0 even for
//                                non-Active tier; bug in formula
//                                (gap=0 due to switch fallthrough?)
//   surcharge_applied > 0     -> path is HEALTHY at the function level —
//                                if estimateGas still returns same gas,
//                                bug is upstream (e.g. estimateGas
//                                running in a path that doesn't invoke
//                                gasSLoadEIP2929)

const sh = require("./shared");
const { ethers } = require("ethers");

async function main() {
  console.log("\n====== Phase 5 Diagnostic ======\n");

  const provider = sh.makeProvider(sh.CONFIG.PRIMARY_RPC);
  const wallet = sh.makeSigner(provider);

  // 1. Show counters BEFORE
  const before = await sh.rpcCall(sh.CONFIG.PRIMARY_RPC, "ethernova_phase5DebugCounters", []);
  if (!before || typeof before !== "object") {
    console.log("ethernova_phase5DebugCounters returned:", before);
    console.log("\n[FATAL] Diagnostic RPC not registered. The diagnostic build of");
    console.log("        operations_acl.go + api_ethernova.go is NOT in the running binary.");
    console.log("        Steps to fix:");
    console.log("          1. Apply phase5-fix-r3-diagnostic.zip");
    console.log("          2. Append the method from api_ethernova_diag_snippet.go");
    console.log("          3. make geth");
    console.log("          4. STOP geth, START again with new binary");
    console.log("          5. Re-run this script.\n");
    process.exit(1);
  }
  console.log("Counters BEFORE test calls:");
  printCounters(before);

  // 2. Load probe and confirm it's Archived per RPC
  let probe;
  try {
    probe = sh.loadDeployment("tier-probe");
  } catch (e) {
    console.log("\n[FAIL] No tier-probe.json. Run 02-tier-transitions first.\n");
    process.exit(1);
  }
  const slot = "0x" + "00".repeat(32);
  const tier = await sh.rpcCall(sh.CONFIG.PRIMARY_RPC, "ethernova_getStateTier", [probe.address, slot]);
  console.log(`\nProbe ${probe.address}`);
  console.log(`  tier=${tier.tier}  isArchived=${tier.isArchived}  lastTouched=${tier.lastTouched}`);
  if (tier.tier === "Active") {
    console.log("\n[NOTE] Probe is currently Active (got restored or never archived).");
    console.log("       This script needs probe in Warm/Cold/Archived to exercise surcharge path.");
    console.log("       Re-run suite 02-tier-transitions to re-archive a probe.\n");
    // continue anyway — we'll still see called>0 if surcharge code is invoked.
  }

  // 3. Build the SLOAD calldata
  const artifact = sh.loadArtifact("LifecycleHarness");
  const iface = new ethers.Interface(artifact.abi);
  const calldata = iface.encodeFunctionData("read", [0]);

  // 4. Call eth_estimateGas a few times to exercise the path
  console.log("\nCalling eth_estimateGas on probe x 5...");
  for (let i = 0; i < 5; i++) {
    try {
      const gas = await sh.rpcCall(sh.CONFIG.PRIMARY_RPC, "eth_estimateGas", [{
        to: probe.address,
        data: calldata,
      }]);
      console.log(`  attempt ${i + 1}: gas=${parseInt(gas, 16)}`);
    } catch (e) {
      console.log(`  attempt ${i + 1}: ERROR ${e.message}`);
    }
  }

  // 5. Counters AFTER
  console.log("");
  const after = await sh.rpcCall(sh.CONFIG.PRIMARY_RPC, "ethernova_phase5DebugCounters", []);
  console.log("Counters AFTER test calls:");
  printCounters(after);

  // 6. Diff
  console.log("\nDelta (after - before):");
  for (const k of Object.keys(after)) {
    const d = Number(after[k]) - Number(before[k] || 0);
    if (d !== 0) console.log(`  ${k.padEnd(22)} +${d}`);
  }

  // 7. Diagnose
  console.log("\n----- DIAGNOSIS -----");
  const calledDelta = Number(after.called) - Number(before.called || 0);
  if (calledDelta === 0) {
    console.log("FATAL: applyLifecycleSurcharge was NEVER called during 5 estimateGas attempts.");
    console.log("       This means either:");
    console.log("         (a) The instrumented binary is not running (forgot to restart geth)");
    console.log("         (b) eth_estimateGas does not go through gasSLoadEIP2929 (different gas path)");
    console.log("         (c) The probe contract has no SLOAD opcode in read(0) (impossible if compiled normally)");
  } else if (Number(after.global_db_miss) > Number(before.global_db_miss || 0)) {
    console.log("BUG: global_db_miss > 0. vm.SetLifecycleDB was not called at startup.");
    console.log("     -> backend.go edit was not applied or geth was not restarted.");
    if (Number(after.type_assert_fail) > Number(before.type_assert_fail || 0)) {
      console.log("     -> Fallback type assertion ALSO failed: StateDB during estimateGas is");
      console.log("        wrapped, so old fallback path is broken. The global registration is");
      console.log("        the only working route. Verify backend.go change is present and you");
      console.log("        actually ran a fresh `make geth` AND restarted the node process.");
    }
  } else if (Number(after.tier_active) > Number(before.tier_active || 0) && tier.tier !== "Active") {
    console.log("BUG: ethernova_getStateTier says probe is " + tier.tier + " but");
    console.log("     applyLifecycleSurcharge sees it as Active.");
    console.log("     -> Different LevelDB or different schema prefix between RPC path and EVM path.");
    console.log("     -> Check that core/state/state_lifecycle.go and the surcharge function read");
    console.log("        from the SAME database handle and the SAME prefix bytes.");
  } else if (Number(after.surcharge_zero) > Number(before.surcharge_zero || 0)) {
    console.log("BUG: surcharge formula returned 0 for a non-Active tier.");
    console.log("     -> ComputeWarmingFee bug: tier_gap probably 0 due to switch fallthrough.");
  } else if (Number(after.surcharge_applied) > Number(before.surcharge_applied || 0)) {
    console.log("HEALTHY: surcharge_applied increased. Last applied: " + after.last_value_added + " gas at block " + after.last_block);
    console.log("         If suite 03 still shows actual=0, the bug is in the TEST not the node.");
    console.log("         Likely cause: eth_estimateGas is run BEFORE the SLOAD path because the");
    console.log("         contract is being called via a path that bypasses dynamic gas (e.g. the");
    console.log("         contract was deployed but storage was never set, so SLOAD returns 0 and");
    console.log("         gas optimizer skips the read).");
  } else {
    console.log("UNCLEAR: counters did not move expectedly. Paste this output to diagnose further.");
  }
  console.log("");
}

function printCounters(c) {
  const keys = [
    "called", "block_number_nil", "pre_fork",
    "global_db_hit", "global_db_miss", "type_assert_fail", "database_nil", "disk_db_nil",
    "tier_active", "tier_warm", "tier_cold", "tier_archived", "tier_other",
    "surcharge_zero", "surcharge_applied", "surcharge_overflow",
    "last_value_added", "last_block",
  ];
  for (const k of keys) {
    console.log(`  ${k.padEnd(22)} = ${c[k] !== undefined ? c[k] : "(missing)"}`);
  }
}

main().catch((e) => { console.error(e); process.exit(2); });
