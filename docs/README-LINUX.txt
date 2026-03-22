Ethernova v1.2.7 - Linux Bundle

Quick start
1) Extract the tarball:
   tar -xzf ethernova-v1.2.7-full-linux-amd64.tar.gz
2) Start the node:
   ./ethernova version
   ./ethernova print-genesis
   ./ethernova --config ./ethernova.toml

Defaults
- Data dir: data-mainnet (per config; otherwise ./data)
- HTTP RPC: 127.0.0.1:8545
- WS RPC: 127.0.0.1:8546
- Logs: ./logs/ethernova.log

Update (no data wipe)
- Download the new release asset and replace the binary in place.

Systemd (optional)
- Example unit file: systemd/ethernova.service

Important
- Upgrade BEFORE block 105000 (Constantinople/Petersburg/Istanbul fork).
- Do NOT replace the genesis file inside your datadir.
- Bootnodes can be passed with --bootnodes (optional).
