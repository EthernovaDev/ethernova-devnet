# Ethernova v1.2.5 Release Notes

## Highlights
- Hardfork at block 138392 switches chainId from legacy-chainid to 121525 (0x1dab5).
- Pre-fork dual-accept: txs signed with legacy-chainid or 121525 are accepted.
- Post-fork: only chainId 121525 is accepted.
- RPC eth_chainId returns 0x1dab5; scripts default to --networkid 121525.

## Hardfork details
Fork block: **138392**

Old chainId: **legacy-chainid**
New chainId: **121525** (0x1dab5)

## Operator action
**MUST UPGRADE BEFORE BLOCK 138392.**
No genesis re-init or datadir wipe required.

## Notes
- Genesis chainId remains legacy-chainid to avoid config mismatch errors.
- Nodes on v1.2.4 will reject post-fork transactions signed with the new chainId.
