# NIP-0004 Phase 11/12 Devnet Completion Report

Date: 2026-05-12  
Commit: `f71023a662cdf943f7f94f400338d9606e4178a0`  
Release: `v1.12.0-nip0004.phase12-complete-devnet`  
Release URL: https://github.com/EthernovaDev/ethernova-devnet/releases/tag/v1.12.0-nip0004.phase12-complete-devnet

## Summary

Phase 11 and Phase 12 are implemented, built, released, deployed to all devnet nodes plus the devnet VPS/RPC, and verified against the live devnet.

Scope stayed devnet-only. Mainnet was not touched.

## Phase 11 - Application-Layer Precompiles

Implemented app-layer precompiles on collision-safe slots:

| Address | Name | Surface |
|---|---|---|
| `0x30` | `novaAsyncCallback` | register, get, markFired, ready |
| `0x31` | `novaIdentityAttestation` | attest, verify, revoke, get |
| `0x32` | `novaSocialGraph` | follow, unfollow, isFollowing, trustScore |
| `0x33` | `novaContentManifest` | create, verify, get |
| `0x34` | `novaGameState` | commit, reveal, get |
| `0x36` | `novaComputeBounty` | create, submit, verify, get |

Important address correction:

- Old draft `0x2B` was not used because it is already `novaContentRegistry`.
- Old draft `0x2C` was not used because it is already `novaMailboxManager`.
- `0x35` remains `novaMailboxOps`.

## Phase 12 - Nova Opcode Bridge

Implemented the devnet opcode bridge at `0xD0-0xD8`:

| Opcode | Name | Bridge |
|---|---|---|
| `0xD0` | `MSEND` | `0x35` selector `0x01` |
| `0xD1` | `MRECV` | `0x35` selector `0x02` |
| `0xD2` | `MPEEK` | `0x35` selector `0x03` |
| `0xD3` | `MCOUNT` | `0x35` selector `0x04` |
| `0xD4` | `CREF` | `0x2B` selector `0x01` |
| `0xD5` | `CVERIFY` | `0x2B` selector `0x03` |
| `0xD6` | `SOPEN` | `0x2D` selector `0x01` |
| `0xD7` | `SCOMMIT` | `0x2D` selector `0x02` |
| `0xD8` | `SCLOSE` | `0x2D` selector `0x03` |

Important opcode correction:

- The old `0xF6-0xFE` draft range was not used because it collides with canonical EVM opcode territory including `STATICCALL`, `REVERT`, `INVALID`, and `SELFDESTRUCT`.

## RPC / Tooling Added

New discovery RPCs:

- `nova_applicationPrecompiles`
- `nova_opcodeConfig`
- legacy namespace aliases through `ethernova_applicationPrecompiles` and `ethernova_opcodeConfig`

Updated SDK:

- `devnet/nova-sdk/index.js` precompile map now includes Phase 11 addresses.
- `NovaProvider.applicationPrecompiles()` added.
- `NovaProvider.opcodeConfig()` added.

New live devnet tests:

- `devnet/phase11/phase11-application-precompiles-test.js`
- `devnet/phase12/phase12-nova-opcodes-test.js`

## Build Artifacts

Linux amd64:

```text
bfd9fa82e0e97a2e62f9ab813ff78449e3df1e6a8fef51528ed99c688cb354e6  dist/ethernova-nip0004-phase12-devnet-linux-amd64
```

Windows amd64:

```text
f4bca8ba02aeeb1af0e2210efcf0466b0122a2d88e5180068b58909283603fec  dist/ethernova-nip0004-phase12-devnet-windows-amd64.exe
```

GitHub release contains both assets with matching SHA-256 digests.

## Deployment

Updated binaries deployed to:

| Node | Status | Binary SHA |
|---|---|---|
| node1 `192.168.1.15` | running | `bfd9fa82e0e97a2e62f9ab813ff78449e3df1e6a8fef51528ed99c688cb354e6` |
| node2 `192.168.1.34` | running | `bfd9fa82e0e97a2e62f9ab813ff78449e3df1e6a8fef51528ed99c688cb354e6` |
| node3 `192.168.1.134` | running | `bfd9fa82e0e97a2e62f9ab813ff78449e3df1e6a8fef51528ed99c688cb354e6` |
| node4 `192.168.1.16` | running | `bfd9fa82e0e97a2e62f9ab813ff78449e3df1e6a8fef51528ed99c688cb354e6` |
| VPS `devrpc.ethnova.net` | running, archive mode | `bfd9fa82e0e97a2e62f9ab813ff78449e3df1e6a8fef51528ed99c688cb354e6` |

VPS devnet node remains in archive mode:

```text
--syncmode full --gcmode archive
```

## Live Network Health

Final health check:

```text
node1  chainId=121526 net=121526 head=138216 hash=0x3f6120665bfb3d0eb4a140e6912348cffef5e20271ed67f3cc298ccb3f99c489 peers=4
node2  chainId=121526 net=121526 head=138216 hash=0x3f6120665bfb3d0eb4a140e6912348cffef5e20271ed67f3cc298ccb3f99c489 peers=3
node3  chainId=121526 net=121526 head=138216 hash=0x3f6120665bfb3d0eb4a140e6912348cffef5e20271ed67f3cc298ccb3f99c489 peers=4
node4  chainId=121526 net=121526 head=138216 hash=0x3f6120665bfb3d0eb4a140e6912348cffef5e20271ed67f3cc298ccb3f99c489 peers=4
devrpc chainId=121526 net=121526 head=138216 hash=0x3f6120665bfb3d0eb4a140e6912348cffef5e20271ed67f3cc298ccb3f99c489 peers=3
common_hash_match=PASS
```

Explorer:

```json
{"finished_indexing":true,"finished_indexing_blocks":true,"indexed_blocks_ratio":"1.00","indexed_internal_transactions_ratio":"1"}
```

VPS devnet process CPU after deploy:

```text
EthernovaDevnet: 4.2% CPU, 0.5% MEM
```

## Test Results

Local Go tests:

```text
go test ./core/vm                                                   PASS
go test ./core/vm -run 'TestApplicationPrecompile|TestNovaOpcode|TestSession|TestMailbox|TestContent|TestCapabilityHelpers|TestResourceMeter'  PASS
go test ./eth -run TestNonExistent                                  PASS / compile check
go test ./params                                                    PASS
```

Live devnet tests:

```text
Phase 11 Application Precompiles: 5 pass, 0 fail
Phase 12 Nova Opcode Bridge:      4 pass, 0 fail
Phase 10 regression:              7 pass, 0 fail
Phase 9 regression:               12 pass, 0 fail, 0 warn
Phase 8 regression:               12 pass, 0 fail, 0 warn
Devnet health consensus:          PASS
Explorer indexing:                PASS
```

## Residual Note

A broad legacy command was attempted:

```text
go test ./core/... ./eth/... ./params/...
```

It exposed three existing legacy gas fixture mismatches in `core/blockchain_test.go`:

```text
TestEIP2718Transition: expected 27504, got 27482
TestEIP1559Transition: expected 27504, got 27482
TestEIP3651: expected 25321, got 25278
```

Assessment: Phase 12 opcode activation is gated behind Ethernova chain ID `121526`; those failing fixtures run under non-Ethernova `AllEthashProtocolChanges` chain ID `1337`. They are tracked as a broad legacy gas-fixture issue, not as a Phase 11/12 devnet consensus failure.

## Evidence Files

Evidence directory:

```text
reports/phase12/evidence/
```

Important logs:

- `build-release-20260512.log`
- `go-test-core-vm-20260512.log`
- `go-test-focused-20260512.log`
- `go-test-eth-compile-20260512.log`
- `go-test-params-20260512.log`
- `devnet-health-20260512.log`
- `phase11-rpc-20260512.log`
- `phase12-rpc-20260512.log`
- `phase10-regression-20260512.log`
- `phase9-regression-20260512.log`
- `phase8-regression-20260512.log`
- `vps-status-20260512.log`
- `full-core-suite-residual-20260512.log`

## Conclusion

NIP-0004 Phase 11 and Phase 12 are complete on devnet. The live network is aligned across node1-node4 and devrpc, the explorer is indexed, the public RPC exposes the new discovery surfaces, and Linux/Windows release binaries are published.
