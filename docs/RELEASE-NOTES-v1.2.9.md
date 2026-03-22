# Ethernova v1.2.9 (Chain 121525)

## Summary
- Adds EIP-658 receipt status at block **110500** to improve compatibility with exchanges and explorers.
- Keeps the EVM compatibility fork (Constantinople/Petersburg/Istanbul) at block **105000**.
- No genesis reset; chainId/networkId remain **121525**.

## Why this update matters
Some tools assume Byzantium/EIP-658 receipts include a `status` field. Without it, transactions appear “missing status”. This release schedules EIP-658 so receipts include `status` from block **110500** onward.

## Activation
- Chain: **Ethernova mainnet (121525 / 0x1dab5)**
- Genesis hash: **0xc3812eb81498965a3f9ff3e73d2f423934e6d440578d4f4fbb6623cc61c453d9**
- EIP-658 activation block: **110500**

## Compatibility notes
- Receipts **before** block 110500 will not include `status` (expected).
- Receipts **at/after** block 110500 include `status`.

## Mandatory upgrade
All RPC/Archive/Explorer nodes must upgrade **before block 110500**.

## Assets
- `ethernova-v1.2.9-linux-amd64`
- `ethernova-v1.2.9-windows-amd64.exe` (if built)
- `SHA256SUMS-v1.2.9.txt`

## Docs
- `docs/UPGRADE-v1.2.9.md`
