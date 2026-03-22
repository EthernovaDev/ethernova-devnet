# Ethernova Hardfork 1.2.5 (CHAINID_SWITCH_125)

## Summary
This hardfork switches the transaction chainId from legacy-chainid to 121525 at block 138392. The
change is applied at runtime to avoid genesis/config mismatches and does not require a
chain reset.

## Fork block
CHAIN_ID_SWITCH_BLOCK = 138392

## Behavior
- Pre-switch (< 138392): accept txs with chainId legacy-chainid or 121525.
- Post-switch (>= 138392): accept only chainId 121525.
- RPC eth_chainId returns 0x1dab5.

## Why this fork
- The new chainId isolates the network identity and avoids replay with the old chainId.
- Runtime switch keeps datadirs intact and prevents config mismatch errors.

## Operator action (required)
**MUST UPGRADE BEFORE BLOCK 138392.**
No genesis re-init required. Update scripts and services to use `--networkid 121525`.

## FAQ
**Do I need to wipe or re-init?**
No. Genesis and stored chain config remain unchanged.
