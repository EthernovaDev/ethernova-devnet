# Phase 8 - Tests

The Phase 8 (Nova RPC Namespace and Tooling) real-usage test suite lives at
`scripts/phase8/` in the repo root, not here. This folder exists so anyone
looking for tests under `tests/` finds the right entry point.

## Quick start (Windows / PowerShell)

```
# one-time
copy scripts\phase8\.env.example scripts\phase8\.env
notepad scripts\phase8\.env
cd scripts\phase8\hardhat-test-project
npm install
cd ..\..\..

# every run
powershell -ExecutionPolicy Bypass -File scripts\phase8\run-phase8-real-usage-tests.ps1
```

## What it covers

The suite drives a live RPC endpoint (default `http://127.0.0.1:8545`,
override via `PHASE8_RPC_URL`) and exercises every `nova_*` method that the
node exposes plus the `devnet/nova-sdk/` JavaScript SDK and the
`devnet/nova-hardhat-plugin/` Hardhat plugin.

Scenarios:

- A: Namespace availability (every spec method is probed; missing endpoints
  fail with severity HIGH for `nova_listProtocolObjects` and MEDIUM for
  `nova_getDeferredStats(blockNumber)` per `PHASE8_AUDIT.md`).
- B through M: Endpoint-by-endpoint shape and pagination tests for
  `nova_getProtocolObject`, `getMailbox`, `getMessages`, `getContentRef`,
  `getSession`, `getStateTier`, `getStateWitness`, `getPendingEffects`,
  `deferredProcessingStats`, `getCapabilities`, `getDomain`, plus Domain 0/1/2
  prefix checks.
- N: Malformed-input liveness checks (null params, wrong types, negative
  numbers, huge strings, etc.) - the node MUST keep accepting `eth_blockNumber`
  after each malformed call.
- O: Concurrent load test (50 in-flight by default, 500 requests total,
  configurable via PHASE8_LOAD_*).
- P: SDK parity - every SDK method is called and its output is compared to
  the raw JSON-RPC equivalent.
- Q: Hardhat plugin - `npx hardhat nova:domain / capabilities / session /
  pending-effects / deferred-stats` are spawned and their JSON output is
  compared to raw RPC.
- R: Standard tooling compatibility (`eth_chainId`, `eth_blockNumber`,
  `net_version`, `web3_clientVersion`, optional ethers v6 round-trip).
- S: Multi-RPC consistency across `PHASE8_NODE1/2/3_RPC`.

## Reports

Every run writes a timestamped folder under `reports/phase8/real-usage/<stamp>/`
containing per-scenario `*.json` and `*.log` files plus aggregate `summary.json`
and `summary.md`.

See `PHASE8_AUDIT.md` at the repo root for the source-code audit that informs
the suite and the four documented bugs (BUG-1 through BUG-4).
