Ethernova v1.2.7 - Windows Bundle

Quick start
1) Extract the zip to a folder.
2) Verify:
   - ethernova.exe version
3) Optional offline check:
   - ethernova.exe print-genesis
   - ethernova.exe sanitycheck --datadir .\data-mainnet
4) Create a config file (ethernova.toml). See docs/README_QUICKSTART.md for a minimal example.
5) Start the node:
   - ethernova.exe --config .\ethernova.toml

Defaults
- Data dir: data-mainnet (per config; otherwise ./data)
- HTTP RPC: 127.0.0.1:8545
- WS RPC: 127.0.0.1:8546
- Logs: .\logs\ethernova.log

Update (no data wipe)
- Download the new release asset and replace the binary in place.

Important
- Upgrade BEFORE block 105000 (Constantinople/Petersburg/Istanbul fork).
- Do NOT replace the genesis file inside your datadir.
- Bootnodes can be passed with --bootnodes (optional).
