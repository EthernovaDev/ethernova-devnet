# Ethernova v1.2.7 Release Notes

## Summary
- Embedded mainnet genesis snapshot (chainId/networkId 121525) with startup verification.
- New `sanitycheck` and `print-genesis` commands for offline validation.
- Portable defaults for datadir/logs under the current working directory.
- P2P version gate now requires v1.2.7+.

## Upgrade
- Update binaries to v1.2.7.
- If you see WRONG GENESIS, delete the datadir and re-init with `genesis-mainnet.json` (or rely on the embedded genesis).

## Verification
- `ethernova version` should show `v1.2.7`.
- `eth_chainId` should return `0x1dab5`.
- `net_version` should return `121525`.
- `eth_getBlockByNumber(0).hash` should return `0xc3812eb81498965a3f9ff3e73d2f423934e6d440578d4f4fbb6623cc61c453d9`.
