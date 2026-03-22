# Ethernova v1.2.4 Release Notes

## Highlights
- Hardfork 1.2.4 at block 70000 activates the missing Byzantium base package.
- Fixes contract-to-contract view calls by enabling STATICCALL (EIP-214).
- Provides updated upgrade genesis and one-click update scripts.

## Hardfork details
Fork block: **70000**

Activated EIPs (all at block 70000):
EIP-2, EIP-7, EIP-100, EIP-150, EIP-160, EIP-161, EIP-170, EIP-140,
EIP-198, EIP-211, EIP-212, EIP-213, EIP-214, EIP-658.

## Operator action
**MUST UPGRADE BEFORE BLOCK 70000.**

Update config in-place (no chain reset):
```
ethernova --datadir <your-datadir> init genesis-upgrade-70000.json
```

## Notes
- This fork only adds missing EVM rules; consensus remains Ethash PoW.
- Existing fork at block 60000 is preserved.
