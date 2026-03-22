# Ethernova Node Configuration

## RPC
- Default bind: `127.0.0.1`.
- HTTP APIs:
  - Dev: `eth,net,web3,personal,miner,txpool,admin,debug`.
  - Mainnet: `eth,net,web3` (expand only if necessary).
- Do not expose RPC publicly without authentication/proxy.

## Ports
- p2p: 30303 (UDP/TCP)
- HTTP: 8545 (localhost)
- WS: 8546 (localhost)

## Data and logs
- Datadir default: `./data` (override via `--datadir` or config `Node.DataDir`).
- Logs: `./logs/ethernova.log` by default (override with `--log.file` or config).

## IPC / Auth RPC
- IPC path: `geth.ipc` (relative to the datadir by default).
- Auth RPC: binds `127.0.0.1:8551` by default.

## Mining
- Set etherbase with `--miner.etherbase 0x...` (or config if you prefer).
- Dev mode: gasprice 0, txpool pricelimit 0.
- Mainnet mode: gasprice default 1 gwei; txpool pricelimit default (non-zero).

## Fees
- EIP-1559 baseFee is redirected to the configured `baseFeeVault`; tips remain with the miner.

## Peering
- Bootnodes: `networks/mainnet/bootnodes.txt` (enodes, one per line).
- Static peers: `networks/mainnet/static-nodes.json` (JSON array). Copied to `data/geth/static-nodes.json` if present.
- Discover peers: `admin.nodeInfo.enode` to share your enode.

## Genesis files
- `genesis-dev.json`: difficulty=0x1, forks at block 0, baseFeeVault set, chainId/networkId 77778 (dev/testnet).
- `genesis-mainnet.json`: higher difficulty (0x400000), baseFeeVault set, chainId/networkId 121525 (genesis). EVM compatibility fork (Constantinople/Petersburg/Istanbul) activates at block 105000.
- Block reward schedule (both): 10 -> 5 -> 2.5 -> 1.25 -> floor 1 NOVA, halving every ~2,102,400 blocks (~1 year at ~15s/block).

## Security
- Avoid `--allow-insecure-unlock` on anything except isolated dev.
- Keep keystore backed up; do not wipe datadir without a backup.
- Use firewall rules to restrict RPC/WS to localhost if running on shared hosts.

## Minimal config example (mainnet)
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
