# Ethernova Devnet v1.0.0 - Node Setup Guide

This is the first release of the Ethernova Devnet binary. This guide covers how to set up and run a devnet node from scratch.

**Chain ID:** 121526
**Network ID:** 121526
**Consensus:** Ethash (PoW)
**Block Reward:** 10 NOVA

---

## Quick Start (Linux)

### 1. Download the binary

```bash
# Download the devnet release
wget https://github.com/EthernovaDev/ethernova-devnet/releases/download/v1.0.0/geth-linux-amd64
chmod +x geth-linux-amd64
mv geth-linux-amd64 geth
```

### 2. Download the devnet genesis file

The devnet uses a **different genesis** than mainnet. You MUST initialize with the correct genesis file.

```bash
# Download genesis
wget https://raw.githubusercontent.com/EthernovaDev/ethernova-devnet/main/devnet/genesis-devnet.json
```

Or create `genesis-devnet.json` manually with this content:

```json
{
  "config": {
    "chainId": 121526,
    "networkId": 121526,
    "eip2FBlock": 0,
    "eip7FBlock": 0,
    "eip150Block": 0,
    "eip155Block": 0,
    "eip160FBlock": 0,
    "eip161FBlock": 0,
    "eip170FBlock": 0,
    "eip100FBlock": 0,
    "eip140FBlock": 0,
    "eip198FBlock": 0,
    "eip211FBlock": 0,
    "eip212FBlock": 0,
    "eip213FBlock": 0,
    "eip214FBlock": 0,
    "eip658FBlock": 0,
    "eip1706FBlock": 0,
    "constantinopleBlock": 0,
    "petersburgBlock": 0,
    "istanbulBlock": 0,
    "ethash": {},
    "blockReward": {
      "0": "0x8ac7230489e80000"
    }
  },
  "difficulty": "0x400",
  "gasLimit": "0x1000000",
  "alloc": {
    "0x1111111111111111111111111111111111111111": {
      "balance": "0x200000000000000000000000000000000000000000000000000000000000"
    },
    "0x2222222222222222222222222222222222222222": {
      "balance": "0x200000000000000000000000000000000000000000000000000000000000"
    },
    "0x3333333333333333333333333333333333333333": {
      "balance": "0x200000000000000000000000000000000000000000000000000000000000"
    },
    "0x4444444444444444444444444444444444444444": {
      "balance": "0x200000000000000000000000000000000000000000000000000000000000"
    }
  }
}
```

### 3. Initialize the data directory

**IMPORTANT:** You must initialize BEFORE starting the node. If you skip this step or use the wrong genesis, you will get the error: `incorrect genesis in datadir (0xc3812e...)`.

```bash
# Create a fresh data directory and initialize with devnet genesis
./geth init --datadir ./devnet-data genesis-devnet.json
```

You should see output like:
```
INFO Successfully wrote genesis state
```

### 4. Start the node

```bash
./geth \
  --networkid 121526 \
  --datadir ./devnet-data \
  --port 30303 \
  --http --http.addr 0.0.0.0 --http.port 8545 \
  --http.api eth,net,web3,admin,ethernova,txpool \
  --http.corsdomain "*" \
  --ws --ws.addr 0.0.0.0 --ws.port 8546 \
  --ws.api eth,net,web3,ethernova \
  --ws.origins "*" \
  --nodiscover \
  --verbosity 3
```

### 5. Connect to the devnet

After starting, add the bootstrap node to sync with the network:

```bash
# From another terminal, attach to the running node
./geth attach ./devnet-data/geth.ipc

# Add the bootstrap peer (Node 1 - miner)
admin.addPeer("enode://6d6f8341c08058a8f966d4e0d75e1cf7009bbe8647741e105e5ef2edd929baf3157292dcb31a1e1bd6cbb9161fe7bfde8e15539bef801ace55950e2e23f92a88@75.86.96.101:30301?discport=0")

# Check peer count (should be >= 1)
net.peerCount

# Check sync status
eth.blockNumber
```

Or via JSON-RPC:

```bash
# Add peer via RPC
curl -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"admin_addPeer","params":["enode://6d6f8341c08058a8f966d4e0d75e1cf7009bbe8647741e105e5ef2edd929baf3157292dcb31a1e1bd6cbb9161fe7bfde8e15539bef801ace55950e2e23f92a88@75.86.96.101:30301?discport=0"],"id":1}' \
  http://localhost:8545

# Check block number
curl -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
  http://localhost:8545
```

### 6. Verify you are synced

```bash
# All nodes should report the same block number
curl -s -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
  http://localhost:8545

# Check chain ID (should be 0x1dab6 = 121526)
curl -s -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}' \
  http://localhost:8545
```

---

## Common Errors

### "incorrect genesis in datadir (0xc3812e...)"

This means you are using a datadir that was initialized with the **mainnet genesis** instead of the devnet genesis.

**Fix:**
```bash
# Delete the old data directory
rm -rf ./devnet-data

# Re-initialize with the devnet genesis
./geth init --datadir ./devnet-data genesis-devnet.json
```

**Important:** Make sure you are using the **devnet binary** from this release, NOT the mainnet binary. The devnet binary skips mainnet genesis hash validation.

### "peer connected on incompatible version"

Make sure you are running the devnet binary (v1.0.0+), not the mainnet binary.

### Node starts but stays at block 0

You need to connect to at least one peer. See step 5 above. The devnet runs with `--nodiscover` so peers must be added manually.

---

## MetaMask Configuration

To connect MetaMask to the devnet:

| Field | Value |
|-------|-------|
| Network Name | Ethernova Devnet |
| RPC URL | http://YOUR_NODE_IP:8545 |
| Chain ID | 121526 |
| Currency Symbol | NOVA |
| Block Explorer | http://192.168.1.34:3000 |

---

## Devnet Resources

| Service | URL |
|---------|-----|
| RPC (HTTP) | http://192.168.1.15:9545 |
| RPC (WebSocket) | ws://192.168.1.15:9546 |
| Explorer | http://192.168.1.34:3000 |
| Explorer API | http://192.168.1.34:4000 |

---

## Mining on the Devnet

To mine on the devnet, start your node with the `--mine` flag:

```bash
./geth \
  --networkid 121526 \
  --datadir ./devnet-data \
  --port 30303 \
  --http --http.addr 0.0.0.0 --http.port 8545 \
  --http.api eth,net,web3,admin,ethernova,txpool \
  --http.corsdomain "*" \
  --nodiscover \
  --mine --miner.threads=1 \
  --miner.etherbase YOUR_WALLET_ADDRESS \
  --verbosity 3
```

**Note:** The first time mining starts, the node will generate the DAG (takes ~3 minutes). After that, blocks will be found quickly due to the low devnet difficulty.

---

## Custom RPC Endpoints

The devnet binary includes experimental Ethernova RPC methods:

```bash
# EVM Profiling
curl -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"ethernova_evmProfile","params":[],"id":1}' \
  http://localhost:8545

# Adaptive Gas status
curl -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"ethernova_adaptiveGas","params":[],"id":1}' \
  http://localhost:8545

# Node health
curl -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"ethernova_nodeHealth","params":[],"id":1}' \
  http://localhost:8545
```

---

## Build from Source

```bash
git clone https://github.com/EthernovaDev/ethernova-devnet.git
cd ethernova-devnet
make geth
# Binary will be at ./build/bin/geth
```

Requires: Go 1.21+, GCC, Make
