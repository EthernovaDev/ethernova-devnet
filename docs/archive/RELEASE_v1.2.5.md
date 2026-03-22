# Ethernova v1.2.5 Release

## Overview
Ethernova v1.2.5 schedules a hard fork at block **138392** that switches the transaction chainId
from legacy-chainid to 121525 (0x1dab5). The switch is runtime-only; genesis and stored config remain
unchanged, so no re-init is required.

**MUST UPGRADE BEFORE BLOCK 138392.**

## Behavior
- Blocks < 138392: accepts txs signed with chainId legacy-chainid or 121525.
- Blocks >= 138392: accepts only chainId 121525.
- RPC eth_chainId returns 0x1dab5.

## Downloads
- ethernova-windows-amd64-v1.2.5.zip
- ethernova-linux-amd64-v1.2.5.tar.gz

Checksums: `checksums-sha256.txt`

## Windows
1) Extract the zip.
2) One-click update (no data wipe):
   `update-1.2.5.bat` or `update.bat`
3) Start the node:
   `run-node.bat`

## Linux
1) Extract the tarball:
   `tar -xzf ethernova-linux-amd64-v1.2.5.tar.gz`
2) One-click update (no data wipe):
   `./update-1.2.5.sh` or `./update.sh`
3) Start the node:
   `./scripts/run-mainnet-node.sh`

## Notes
- Do NOT replace the genesis file inside your datadir.
- Set `--networkid 121525` to avoid old peers (scripts do this by default).
