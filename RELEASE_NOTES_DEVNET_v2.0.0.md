# Ethernova Devnet v2.0.0 — Clean Baseline

**Release:** 2026-04-12
**ChainID:** 121526
**Previous line:** archived as git tag `v1.x-final`

## What this release is

A clean devnet baseline built on top of the finalized Ethernova 2.0 (Noven Fork) mainnet codebase. This release exists so that work on **NIP-0004 (Layered Deterministic Computer)** can start from a fresh, predictable chain that mirrors mainnet behavior exactly — with no leftover state from the v1.x experimental line.

## What changed vs v1.1.10

- All experimental v1.1.x code from the mainnet upgrade path (Adaptive Gas v2 iterations, parallel classifier tuning, slot-level classifier, fast mode, extended precompiles 0x25–0x28, GlobalFairOrdering experiments, state-expiry scaffolding) has been **replaced** with the canonical mainnet 2.0 code. Anything that did not make it into mainnet is archived in tag `v1.x-final`.
- All Noven Fork fork blocks are set to **block 0** on devnet, so the chain starts with the full v2.0.0 feature set active from genesis:
  - Adaptive Gas V2 (trace-based)
  - Per-EVM reentrancy guard
  - Gas refund on revert (90%)
  - Native precompiles 0x20–0x28
  - State expiry (short 1000-block period for testing)
  - Tempo transactions
  - Frame Account Abstraction
- `params/ethernova/` devnet overrides kept:
  - `chainid.go` — ChainID **121526** (vs mainnet 121525)
  - `forks.go` — all fork blocks at **0**, `StateExpiryPeriod = 1000` for fast testing
  - `genesis.go` + `genesis-121526-devnet.json` — embedded devnet genesis
- `params/version.go` — reports `Ethernova-Devnet 2.0.0-devnet`
- `VERSION` file — `v2.0.0-devnet`

## What got preserved

- `devnet/` — 4-node cluster scripts, deployer, faucet, dashboard, benchmark, contracts, hardhat config, local `genesis-devnet.json`
- `devnet-config.toml` and `static-nodes-devnet.json` — VM fleet enodes
- `README.md`, `ROADMAP.md`, `RELEASE_NOTES_DEVNET_v1.0.0.md` — historical devnet docs
- `ethernova_v1.1.1.patch` — historical patch
- All devnet-only infra

## What got removed

- **`EthernovaDevnet-linux-amd64`** and **`EthernovaDevnet.exe`** — the old v1.1.10 binaries are incompatible with this release (different consensus rules, different genesis hash). These must be rebuilt from source after pulling this baseline.

## Upgrade procedure (VM fleet)

**This release breaks the chain.** All devnet nodes must be wiped and restarted with a fresh genesis. The chain starts over from block 0.

On each of the 4 devnet VMs:

```bash
cd ~/ethernova-devnet
git fetch --tags origin
git pull origin main

# Rebuild the node binary (Linux amd64)
make geth
# or: cd cmd/geth && go build -o ../../build/bin/geth .

# Wipe old chain and restart
cd devnet
./stop-all.sh
./reset-all.sh        # deletes data/node{1..4}
./start-all.sh
```

After restart all 4 nodes should begin mining from block 0 with chainId 121526.

## For NIP-0004 development

The devnet is now a clean slate for **Noven** to start implementing NIP-0004 (Layered Deterministic Computer — Protocol Objects, Mailboxes, Protocol Channels, Deferred Execution, Multi-Dimensional Resource Metering). NIP-0003 (Native Chat Protocol) is superseded by NIP-0004 §7.2 and will not be implemented standalone.

## Archived history

Everything from the v1.x line — all 70+ commits from v1.0.0 through v1.1.10, including every experimental feature and consensus fix — is preserved at:

```bash
git checkout v1.x-final
```

Nothing was lost. This baseline is additive: the old history lives on the tag, the new work starts here.
