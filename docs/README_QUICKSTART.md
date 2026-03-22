# README_QUICKSTART.md

## Ethernova v1.2.7 Quickstart (Windows + Linux)

## Release status
- Current release: v1.2.7 (only supported version).
- Archived releases (deprecated): v1.2.6 and older. See `docs/archive/RELEASE-NOTES_v1.2.6.md` and `docs/archive/RELEASE_NOTES_v1.2.5.md`.

Mainnet chainId/networkId: **121525** (0x1dab5). Genesis is embedded and verified at startup.

---

## Quick start (no scripts required)

✅ No scripts required (scripts are optional helpers).

### 1) Download and extract
- Windows: download the release ZIP and extract anywhere.
- Linux: download the tar.gz and extract (e.g., `tar -xzf ethernova-v1.2.7-full-linux-amd64.tar.gz`).

### 2) Verify the binary
- Windows: `.\ethernova.exe version`
- Linux: `./ethernova version`

### 3) Optional offline validation
- Windows: `.\ethernova.exe print-genesis`
- Linux: `./ethernova print-genesis`

If you want to validate a specific datadir without starting networking:
- Windows: `.\ethernova.exe sanitycheck --datadir .\data-mainnet`
- Linux: `./ethernova sanitycheck --datadir ./data-mainnet`

### 4) Create a minimal config (ethernova.toml)
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

### 5) Start the node
- Windows: `.\ethernova.exe --config .\ethernova.toml`
- Linux: `./ethernova --config ./ethernova.toml`

Defaults:
- Datadir: `data-mainnet` (per the config above; otherwise defaults to `./data`)
- Logs: `./logs/ethernova.log` by default (override with `--log.file` or in config)

### Optional: explicit init
Only needed if you want to seed a datadir manually with a local genesis file:
- Windows: `.\ethernova.exe init --datadir .\data-mainnet .\genesis-mainnet.json`
- Linux: `./ethernova init --datadir ./data-mainnet ./genesis-mainnet.json`

---

## Optional helper scripts (convenience only)
These scripts are not required for normal operation; they just wrap the same commands.
- Mainnet run helpers: `scripts/run-mainnet-node.ps1`, `scripts/run-mainnet-node.sh`
- Update helpers: `scripts/update.ps1`, `scripts/update.sh`, `scripts/update-1.2.7.*`
- Devnet smoke tests: `scripts/run-devnet-test.*`

---

## Devnet (optional)
- Devnet chainId/networkId: **177778** (devnet only, never use for mainnet).
- Devnet keys are public; do not reuse on mainnet.
