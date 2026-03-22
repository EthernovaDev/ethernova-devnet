# Ethernova v1.2.4 Release

## Overview
Ethernova v1.2.4 schedules a new hard fork at block **70000** to activate the missing Byzantium base package. This fixes contract-to-contract view calls by enabling STATICCALL (EIP-214) and related base EVM rules.

**MUST UPGRADE BEFORE BLOCK 70000.**

## Activated EIPs at block 70000
EIP-2, EIP-7, EIP-100, EIP-150, EIP-160, EIP-161, EIP-170, EIP-140,
EIP-198, EIP-211, EIP-212, EIP-213, EIP-214, EIP-658.

## Downloads
- ethernova-windows-amd64-v1.2.4.zip
- ethernova-linux-amd64-v1.2.4.tar.gz

Checksums: `checksums-sha256.txt`

## Windows
1) Extract the zip.
2) Run the config upgrade (no chain reset):
   `scripts\apply-upgrade-mainnet.bat`
3) Start the node:
   `run-node.bat`
4) One-click update:
   `update.bat`

## Linux
1) Extract the tarball:
   `tar -xzf ethernova-linux-amd64-v1.2.4.tar.gz`
2) Run the config upgrade (no chain reset):
   `./scripts/apply-upgrade-mainnet.sh`
3) Start the node:
   `./scripts/run-mainnet-node.sh`
4) One-command update:
   `./update.sh`

## Notes
- Do NOT replace the genesis file inside your datadir.
- This upgrade does not change consensus (Ethash PoW remains).
 - Dry run update: set `DRY_RUN=1` when running update scripts.
