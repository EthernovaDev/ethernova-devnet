# Ethernova Devnet Phase 10C Report - Adaptive Resource Pricing

Date: 2026-05-11
Network: Ethernova devnet only
Chain ID / Network ID: 121526 (`0x1dab6`)
Mainnet touched: no
Feature commit: `265f5a3ca49954d8a2c361faea273143ce42bfe2`
Previous checkpoint: `4a0ca92 docs(devnet): add phase10b rollout report`
Release: `v1.10.2-nip0004.phase10c-adaptive-resource-pricing`
Release URL: https://github.com/EthernovaDev/ethernova-devnet/releases/tag/v1.10.2-nip0004.phase10c-adaptive-resource-pricing

## Summary

Phase 10C is implemented, built, released, deployed, and verified on the full devnet fleet.

This substage adds adaptive per-dimension resource pricing as an RPC/SDK quote and telemetry layer. It validates the NIP-0004 congestion-isolation model without changing consensus gas charging. Extended transaction enforcement remains future scope.

## Implementation

Added:

- Adaptive basis-point price table where `10000 = 1.00x`.
- Per-dimension base prices inherited from Phase 10B:
  - `compute = 10000`
  - `state_read = 20000`
  - `state_write = 40000`
  - `protocol_ops = 10000`
  - `proof_verify = 30000`
- EIP-1559-style per-block price movement capped at `12.5%` per dimension.
- Dimension-isolated updates: each price only reacts to its own usage/target ratio.
- `nova_resourceCongestion` RPC for controller snapshots.
- `nova_quoteResourceFee` now quotes with current adaptive basis-point prices.
- SDK helper: `NovaProvider.resourceCongestion()`.
- Unit tests for adaptive quote math and congestion isolation.
- Public Phase 10 validation script upgraded to Phase 10C.

Important safety scope:

- `consensusGasChanged=false`.
- Legacy gas charging is unchanged.
- No receipt/header/state-root format changes.
- No extended transaction format yet.
- Empty blocks do not decay prices below the Phase 10B base table.
- `protocol_ops` has an independent controller, so compute/storage pressure does not raise chat/mailbox quotes.

## Build Artifacts

Linux devnet binary:

```text
6f19d2bc9651c947ddbd6c8b37b6de438210682ec3bc74bf470b3e446681055d  dist/ethernova-phase10c-devnet-linux-amd64
```

Windows devnet binary:

```text
cf76d4a84c83fe3ab8c28987369f1dd0c172cadbed83b38e74050398179ce994  dist/ethernova-phase10c-devnet-windows-amd64.exe
```

## Deployment

The Linux binary with SHA `6f19d2bc9651c947ddbd6c8b37b6de438210682ec3bc74bf470b3e446681055d` was deployed to:

- `novanode1@192.168.1.15`
- `novanode2@192.168.1.34`
- `novanode3@192.168.1.134`
- `novanode4@192.168.1.16`
- VPS RPC/explorer node `root@207.180.230.125`

Remote hash verification:

```text
node1 6f19d2bc9651c947ddbd6c8b37b6de438210682ec3bc74bf470b3e446681055d
node2 6f19d2bc9651c947ddbd6c8b37b6de438210682ec3bc74bf470b3e446681055d
node3 6f19d2bc9651c947ddbd6c8b37b6de438210682ec3bc74bf470b3e446681055d
node4 6f19d2bc9651c947ddbd6c8b37b6de438210682ec3bc74bf470b3e446681055d
vps   6f19d2bc9651c947ddbd6c8b37b6de438210682ec3bc74bf470b3e446681055d
```

VPS service:

- Service: `ethernova-devnet.service`
- Internal RPC: `127.0.0.1:28545`
- Public RPC: `https://devrpc.ethnova.net`
- Explorer: `https://devexplorer.ethnova.net`
- Archive mode preserved: `--gcmode archive`

## Local Validation

Command log: `reports/phase10/evidence/phase10c-local-tests-20260511T163454Z.log`

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
 Phase 10C - Adaptive Multi-Dimensional Resource Pricing
========================================================================
RPC: https://devrpc.ethnova.net
[PASS] eth_chainId is Ethernova devnet - 0x1dab6
[PASS] nova_resourceConfig adaptive pricing - 10C compute,state_read,state_write,protocol_ops,proof_verify
[PASS] nova_resourcePrices adaptive bips - block=135957 computeBips=10000 protocolOpsBips=10000
[PASS] legacy gasLimit maps to resource vector - {"compute":3000000,"stateRead":1000000,"stateWrite":500000,"protocolOps":200000,"proofVerify":100000}
[PASS] nova_quoteResourceFee applies adaptive prices - total=1450
[PASS] nova_resourceCongestion exposes isolated controller - block=135957
[PASS] nova_developerTooling advertises Phase 10 methods - 19 methods
------------------------------------------------------------------------
RESULT: 7 pass, 0 fail
```

Regression tests after deploy:

```text
Phase 8 RPC tooling: 12 pass, 0 fail, 0 warn
Phase 9 chat proving ground: 12 pass, 0 fail, 0 warn
```

## Consensus / Node Health

Final devnet health check:

```text
Devnet health check 2026-05-11T16:36:59Z
node1  chainId=121526 net=121526 head=135976 hash=0x2d82554eb0d6af78ae346eab431382d0c878f28de781c0857dee2922dd391606 peers=4
node2  chainId=121526 net=121526 head=135976 hash=0x2d82554eb0d6af78ae346eab431382d0c878f28de781c0857dee2922dd391606 peers=4
node3  chainId=121526 net=121526 head=135976 hash=0x2d82554eb0d6af78ae346eab431382d0c878f28de781c0857dee2922dd391606 peers=4
node4  chainId=121526 net=121526 head=135976 hash=0x2d82554eb0d6af78ae346eab431382d0c878f28de781c0857dee2922dd391606 peers=3
devrpc chainId=121526 net=121526 head=135976 hash=0x2d82554eb0d6af78ae346eab431382d0c878f28de781c0857dee2922dd391606 peers=3
common_hash_match=PASS
```

Phase 9 regression also checked a later consensus target:

```text
target=0x21314 node1=node2=node3=node4=devrpc hash 0x0e18d731e11c78d6f680c2ff5fdefb3b89f1df20e0c6ea494f4f9da806fa66ed
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

CPU after timer settled:

```text
ethernova-devnet-backend 22.52% 294.5MiB / 1GiB
ethernova-devnet-db 9.98% 122.4MiB / 23.47GiB
ethernova-devnet-frontend 0.00% 201.7MiB / 23.47GiB
ethernova-devnet-proxy 0.00% 8.441MiB / 23.47GiB
```

Reward display check after cache/backfill tick:

```text
135978 1 [{'reward': '10000000000000000000', 'type': 'Miner Reward'}]
135976 1 [{'reward': '10000000000000000000', 'type': 'Miner Reward'}]
```

DB confirmation:

```text
135978 rewards=1 max_reward=10000000000000000000
135977 rewards=1 max_reward=10000000000000000000
135976 rewards=1 max_reward=10000000000000000000
```

## Evidence

Detailed evidence logs are under:

```text
reports/phase10/evidence/
```

Key logs:

- `reports/phase10/evidence/phase10c-local-tests-20260511T163454Z.log`
- `reports/phase10/evidence/phase10c-build-20260511T162600Z.log`
- `reports/phase10/evidence/phase10c-deploy-20260511T162837Z.log`
- `reports/phase10/evidence/phase10c-postdeploy-wait-20260511T163211Z.log`
- `reports/phase10/evidence/phase10c-health-20260511T163236Z.log`
- `reports/phase10/evidence/phase10c-public-rpc-20260511T163236Z.log`
- `reports/phase10/evidence/phase10c-phase8-regression-20260511T163236Z.log`
- `reports/phase10/evidence/phase10c-phase9-regression-20260511T163236Z.log`
- `reports/phase10/evidence/phase10c-remote-hashes-20260511T163454Z.log`
- `reports/phase10/evidence/phase10c-critical-scan-20260511T163454Z.log`
- `reports/phase10/evidence/phase10c-final-health-20260511T163659Z.log`
- `reports/phase10/evidence/phase10c-explorer-vps-20260511T163629Z.log`
- `reports/phase10/evidence/phase10c-reward-confirmed-20260511T164030Z.log`

Evidence bundle:

```text
reports/phase10/phase10c-devnet-evidence-20260511.tar.gz
```

## Notes For Noven

- Phase 10C validates adaptive resource pricing as an operational quote layer, not as consensus gas enforcement.
- The controller uses independent per-dimension usage/target ratios, which is the important congestion-isolation property for NIP-0004.
- Empty devnet blocks do not decay prices below the Phase 10B base table, keeping quotes stable during low traffic.
- Extended transaction format and enforced per-dimension limits remain pending.
- All devnet nodes and the VPS RPC/explorer node are updated and running.
- Mainnet was not touched.

## Status

Phase 10C devnet rollout is complete.

Next work: Phase 11 application-layer precompiles. Address assignment must be reviewed before coding because the draft Phase 11 addresses for identity/social overlap earlier NIP-0004 precompiles.
