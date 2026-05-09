# Ethernova Devnet Phase 9 Report - NIP-0003 Chat Rebase Proving Ground

Date: 2026-05-09
Network: Ethernova devnet only
Chain ID / Network ID: 121526 (`0x1dab6`)
Mainnet touched: no
Code commit: `12844a28374e`

## Summary

Phase 9 is implemented and deployed as a proving-ground layer that maps NIP-0003 chat onto the NIP-0004 primitives already live on devnet.

This release intentionally does not change `MailboxConfig` RLP or consensus object layout. Chat profile metadata is anchored through ContentRef conventions, direct messages use the Phase 7 `SessionTypeChat` convention, and mailbox notifications use Phase 4 mailbox operations.

## Implementation

Added:

- `nova_chatConfig` / `ethernova_chatConfig`
- `nova_getChatMailbox(owner, offset, limit)` / `ethernova_getChatMailbox(owner, offset, limit)`
- SDK chat helpers for X25519 identity generation, AES-256-GCM payload encryption, deterministic profile/message hashes, and calldata builders.
- Phase 9 proving-ground test script.
- Read-only browser chat harness.

Primitive mapping:

- Chat registry: `nova_getChatMailbox(owner)` mailbox owner lookup.
- Chat profile: canonical JSON with content type `application/ethernova.chat-profile+json`, hash-anchored by ContentRef.
- Direct messages: Phase 7 Session type `Chat` (`sessionTypeCode = 1`) plus encrypted P2P payloads.
- Mailbox notification: `novaMailboxOps.sendMessage(mailboxId, payloadHash, postage)`.
- Message body: canonical JSON with content type `application/ethernova.chat-message+json`, hash-anchored by ContentRef.
- Group chat: Domain 1 ChatRoom fanout convention through mailbox notification hashes and Deferred Processing.

## Build Artifacts

Linux devnet binary:

```text
708c6dce83c59b8ab586d839e33912576a072e626dc1abeaf864c32afcf2c144  dist/ethernova-phase9-devnet-linux-amd64
```

Windows devnet binary:

```text
1ddca5c43454d1ae6c3e1af32ac6a866f48ba5d2cbd9c164114a92edfd0b9b77  dist/ethernova-phase9-devnet-windows-amd64.exe
```

## Deployment

The Linux binary with SHA `708c6dce83c59b8ab586d839e33912576a072e626dc1abeaf864c32afcf2c144` was deployed to:

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

## Validation Logs

Evidence directory:

```text
reports/phase9/evidence/
```

Key results:

```text
node --check devnet/nova-sdk/index.js
node --check devnet/phase9/phase9-chat-proving-ground-test.js
go test ./eth -run '^$'
go test ./cmd/geth -run '^$'
go test ./core/vm -run 'Test(Session|Mailbox|Content|Capability|Inspect|ExecutionDomain)'
```

All passed.

Phase 9 live public RPC test:

```text
========================================================================
 Phase 9 - NIP-0003 Chat Rebase Proving Ground
========================================================================
RPC: https://devrpc.ethnova.net
[PASS] eth_chainId is Ethernova devnet - 0x1dab6
[PASS] nova_chatConfig exposes Phase 9 conventions - NIP-0003 chat rebase onto NIP-0004 primitives: mailbox discovery, ContentRef payload anchors, Session channels, and Domain 1 group fanout.
[PASS] nova_getChatMailbox owner lookup works - owner=0x0000000000000000000000000000000000000000 count=0
[PASS] X25519 chat identity generation - X25519 alice=0xaee17e03c329a72e...
[PASS] Direct message encrypt/decrypt round trip - X25519+AES-256-GCM bytes=24
[PASS] Chat profile ContentRef hash - 0x702c10164edd1636... inputBytes=253
[PASS] Message envelope hash and mailbox send input - 0x10f268c941466740...
[PASS] Chat mailbox create input shape - bytes=257
[PASS] Chat session open input shape - bytes=161
[PASS] Group chat fanout envelopes are deterministic and unique - 0xc98d6520,0xb2a54c11,0xaf423ab7
[PASS] Phase 3/4/7 primitive RPCs are present - pending=0
[PASS] multi-node consensus still aligned - target=0x1e4d3 node1:0x960eda71445093dfe7a9f097b790582c193156a7fea11cb23e1843d68ded096a node2:0x960eda71445093dfe7a9f097b790582c193156a7fea11cb23e1843d68ded096a node3:0x960eda71445093dfe7a9f097b790582c193156a7fea11cb23e1843d68ded096a node4:0x960eda71445093dfe7a9f097b790582c193156a7fea11cb23e1843d68ded096a devrpc:0x960eda71445093dfe7a9f097b790582c193156a7fea11cb23e1843d68ded096a
------------------------------------------------------------------------
RESULT: 12 pass, 0 fail, 0 warn
```

RPC deployment verification:

```text
node1 chain=0x1dab6 block=0x1e4d6/124118 hash=0x3605e35e07a94ca1cf5b195e128fcc2476c66e484f06c591b33d5c3cffa7ebea peers=4 phase=9 toolingHasChat=true
node2 chain=0x1dab6 block=0x1e4d6/124118 hash=0x3605e35e07a94ca1cf5b195e128fcc2476c66e484f06c591b33d5c3cffa7ebea peers=4 phase=9 toolingHasChat=true
node3 chain=0x1dab6 block=0x1e4d6/124118 hash=0x3605e35e07a94ca1cf5b195e128fcc2476c66e484f06c591b33d5c3cffa7ebea peers=4 phase=9 toolingHasChat=true
node4 chain=0x1dab6 block=0x1e4d6/124118 hash=0x3605e35e07a94ca1cf5b195e128fcc2476c66e484f06c591b33d5c3cffa7ebea peers=3 phase=9 toolingHasChat=true
devrpc chain=0x1dab6 block=0x1e4d6/124118 hash=0x3605e35e07a94ca1cf5b195e128fcc2476c66e484f06c591b33d5c3cffa7ebea peers=3 phase=9 toolingHasChat=true
```

Explorer and public RPC:

```text
https://devexplorer.ethnova.net -> HTTP/2 200
https://devrpc.ethnova.net nova_chatConfig -> phase=9, sessionTypeCode=1
```

Post-deploy critical scan:

```text
Patterns: BAD BLOCK|Fatal|panic|ERROR
node1: no critical matches
node2: no critical matches
node3: no critical matches
node4: no critical matches
vps journal since restart: no critical matches
vps devnet log last 250 critical: no critical matches
```

## Notes For Noven

- Phase 9 is a proving-ground/client-tooling release, not a consensus-hardfork.
- No mainnet constants or mainnet activation values were changed.
- `MailboxConfig` RLP remains unchanged to avoid breaking existing mailbox objects.
- The public VPS is still running the normal devnet service in archive mode.
- The previous separate historical archive rebuild datadir remains untouched and is not part of this deployment.

## Status

Phase 9 devnet rollout is complete.

All devnet nodes and the VPS RPC/explorer node are updated and running.
