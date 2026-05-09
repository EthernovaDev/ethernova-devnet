# Ethernova Devnet Phase 8 Report - Nova RPC Namespace & Developer Tooling

Date: 2026-05-09
Scope: devnet only (`chainId/networkId 121526`). Mainnet (`121525`) was not touched.
Feature commit: `694a36b feat(devnet): add phase8 nova rpc tooling`

## Summary

Phase 8 is implemented, built, deployed, and verified across the four LAN devnet nodes plus the VPS public RPC/explorer backend.

What changed:

- Added canonical `nova_*` RPC namespace while keeping `ethernova_*` backward compatibility.
- Added missing Phase 8 RPCs: `nova_getDomain`, `nova_getCapabilities`, `nova_getSession`, `nova_sessionConfig`, and `nova_developerTooling`.
- Exposed VM helper metadata for execution domains and capability masks without changing consensus enforcement.
- Added dependency-free Nova SDK helper under `devnet/nova-sdk`.
- Added minimal Hardhat helper under `devnet/nova-hardhat-plugin` for constructor-free Domain 1/2 runtime deployment.
- Added explorer extension spec under `devnet/phase8/explorer-extension-spec.md`.
- Updated local/devnet launch API lists to include `nova` over HTTP and WS.

## Binaries

Linux amd64:

```text
bd9b1a17cb42628c2fec2b2e7c064418d001a931d14b7c06df3e2b26750d0987  dist/ethernova-phase8-devnet-linux-amd64
```

Windows amd64:

```text
074ad687b368831de4c2886b7a76c136c1bafb8549c53816879c318cb91881d9  dist/ethernova-phase8-devnet-windows-amd64.exe
```

## Deployed Targets

All running devnet nodes were updated to the Linux binary SHA `bd9b1a17cb42628c2fec2b2e7c064418d001a931d14b7c06df3e2b26750d0987` and their startup config now exposes `ethernova,nova`.

```text
node1 sha=bd9b1a17cb42628c2fec2b2e7c064418d001a931d14b7c06df3e2b26750d0987 api=ethernova,nova pid=55602
node2 sha=bd9b1a17cb42628c2fec2b2e7c064418d001a931d14b7c06df3e2b26750d0987 api=ethernova,nova pid=54096
node3 sha=bd9b1a17cb42628c2fec2b2e7c064418d001a931d14b7c06df3e2b26750d0987 api=ethernova,nova pid=52933
node4 sha=bd9b1a17cb42628c2fec2b2e7c064418d001a931d14b7c06df3e2b26750d0987 api=ethernova,nova pid=52271
VPS   sha=bd9b1a17cb42628c2fec2b2e7c064418d001a931d14b7c06df3e2b26750d0987 service=active api=ethernova,nova
```

## Public RPC Validation

Command:

```bash
node devnet/phase8/phase8-rpc-tooling-test.js https://devrpc.ethnova.net
```

Result:

```text
[PASS] eth_chainId is Ethernova devnet - 0x1dab6
[PASS] nova_developerTooling - 11 methods
[PASS] nova_getDomain for EOA - Domain 0 / Legacy
[PASS] nova_getCapabilities for EOA - 0x7f protocolObjects,deferredQueue,contentRegistry,mailboxManager,stateWitness,mailboxOps,sessionArbiter
[PASS] nova_getSession empty result - exists=false
[PASS] nova_sessionConfig active - fork=0 maxStateBytes=512
[PASS] nova_getPendingEffects - pending=0
[PASS] nova_getProtocolObjectTier empty object - tier=Active
[PASS] nova_getStateTier - tier=Active age=0
[PASS] nova_getStateWitness archive method - nodes=0
[PASS] SDK Domain 1 runtime helper - 0xef0160006000f3
[PASS] SDK Domain 2 initcode helper - bytes=22
RESULT: 12 pass, 0 fail, 0 warn
```

## Consensus / Node Health

All nodes were on the same block and same hash during verification:

```text
http://192.168.1.15:8551   block=0x1e2d1 hash=0x0b76f4a2d4a6211fe1227c00b6e85979979637f0b77a3e8457eede1658ee2017 peers=0x4
http://192.168.1.34:8551   block=0x1e2d1 hash=0x0b76f4a2d4a6211fe1227c00b6e85979979637f0b77a3e8457eede1658ee2017 peers=0x4
http://192.168.1.134:8551  block=0x1e2d1 hash=0x0b76f4a2d4a6211fe1227c00b6e85979979637f0b77a3e8457eede1658ee2017 peers=0x3
http://192.168.1.16:8551   block=0x1e2d1 hash=0x0b76f4a2d4a6211fe1227c00b6e85979979637f0b77a3e8457eede1658ee2017 peers=0x4
https://devrpc.ethnova.net block=0x1e2d1 hash=0x0b76f4a2d4a6211fe1227c00b6e85979979637f0b77a3e8457eede1658ee2017 peers=0x3
```

Log scan:

- LAN node recent logs: no `BAD BLOCK`, `Fatal`, or `panic` in the checked tail window.
- VPS public service recent logs: no `BAD BLOCK`; only known existing warnings for unavailable historical trie and unsupported `trace_block` method.

## Explorer Health

Explorer responds HTTP 200 and `/api/v2/stats` is live.

```text
HTTP/2 200
server: nginx
content-type: text/html; charset=utf-8
```

Explorer stats snapshot:

```json
{"total_blocks":"27138","total_transactions":"580","total_addresses":"142","network_utilization_percentage":0.0615368}
```

## Local Tests

Passed:

```bash
go test ./core/vm -run 'TestInspectExecutionDomain|TestCapabilityHelpers|TestSession|TestMailbox'
go test ./eth -run '^$'
go test ./cmd/geth -run '^$'
go test ./cmd/ethernova-launcher
node --check devnet/nova-sdk/index.js
node --check devnet/nova-hardhat-plugin/index.js
node --check devnet/phase8/phase8-rpc-tooling-test.js
git diff --check
```

Note: full upstream `go test ./cmd/geth` still fails on pre-existing devnet-specific network/genesis enforcement tests that expect non-devnet networks. Compile-only for `cmd/geth` passes and the devnet binary builds cleanly.

## Archive Rebuild Caveat

The public VPS devnet service is still running with `--gcmode archive` and `nova_getStateWitness` passed against the public RPC. However, the separate experimental archive rebuild process (`data-archive-rebuild-20260509185309`) hit a historical replay `BAD BLOCK` at block `21404` due a gas-used mismatch while syncing old devnet history from genesis.

Action taken:

- Did not swap any datadir.
- Kept the public RPC/explorer service active and healthy.
- Stopped the separate rebuild process to avoid noisy BAD BLOCK loops.

Follow-up if full historical archive proofs are required: rebuild with a staged historical-compatible binary sequence or a known-good archive snapshot, then only swap after historical proof probes pass.

## Evidence

Detailed logs are in `reports/phase8/logs/` locally and bundled in `reports/phase8/phase8-devnet-evidence-20260509.tar.gz`.
