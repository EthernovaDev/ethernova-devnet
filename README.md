# Ethernova Devnet

Public development network for experimental EVM features. Based on Ethernova v1.3.2 (core-geth fork).

## Quick Start

Download `EthernovaDevnet.exe` (Windows) or `EthernovaDevnet-linux-amd64` (Linux) from [Releases](https://github.com/EthernovaDev/ethernova-devnet/releases) and run it. That's it — genesis, peers, and RPC are configured automatically.

```bash
# Linux
chmod +x EthernovaDevnet-linux-amd64
./EthernovaDevnet-linux-amd64

# Windows - just double-click EthernovaDevnet.exe
```

The node will:
1. Initialize the devnet genesis automatically (embedded, chainId 121526)
2. Connect to public bootnodes
3. Start syncing the chain
4. Open RPC on `http://127.0.0.1:8545`

## Network Info

| | |
|---|---|
| **Chain ID** | 121526 |
| **Network ID** | 121526 |
| **Consensus** | Ethash (Proof of Work) |
| **Block Reward** | 10 NOVA |
| **Currency** | NOVA |

## Public Endpoints

| Service | URL |
|---------|-----|
| **Explorer** | https://devexplorer.ethnova.net |
| **RPC (HTTPS)** | https://devrpc.ethnova.net |
| **RPC (HTTP)** | http://localhost:8545 (local node) |

## MetaMask Setup

| Field | Value |
|-------|-------|
| Network Name | Ethernova Devnet |
| RPC URL | https://devrpc.ethnova.net |
| Chain ID | 121526 |
| Currency Symbol | NOVA |
| Block Explorer | https://devexplorer.ethnova.net |

## What is this?

Ethernova mainnet is a PoW Ethash EVM chain (chainId 121525). This devnet (chainId 121526) is a sandbox to test protocol-level improvements that could make Ethernova's EVM execution faster, more predictable, and more efficient than standard EVM chains — without risking mainnet stability.

The goal is to build an **adaptive, self-optimizing EVM** where:
- Efficient contracts pay **less gas** (25% discount)
- Heavy/complex contracts pay **slightly more** (10% surcharge)
- The protocol learns from execution patterns and rewards optimization

## How Adaptive Gas Works

The EVM profiler monitors every contract execution and classifies each opcode:

**Pure opcodes** (cheap for the network — only local computation):
- Arithmetic: `ADD`, `MUL`, `SUB`, `DIV`, `EXP`
- Stack: `PUSH`, `DUP`, `SWAP`, `POP`
- Memory: `MLOAD`, `MSTORE`
- Control: `JUMP`, `JUMPI`
- Hash: `KECCAK256`

**Impure opcodes** (expensive — modify blockchain state):
- `SSTORE` (write to storage)
- `CREATE`, `CREATE2` (deploy contracts)
- `CALL` with value (send ETH)
- `LOG0-LOG4` (emit events)
- `SELFDESTRUCT`

After **10 calls** and **100 opcodes**, the system calculates a pattern score:

```
patternScore = (pure opcodes / total opcodes) x 100
```

| Pattern Score | Effect | Example |
|---------------|--------|---------|
| >= 70% pure   | **25% gas discount** | Math libraries, pure computation |
| 30-70% pure   | Normal gas (no change) | Typical contracts |
| < 30% pure    | **10% gas surcharge** | Storage-heavy contracts |

This incentivizes developers to write efficient code — the network rewards optimization.

## Custom RPC Endpoints

### Core
| Method | Description |
|--------|-------------|
| `ethernova_forkStatus` | Status of all forks |
| `ethernova_chainConfig` | Chain info (chainId, consensus, version) |
| `ethernova_nodeHealth` | Block, peers, sync, uptime, memory |

### EVM Profiling
| Method | Description |
|--------|-------------|
| `ethernova_evmProfile` | Opcode execution stats (top opcodes, top contracts) |
| `ethernova_evmProfileReset` | Clear profiling data |
| `ethernova_evmProfileToggle(bool)` | Enable/disable profiling |

### Adaptive Gas
| Method | Description |
|--------|-------------|
| `ethernova_adaptiveGas` | Current config + per-contract pattern analysis |
| `ethernova_adaptiveGasToggle(bool)` | Enable/disable adaptive gas |
| `ethernova_adaptiveGasSetDiscount(uint)` | Set discount % for efficient contracts (0-50) |
| `ethernova_adaptiveGasSetPenalty(uint)` | Set penalty % for complex contracts (0-50) |
| `ethernova_adaptiveGasReset` | Clear all pattern data |

### Execution Modes
| Method | Description |
|--------|-------------|
| `ethernova_executionMode` | Current mode + fast mode stats + verified contracts |
| `ethernova_executionModeSet(uint)` | Set mode: 0=standard, 1=fast, 2=parallel |
| `ethernova_parallelStats` | Parallel execution statistics |

### Runtime Optimization
| Method | Description |
|--------|-------------|
| `ethernova_callCache` | Call cache stats (hits, misses, hit rate) |
| `ethernova_callCacheToggle(bool)` | Enable/disable pure call caching |
| `ethernova_callCacheReset` | Clear cached results |
| `ethernova_bytecodeAnalysis` | Static bytecode analysis for all contracts |

### Optimizer & Auto-Tuner
| Method | Description |
|--------|-------------|
| `ethernova_optimizer` | Opcode sequence optimizer stats |
| `ethernova_optimizerToggle(bool)` | Enable/disable sequence optimizer |
| `ethernova_autoTuner` | Auto-tuner status (ranges, last tuned block) |
| `ethernova_autoTunerToggle(bool)` | Enable/disable auto-tuning of gas percentages |

### Example

```bash
curl -s -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"ethernova_adaptiveGas","params":[],"id":1}' \
  https://devrpc.ethnova.net
```

## Build from Source

```bash
git clone https://github.com/EthernovaDev/ethernova-devnet.git
cd ethernova-devnet
make geth
# Binary at ./build/bin/geth
```

Requires: Go 1.21+, GCC, Make

## Completed Features

- EVM opcode profiler (global + per-contract)
- Adaptive gas system (25% discount / 10% penalty)
- Execution modes (standard, fast, parallel)
- Call result caching for pure contracts
- Bytecode static analysis at deploy time
- Opcode sequence optimizer
- Auto-tuner for gas parameters
- Public explorer and HTTPS RPC

## Upstream

Fork of [EthernovaDev/ethernova-coregeth](https://github.com/EthernovaDev/ethernova-coregeth), downstream of CoreGeth / go-ethereum.
