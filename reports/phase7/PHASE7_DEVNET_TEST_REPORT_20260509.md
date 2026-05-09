# Ethernova Devnet Phase 7 Test Report

Date: 2026-05-09
Scope: NIP-0004 Phase 7 Session / Channel Primitive, devnet only.
Commit: `3cefca9 feat(devnet): add phase7 session arbiter`
Release: `v1.8.0-nip0004.phase7-devnet-session`
Release URL: https://github.com/EthernovaDev/ethernova-devnet/releases/tag/v1.8.0-nip0004.phase7-devnet-session

## Build Artifacts

Linux amd64:
`ethernova-phase7-devnet-linux-amd64`
SHA256: `7ff3a676770081454a334e4f69a883d1a65ac0ab91c264dc118ed5d725126053`

Windows amd64:
`ethernova-phase7-devnet-windows-amd64.exe`
SHA256: `c783b95e13b89d6e60be965f43de8f9360155f493f344f307f304647261cdab9`

## Local Test Commands

```text
$ git diff --check
PASS

$ go test ./core/types ./core/vm ./p2p/sessionrelay -count=1
ok   github.com/ethereum/go-ethereum/core/types
ok   github.com/ethereum/go-ethereum/core/vm
ok   github.com/ethereum/go-ethereum/p2p/sessionrelay

$ go test ./core -run 'TestDeferredProcessing' -count=1
ok   github.com/ethereum/go-ethereum/core
```

Full raw log: `reports/phase7/logs-20260509/local-targeted-tests.log`

## Live Devnet Phase 7 Precompile Test

RPC: `https://devrpc.ethnova.net`
Precompile: `0x2D novaSessionArbiter`

```text
initiator=0x246Cbae156Cf083F635C0E1a01586b730678f5Cb
counterparty=0x6f38499625F2FEbD72DB0af0A6C25ee3ce115ec5
openSession tx=0x9911761cd97c09cb31bbac6eec211ed5ac22a83ad053959779f50a34549c4a5b
openSession status=1 block=121596 gasUsed=66944
actualSession=0x2ef384f151715ca258977673312fee2000c43762816f6f5a74b2c8f7011f6715
commitState tx=0xf3bb0f08c02776a6a571eac45225cd8f47aa3a5d6b361d64d6cd120db13ba69c
commitState status=1 block=121599 gasUsed=100376
getSession status=1 seq=1
stateHash=0xb53ed6aec26bc417f14d181ea21ba6fef0ea79f4f18d8fb1864b5f6ef4e52198
timeout=121716
```

Result: `openSession`, bilateral-signed `commitState`, and `getSession` all passed on public devnet RPC.

## 500+ Block Consensus Soak

Start block: `121632`
Target block: `122132`
Final checked block: `122196`
Blocks covered: `564`

Final health:

```text
node1  chainId=121526 net=121526 head=122196 hash=0x805a5c38ea16e5b709c714b60b631ddf67a2061f747d0cf17ae101f759b2accf peers=4
node2  chainId=121526 net=121526 head=122196 hash=0x805a5c38ea16e5b709c714b60b631ddf67a2061f747d0cf17ae101f759b2accf peers=4
node3  chainId=121526 net=121526 head=122196 hash=0x805a5c38ea16e5b709c714b60b631ddf67a2061f747d0cf17ae101f759b2accf peers=4
node4  chainId=121526 net=121526 head=122196 hash=0x805a5c38ea16e5b709c714b60b631ddf67a2061f747d0cf17ae101f759b2accf peers=3
devrpc chainId=121526 net=121526 head=122196 hash=0x805a5c38ea16e5b709c714b60b631ddf67a2061f747d0cf17ae101f759b2accf peers=3
common_hash_match=PASS
```

Explorer:

```text
finished_indexing=true
indexed_blocks_ratio=1.00
```

Latest raw health log: `reports/phase7/logs-20260509/devnet-health-final.log`

## BAD BLOCK / State Root Scan

Searched node1/node2/node3/node4/VPS logs for:

```text
BAD BLOCK
state root mismatch
invalid merkle
```

Result: no matches.

Raw scan log: `reports/phase7/logs-20260509/bad-block-scan.log`

## Known Non-Phase-7 Note

A broad `go test ./core` still reports existing EIP gas expectation failures:

```text
TestEIP2718Transition
TestEIP1559Transition
TestEIP3651
```

Those failures are outside the Phase 7 Session path. The Phase 7 targeted tests and live devnet validation passed.
