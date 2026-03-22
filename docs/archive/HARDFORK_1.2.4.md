# Ethernova Hardfork 1.2.4 (FORK_BLOCK_124)

## Summary
This hardfork activates the missing base Byzantium package (including EIP-214 STATICCALL)
to fix contract-to-contract view calls and unblock modern dApps (DEX/NFT/marketplaces).

## Fork block
FORK_BLOCK_124 = 70000

## Activated EIPs at block 70000
- EIP-2: Homestead changes (eip2FBlock)
- EIP-7: DELEGATECALL (eip7FBlock)
- EIP-100: Difficulty adjustment (eip100FBlock)
- EIP-150: Gas repricing (eip150Block)
- EIP-160: EXP cost increase (eip160Block)
- EIP-161: State trie clearing (eip161FBlock)
- EIP-170: Contract code size limit (eip170FBlock)
- EIP-140: REVERT opcode (eip140FBlock)
- EIP-198: Modexp precompile (eip198FBlock)
- EIP-211: RETURNDATA opcodes (eip211FBlock)
- EIP-212: Alt_bn128 pairing precompile (eip212FBlock)
- EIP-213: Alt_bn128 add/mul precompiles (eip213FBlock)
- EIP-214: STATICCALL opcode (eip214FBlock) **critical**
- EIP-658: Receipt status (eip658FBlock)

## Why this fork
Contract-to-contract view calls (e.g., ERC20.balanceOf from another contract) were
failing with out-of-gas because STATICCALL was not active. This fork enables the
missing Byzantium base to restore correct EVM behavior.

## Operator action (required)
**MUST UPGRADE BEFORE BLOCK 70000.**

Use the new upgrade genesis (config only, no chain reset):
```
ethernova --datadir <your-datadir> init genesis-upgrade-70000.json
```

## FAQ
**What happens if I do not upgrade?**
Your node will follow a minority chain after block 70000 (chain split).
