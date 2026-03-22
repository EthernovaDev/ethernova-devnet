# Ethernova Launch Guide (Mainnet)

This guide uses the direct `ethernova` binary. Helper scripts are optional convenience tools only.

## Modes and chain IDs
- Mainnet: chainId **121525** / networkId **121525** (genesis + runtime).
- Dev/Testnet: chainId/networkId **77778** (`genesis-dev.json`, dev/test only).

## Get the binary
- Prefer release assets: extract the ZIP (Windows) or tar.gz (Linux).
- Optional build-from-source: see `docs/DEV.md`.

## Launch checklist (mainnet)
1) Verify version:
   - Windows: `.\ethernova.exe version`
   - Linux: `./ethernova version`
2) Offline validation (optional):
   - `ethernova print-genesis`
   - `ethernova sanitycheck --datadir <path>`
3) Create a config file (example `ethernova.toml`):
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
4) Start the node:
   - Windows: `.\ethernova.exe --config .\ethernova.toml`
   - Linux: `./ethernova --config ./ethernova.toml`
5) Verify RPC:
   - `eth_chainId` -> `0x1dab5`
   - `net_version` -> `121525`
   - `eth_getBlockByNumber(0).hash` matches the genesis hash below.

## Mainnet Genesis Fingerprint
| Field              | Value                                                               |
|--------------------|---------------------------------------------------------------------|
| ChainId/NetworkId (genesis) | 121525                                                        |
| Consensus          | Ethash                                                              |
| Genesis Block Hash | 0xc3812eb81498965a3f9ff3e73d2f423934e6d440578d4f4fbb6623cc61c453d9  |
| BaseFeeVault       | 0x3a38560b66205bb6a31decbcb245450b2f15d4fd                          |
| GasLimit           | 0x1c9c380                                                           |
| Difficulty         | 0x400000                                                            |
| BaseFeePerGas      | 0x3b9aca00 (1 gwei)                                                 |
| extraData          | "NOVA MAINNET"                                                      |

EVM compatibility fork (Constantinople/Petersburg/Istanbul) activates at block **105000** (no re-init required).

## Bootnodes / static peers
- Pass bootnodes at runtime: `--bootnodes enode://...`
- Static peers: create `<datadir>/geth/static-nodes.json` with a JSON array of enode URLs.
- `networks/mainnet/bootnodes.txt` and `networks/mainnet/static-nodes.json` are used only by optional helper scripts.

## Running a second node (local peering)
Example (first node running on 30303/8545/8546):
```
ethernova --datadir data-node2 --port 30304 --http --http.addr 127.0.0.1 --http.port 8547 --ws --ws.addr 127.0.0.1 --ws.port 8548 --bootnodes <enode://of-first-node>
```
Expected: `net_peerCount > 0` on both nodes.

## Miningcore quickstart (solo mining / pool daemon)
- Keep RPC on localhost; run Miningcore on the same host or via SSH tunnel.
- Start with mining enabled:
  - Windows: `.\ethernova.exe --config .\ethernova.toml --mine --miner.etherbase 0xPOOL_ADDRESS`
  - Linux: `./ethernova --config ./ethernova.toml --mine --miner.etherbase 0xPOOL_ADDRESS`
- Verify RPC responses for chainId/block0/getWork before attaching the pool.

## Optional systemd unit (Linux)
An example unit file is provided at `systemd/ethernova.service`. Use it as a starting point and adjust paths/users:
```
sudo cp systemd/ethernova.service /etc/systemd/system/ethernova.service
sudo systemctl daemon-reload
sudo systemctl enable --now ethernova
```

## Optional helper scripts (convenience only)
- `scripts/run-mainnet-node.ps1` / `scripts/run-mainnet-node.sh`
- `scripts/run-bootnode.ps1` and `scripts/check-peering.ps1`
- `scripts/verify-mainnet.ps1`, `scripts/test-rpc.ps1`

## Security defaults (mainnet)
- RPC binds to `127.0.0.1` only.
- HTTP APIs: `eth,net,web3` only (expand only if necessary).
- Avoid `--allow-insecure-unlock`.
- Keep keystore backed up and restrict RPC/WS to localhost.

## Dev/Testnet mode
Use dev/testnet only for local testing. See `docs/DEV.md` for dev workflows.

## Explorer and wallets
- MetaMask (mainnet): RPC `http://127.0.0.1:8545`, Chain ID `121525`, Symbol `NOVA`, Explorer URL (set to your deployment).
- Suggested explorer: Blockscout. Provide env pointing to your RPC, chainId 121525, and publish the explorer URL alongside bootnodes.

## Release packaging (dev-only)
Optional helper script: `scripts/package-release.ps1`.
