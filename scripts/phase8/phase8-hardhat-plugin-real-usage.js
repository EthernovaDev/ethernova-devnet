"use strict";

// phase8-hardhat-plugin-real-usage.js
//
// Scenario Q. Spawns `npx hardhat nova:*` against scripts/phase8/hardhat-test-project/
// and compares each task's JSON output to a parallel raw JSON-RPC call.
// Validates that the additive tasks.js wrapper produces the same wire data
// that nova_* returns directly.
//
// Skipped if PHASE8_ENABLE_HARDHAT_PLUGIN != true.
//
// Output: $PHASE8_REPORT_DIR_RUN/hardhat-plugin-test.json

const fs = require("fs");
const path = require("path");
const { spawnSync } = require("child_process");

const H = require("./phase8-helpers");
H.loadEnv(path.join(__dirname, ".env"));

const REPORT_DIR = H.envString(
  "PHASE8_REPORT_DIR_RUN",
  path.resolve(__dirname, "..", "..", "reports", "phase8", "real-usage", "manual")
);

const suite = new H.Suite("phase8-hardhat-plugin-real-usage");

// ----- helpers -----

function runHardhatTask(projectDir, args, env) {
  const isWindows = process.platform === "win32";
  const cmd = isWindows ? "npx.cmd" : "npx";
  const result = spawnSync(cmd, ["hardhat", ...args], {
    cwd: projectDir,
    env: { ...process.env, ...env },
    encoding: "utf8",
    timeout: 60_000,
    shell: false,
    windowsHide: true,
  });
  return {
    status: result.status,
    signal: result.signal,
    stdout: result.stdout || "",
    stderr: result.stderr || "",
    error: result.error ? result.error.message : null,
  };
}

function extractLastJsonLine(text) {
  // Hardhat may print plugin banners; we look for the last line of stdout that
  // parses as JSON. The task itself emits a single-line JSON object.
  const lines = text.split(/\r?\n/).map((s) => s.trim()).filter(Boolean);
  for (let i = lines.length - 1; i >= 0; i--) {
    const line = lines[i];
    if (!line.startsWith("{") && !line.startsWith("[")) continue;
    try {
      return JSON.parse(line);
    } catch (_) {
      // skip non-JSON lines
    }
  }
  return null;
}

function deepEqualNormalized(a, b) {
  // Normalize hex strings to lowercase. Treat undefined === null for envelope
  // tolerance. Numbers parsed from hex are compared as decimal strings if one
  // side is hex and the other is decimal.
  if (a === b) return true;
  if (a == null && b == null) return true;
  if (typeof a !== typeof b) {
    if (typeof a === "string" && typeof b === "number") return a.toLowerCase() === String(b).toLowerCase();
    if (typeof b === "string" && typeof a === "number") return String(a).toLowerCase() === b.toLowerCase();
    return false;
  }
  if (typeof a === "string") return a.toLowerCase() === b.toLowerCase();
  if (typeof a === "number" || typeof a === "boolean") return a === b;
  if (Array.isArray(a)) {
    if (!Array.isArray(b) || a.length !== b.length) return false;
    for (let i = 0; i < a.length; i++) if (!deepEqualNormalized(a[i], b[i])) return false;
    return true;
  }
  if (typeof a === "object") {
    const ak = Object.keys(a), bk = Object.keys(b);
    if (ak.length !== bk.length) return false;
    for (const k of ak) {
      if (!Object.prototype.hasOwnProperty.call(b, k)) return false;
      if (!deepEqualNormalized(a[k], b[k])) return false;
    }
    return true;
  }
  return false;
}

// ----- main -----

async function main() {
  console.log("========================================================================");
  console.log(" Phase 8 - Hardhat Plugin Real-Usage (Scenario Q)");
  console.log("========================================================================");

  const enable = H.envBool("PHASE8_ENABLE_HARDHAT_PLUGIN", true);
  if (!enable) {
    suite.skip("hardhat-plugin", "PHASE8_ENABLE_HARDHAT_PLUGIN=false");
    return finalize();
  }

  const rpcUrl = H.envString("PHASE8_RPC_URL", "");
  if (!rpcUrl) {
    suite.skip("hardhat-plugin", "PHASE8_RPC_URL not set");
    return finalize();
  }

  const projectDir = path.resolve(__dirname, "hardhat-test-project");
  if (!fs.existsSync(path.join(projectDir, "hardhat.config.js"))) {
    suite.fail(
      "project-missing",
      "hardhat-test-project/hardhat.config.js not found at " + projectDir,
      H.SEVERITY.HIGH
    );
    return finalize();
  }
  if (!fs.existsSync(path.join(projectDir, "node_modules"))) {
    suite.fail(
      "project-not-installed",
      "hardhat-test-project/node_modules missing; run `npm install` in that folder first",
      H.SEVERITY.HIGH,
      { hint: "cd scripts/phase8/hardhat-test-project && npm install" }
    );
    return finalize();
  }

  const env = { PHASE8_RPC_URL: rpcUrl };

  // Pick an address to query. Use Domain 1 / Domain 2 / Domain 0 in priority
  // order, else fall back to zero address (still valid for getDomain - any
  // EOA returns code-less domain 0).
  const addrCandidate =
    H.envString("PHASE8_DOMAIN1_ADDRESS", "") ||
    H.envString("PHASE8_DOMAIN2_ADDRESS", "") ||
    H.envString("PHASE8_DOMAIN0_ADDRESS", "") ||
    "0x0000000000000000000000000000000000000000";

  // ----- Task: nova:domain -----
  await runAndCompare("nova:domain", ["nova:domain", "--address", addrCandidate], env, {
    rpcMethod: "nova_getDomain",
    rpcParams: [addrCandidate],
    projectDir,
  });

  // ----- Task: nova:capabilities -----
  await runAndCompare("nova:capabilities", ["nova:capabilities", "--address", addrCandidate], env, {
    rpcMethod: "nova_getCapabilities",
    rpcParams: [addrCandidate],
    projectDir,
  });

  // ----- Task: nova:session -----
  const sessionId = H.envString("PHASE8_EXISTING_SESSION_ID", "");
  if (sessionId) {
    await runAndCompare("nova:session", ["nova:session", "--id", sessionId], env, {
      rpcMethod: "nova_getSession",
      rpcParams: [sessionId],
      projectDir,
    });
  } else {
    // Even without a real session, the task should return a {exists:false}
    // envelope (per audit, getSession does not return null).
    const zeroId = "0x" + "0".repeat(64);
    await runAndCompare("nova:session(zero)", ["nova:session", "--id", zeroId], env, {
      rpcMethod: "nova_getSession",
      rpcParams: [zeroId],
      projectDir,
    });
  }

  // ----- Task: nova:pending-effects -----
  await runAndCompare(
    "nova:pending-effects",
    ["nova:pending-effects", "--offset", "0", "--limit", "5"],
    env,
    {
      rpcMethod: "nova_getPendingEffects",
      rpcParams: [0, 5],
      projectDir,
    }
  );

  // ----- Task: nova:deferred-stats -----
  await runAndCompare("nova:deferred-stats", ["nova:deferred-stats"], env, {
    rpcMethod: "nova_deferredProcessingStats",
    rpcParams: [],
    projectDir,
  });

  // ----- Negative path: invalid address must fail clearly -----
  try {
    const r = runHardhatTask(projectDir, ["nova:domain", "--address", "0xnotanaddress"], env);
    if (r.status === 0) {
      const out = extractLastJsonLine(r.stdout);
      if (out && out.error) {
        suite.pass("nova:domain-bad-address", "task exited 0 but emitted error JSON: " + out.error);
      } else {
        suite.warn(
          "nova:domain-bad-address",
          "task exited 0 with no error envelope; node may have accepted malformed address"
        );
      }
    } else {
      suite.pass("nova:domain-bad-address", "task correctly exited non-zero (status=" + r.status + ")");
    }
  } catch (err) {
    suite.fail("nova:domain-bad-address", err && err.message, H.SEVERITY.MEDIUM);
  }

  // ----- Negative path: missing required param -----
  try {
    const r = runHardhatTask(projectDir, ["nova:domain"], env);
    // Hardhat itself rejects missing required args before our action runs.
    if (r.status !== 0) {
      suite.pass("nova:domain-missing-arg", "Hardhat rejected missing --address (status=" + r.status + ")");
    } else {
      suite.fail(
        "nova:domain-missing-arg",
        "Hardhat accepted missing --address (status=0); task arg validation broken",
        H.SEVERITY.MEDIUM
      );
    }
  } catch (err) {
    suite.fail("nova:domain-missing-arg", err && err.message, H.SEVERITY.MEDIUM);
  }

  return finalize();
}

async function runAndCompare(label, args, env, opts) {
  const { rpcMethod, rpcParams, projectDir } = opts;
  try {
    const r = runHardhatTask(projectDir, args, env);
    if (r.status !== 0) {
      suite.fail(
        label,
        "hardhat task exited " + r.status + "; stderr=" + r.stderr.slice(0, 400),
        H.SEVERITY.HIGH,
        { stdout: r.stdout.slice(0, 800) }
      );
      return;
    }
    const taskOutput = extractLastJsonLine(r.stdout);
    if (!taskOutput) {
      suite.fail(
        label,
        "could not parse JSON from hardhat stdout",
        H.SEVERITY.HIGH,
        { stdout: r.stdout.slice(0, 800) }
      );
      return;
    }
    if (taskOutput.error) {
      suite.fail(label, "task emitted error: " + taskOutput.error, H.SEVERITY.HIGH);
      return;
    }
    let rpcOutput;
    try {
      rpcOutput = await H.rpcResult(env.PHASE8_RPC_URL, rpcMethod, rpcParams);
    } catch (err) {
      suite.fail(
        label,
        "parallel raw RPC call failed: " + (err && err.message),
        H.SEVERITY.HIGH
      );
      return;
    }
    if (deepEqualNormalized(taskOutput, rpcOutput)) {
      suite.pass(label, "task output matches raw " + rpcMethod);
    } else {
      suite.fail(
        label,
        "task output diverges from raw " + rpcMethod,
        H.SEVERITY.HIGH,
        {
          task: taskOutput,
          rpc: rpcOutput,
        }
      );
    }
  } catch (err) {
    suite.fail(label, "unexpected error: " + (err && err.message), H.SEVERITY.HIGH);
  }
}

function finalize() {
  suite.printFooter();
  const summary = suite.summarize();
  H.writeJson(path.join(REPORT_DIR, "hardhat-plugin-test.json"), summary);
  console.log("Wrote: " + path.join(REPORT_DIR, "hardhat-plugin-test.json"));

  const counts = summary.counts;
  const sev = summary.highestSeverity;
  if (counts.fail > 0 && (sev === H.SEVERITY.CRITICAL || sev === H.SEVERITY.HIGH)) {
    process.exit(1);
  }
  process.exit(0);
}

main().catch((err) => {
  console.error("phase8-hardhat-plugin-real-usage crashed:", err && err.stack ? err.stack : err);
  try {
    suite.fail("uncaught", String((err && err.message) || err), H.SEVERITY.HIGH);
    H.writeJson(path.join(REPORT_DIR, "hardhat-plugin-test.json"), suite.summarize());
  } catch (_) {}
  process.exit(1);
});
