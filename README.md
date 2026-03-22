# Ethernova (CoreGeth fork)

[![Windows Build](https://github.com/EthernovaDev/ethernova-coregeth/actions/workflows/windows.yml/badge.svg)](https://github.com/EthernovaDev/ethernova-coregeth/actions/workflows/windows.yml)

**Ethernova is a Windows-focused fork of CoreGeth** built to run the Ethernova EVM network with **Ethash PoW**, embedded genesis verification, and optional helper tooling.

> For node operators, pool operators (Miningcore), and devs who need an Ethernova-compatible execution client on Windows.

---

## MANDATORY UPDATE (v1.2.7)
- Fork enforcement at block **138,396** requires chainId **121525** for protected transactions.
- Peers running < **v1.2.7** are rejected at handshake; older clients cannot mine valid blocks after the fork.
- Network chainId/networkId remain **121525**.

---

## Release status
- Current release: **v1.2.7** (only supported version).
- Archived releases (deprecated): **v1.2.6** and older. See `docs/archive/RELEASE-NOTES_v1.2.6.md` and `docs/archive/RELEASE_NOTES_v1.2.5.md`.

---

## Quick start (no scripts required)

Mainnet chainId/networkId: **121525** (0x1dab5). Genesis is embedded and verified at startup.

### Windows
1) Download the release ZIP and extract anywhere.
2) Verify version: `.\ethernova.exe version` (should show `v1.2.7`).
3) Optional offline validation:
   - `.\ethernova.exe print-genesis`
   - `.\ethernova.exe sanitycheck --datadir .\data-mainnet`
4) Create `ethernova.toml` (example):
   ```toml
   [Node]
   DataDir = "data-mainnet"
   HTTPHost = "127.0.0.1"
   HTTPPort = 8545
   HTTPModules = ["eth","net","web3"]
   WSHost = "127.0.0.1"
   WSPort = 8546
   WSModules = ["eth","net","web3"]

   [Eth]
   NetworkId = 121525
   ```
5) Start (Windows): `.\ethernova.exe --config .\ethernova.toml`

Logs go to `.\logs\ethernova.log` by default; datadir defaults to `.\data` if not set.

### Linux
1) Download the tar.gz, extract, and `chmod +x ./ethernova` if needed.
2) Verify version: `./ethernova version`.
3) Optional offline validation:
   - `./ethernova print-genesis`
   - `./ethernova sanitycheck --datadir ./data-mainnet`
4) Use the same `ethernova.toml` as above.
5) Start (Linux): `./ethernova --config ./ethernova.toml`

Optional explicit init (only if you want to seed a datadir manually):
- `.\ethernova.exe init --datadir .\data-mainnet .\genesis-mainnet.json`
- `./ethernova init --datadir ./data-mainnet ./genesis-mainnet.json`

---

## What you get

- **Ethash PoW** execution client for Ethernova
- **Ethernova genesis** (mainnet + dev) and init tooling
- **Base fee vault redirection** (project feature; see docs)
- **Optional helper scripts** for build/run/verification/smoke tests
- **RPC smoke tests** for quick validation (chainId/genesis/getWork)

---

## Before you begin

### Requirements (Windows)
- Windows 10/11 x64
- Go 1.21 (per CI); install via `actions/setup-go` equivalent locally
- Build tools: MSYS2 mingw-w64 (mingw-w64-x86_64-gcc/make/pkgconf)
- Disk: dev is small; mainnet grows over time

> Toolchain specifics live in `docs/DEV.md` and CI; helper scripts are optional.

---

## Networks

| Network            | chainId | networkId | Consensus  | Genesis file            | Block 0 hash                                                |
|--------------------|--------:|----------:|------------|-------------------------|------------------------------------------------------------|
| Ethernova Mainnet  | 121525  | 121525    | Ethash PoW | `genesis-mainnet.json`  | `0xc3812eb81498965a3f9ff3e73d2f423934e6d440578d4f4fbb6623cc61c453d9` |
| Ethernova Dev      | 77778   | 77778     | Ethash PoW | `genesis-dev.json`      | (derive via verify script after init)                      |

---
Note: `genesis-mainnet.json` encodes chainId 121525. v1.2.9 keeps the EVM compatibility fork (Constantinople/Petersburg/Istanbul) at block 105000 and schedules EIP-658 (receipt status) at block 110500 without a re-init.

Current mainnet fork schedule (chainId 121525):
- Block 105000: Constantinople + Petersburg + Istanbul (EVM opcodes for SHL/CHAINID/SELFBALANCE)
- Block 110500: EIP-658 receipt status

Upgrade guidance:
- `docs/UPGRADE-v1.2.9.md`
- `docs/RELEASE-NOTES-v1.2.9.md`

## Mining / pool mode (optional)

RPC should remain on localhost; run Miningcore on the same host or via SSH tunnel.

Example (Windows):
```
.\ethernova.exe --config .\ethernova.toml --mine --miner.etherbase 0xPOOL_ADDRESS
```

Example (Linux):
```
./ethernova --config ./ethernova.toml --mine --miner.etherbase 0xPOOL_ADDRESS
```

Expected:
- `eth_chainId` == 0x1dab5 (121525)
- Genesis/block0 matches fingerprint
- `eth_getWork` responds when mining/getWork is enabled

---

## Default endpoints & ports
- HTTP RPC: `http://127.0.0.1:8545`
- WS RPC: `ws://127.0.0.1:8546` (or HttpPort+1)
- P2P: `30303`
- Data dir: `data\` by default (override in config)
- Logs: `logs\ethernova.log` by default (override with `--log.file`)

---

## Bootnodes
Mainnet bootnodes (enode): add stable entries in `networks/mainnet/bootnodes.txt` and `static-nodes.json`.
> Provide at least 2-5 stable bootnodes before launch.

---

## Documentation
- Launch & operations: `docs/LAUNCH.md`
- Dev workflow: `docs/DEV.md`
- Config reference: `docs/CONFIG.md`
- Optional helper scripts (not required): `scripts/run-mainnet-node.ps1`, `scripts/test-rpc.ps1`, `scripts/init-ethernova.ps1`, `scripts/verify-mainnet.ps1`, `scripts/smoke-test-fees.ps1`

---

## Troubleshooting
- RPC works but `eth_getWork` is null: ensure `-Mine`/getWork enabled; hit `http://127.0.0.1:8545`.
- Genesis mismatch: run `ethernova sanitycheck --datadir <path>` and re-init with the correct genesis if needed.

---

## Contributing
PRs welcome for Ethernova-specific changes (scripts, docs, chain config, ops hardening). Keep upstream-friendly changes isolated.

---

## Upstream / Credits
Ethernova is a fork of CoreGeth, downstream of `ethereum/go-ethereum`.
- CoreGeth: https://github.com/etclabscore/core-geth
- go-ethereum: https://github.com/ethereum/go-ethereum

---

## Licensing
- Library code (outside `cmd/`): GNU LGPL-3.0-or-later
- Binaries under `cmd/`: GNU GPL-3.0-or-later

See `LICENSE`, `COPYING`, and `COPYING.LESSER`.
