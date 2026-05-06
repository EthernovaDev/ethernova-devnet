// scripts/run-all.js — runs all Phase 5 suites in sequence.

const { spawnSync } = require("child_process");
const path = require("path");
const fs = require("fs");
const sh = require("./shared");

const suites = [
  { name: "00-preflight",            cmd: "node", args: ["scripts/00-preflight.js"], required: true, timeoutSec: 60 },
  { name: "01-deploy",               cmd: "npx",  args: ["hardhat", "run", "scripts/01-deploy.js", "--network", "ethernova"], required: true, timeoutSec: 300 },
  { name: "02-tier-transitions",     cmd: "node", args: ["scripts/02-tier-transitions.js"], required: true, timeoutSec: sh.CONFIG.MAX_WAIT_SECONDS + 600 },
  { name: "03-warming-fee",          cmd: "node", args: ["scripts/03-warming-fee.js"], required: true, timeoutSec: 600 },
  { name: "04-witness-restore",      cmd: "node", args: ["scripts/04-witness-restore.js"], required: !sh.CONFIG.SKIP_WITNESS, timeoutSec: Math.max(1800, sh.CONFIG.MAX_WAIT_SECONDS + 600) },
  { name: "05-eip214-staticcall",    cmd: "npx",  args: ["hardhat", "run", "scripts/05-eip214-staticcall.js", "--network", "ethernova"], required: true, timeoutSec: 900 },
  { name: "06-active-immune",        cmd: "node", args: ["scripts/06-active-immune.js"], required: true, timeoutSec: sh.CONFIG.MAX_WAIT_SECONDS + 600 },
  { name: "07-multinode-consensus",  cmd: "node", args: ["scripts/07-multinode-consensus.js"], required: true, timeoutSec: 180 },
  { name: "08-brutal-stress",        cmd: "node", args: ["scripts/08-brutal-stress.js"], required: !sh.CONFIG.SKIP_BRUTAL, timeoutSec: sh.CONFIG.MAX_WAIT_SECONDS + 1200 },
  { name: "09-regression",           cmd: "npx",  args: ["hardhat", "run", "scripts/09-regression.js", "--network", "ethernova"], required: !sh.CONFIG.SKIP_REGRESSION, timeoutSec: 900 },
];

const root = path.resolve(__dirname, "..");

function parseList(value) {
  return new Set(String(value || "").split(",").map((x) => x.trim()).filter(Boolean));
}

function suiteId(name) {
  return name.split("-")[0];
}

function parseArgs(argv) {
  const opts = {
    start: process.env.START_TEST || "",
    skip: parseList(process.env.SKIP_TESTS || ""),
    continueOnFail: process.env.CONTINUE_ON_FAIL === "1" || process.env.CONTINUE_ON_FAIL === "true",
  };
  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i];
    if (arg === "--start" && argv[i + 1]) opts.start = argv[++i];
    else if (arg.startsWith("--start=")) opts.start = arg.slice("--start=".length);
    else if (arg === "--skip" && argv[i + 1]) opts.skip = parseList(argv[++i]);
    else if (arg.startsWith("--skip=")) opts.skip = parseList(arg.slice("--skip=".length));
    else if (arg === "--continue-on-fail") opts.continueOnFail = true;
  }
  return opts;
}

const runOptions = parseArgs(process.argv.slice(2));

function header() {
  const bar = "#".repeat(72);
  console.log("");
  console.log(sh.C.bold + sh.C.cyan + bar + sh.C.reset);
  console.log(sh.C.bold + sh.C.cyan + "  Ethernova Phase 5 - Brutal Test Suite" + sh.C.reset);
  console.log(sh.C.bold + sh.C.cyan + "  State Lifecycle Tiers - Real-Network Multi-Node Consensus" + sh.C.reset);
  console.log(sh.C.bold + sh.C.cyan + bar + sh.C.reset);
  console.log("");
  console.log(`  Primary RPC:     ${sh.CONFIG.PRIMARY_RPC}`);
  console.log(`  Chain ID:        ${sh.CONFIG.PRIMARY_CHAIN_ID}`);
  console.log(`  Consensus nodes: ${sh.CONFIG.NODES.map((n) => n.label).join(", ")}`);
  console.log(`  Thresholds:      active=${sh.CONFIG.ACTIVE_TIER_BLOCKS}, warm=${sh.CONFIG.WARM_TIER_BLOCKS}, cold=${sh.CONFIG.COLD_TIER_BLOCKS}`);
  console.log(`  Fee per byte:    ${sh.CONFIG.WARMING_FEE_PER_BYTE}`);
  if (runOptions.start) console.log(`  Start suite:     ${runOptions.start}`);
  if (runOptions.skip.size > 0) console.log(`  Skip suites:     ${Array.from(runOptions.skip).join(",")}`);
  console.log("");
}

function runSuite(suite) {
  console.log("");
  console.log(sh.C.dim + "-".repeat(72) + sh.C.reset);
  console.log(`  Running: ${sh.C.bold}${suite.name}${sh.C.reset}`);
  console.log(sh.C.dim + "-".repeat(72) + sh.C.reset);

  const start = Date.now();
  const result = spawnSync(suite.cmd, suite.args, {
    cwd: root,
    stdio: "inherit",
    shell: process.platform === "win32",
    timeout: suite.timeoutSec * 1000,
  });
  const elapsed = ((Date.now() - start) / 1000).toFixed(1);

  if (result.error) {
    const timeoutDetail = result.error.code === "ETIMEDOUT" ? ` after ${suite.timeoutSec}s timeout` : "";
    console.log(`  ${sh.C.red}ERROR: ${result.error.message}${timeoutDetail}${sh.C.reset}`);
    return { name: suite.name, status: "ERROR", elapsed, code: -1 };
  }
  if (result.status !== 0) {
    console.log(`  ${sh.C.red}EXIT ${result.status} after ${elapsed}s${sh.C.reset}`);
    return { name: suite.name, status: "FAIL", elapsed, code: result.status };
  }
  return { name: suite.name, status: "PASS", elapsed, code: 0 };
}

function printSummary(results) {
  const reportPath = path.resolve(root, sh.CONFIG.REPORT_PATH);
  let report = null;
  if (fs.existsSync(reportPath)) {
    try { report = JSON.parse(fs.readFileSync(reportPath, "utf8")); } catch (e) {}
  }

  console.log("");
  console.log(sh.C.bold + sh.C.cyan + "=".repeat(72) + sh.C.reset);
  console.log(sh.C.bold + sh.C.cyan + "  FINAL SUMMARY" + sh.C.reset);
  console.log(sh.C.bold + sh.C.cyan + "=".repeat(72) + sh.C.reset);
  console.log("");

  let totalPass = 0, totalFail = 0, totalSkip = 0;
  for (const r of results) {
    let suiteDetail = "";
    if (report) {
      const sr = report.suites.find((s) => s.suite === r.name);
      if (sr) {
        suiteDetail = ` (${sr.pass}P/${sr.fail}F/${sr.skip}S of ${sr.total})`;
        totalPass += sr.pass; totalFail += sr.fail; totalSkip += sr.skip;
      }
    }
    const statusColor = r.status === "PASS" ? sh.C.green : (r.status === "SKIP" ? sh.C.yellow : sh.C.red);
    const marker = r.status === "PASS" ? "[PASS]" : (r.status === "SKIP" ? "[SKIP]" : "[FAIL]");
    console.log(`  ${statusColor}${marker}${sh.C.reset} ${r.name.padEnd(28)} ${r.elapsed}s${suiteDetail}`);
  }

  console.log("");
  console.log(`  Suite-level: ${results.filter((r) => r.status === "PASS").length} pass, ${results.filter((r) => r.status === "FAIL" || r.status === "ERROR").length} fail, ${results.filter((r) => r.status === "SKIP").length} skip`);
  if (totalPass + totalFail + totalSkip > 0) {
    console.log(`  Check-level: ${sh.C.green}${totalPass} pass${sh.C.reset}, ${sh.C.red}${totalFail} fail${sh.C.reset}, ${sh.C.yellow}${totalSkip} skip${sh.C.reset}`);
  }
  console.log("");

  const failedSuite = results.filter((r) => r.status === "FAIL" || r.status === "ERROR").length;
  if (failedSuite === 0 && totalFail === 0) {
    console.log(sh.C.bold + sh.C.green + "  [PASS] ALL SUITES PASSED - Phase 5 ready to promote." + sh.C.reset);
  } else {
    console.log(sh.C.bold + sh.C.red + "  [FAIL] SOME SUITES FAILED - see above for details." + sh.C.reset);
    console.log(sh.C.dim + "  Full report: " + reportPath + sh.C.reset);
  }
  console.log("");
}

async function main() {
  header();

  if (!fs.existsSync(path.join(root, "artifacts/contracts/LifecycleHarness.sol/LifecycleHarness.json"))) {
    console.log(sh.C.dim + "  Compiling contracts (one-time)..." + sh.C.reset);
    const compile = spawnSync("npx", ["hardhat", "compile"], { cwd: root, stdio: "inherit", shell: process.platform === "win32" });
    if (compile.status !== 0) {
      console.log(sh.C.red + "  Compile failed - abort" + sh.C.reset);
      process.exit(2);
    }
  }

  const results = [];
  let runnable = suites;
  if (runOptions.start) {
    runnable = runnable.filter((suite) => suiteId(suite.name) >= runOptions.start);
  }

  for (const suite of runnable) {
    if (runOptions.skip.has(suiteId(suite.name)) || runOptions.skip.has(suite.name)) {
      console.log(`  ${sh.C.yellow}[SKIP] ${suite.name} (--skip)${sh.C.reset}`);
      results.push({ name: suite.name, status: "SKIP", elapsed: "0", code: 0 });
      continue;
    }
    if (!suite.required) {
      console.log(`  ${sh.C.yellow}[SKIP] ${suite.name} (skip flag set)${sh.C.reset}`);
      results.push({ name: suite.name, status: "SKIP", elapsed: "0", code: 0 });
      continue;
    }
    const r = runSuite(suite);
    results.push(r);
    if (!runOptions.continueOnFail && r.status !== "PASS" && (suite.name === "00-preflight" || suite.name === "01-deploy")) {
      console.log(sh.C.red + `  Critical suite ${suite.name} failed - aborting orchestrator` + sh.C.reset);
      break;
    }
  }

  printSummary(results);
  const failedCount = results.filter((r) => r.status === "FAIL" || r.status === "ERROR").length;
  process.exit(failedCount > 0 ? 1 : 0);
}

main().catch((e) => { console.error(e); process.exit(2); });
