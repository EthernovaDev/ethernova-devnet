# RELEASE_NOTES_FORK60000.md

## Ethernova Fork 60000 (Modern EVM Upgrade)

### Summary
- Activates modern EVM opcodes and gas rules needed for Solidity 0.8.20+, UniswapV2-style DEX deployments, and NFT tooling.
- Fixes failed deployments caused by missing CHAINID and CREATE2 support.

### Activation
- Fork height: **60000** (block-based activation).

### Changes
**Minimal DEX compatibility**
- EIP-1014 (CREATE2)
- EIP-1344 (CHAINID)
- EIP-145 (SHL/SHR/SAR)
- EIP-1052 (EXTCODEHASH)
- EIP-152 (BLAKE2F precompile)
- EIP-1108 (alt_bn128 precompile gas reductions)
- EIP-1884 (opcode repricing)
- EIP-2028 (calldata gas reduction)
- EIP-2200 (SSTORE gas changes)

**Modern Solidity (Shanghai/Cancun opcodes)**
- EIP-3198 (BASEFEE opcode)
- EIP-3651 (warm COINBASE)
- EIP-3855 (PUSH0)
- EIP-3860 (initcode limits)
- EIP-1153 (TLOAD/TSTORE transient storage)
- EIP-5656 (MCOPY)
- EIP-6780 (SELFDESTRUCT semantics)

### Excluded (PoW safety)
- PoS/Merge fields (terminalTotalDifficulty, terminalTotalDifficultyPassed) remain unset.
- Beacon/withdrawal/blob features not enabled: EIP-4895, EIP-4788, EIP-4844, EIP-7516.

### Action Required (Nodes/Miners)
- Upgrade node config before block 60000 using:
  `ethernova --datadir <your-datadir> init genesis-upgrade-60000.json`
- Restart your node after the config update.

### Deadline Recommendation
- Complete the upgrade before block 59000 to avoid a chain split at 60000.
