"use strict";

// Phase 8 test-only Hardhat config.
//
// Two-tier plugin loading:
//   1. index.js (the BASE Nova Hardhat plugin) - lives in the real
//      ethernova-devnet repo at devnet/nova-hardhat-plugin/index.js. Located
//      via PHASE8_PLUGIN_PATH (preferred) or by walking up from this file.
//   2. tasks.js (Phase 8 ADDITIVE task registrations) - ships with this test
//      suite at <repo>/devnet/nova-hardhat-plugin/tasks.js. Located relative
//      to this file. Falls back to base plugin dir if missing locally.

const path = require("path");
const fs = require("fs");

function resolveBasePluginDir() {
  const envOverride = (process.env.PHASE8_PLUGIN_PATH || "").trim();
  if (envOverride) {
    const abs = path.isAbsolute(envOverride)
      ? envOverride
      : path.resolve(__dirname, envOverride);
    if (fs.existsSync(path.join(abs, "index.js"))) {
      return abs;
    }
    throw new Error(
      "PHASE8_PLUGIN_PATH is set to '" + abs +
      "' but no index.js is there. Point it at devnet/nova-hardhat-plugin/ " +
      "from your ethernova-devnet checkout."
    );
  }
  // Default: assume the test suite is extracted into the ethernova-devnet
  // repo root.
  const defaultPath = path.resolve(__dirname, "..", "..", "..", "devnet", "nova-hardhat-plugin");
  if (fs.existsSync(path.join(defaultPath, "index.js"))) {
    return defaultPath;
  }
  throw new Error(
    "Could not locate devnet/nova-hardhat-plugin/index.js. Either extract " +
    "the test suite into your ethernova-devnet repo root, or set " +
    "PHASE8_PLUGIN_PATH in scripts/phase8/.env to the absolute path of " +
    "devnet/nova-hardhat-plugin/ in your ethernova-devnet checkout."
  );
}

function resolveTasksFile(basePluginDir) {
  // tasks.js is part of the test suite payload, not the real repo. Prefer the
  // copy that shipped with the zip.
  const ownTasks = path.resolve(__dirname, "..", "..", "..", "devnet", "nova-hardhat-plugin", "tasks.js");
  if (fs.existsSync(ownTasks)) return ownTasks;
  // Fall back to the real repo location (only useful when the user manually
  // copied tasks.js there).
  const repoTasks = path.join(basePluginDir, "tasks.js");
  if (fs.existsSync(repoTasks)) return repoTasks;
  throw new Error(
    "Could not locate Phase 8 tasks.js. Expected at " + ownTasks + " (zip payload) " +
    "or " + repoTasks + " (real repo). Re-extract the test suite zip."
  );
}

const basePluginDir = resolveBasePluginDir();
require(basePluginDir);
require(resolveTasksFile(basePluginDir));

const rpcUrl = process.env.PHASE8_RPC_URL || "http://127.0.0.1:8545";
const chainId = Number(process.env.PHASE8_CHAIN_ID || "121526");

module.exports = {
  solidity: "0.8.24",
  defaultNetwork: "ethernova",
  networks: {
    ethernova: {
      url: rpcUrl,
      chainId: chainId,
      timeout: 60000,
    },
  },
};
