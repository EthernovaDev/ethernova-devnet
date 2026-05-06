// scripts/print-report.js — pretty-print report.json

const fs = require("fs");
const path = require("path");
const sh = require("./shared");

const reportPath = path.resolve(__dirname, "..", sh.CONFIG.REPORT_PATH);
if (!fs.existsSync(reportPath)) {
  console.log("No report.json found. Run a suite first.");
  process.exit(1);
}

const report = JSON.parse(fs.readFileSync(reportPath, "utf8"));
console.log(`Started:      ${report.started_at}`);
console.log(`Last updated: ${report.last_updated_at || "n/a"}`);
console.log("");

let totalPass = 0, totalFail = 0, totalSkip = 0;
for (const suite of report.suites) {
  const status = suite.fail > 0 ? sh.C.red + "FAIL" : (suite.pass === 0 && suite.skip > 0 ? sh.C.yellow + "SKIP" : sh.C.green + "PASS");
  console.log(`${status}${sh.C.reset}  ${suite.suite.padEnd(30)}  ${suite.pass}P/${suite.fail}F/${suite.skip}S  (${suite.elapsed_seconds}s)`);
  totalPass += suite.pass; totalFail += suite.fail; totalSkip += suite.skip;
  if (suite.fail > 0) {
    for (const c of suite.checks.filter((c) => c.status === "FAIL")) {
      console.log(`       ${sh.C.red}[FAIL]${sh.C.reset} ${c.name}${c.detail ? sh.C.dim + " - " + c.detail + sh.C.reset : ""}`);
    }
  }
}
console.log("");
console.log(`Total: ${sh.C.green}${totalPass} pass${sh.C.reset}, ${sh.C.red}${totalFail} fail${sh.C.reset}, ${sh.C.yellow}${totalSkip} skip${sh.C.reset}`);
process.exit(totalFail > 0 ? 1 : 0);
