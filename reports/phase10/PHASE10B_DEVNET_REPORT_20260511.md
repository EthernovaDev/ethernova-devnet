# Ethernova Devnet Phase 10B Report - Static Resource Pricing

Date: 2026-05-11
Network: Ethernova devnet only
Chain ID / Network ID: 121526 (`0x1dab6`)
Mainnet touched: no
Feature commit: `d497a898e4f92baec8fa54cc6609ee014fc86483`
Previous checkpoint: `e560f9d docs(devnet): add phase10a rollout report`
Release: `v1.10.1-nip0004.phase10b-static-resource-pricing`
Release URL: https://github.com/EthernovaDev/ethernova-devnet/releases/tag/v1.10.1-nip0004.phase10b-static-resource-pricing

## Summary

Phase 10B is implemented, built, released, deployed, and verified on the full devnet fleet.

This substage activates static per-dimension resource pricing as an RPC/SDK quote surface while keeping consensus gas charging unchanged. The goal is to let SDKs, explorers, and app tests start reasoning in the five NIP-0004 resource dimensions before Phase 10C introduces adaptive pricing and extended transaction enforcement.

## Implementation

Added:

- `vm.ResourcePrices` and `vm.ResourceCharge`.
- `vm.Phase10BResourcePrices()` with conservative devnet multipliers:
  - `compute = 1`
  - `state_read = 2`
  - `state_write = 4`
  - `protocol_ops = 1`
  - `proof_verify = 3`
- `vm.PriceResourceVector()` with saturating arithmetic.
- `nova_quoteResourceFee` RPC.
- SDK helper: `NovaProvider.quoteResourceFee(vector)`.
- Phase 10 test upgraded from 10A monitoring checks to 10B static pricing checks.
- README roadmap marks 10B complete and keeps 10C pending.

Important safety scope:

- `consensusGasChanged=false`.
- Legacy gas charging is unchanged.
- No receipt/header/state-root format changes.
- No extended transaction format yet.
- Adaptive per-block pricing remains Phase 10C scope.
- Protocol ops stay price `1` so chat/mailbox traffic is not penalized by the storage-heavy pricing multiplier.

## Build Artifacts

Linux devnet binary:

```text
8c3f3b52fbc07f20d3665c5e0c0391d5c2d3b34c3f1d83be8728a7c8500c2cda  dist/ethernova-phase10b-devnet-linux-amd64
```

Windows devnet binary:

```text
c7978a7e801ac3ff971a8cc23fcbeb1fdf180c4f407341f16854b0fdd7e06fad  dist/ethernova-phase10b-devnet-windows-amd64.exe
```

Build note:

- Linux/Windows release binaries were built with `CGO_ENABLED=0` for clean cross-platform devnet artifacts from macOS.

## Deployment

The Linux binary with SHA `8c3f3b52fbc07f20d3665c5e0c0391d5c2d3b34c3f1d83be8728a7c8500c2cda` was deployed to:

- `novanode1@192.168.1.15`
- `novanode2@192.168.1.34`
- `novanode3@192.168.1.134`
- `novanode4@192.168.1.16`
- VPS RPC/explorer node `root@207.180.230.125`

Remote hash verification:

```text
node1 8c3f3b52fbc07f20d3665c5e0c0391d5c2d3b34c3f1d83be8728a7c8500c2cda
node2 8c3f3b52fbc07f20d3665c5e0c0391d5c2d3b34c3f1d83be8728a7c8500c2cda
node3 8c3f3b52fbc07f20d3665c5e0c0391d5c2d3b34c3f1d83be8728a7c8500c2cda
node4 8c3f3b52fbc07f20d3665c5e0c0391d5c2d3b34c3f1d83be8728a7c8500c2cda
vps   8c3f3b52fbc07f20d3665c5e0c0391d5c2d3b34c3f1d83be8728a7c8500c2cda
```

VPS service:

- Service: `ethernova-devnet.service`
- Internal RPC: `127.0.0.1:28545`
- Public RPC: `https://devrpc.ethnova.net`
- Explorer: `https://devexplorer.ethnova.net`
- Archive mode preserved: `--gcmode archive`

## Local Validation

Command log: `reports/phase10/evidence/phase10b-local-tests-20260511T160906Z.log`

Passed:

```bash
go test ./core/vm
go test ./eth -run '^$'
go test ./core -run '^$'
go test ./cmd/geth -run '^$'
node --check devnet/nova-sdk/index.js
node --check devnet/phase10/phase10-resource-metering-test.js
git diff --check
```

The Go output includes normal local macOS cgo/libusb warnings; all listed tests passed.

## Public RPC Validation

Command:

```bash
node devnet/phase10/phase10-resource-metering-test.js https://devrpc.ethnova.net
```

Result:

```text
========================================================================
 Phase 10B - Multi-Dimensional Resource Pricing (Static Quote)
========================================================================
RPC: https://devrpc.ethnova.net
[PASS] eth_chainId is Ethernova devnet - 0x1dab6
[PASS] nova_resourceConfig static pricing - 10B compute,state_read,state_write,protocol_ops,proof_verify
[PASS] nova_resourcePrices static multipliers - compute=1 state_read=2 state_write=4 protocol_ops=1 proof_verify=3
[PASS] legacy gasLimit maps to resource vector - {"compute":3000000,"stateRead":1000000,"stateWrite":500000,"protocolOps":200000,"proofVerify":100000}
[PASS] nova_quoteResourceFee applies per-dimension prices - total=1450
[PASS] nova_developerTooling advertises Phase 10 methods - 18 methods
------------------------------------------------------------------------
RESULT: 6 pass, 0 fail
```

Regression tests after deploy:

```text
Phase 8 RPC tooling: 12 pass, 0 fail, 0 warn
Phase 9 chat proving ground: 12 pass, 0 fail, 0 warn
```

## Consensus / Node Health

Final devnet health check:

```text
Devnet health check 2026-05-11T15:59:01Z
node1  chainId=121526 net=121526 head=135771 hash=0xe268e4c4eaa6b4683e6321b782d3b50dcf317f77d541211e1a0244ccc843f898 peers=4
node2  chainId=121526 net=121526 head=135771 hash=0xe268e4c4eaa6b4683e6321b782d3b50dcf317f77d541211e1a0244ccc843f898 peers=4
node3  chainId=121526 net=121526 head=135771 hash=0xe268e4c4eaa6b4683e6321b782d3b50dcf317f77d541211e1a0244ccc843f898 peers=4
node4  chainId=121526 net=121526 head=135771 hash=0xe268e4c4eaa6b4683e6321b782d3b50dcf317f77d541211e1a0244ccc843f898 peers=3
devrpc chainId=121526 net=121526 head=135771 hash=0xe268e4c4eaa6b4683e6321b782d3b50dcf317f77d541211e1a0244ccc843f898 peers=3
common_hash_match=PASS
```

Phase 9 regression also checked a later consensus target:

```text
target=0x2125a node1=node2=node3=node4=devrpc hash 0x1c5e65cbd17d6289c1cf690a586adf4907ee9479e226b6ddff77fa5f54a9a479
```

Critical scan after deploy:

```text
node1: no critical matches
node2: no critical matches
node3: no critical matches
node4: no critical matches
vps journal: no critical matches
```

## Explorer / VPS Status

Explorer is live and caught up:

```text
https://devexplorer.ethnova.net -> HTTP/2 200
finished_indexing=true
finished_indexing_blocks=true
indexed_blocks_ratio=1.00
indexed_internal_transactions_ratio=1
```

VPS services/timers:

```text
ethernova-devnet.service active
ethnova-rpc-proxy.service active
ethnova-devnet-explorer-catchup.timer active
ethnova-block-rewards-backfill.timer active
```

CPU snapshot:

```text
load average: 3.05, 4.02, 4.01
ethernova-devnet-backend 9.60% 294.1MiB / 1GiB
ethernova-devnet-db 4.25% 120.9MiB / 23.47GiB
ethernova-devnet-frontend 0.00% 201.7MiB / 23.47GiB
ethernova-devnet-proxy 0.00% 8.441MiB / 23.47GiB
```

Reward display check:

```text
135796 1 [{'reward': '10000000000000000000', 'type': 'Miner Reward'}]
135790 1 [{'reward': '10000000000000000000', 'type': 'Miner Reward'}]
```

DB confirmation for recent devnet blocks:

```text
135796 rewards=1 max_reward=10000000000000000000
135795 rewards=1 max_reward=10000000000000000000
135794 rewards=1 max_reward=10000000000000000000
135793 rewards=1 max_reward=10000000000000000000
135792 rewards=1 max_reward=10000000000000000000
135791 rewards=1 max_reward=10000000000000000000
135790 rewards=1 max_reward=10000000000000000000
```

## Evidence

Detailed evidence logs are under:

```text
reports/phase10/evidence/
```

Key logs:

- `reports/phase10/evidence/phase10b-local-tests-20260511T160906Z.log`
- `reports/phase10/evidence/phase10b-build-20260511T155013Z.log`
- `reports/phase10/evidence/phase10b-deploy-20260511T155517Z.log`
- `reports/phase10/evidence/phase10b-postdeploy-wait-20260511T155832Z.log`
- `reports/phase10/evidence/phase10b-health-20260511T155901Z.log`
- `reports/phase10/evidence/phase10b-public-rpc-20260511T155901Z.log`
- `reports/phase10/evidence/phase10b-phase8-regression-20260511T155901Z.log`
- `reports/phase10/evidence/phase10b-phase9-regression-20260511T155901Z.log`
- `reports/phase10/evidence/phase10b-critical-scan-20260511T160102Z.log`
- `reports/phase10/evidence/phase10b-explorer-vps-20260511T160102Z.log`
- `reports/phase10/evidence/phase10b-remote-hashes-20260511T160102Z.log`
- `reports/phase10/evidence/phase10b-reward-confirmed-20260511T160615Z.log`

Evidence bundle:

```text
reports/phase10/phase10b-devnet-evidence-20260511.tar.gz
```

## Notes For Noven

- Phase 10B is intentionally static and quote-only. This gives developers a real pricing surface without risking a consensus gas change yet.
- Chat/mailbox resource class is isolated by keeping `protocol_ops=1` while storage-heavy dimensions use higher multipliers.
- The deployed binary includes Noven's Phase 7.1 signer/revert-reason fix inherited from `9256bac`.
- All devnet nodes and the VPS RPC/explorer node are updated and running.
- Mainnet was not touched.

## Status

Phase 10B devnet rollout is complete.

Next work: Phase 10C adaptive pricing and congestion-isolation validation, still devnet-only.
