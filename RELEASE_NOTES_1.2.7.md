# RELEASE_NOTES_1.2.7

Ethernova v1.2.7 is a mandatory update that embeds the mainnet genesis snapshot and verifies it at startup.

Highlights:
- Embedded mainnet genesis (chainId/networkId 121525) with hash verification.
- New `sanitycheck` and `print-genesis` commands.
- P2P version gate now requires v1.2.7+.

Verification:
- `ethernova version` should show `v1.2.7`.
- `eth_chainId` should return `0x1dab5`.
- `net_version` should return `121525`.
- `eth_getBlockByNumber(0).hash` should return `0xc3812eb81498965a3f9ff3e73d2f423934e6d440578d4f4fbb6623cc61c453d9`.
