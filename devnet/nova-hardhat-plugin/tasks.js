"use strict";

// devnet/nova-hardhat-plugin/tasks.js
//
// ADDITIVE module that registers Phase 8 read-only CLI tasks for the
// Hardhat plugin. The base plugin (index.js) only exposes hre.nova.* helpers;
// this file plugs CLI commands on top so devs can do:
//
//   npx hardhat nova:domain --address 0x...
//   npx hardhat nova:capabilities --address 0x...
//   npx hardhat nova:session --id 0x...
//   npx hardhat nova:pending-effects [--offset 0] [--limit 50]
//   npx hardhat nova:deferred-stats
//
// Each task prints a single JSON object to stdout. Errors print
// { "error": "<msg>" } and exit non-zero.
//
// Opt-in: add `require("../path/to/nova-hardhat-plugin/tasks.js");` to your
// hardhat.config.js AFTER requiring the base plugin. The base plugin keeps
// working untouched even if tasks.js is never loaded.

let task, types;
try {
  ({ task, types } = require("hardhat/config"));
} catch (_) {
  // Hardhat not available; this file is a no-op when required outside Hardhat.
  module.exports = { available: false };
  return;
}

const sdk = require("../nova-sdk");

function rpcUrl(hre) {
  return (hre.network && hre.network.config && hre.network.config.url) || "http://127.0.0.1:8545";
}

function emitJson(obj) {
  // Single-line JSON makes it trivial for the test harness to parse.
  process.stdout.write(JSON.stringify(obj) + "\n");
}

function emitError(err) {
  const msg = err && err.message ? err.message : String(err);
  process.stdout.write(JSON.stringify({ error: msg }) + "\n");
  process.exitCode = 1;
}

task("nova:domain", "Print the nova_getDomain result for an address as JSON")
  .addParam("address", "Account address to inspect", undefined, types.string)
  .setAction(async (args, hre) => {
    try {
      const provider = new sdk.NovaProvider(rpcUrl(hre));
      const result = await provider.getDomain(args.address);
      emitJson(result);
    } catch (err) {
      emitError(err);
    }
  });

task("nova:capabilities", "Print the nova_getCapabilities result for an address as JSON")
  .addParam("address", "Account address to inspect", undefined, types.string)
  .setAction(async (args, hre) => {
    try {
      const provider = new sdk.NovaProvider(rpcUrl(hre));
      const result = await provider.getCapabilities(args.address);
      emitJson(result);
    } catch (err) {
      emitError(err);
    }
  });

task("nova:session", "Print the nova_getSession result for an id as JSON")
  .addParam("id", "32-byte session id (0x...)", undefined, types.string)
  .setAction(async (args, hre) => {
    try {
      const provider = new sdk.NovaProvider(rpcUrl(hre));
      const result = await provider.getSession(args.id);
      emitJson(result);
    } catch (err) {
      emitError(err);
    }
  });

task("nova:pending-effects", "Print the nova_getPendingEffects window as JSON")
  .addOptionalParam("offset", "Offset into the pending queue (default 0)", "0", types.string)
  .addOptionalParam("limit", "Max effects to return (default 50)", "50", types.string)
  .setAction(async (args, hre) => {
    try {
      const provider = new sdk.NovaProvider(rpcUrl(hre));
      const offset = Number(args.offset);
      const limit = Number(args.limit);
      if (!Number.isFinite(offset) || offset < 0) throw new Error("invalid --offset");
      if (!Number.isFinite(limit) || limit <= 0) throw new Error("invalid --limit");
      const result = await provider.getPendingEffects(offset, limit);
      emitJson(result);
    } catch (err) {
      emitError(err);
    }
  });

task("nova:deferred-stats", "Print the nova_deferredProcessingStats result as JSON")
  .setAction(async (_args, hre) => {
    try {
      const provider = new sdk.NovaProvider(rpcUrl(hre));
      // SDK does not expose a dedicated wrapper for this RPC; use the
      // generic nova(methodSuffix, params) helper that already handles the
      // ethernova_* fallback for older nodes.
      const result = await provider.nova("deferredProcessingStats", []);
      emitJson(result);
    } catch (err) {
      // Fall back to alternate spelling if SDK adds one later. For now surface
      // the original error.
      emitError(err);
    }
  });

module.exports = { available: true };
