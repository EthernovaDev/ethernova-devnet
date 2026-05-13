# Phase 8 - Real-Usage Reports

Every invocation of `scripts\phase8\run-phase8-real-usage-tests.ps1` creates
a timestamped subdirectory here (format `yyyyMMdd-HHmmss`).

## Files per run

| File | Producer |
|---|---|
| `summary.json` | PowerShell runner aggregate |
| `summary.md` | PowerShell runner aggregate (human-readable) |
| `create-fixtures.json` | `phase8-create-fixtures.js` suite results |
| `create-fixtures.log` | stdout/stderr capture |
| `fixtures.json` | `phase8-create-fixtures.js` minted IDs (mailbox, contentRef) |
| `rpc-real-usage.json` | Scenarios A-M (raw JSON-RPC) |
| `rpc-real-usage.log` | stdout/stderr capture |
| `malformed-rpc.json` | Scenario N (liveness under malformed input) |
| `malformed-rpc.log` | stdout/stderr capture |
| `load-test.json` | Scenario O (concurrent load) |
| `load-test.log` | stdout/stderr capture |
| `sdk-test.json` | Scenario P (SDK parity) |
| `sdk-test.log` | stdout/stderr capture |
| `hardhat-plugin-test.json` | Scenario Q (Hardhat tasks) |
| `hardhat-plugin-test.log` | stdout/stderr capture |
| `multirpc-consistency.json` | Scenario S (multi-node agreement) |
| `multirpc-consistency.log` | stdout/stderr capture |
| `standard-tooling-compat.json` | Scenario R (eth_/net_/web3_ compat) |
| `standard-tooling-compat.log` | stdout/stderr capture |

A scenario JSON file always contains:

```
{
  "suite": "phase8-<name>",
  "startedAt": "...",
  "endedAt": "...",
  "counts": { "pass": N, "fail": N, "warn": N, "skip": N },
  "highestSeverity": "low" | "medium" | "high" | "critical" | null,
  "results": [ { "scenario", "status", "detail", ... }, ... ]
}
```

The runner aggregates these into `summary.json` with a final
`verdict` field of `PASS`, `PARTIAL`, or `FAIL`. The verdict reflects only
the test run; the standing audit bugs documented in `PHASE8_AUDIT.md` are
listed under `knownGaps` for context.

## Privacy / hygiene

These reports may contain on-chain addresses, transaction hashes, and the
public address derived from `PHASE8_PRIVATE_KEY_A` (never the private key
itself). The suite never reads or writes private keys to disk. Treat the
reports folder as suitable for sharing externally.
