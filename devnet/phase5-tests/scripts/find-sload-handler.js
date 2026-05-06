// scripts/find-sload-handler.js — locates the actual SLOAD gas handler
// in your Ethernova source tree by scanning the Go source files.
//
// Phase 5 surcharge wasn't being applied because gasSLoadEIP2929 (the
// standard handler) is not on the active code path. This script finds
// the REAL handler by scanning your repo for:
//   1. All function definitions that look like SLOAD gas handlers
//   2. All places where SLOAD opcode (0x54) is wired in jump tables
//   3. Adaptive gas v2 trace hooks
//   4. Custom EVM execution paths
//
// USAGE: run this from the p5test directory, AFTER setting REPO_ROOT
//        in .env to the absolute path of your ethernova-devnet repo.
//
//   .env: REPO_ROOT=D:\Ethernova\devnet\ethernova-devnet
//
// node scripts/find-sload-handler.js

const fs = require("fs");
const path = require("path");
const sh = require("./shared");

const REPO_ROOT = process.env.REPO_ROOT || "";
if (!REPO_ROOT || !fs.existsSync(REPO_ROOT)) {
  console.log("ERROR: REPO_ROOT not set or doesn't exist.");
  console.log("       Add to .env: REPO_ROOT=D:\\Ethernova\\devnet\\ethernova-devnet");
  process.exit(1);
}

console.log(`\n====== Phase 5 SLOAD Handler Locator ======`);
console.log(`Scanning: ${REPO_ROOT}`);
console.log(``);

// ---- helpers ----
function* walk(dir, depth = 0) {
  if (depth > 5) return;
  let entries;
  try {
    entries = fs.readdirSync(dir, { withFileTypes: true });
  } catch (e) {
    return;
  }
  for (const e of entries) {
    if (e.name === "node_modules" || e.name === ".git" || e.name === "vendor") continue;
    const p = path.join(dir, e.name);
    if (e.isDirectory()) yield* walk(p, depth + 1);
    else if (e.name.endsWith(".go")) yield p;
  }
}

function scan(label, dir, regex, maxResults = 100) {
  console.log(`\n--- ${label} ---`);
  let found = 0;
  for (const file of walk(dir)) {
    let lines;
    try {
      lines = fs.readFileSync(file, "utf8").split("\n");
    } catch (e) {
      continue;
    }
    for (let i = 0; i < lines.length; i++) {
      if (regex.test(lines[i])) {
        const rel = path.relative(REPO_ROOT, file);
        const trimmed = lines[i].trim().substring(0, 200);
        console.log(`  ${rel}:${i + 1}  ${trimmed}`);
        found++;
        if (found >= maxResults) {
          console.log(`  (... ${maxResults}+ matches, truncated)`);
          return;
        }
      }
    }
  }
  if (found === 0) console.log(`  (no matches)`);
}

// ---- 1. SLOAD gas handler functions ----
scan(
  "1. SLOAD gas handler function definitions",
  path.join(REPO_ROOT, "core", "vm"),
  /^func\s+(gas)?[Ss][Ll]oad/
);

// ---- 2. SLOAD jump table entries ----
scan(
  "2. SLOAD wired to jump tables",
  path.join(REPO_ROOT, "core", "vm"),
  /SLOAD\s*[=:]|opSload\s*\(/
);

// ---- 3. Adaptive gas v2 trace ----
scan(
  "3. Adaptive Gas v2 trace-based hooks",
  path.join(REPO_ROOT, "core"),
  /applyAdaptiveGas|adaptiveGasV2|TraceBased|tracebased|patternScore|gasMultiplier|adjustExecGas/
);

// ---- 4. Parallel exec custom EVM ----
scan(
  "4. Parallel execution / custom EVM paths",
  path.join(REPO_ROOT, "core"),
  /parallelExec|ParallelExec|parallelStateProcessor|FastEVM|fastEVM|FastInterpreter|fastInterpret/
);

// ---- 5. Interpreter Run / Step ----
scan(
  "5. EVM Interpreter Run/Step entry points",
  path.join(REPO_ROOT, "core", "vm"),
  /^func\s+\(in\s+\*EVMInterpreter\)\s+Run\b|^func\s+\(\w+\s+\*?Interpreter\)\s+Run\b/
);

// ---- 6. operation table SLOAD costs ----
scan(
  "6. operation table dynamic gas wiring",
  path.join(REPO_ROOT, "core", "vm"),
  /dynamicGas\s*:\s*\w*[Ss]load|SLOAD.*dynamicGas|dynamicGas.*SLOAD/
);

// ---- 7. Archive marker writer (for the second bug) ----
scan(
  "7. WriteArchiveMarker callers (Phase 5 second bug)",
  path.join(REPO_ROOT),
  /WriteArchiveMarker|stampArchive|MarkArchived|markAsArchived/
);

// ---- 8. Read operations_acl.go to confirm our patch is present ----
console.log(`\n--- 8. Verify operations_acl.go patch is present ---`);
const aclPath = path.join(REPO_ROOT, "core", "vm", "operations_acl.go");
if (fs.existsSync(aclPath)) {
  const content = fs.readFileSync(aclPath, "utf8");
  const hasFunc = content.includes("applyLifecycleSurcharge");
  const hasCall = content.match(/applyLifecycleSurcharge\s*\(/g);
  const hasGasSload = content.match(/gasSLoadEIP2929/g);
  console.log(`  applyLifecycleSurcharge function:  ${hasFunc ? "PRESENT" : "MISSING"}`);
  console.log(`  applyLifecycleSurcharge calls:     ${hasCall ? hasCall.length : 0}`);
  console.log(`  gasSLoadEIP2929 references:        ${hasGasSload ? hasGasSload.length : 0}`);
} else {
  console.log(`  operations_acl.go NOT FOUND at ${aclPath}`);
}

// ---- 9. Find what builds the jump table ----
scan(
  "9. Jump table builders (newXxxxInstructionSet)",
  path.join(REPO_ROOT, "core", "vm"),
  /^func\s+new\w+InstructionSet/
);

// ---- 10. Look for SLOAD gas reassignment ----
scan(
  "10. Possible SLOAD gas overrides (look for fork-specific tweaks)",
  path.join(REPO_ROOT, "core", "vm"),
  /\.dynamicGas\s*=\s*gas/
);

console.log(`\n====== END OF SCAN ======`);
console.log(`\nNext step: paste this output to Claude. The locations above`);
console.log(`tell us EXACTLY which file+line needs the surcharge call.`);
console.log(``);
