# Ethernova v1.3.0 (Chain 121525)

## Summary
- **Mega Fork (EVM-compat)** at block **118200**: enables missing historical EVM forks (Homestead, Tangerine Whistle, Spurious Dragon, Byzantium suite, and Petersburg fix EIP-1706).
- **Explicit Constantinople/Istanbul fields** remain at **105000**.
- **EIP-658** receipt status remains at **110500**.
- **London sibling consistency**: `eip3198FBlock`, `eip3529FBlock`, `eip3541FBlock` set to `0` to match `eip1559FBlock=0`.

## Why this update
Modern tooling expects historical forks to be configured explicitly. This release:
- Fixes gas estimation and internal `CALL{value:}` forwarding behavior.
- Aligns wallet/exchange simulation with standard EVM semantics.
- Ensures forkid reflects the mega fork, rejecting incompatible peers.

## Activation
- Chain: **Ethernova mainnet (121525 / 0x1dab5)**
- Genesis hash: **0xc3812eb81498965a3f9ff3e73d2f423934e6d440578d4f4fbb6623cc61c453d9**
- Mega fork block: **118200**
- EIP-658 receipt status: **110500**

## Peer compatibility
ForkID now advertises **118200** as the next fork. Peers with older schedules are dropped during handshake (forkid mismatch).

## Multiverse Safety
- ForkID next = **118200**.
- Incompatible peers are dropped during handshake (forkid mismatch logging enabled).
- Nodes refuse to start if mega-fork fields are missing at/after **118200**.

## Assets
- `ethernova-v1.3.0-linux-amd64`
- `ethernova-v1.3.0-windows-amd64.exe`
- `SHA256SUMS-v1.3.0.txt`

## Docs
- `docs/UPGRADE-v1.3.0.md`
