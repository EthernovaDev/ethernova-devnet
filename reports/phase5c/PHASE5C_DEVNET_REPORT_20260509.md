# Ethernova Devnet Phase 5C Report - Protocol Object Lifecycle

Date: 2026-05-09
Scope: Devnet only, chain/network ID 121526. Mainnet was not touched.

## Executive Summary

Phase 5C is implemented, built, pushed, tagged, and deployed to all active devnet nodes plus the devnet VPS/RPC explorer host.

The devnet is currently healthy:

- All five RPC endpoints report chainId/networkId 121526.
- All five endpoints converged on the same head/hash in the final health check.
- Public dev RPC is online at `https://devrpc.ethnova.net`.
- Dev explorer indexing reports complete with `indexed_blocks_ratio=1.00`.
- Node logs are clean for BAD BLOCK / panic / fatal / state-root errors after deploy.

One operational caveat is tracked separately: the current VPS service is running with `--gcmode archive`, but its existing datadir does not contain complete historical state for older blocks. A parallel archive rebuild was started in a separate datadir and must finish before swapping the public RPC datadir for full historical witness support.

## Git State

- Branch: `main`
- Commit: `3ddfbd6 feat(devnet): complete phase5c object lifecycle`
- Tag pushed: `v1.8.1-nip0004.phase5c-object-lifecycle`
- GitHub Release page/assets: blocked by GitHub API token permissions. `gh` reports repository API permission as pull-only, so release creation returns 404 despite the tag being pushed.

## Code Changes

Phase 5C completes Protocol Object lifecycle tracking for devnet:

- `core/state/statedb.go`
  - Added journaled protocol-object touch tracking.
  - Added `RecordProtocolObjectTouch(id common.Hash)`.
  - Added `LifecycleTouchedObjects() []common.Hash`.
  - Ensured touches copy/revert/clear correctly across StateDB lifecycle.

- `core/state/journal.go`
  - Added `protocolObjectTouchChange` for revert-safe lifecycle touch tracking.

- `core/state/state_lifecycle.go`
  - Added `LastTouchedObject(id common.Hash) uint64`.

- `core/vm/ethernova_protocol_objects.go`
  - Protocol Object writes and clears now record lifecycle touches.

- `core/vm/ethernova_mailbox.go`
  - Deferred mailbox delivery updates object `LastTouchedBlock` and records lifecycle touch.

- `consensus/ethash/state_lifecycle_hook.go`
- `consensus/lyra2/state_lifecycle_hook.go`
  - Consensus hooks now record protocol object block touches through the lifecycle engine.

- `eth/api_ethernova.go`
  - Added live RPC: `ethernova_getProtocolObjectTier(id)`.
  - Includes body `LastTouchedBlock` fallback for legacy objects that predate the index.

- `README.md`
  - Updated Phase 5/6/7 status and corrected stale docs.

## Tests Run

Primary test log:

- `reports/phase5c/logs/20260509-112456-phase5c-local-tests.log`

Results:

- `go test ./core/state ./core/vm -v` - PASS
- `go test ./consensus/ethash ./consensus/lyra2 -v` - PASS
- `go test ./eth -run '^$' -v` - PASS compile-only

Note:

- Full `go test ./eth` was attempted earlier and timed out in a pre-existing `TestCheckpointChallenge` path after the state/vm/consensus packages had passed. No Phase 5C failure was observed there, but the package has a long-running pre-existing test path.

New targeted tests included:

- `TestProtocolObjectTouchesAreJournaled`
- `TestProtocolObjectTouchesSurviveFinaliseAndClearOnCommit`
- `TestProtocolObjectRegistryRecordsLifecycleTouch`

## Builds

Build log:

- `reports/phase5c/logs/20260509-113054-phase5c-build.log`

Artifacts built locally:

- Linux amd64: `dist/ethernova-phase5c-devnet-linux-amd64`
- Windows amd64: `dist/ethernova-phase5c-devnet-windows-amd64.exe`
- Checksums: `dist/phase5c-checksums-sha256.txt`

SHA256:

```text
91f67e62f3c6120fa646c42e3860160d2d43fd1c779cc8935bd05a29f6580abc  dist/ethernova-phase5c-devnet-linux-amd64
4d9a5d8056f002f973e7f0f757655f14e27cda45fe9edb58988dce520d9db633  dist/ethernova-phase5c-devnet-windows-amd64.exe
```

Binary version output on VPS:

```text
Ethernova
Version: v2.0.0-devnet
Git Commit: v1.8.1-nip0004.phase5c-object-lifecycle
Git Commit Date: 20260509
Architecture: amd64
Go Version: go1.21.13
Operating System: linux
```

## Deployment

Deploy log:

- `reports/phase5c/logs/20260509-114532-phase5c-devnet-deploy.log`

Targets updated:

- node1: `novanode1@192.168.1.15`
- node2: `novanode2@192.168.1.34`
- node3: `novanode3@192.168.1.134`
- node4: `novanode4@192.168.1.16`
- VPS/devrpc/devexplorer host: `root@207.180.230.125`

Remote hash audit:

- `reports/phase5c/logs/20260509-114834-phase5c-remote-hashes.log`

All deployed devnet binaries now match:

```text
91f67e62f3c6120fa646c42e3860160d2d43fd1c779cc8935bd05a29f6580abc
```

VPS service remains archive-mode configured:

```text
--syncmode full --gcmode archive
```

## Final Network Health

Final health log:

- `reports/phase5c/logs/20260509-120307-phase5c-final-health-check.log`

Final health output:

```text
node1  chainId=121526 net=121526 head=123462 hash=0xd4565042e62bbf9b149d778028603771c2fb8f9644616aca0813b42fdcbb5231 peers=4
node2  chainId=121526 net=121526 head=123462 hash=0xd4565042e62bbf9b149d778028603771c2fb8f9644616aca0813b42fdcbb5231 peers=4
node3  chainId=121526 net=121526 head=123462 hash=0xd4565042e62bbf9b149d778028603771c2fb8f9644616aca0813b42fdcbb5231 peers=4
node4  chainId=121526 net=121526 head=123462 hash=0xd4565042e62bbf9b149d778028603771c2fb8f9644616aca0813b42fdcbb5231 peers=3
devrpc chainId=121526 net=121526 head=123462 hash=0xd4565042e62bbf9b149d778028603771c2fb8f9644616aca0813b42fdcbb5231 peers=4
common_hash_match=PASS
```

Explorer indexing status:

```json
{"finished_indexing":true,"finished_indexing_blocks":true,"indexed_blocks_ratio":"1.00","indexed_internal_transactions_ratio":"1"}
```

## Live RPC Validation

RPC validation log:

- `reports/phase5c/logs/20260509-114834-phase5c-rpc-live-check.log`

Confirmed on node1, node2, node3, node4, and devrpc:

- `eth_chainId` returns `0x1dab6`.
- `net_version` returns `121526`.
- `ethernova_stateLifecycleConfig` returns devnet Phase 5 thresholds:
  - `activeTierBlocks=10`
  - `warmTierBlocks=25`
  - `coldTierBlocks=50`
  - `forkBlock=0`
  - `warmingFeePerByte=5`
- `ethernova_getProtocolObjectTier` is available on all endpoints.

Sample `ethernova_getProtocolObjectTier` response for zero object id:

```json
{
  "exists": false,
  "phase5cIndexAvailable": false,
  "tier": "Active",
  "tierCode": 0,
  "lastTouchedSource": "lifecycle-index"
}
```

## Critical Log Scan

Post-deploy scan log:

- `reports/phase5c/logs/20260509-114834-phase5c-postdeploy-log-scan.log`

Results:

- node1: CLEAN
- node2: CLEAN
- node3: CLEAN
- node4: CLEAN
- VPS systemd journal since deploy: CLEAN

The VPS service log tail did show `missing trie node` warnings for historical `eth_getBalance` calls. This is not a consensus failure and did not affect latest-state health or explorer indexing, but it confirms the existing VPS datadir is not a complete historical archive even though the service is currently configured with `--gcmode archive`.

## VPS Archive Datadir Rebuild

Why this matters:

- `ethernova_getStateWitness` needs an archive node for reliable historical proofs.
- Current public RPC works for latest state, but historical state probes fail for older blocks.

Archive probe log:

- `reports/phase5c/logs/20260509-114937-phase5c-archive-rpc-probe.log`

Observed:

- `eth_getBalance(..., latest)` works.
- `eth_getBalance(..., 0x0)` works.
- Old historical blocks such as `0x1`, `0x100`, `0x1000`, and `0x10000` return `missing trie node` on the current public RPC datadir.

Action taken:

- Started a parallel archive rebuild on the VPS in a separate datadir.
- The live public RPC service was not swapped and remains online.

Parallel rebuild details:

```text
datadir: /opt/ethernova-devnet/data-archive-rebuild-20260509185309
local RPC: http://127.0.0.1:29545
p2p port: 30311
mode: --syncmode full --gcmode archive --fakepow
peer: live devnet service on 127.0.0.1:30301
```

Latest captured rebuild status:

- `reports/phase5c/logs/20260509-120307-phase5c-final-vps-rebuild-status.log`

```text
service head: 0x1e246 / 123462
archive_rebuild head: 0x705 / 1797
archive_rebuild syncing: true
archive_rebuild datadir size: 1.7G
```

Important:

- Do not swap the public RPC datadir until the rebuild reaches the live head and historical state probes pass.
- The public devrpc/explorer service remains on the current datadir for availability.
- When rebuild finishes, the safe swap sequence is: stop `ethernova-devnet.service`, backup `/opt/ethernova-devnet/data`, move rebuilt archive datadir into place, start service, verify chain/head/common hash/explorer, then rerun historical archive probes.

## Release Status

The git tag was pushed successfully:

```text
v1.8.1-nip0004.phase5c-object-lifecycle
```

GitHub Release creation is blocked by credentials, not by build/test failure.

Observed permission problem:

```json
{"admin":false,"maintain":false,"pull":true,"push":false,"triage":false}
```

Impact:

- Tag exists remotely.
- Linux and Windows binaries exist locally.
- GitHub Release page/assets were not created because the GitHub API token does not have repo write/release permission.

## Evidence Bundle

Evidence tarball:

- `reports/phase5c/phase5c-devnet-evidence-20260509.tar.gz`

This bundle contains the local test, build, deploy, health, RPC, log-scan, and archive-rebuild logs captured during the Phase 5C rollout.

## Current Status

Ready for Noven to test Phase 5C against the active devnet:

- Devnet nodes: running
- Public dev RPC: running
- Dev explorer: running and indexed
- Phase 5C binary: deployed everywhere
- Linux binary: built
- Windows binary: built
- Git tag: pushed
- GitHub Release page/assets: blocked by token permission
- Full historical archive datadir: rebuild in progress, not swapped yet
