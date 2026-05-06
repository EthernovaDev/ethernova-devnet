// scripts/01-deploy.js — deploy LifecycleHarness + WitnessProbe + RegressionToken
// Run via: npx hardhat run scripts/01-deploy.js --network ethernova

const path = require("path");
require("dotenv").config({ path: path.resolve(__dirname, "..", ".env") });
const sh = require("./shared");
const hre = require("hardhat");

async function main() {
  sh.logHeader("Suite 01 - Deploy Test Harnesses");
  const tracker = new sh.ResultTracker("01-deploy");

  const [deployer] = await hre.ethers.getSigners();
  if (!deployer) {
    tracker.fail("Hardhat signer", "no signers — check PRIVATE_KEY in .env");
    sh.appendReport(tracker.finalize());
    process.exit(1);
  }
  const deployerAddr = await deployer.getAddress();
  const balBefore = await hre.ethers.provider.getBalance(deployerAddr);
  sh.logInfo(`deployer: ${deployerAddr}`);
  sh.logInfo(`balance: ${(Number(balBefore) / 1e18).toFixed(4)} NOVA`);

  const deployments = { deployer: deployerAddr };

  try {
    const LH = await hre.ethers.getContractFactory("LifecycleHarness");
    const lh = await LH.deploy();
    await lh.waitForDeployment();
    const addr = await lh.getAddress();
    const tx = lh.deploymentTransaction();
    deployments.lifecycleHarness = addr;
    deployments.lifecycleHarnessDeployedAt = tx.blockNumber || (await hre.ethers.provider.getBlockNumber());
    tracker.pass("Deploy LifecycleHarness", `addr=${addr}`);
  } catch (e) {
    tracker.fail("Deploy LifecycleHarness", e.message);
    sh.appendReport(tracker.finalize());
    process.exit(1);
  }

  try {
    const WP = await hre.ethers.getContractFactory("WitnessProbe");
    const wp = await WP.deploy();
    await wp.waitForDeployment();
    const addr = await wp.getAddress();
    deployments.witnessProbe = addr;
    tracker.pass("Deploy WitnessProbe", `addr=${addr}`);
  } catch (e) {
    tracker.fail("Deploy WitnessProbe", e.message);
  }

  try {
    const RT = await hre.ethers.getContractFactory("RegressionToken");
    const rt = await RT.deploy(hre.ethers.parseUnits("1000000", 18));
    await rt.waitForDeployment();
    const addr = await rt.getAddress();
    deployments.regressionToken = addr;
    tracker.pass("Deploy RegressionToken", `addr=${addr}`);
  } catch (e) {
    tracker.fail("Deploy RegressionToken", e.message);
  }

  try {
    const lh = await hre.ethers.getContractAt("LifecycleHarness", deployments.lifecycleHarness, deployer);
    const setTx = await lh.set(0, 42);
    await setTx.wait();
    const v = await lh.read(0);
    if (Number(v) === 42) {
      tracker.pass("Smoke test LifecycleHarness", "set/read roundtrip OK");
    } else {
      tracker.fail("Smoke test LifecycleHarness", `expected 42, got ${v}`);
    }
  } catch (e) {
    tracker.fail("Smoke test LifecycleHarness", e.message);
  }

  sh.saveDeployment("deployments", deployments);
  sh.logInfo(`Saved deployments to .deployments.json`);

  const result = tracker.finalize();
  sh.appendReport(result);
  console.log(`\n  Suite 01: ${result.pass} pass, ${result.fail} fail, ${result.skip} skip (${result.elapsed_seconds}s)\n`);
  process.exit(result.fail > 0 ? 1 : 0);
}

main().catch((e) => { console.error(e); process.exit(2); });
