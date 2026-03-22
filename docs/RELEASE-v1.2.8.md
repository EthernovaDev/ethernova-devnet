# RELEASE v1.2.8

## Summary

Ethernova v1.2.8 schedules a single EVM compatibility fork at block **105000** for
chainId/networkId **121525**.

This fork enables **Constantinople + Petersburg + Istanbul** EVM rules to add
missing opcodes required by modern Solidity bytecode:

- Constantinople: SHL/SHR/SAR, CREATE2, EXTCODEHASH
- Petersburg: disables the Constantinople EIP-1283 gas bug
- Istanbul: CHAINID (0x46) and SELFBALANCE (0x47), plus Istanbul gas repricing

## Impact

- Contracts using SHL/CHAINID/SELFBALANCE will work after block 105000.
- Old clients will diverge at the fork and be rejected via ForkID mismatch.
- **No genesis re-init is required.**

## Enforcement block (chain-aware)

The client derives the fork enforcement block from the active chain identity:

- chainId **121525** + genesis **0xc3812e...c453d9** => enforcement **105000**
- legacy chainId **77777** (or legacy genesis) => enforcement **138396**
- unknown chains **do not** default to the legacy value

Verify with:

- Windows: `scripts/verify-enforcement-windows.ps1`
- Linux: `scripts/verify-enforcement-linux.sh`

These scripts also check for the startup log line:
`Ethernova fork enforcement block=105,000`

## Mandatory Upgrade

Upgrade **before block 105000**.

## Verification

Use the provided scripts:

- Windows: `scripts/verify-fork-windows.ps1`
- Linux: `scripts/verify-fork-linux.sh`

They confirm SHL fails at block 104999 and succeeds at block 105000, and print
`web3_clientVersion`.
