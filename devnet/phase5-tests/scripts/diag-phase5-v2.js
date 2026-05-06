// scripts/diag-phase5-v2.js — comprehensive diagnostic to pinpoint
// where Phase 5 surcharge fails.
//
// Splits the question into concrete checks:
//   A. Call Cache hypothesis: eth_estimateGas may return cached value
//      bypassing EVM. Check by toggling cache + measuring gas before/after.
//   B. Archive marker: tier=Archived but isArchived=false means sweep
//      didn't stamp marker. Verify by probing rawdb directly.
//   C. Direct EVM exec: bypass call cache by calling via tx (not estimateGas)
//      and observe gas consumed in receipt.
//   D. Different addresses: check if applyLifecycleSurcharge is called
//      AT ALL on any contract by exercising a known-non-Active address.

const sh = require("./shared");
const { ethers } = require("ethers");

async function main() {
  console.log("\n================== Phase 5 Diagnostic v2 ==================\n");

  const provider = sh.makeProvider(sh.CONFIG.PRIMARY_RPC);
  const wallet = sh.makeSigner(provider);

  // ============================================================
  // A. Verify diagnostic instrumentation IS in the binary
  // ============================================================
  console.log("--- A. Diagnostic instrumentation present? ---");
  let counters;
  try {
    counters = await sh.rpcCall(sh.CONFIG.PRIMARY_RPC, "ethernova_phase5DebugCounters", []);
    if (counters && typeof counters === "object") {
      console.log("  OK: ethernova_phase5DebugCounters responds.");
    } else {
      console.log("  FAIL: response was: " + JSON.stringify(counters));
      process.exit(1);
    }
  } catch (e) {
    console.log("  FAIL: " + e.message);
    process.exit(1);
  }

  // ============================================================
  // B. Probe state probe contract from suite 02
  // ============================================================
  console.log("\n--- B. Probe contract state ---");
  let probe;
  try {
    probe = sh.loadDeployment("tier-probe");
  } catch (e) {
    console.log("  FAIL: no tier-probe.json — re-run 02-tier-transitions");
    process.exit(1);
  }
  const slot = "0x" + "00".repeat(32);
  const tier = await sh.rpcCall(sh.CONFIG.PRIMARY_RPC, "ethernova_getStateTier", [probe.address, slot]);
  console.log(`  Address:        ${probe.address}`);
  console.log(`  tier:           ${tier.tier}`);
  console.log(`  isArchived:     ${tier.isArchived}`);
  console.log(`  lastTouched:    ${tier.lastTouched}`);
  console.log(`  currentBlock:   ${tier.currentBlock}`);
  console.log(`  ageBlocks:      ${tier.ageBlocks}`);

  if (tier.tier === "Archived" && tier.isArchived === false) {
    console.log("  ANOMALY: tier classified Archived by age, but archive marker NOT stamped.");
    console.log("           This means sweep didn't run for this candidate block, OR");
    console.log("           marker was written but later deleted, OR");
    console.log("           rawdb.WriteArchiveMarker is buggy.");
  }

  // ============================================================
  // C. Check if eth_call (different RPC method) hits surcharge
  //    eth_call is mostly identical to eth_estimateGas but may take
  //    a different code path through the call cache layer.
  // ============================================================
  console.log("\n--- C. eth_call (potentially un-cached) ---");
  const artifact = sh.loadArtifact("LifecycleHarness");
  const iface = new ethers.Interface(artifact.abi);
  const calldataRead = iface.encodeFunctionData("read", [0]);

  const beforeC = await sh.rpcCall(sh.CONFIG.PRIMARY_RPC, "ethernova_phase5DebugCounters", []);

  // Try eth_call (read-only)
  for (let i = 0; i < 3; i++) {
    try {
      await sh.rpcCall(sh.CONFIG.PRIMARY_RPC, "eth_call", [{
        to: probe.address,
        data: calldataRead,
      }, "latest"]);
    } catch (e) {
      console.log(`  eth_call attempt ${i + 1}: ERROR ${e.message}`);
    }
  }

  const afterC = await sh.rpcCall(sh.CONFIG.PRIMARY_RPC, "ethernova_phase5DebugCounters", []);
  const dC_called = Number(afterC.called) - Number(beforeC.called);
  console.log(`  applyLifecycleSurcharge called: ${dC_called} times during 3x eth_call`);
  if (dC_called === 0) {
    console.log("  CONFIRMED: eth_call also bypasses surcharge. Both estimateGas AND eth_call are cached/skipped.");
  } else {
    console.log("  eth_call DOES go through surcharge path. Bug is specific to eth_estimateGas.");
    printDelta(beforeC, afterC);
  }

  // ============================================================
  // D. Check if a real tx (not simulation) hits the surcharge.
  //    Sending a tx forces actual block execution, NOT cached.
  // ============================================================
  console.log("\n--- D. Real transaction (forces actual EVM execution) ---");
  console.log("  Sending tx to probe.read(0) — observes actual gas consumed in receipt.");

  const beforeD = await sh.rpcCall(sh.CONFIG.PRIMARY_RPC, "ethernova_phase5DebugCounters", []);

  try {
    // Build a tx that calls read() — read returns nothing useful but
    // the EVM will still execute SLOAD, paying gas as part of the tx.
    const tx = await wallet.sendTransaction({
      to: probe.address,
      data: calldataRead,
      gasLimit: 100000n,
    });
    const receipt = await tx.wait();
    console.log(`  tx hash:    ${tx.hash}`);
    console.log(`  block:      ${receipt.blockNumber}`);
    console.log(`  gasUsed:    ${receipt.gasUsed}`);
    console.log(`  status:     ${receipt.status}`);
  } catch (e) {
    console.log(`  ERROR: ${e.message}`);
  }

  const afterD = await sh.rpcCall(sh.CONFIG.PRIMARY_RPC, "ethernova_phase5DebugCounters", []);
  const dD_called = Number(afterD.called) - Number(beforeD.called);
  console.log(`  applyLifecycleSurcharge called: ${dD_called} times during 1 real tx`);
  if (dD_called === 0) {
    console.log("  CRITICAL: Even a REAL transaction doesn't hit gasSLoadEIP2929!");
    console.log("            Either:");
    console.log("            (a) The contract's read() function got optimized away by Solidity");
    console.log("                so it never executes SLOAD (verify: gasUsed should be ~24k).");
    console.log("            (b) The mining path doesn't go through gasSLoadEIP2929 either,");
    console.log("                meaning EthernovaDev forked/replaced the standard SLOAD gas");
    console.log("                handler with their own. Likely candidate: 'parallel transaction");
    console.log("                execution' (Phase 23) replaced gas calc functions.");
    console.log("            (c) Adaptive gas v2 (Phase 24) replaced the gas table entirely.");
  } else {
    console.log("  GOOD: Real tx hits surcharge path. Bug is specific to gas estimation paths.");
    printDelta(beforeD, afterD);
  }

  // ============================================================
  // E. Check contract bytecode actually has SLOAD
  // ============================================================
  console.log("\n--- E. Contract bytecode inspection ---");
  const code = await sh.rpcCall(sh.CONFIG.PRIMARY_RPC, "eth_getCode", [probe.address, "latest"]);
  if (!code || code === "0x") {
    console.log("  FAIL: contract has no code at this address.");
  } else {
    const codeBytes = code.length / 2 - 1; // hex chars / 2 - "0x"
    console.log(`  Code length: ${codeBytes} bytes`);
    // Count SLOAD opcode (0x54) occurrences
    const sloadHex = "54";
    let count = 0;
    for (let i = 2; i < code.length - 1; i += 2) {
      if (code.substr(i, 2).toLowerCase() === sloadHex) count++;
    }
    console.log(`  SLOAD (0x54) byte occurrences: ${count}`);
    if (count === 0) {
      console.log("  [WARN] No SLOAD opcode in bytecode — read() got compiled to immediate return?");
      console.log("         Check Solidity compiler optimizer settings.");
    }
  }

  // ============================================================
  // F. Cross-check call cache RPC (if Phase 4 endpoint exists)
  // ============================================================
  console.log("\n--- F. Call Cache stats (Phase 4) ---");
  try {
    const cc = await sh.rpcCall(sh.CONFIG.PRIMARY_RPC, "ethernova_callCache", []);
    console.log(`  ethernova_callCache response:`);
    for (const [k, v] of Object.entries(cc || {})) {
      console.log(`    ${k}: ${JSON.stringify(v)}`);
    }
  } catch (e) {
    console.log(`  ethernova_callCache not available: ${e.message}`);
  }

  // ============================================================
  // G. Other custom RPC related to Phase 5 / gas
  // ============================================================
  console.log("\n--- G. Adaptive Gas stats (Phase 2/24) ---");
  try {
    const ag = await sh.rpcCall(sh.CONFIG.PRIMARY_RPC, "ethernova_adaptiveGas", []);
    console.log(`  ethernova_adaptiveGas response (truncated):`);
    const json = JSON.stringify(ag, null, 2);
    console.log(json.length > 1500 ? json.substring(0, 1500) + "\n    ... (truncated)" : json);
  } catch (e) {
    console.log(`  ethernova_adaptiveGas not available: ${e.message}`);
  }

  // ============================================================
  // FINAL DIAGNOSIS
  // ============================================================
  console.log("\n================== FINAL DIAGNOSIS ==================");
  const finalCounters = await sh.rpcCall(sh.CONFIG.PRIMARY_RPC, "ethernova_phase5DebugCounters", []);
  if (Number(finalCounters.called) === 0) {
    console.log("CRITICAL: applyLifecycleSurcharge was NOT CALLED at all.");
    console.log("         Even after eth_call AND a real on-chain tx.");
    console.log("");
    console.log("Most likely cause: Ethernova has a CUSTOM SLOAD gas handler that");
    console.log("                   bypasses the standard gasSLoadEIP2929 we patched.");
    console.log("");
    console.log("Next step: please paste the output of:");
    console.log("           grep -rn 'gasSLoad\\|gasSloadEIP\\|SLOAD' core/vm/jump_table*.go");
    console.log("           grep -n 'SLOAD' core/vm/instructions.go core/vm/eips.go");
    console.log("           ls core/vm/*.go | xargs grep -l 'parallel\\|Parallel'");
    console.log("Then we can find exactly where the actual SLOAD gas calculation lives.");
  } else if (Number(finalCounters.tier_archived) === 0 && Number(finalCounters.tier_active) > 0) {
    console.log("BUG: surcharge invoked but TierOf returned Active despite getStateTier saying Archived.");
    console.log("     -> Engine reads different LevelDB or different prefix than RPC path.");
  } else if (Number(finalCounters.surcharge_applied) > 0) {
    console.log("Surcharge IS being applied somewhere; estimateGas just doesn't trigger it.");
    console.log("Bug is in the cached/optimized gas path used by simulation.");
  }
  console.log("");
}

function printDelta(before, after) {
  console.log("    Delta:");
  for (const k of Object.keys(after)) {
    const d = Number(after[k]) - Number(before[k] || 0);
    if (d !== 0) console.log(`      ${k.padEnd(22)} +${d}`);
  }
}

main().catch((e) => { console.error(e); process.exit(2); });
