# Ethernova Devnet Phase 10A Report - Resource Metering Observability

Date: 2026-05-11
Network: Ethernova devnet only
Chain ID / Network ID: 121526 (`0x1dab6`)
Mainnet touched: no
Noven fix integrated: `9256bac NIP-0004 Phase 7.1: explicit channel signers and revert reasons`
Feature commit: `d6688aa feat(devnet): add phase10a resource metering observability`
Release: `v1.10.0-nip0004.phase10a-resource-metering`
Release URL: https://github.com/EthernovaDev/ethernova-devnet/releases/tag/v1.10.0-nip0004.phase10a-resource-metering

## Summary

Phase 10A is implemented, built, released, deployed, and verified on the full devnet fleet.

This is intentionally monitoring-only. It records deterministic multi-dimensional execution telemetry without changing transaction validity, block headers, receipts, gas accounting, or state roots. That keeps Phase 10A safe as a devnet observability layer before Phase 10B/10C activate pricing behavior.

## Implementation

Added:

- `ResourceVector` dimensions: `compute`, `stateRead`, `stateWrite`, `protocolOps`, `proofVerify`.
- VM-local resource metering reset per transaction.
- Precompile resource accounting for Nova protocol precompiles.
- In-memory recent transaction resource monitor.
- Public RPC methods:
  - `nova_resourceConfig`
  - `nova_resourcePrices`
  - `nova_estimateResourceLimits`
  - `nova_getResourceVector`
- Nova SDK wrappers for Phase 10 methods.
- Phase 10 public RPC smoke test.

Important scope note:

- Phase 10A does not activate adaptive pricing.
- Phase 10A does not add an extended transaction format.
- Phase 10A does not change mainnet constants or activation blocks.

## Build Artifacts

Linux devnet binary:

```text
b98988d0d4c824bcce92d12e79331b68591c860280c5591a2c5da12f36bc5977  dist/ethernova-phase10a-devnet-linux-amd64
```

Windows devnet binary:

```text
3c11933f4f38684bcce955282556ddd470cf2ddabf575876757fe5f4eba6a489  dist/ethernova-phase10a-devnet-windows-amd64.exe
```

Checksums file:

```text
dist/phase10a-checksums-sha256.txt
```

## Deployment

The Linux binary with SHA `b98988d0d4c824bcce92d12e79331b68591c860280c5591a2c5da12f36bc5977` was deployed to:

- `novanode1@192.168.1.15`
- `novanode2@192.168.1.34`
- `novanode3@192.168.1.134`
- `novanode4@192.168.1.16`
- VPS RPC/explorer node `root@207.180.230.125`

VPS service:

- Service: `ethernova-devnet.service`
- Internal RPC: `127.0.0.1:28545`
- Public RPC: `https://devrpc.ethnova.net`
- Explorer: `https://devexplorer.ethnova.net`
- Archive mode preserved: `--gcmode archive`

## Local Validation

Passed before release/deploy:

```bash
go test ./core/types ./core/vm
go test ./eth -run '^$'
go test ./core -run '^$'
go test ./cmd/geth -run '^$'
node --check devnet/nova-sdk/index.js
node --check devnet/phase10/phase10-resource-metering-test.js
```

## Public RPC Validation

Command:

```bash
node devnet/phase10/phase10-resource-metering-test.js https://devrpc.ethnova.net
```

Result:

```text
========================================================================
 Phase 10A - Multi-Dimensional Resource Metering (Monitoring Only)
========================================================================
RPC: https://devrpc.ethnova.net
[PASS] eth_chainId is Ethernova devnet - 0x1dab6
[PASS] nova_resourceConfig monitoring-only - 10A compute,state_read,state_write,protocol_ops,proof_verify
[PASS] nova_resourcePrices fixed placeholders - all dimensions price=1
[PASS] legacy gasLimit maps to resource vector - {"compute":3000000,"stateRead":1000000,"stateWrite":500000,"protocolOps":200000,"proofVerify":100000}
[PASS] nova_developerTooling advertises Phase 10 methods - 17 methods
------------------------------------------------------------------------
RESULT: 5 pass, 0 fail
```

Regression tests after deploy:

```text
Phase 8 RPC tooling: 12 pass, 0 fail, 0 warn
Phase 9 chat proving ground: 12 pass, 0 fail, 0 warn
```

## Consensus / Node Health

Final devnet health check:

```text
Devnet health check 2026-05-11T15:27:55Z
node1  chainId=121526 net=121526 head=135633 hash=0xb2a799c5da62b3d911e9b3cdbb38721d05addd7e4f7c0d563bbf94176d2e75f4 peers=4
node2  chainId=121526 net=121526 head=135633 hash=0xb2a799c5da62b3d911e9b3cdbb38721d05addd7e4f7c0d563bbf94176d2e75f4 peers=4
node3  chainId=121526 net=121526 head=135633 hash=0xb2a799c5da62b3d911e9b3cdbb38721d05addd7e4f7c0d563bbf94176d2e75f4 peers=4
node4  chainId=121526 net=121526 head=135633 hash=0xb2a799c5da62b3d911e9b3cdbb38721d05addd7e4f7c0d563bbf94176d2e75f4 peers=3
devrpc chainId=121526 net=121526 head=135633 hash=0xb2a799c5da62b3d911e9b3cdbb38721d05addd7e4f7c0d563bbf94176d2e75f4 peers=3
common_hash_match=PASS
```

Final critical scan:

```text
node1: no critical matches
node2: no critical matches
node3: no critical matches
node4: no critical matches
vps journal: no critical matches
```

## Explorer / VPS Status

The devnet explorer is live and caught up:

```text
https://devexplorer.ethnova.net -> HTTP/2 200
finished_indexing=true
finished_indexing_blocks=true
indexed_blocks_ratio=1.00
indexed_internal_transactions_ratio=1
```

CPU after the low-CPU explorer fix:

```text
load average: 3.20, 4.37, 4.32
ethernova-devnet-backend 6.09% 292.7MiB / 1GiB
ethernova-devnet-db 4.52% 117MiB / 23.47GiB
ethernova-devnet-frontend 0.00% 201.7MiB / 23.47GiB
ethernova-devnet-proxy 0.15% 8.445MiB / 23.47GiB
```

Permanent low-CPU dev explorer follow mode:

- `DISABLE_REALTIME_INDEXER=true`
- `INDEXER_CATCHUP_BLOCKS_BATCH_SIZE=20`
- `ethnova-devnet-explorer-catchup.timer` active, persistent, runs every 60 seconds
- `ethnova-block-rewards-backfill.timer` active, persistent, runs every 60 seconds

Explorer reward note:

- The per-block endpoint returns rewards correctly, for example latest block detail returned `Miner Reward` with `10000000000000000000` wei.
- The `main-page/blocks` list endpoint may still return `rewards: []` for the newest rows. The DB and per-block endpoint are correct; this appears to be a Blockscout list/cache behavior rather than missing reward rows.

## Evidence

Detailed evidence logs are under:

```text
reports/phase10/evidence/
```

Key logs:

- `reports/phase10/evidence/phase10a-deploy-20260511T143232Z.log`
- `reports/phase10/evidence/phase10a-final-health-20260511T152755Z.log`
- `reports/phase10/evidence/phase10a-final-phase10-rpc-20260511T152755Z.log`
- `reports/phase10/evidence/phase10a-final-explorer-api-20260511T152755Z.log`
- `reports/phase10/evidence/phase10a-final-vps-cpu-20260511T152755Z.log`
- `reports/phase10/evidence/phase10a-final-critical-scan-20260511T153239Z.log`

## Notes For Noven

- Noven's Phase 7.1 signer/revert-reason fix was pulled before Phase 10A work and is included in the deployed binary.
- Phase 10A is intentionally safe observability. It prepares the network for Phase 10B pricing without changing consensus economics yet.
- All devnet nodes and the VPS RPC/explorer node are updated and running.
- Mainnet was not touched.

## Status

Phase 10A devnet rollout is complete.

Next recommended work: Phase 10B, devnet-only per-dimension pricing activation with conservative constants, followed by Phase 10C congestion-isolation validation.
