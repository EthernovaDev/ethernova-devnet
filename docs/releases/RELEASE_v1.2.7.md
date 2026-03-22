# Ethernova v1.2.7 Release

## Overview
Ethernova v1.2.7 is a mandatory update that embeds the mainnet genesis snapshot and verifies it at startup. Nodes on older versions are rejected by the P2P version gate.

## Downloads
- ethernova-v1.2.7-full-windows-amd64.zip
- ethernova-v1.2.7-full-linux-amd64.tar.gz

Checksums: `SHA256SUMS.txt`

## Windows
1) Extract the zip.
2) Verify:
   `ethernova.exe version`
3) Start the node:
   `ethernova.exe --config .\\ethernova.toml`

## Linux
1) Extract the tarball:
   `tar -xzf ethernova-v1.2.7-full-linux-amd64.tar.gz`
2) Verify:
   `./ethernova version`
3) Start the node:
   `./ethernova --config ./ethernova.toml`

## Notes
- If you see WRONG GENESIS, delete the datadir and re-init with `genesis-mainnet.json`.
- RPC binds to localhost by default.
