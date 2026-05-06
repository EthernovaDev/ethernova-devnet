// scripts/09-regression.js — confirm Phase 5 doesn't break existing EVM/ERC20.
// Run via: npx hardhat run scripts/09-regression.js --network ethernova

const path = require("path");
require("dotenv").config({ path: path.resolve(__dirname, "..", ".env") });
const sh = require("./shared");
const hre = require("hardhat");

async function waitForConvergedHeads(maxAttempts = 30) {
  for (let i = 0; i < maxAttempts; i++) {
    const heads = {};
    for (const node of sh.CONFIG.NODES) {
      try {
        heads[node.label] = await sh.getBlockNumber(node.url);
      } catch (_) {
        heads[node.label] = null;
      }
    }
    const valid = Object.values(heads).filter((h) => h !== null);
    if (valid.length >= 2 && Math.max(...valid) - Math.min(...valid) <= 1) {
      return heads;
    }
    await new Promise((resolve) => setTimeout(resolve, 3000));
  }
  return null;
}

async function main() {
  sh.logHeader("Suite 09 - Regression Test (existing EVM/ERC20)");

  if (sh.CONFIG.SKIP_REGRESSION) {
    console.log(`  ${sh.C.yellow}[SKIP] (SKIP_REGRESSION=1)${sh.C.reset}`);
    sh.appendReport({ suite: "09-regression", pass: 0, fail: 0, skip: 1, total: 1, elapsed_seconds: 0, checks: [{ name: "all", status: "SKIP", detail: "SKIP_REGRESSION=1" }] });
    process.exit(0);
  }

  const tracker = new sh.ResultTracker("09-regression");

  let dep;
  try {
    dep = sh.loadDeployment("deployments");
  } catch (e) {
    tracker.fail("Load deployments", e.message);
    sh.appendReport(tracker.finalize());
    process.exit(1);
  }

  if (!dep.regressionToken) {
    tracker.fail("RegressionToken address", "not deployed (suite 01 incomplete)");
    sh.appendReport(tracker.finalize());
    process.exit(1);
  }

  const [deployer] = await hre.ethers.getSigners();
  const token = await hre.ethers.getContractAt("RegressionToken", dep.regressionToken, deployer);

  // Total supply
  try {
    const supply = await token.totalSupply();
    const expected = hre.ethers.parseUnits("1000000", 18);
    if (supply === expected) {
      tracker.pass("Total supply correct", `${supply} == 1M tokens`);
    } else {
      tracker.fail("Total supply correct", `${supply} != ${expected}`);
    }
  } catch (e) {
    tracker.fail("Total supply", e.message);
  }

  // Balance
  try {
    const bal = await token.balanceOf(deployer.address);
    if (bal > 0n) {
      tracker.pass("Deployer balance > 0", `${bal} units`);
    } else {
      tracker.fail("Deployer balance > 0", `bal=${bal}`);
    }
  } catch (e) {
    tracker.fail("Deployer balance", e.message);
  }

  // Tier check
  const tokenTier = await sh.rpcCall(sh.CONFIG.PRIMARY_RPC, "ethernova_getStateTier", [dep.regressionToken, "0x" + "00".repeat(32)]);
  if (tokenTier.tier === "Active") {
    tracker.pass("RegressionToken is Active", `lastTouched=${tokenTier.lastTouched}`);
  } else {
    tracker.skip("RegressionToken is Active", `tier=${tokenTier.tier} - non-fatal (token aged during suites)`);
  }

  // transfer
  const recipient = "0x" + "ab".repeat(20);
  try {
    const before = await token.balanceOf(recipient);
    const tx = await token.transfer(recipient, 100n);
    const receipt = await tx.wait();
    const after = await token.balanceOf(recipient);
    if (after === before + 100n && receipt.status === 1) {
      tracker.pass("transfer() succeeds", `gasUsed=${receipt.gasUsed}`);
    } else {
      tracker.fail("transfer() succeeds", `bal before=${before} after=${after}`);
    }
  } catch (e) {
    tracker.fail("transfer()", e.message);
  }

  // approve + transferFrom
  try {
    const spender = deployer.address;
    const approveTx = await token.approve(spender, 50n);
    await approveTx.wait();
    const allowance = await token.allowance(deployer.address, spender);
    if (allowance === 50n) {
      tracker.pass("approve() succeeds", `allowance=${allowance}`);
    } else {
      tracker.fail("approve() succeeds", `allowance=${allowance}`);
    }
    const recipient2 = "0x" + "cd".repeat(20);
    const tfTx = await token.transferFrom(deployer.address, recipient2, 50n);
    const tfReceipt = await tfTx.wait();
    if (tfReceipt.status === 1) {
      tracker.pass("transferFrom() succeeds", `gasUsed=${tfReceipt.gasUsed}`);
    } else {
      tracker.fail("transferFrom() succeeds", `status=${tfReceipt.status}`);
    }
  } catch (e) {
    tracker.fail("approve + transferFrom", e.message);
  }

  // Gas profile
  try {
    const recipient3 = "0x" + "ef".repeat(20);
    const tx = await token.transfer(recipient3, 1n);
    const receipt = await tx.wait();
    const gas = Number(receipt.gasUsed);
    if (gas >= 25000 && gas <= 80000) {
      tracker.pass("transfer() gas in normal range", `${gas} gas`);
    } else {
      tracker.fail("transfer() gas in normal range", `${gas} gas - outside 25k-80k`);
    }
  } catch (e) {
    tracker.fail("Gas profile", e.message);
  }

  // Multi-node consensus on token state
  await waitForConvergedHeads();
  const proofResults = await sh.rpcAll("eth_getProof", [dep.regressionToken, [], "latest"]);
  const tokenRoots = {};
  for (const [l, r] of Object.entries(proofResults)) {
    if (r && !r.__error && r.storageHash) tokenRoots[l] = r.storageHash;
    else tokenRoots[l] = r?.__error || "no-data";
  }
  const tokenAgree = sh.allAgree(tokenRoots);
  if (tokenAgree.ok) {
    tracker.pass("Token storage root - multi-node consensus", `root=${Object.values(tokenRoots)[0]}`);
  } else {
    tracker.fail("Token storage root - multi-node consensus", tokenAgree.reason);
  }

  const result = tracker.finalize();
  sh.appendReport(result);
  console.log(`\n  Suite 09: ${result.pass} pass, ${result.fail} fail, ${result.skip} skip (${result.elapsed_seconds}s)\n`);
  process.exit(result.fail > 0 ? 1 : 0);
}

main().catch((e) => { console.error(e); process.exit(2); });
