# README_NETWORK.md

This folder contains bootstrap peer lists for Ethernova mainnet.

## Where to place files

**bootnodes.txt**
- Used by the `--bootnodes` flag (comma-separated enodes).

**static-nodes.json / trusted-nodes.json**
- If used, place these inside your datadir:
  - Windows: `<datadir>\geth\static-nodes.json` and `<datadir>\geth\trusted-nodes.json`
  - Linux: `<datadir>/geth/static-nodes.json` and `<datadir>/geth/trusted-nodes.json`
- Note: this client logs these files as deprecated and may ignore them. Prefer config.toml (P2P.StaticNodes / P2P.TrustedNodes).

## Flags and examples

**Bootnodes (recommended)**
```
ethernova --datadir <datadir> --bootnodes "enode://<id>@<ip>:30303,enode://<id>@<ip>:30303"
```

**Config file (static/trusted peers)**
```
ethernova --config <path-to-config.toml> --datadir <datadir>
```
Config example:
```
[P2P]
StaticNodes = ["enode://<id>@<ip>:30303"]
TrustedNodes = ["enode://<id>@<ip>:30303"]
```

## One-click scripts

- `scripts/run-mainnet-node.ps1` auto-loads `network/bootnodes.txt` if present.
  - Override with `-Bootnodes` or `-BootnodesFile`.
- `scripts/run-mainnet-node.sh` auto-loads `network/bootnodes.txt` if present.
  - Override with `--bootnodes` or `--bootnodes-file`.

## Ports and NAT

- Default P2P port: `30303` (TCP+UDP). Open/forward both.
- Change port with `--port 30303` and discovery port with `--discovery.port 30303` if needed.
- For NAT, use `--nat extip:YOUR_PUBLIC_IP` (or `--nat any` for automatic mapping).
