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

## Roadmap

### Phase 1: Profiling (completed)
- [x] EVM opcode profiler (global + per-contract)
- [x] `ethernova_evmProfile` RPC endpoints
- [x] Devnet genesis (chainId 121526), scripts, and topology
- [x] Deploy on ESXi VMs (4 nodes, GPU mining)
- [x] Deploy test contracts and collect profiling data

### Phase 2: Adaptive Gas (completed)
- [x] Gas discount (25%) for optimized/predictable execution patterns
- [x] Gas penalty (10%) for complex non-parallelizable workloads
- [x] Contract pattern tracker (pure vs impure opcode classification)
- [x] `ethernova_adaptiveGas` RPC endpoints (toggle, setDiscount, setPenalty, reset)
- [x] Validate consensus across all 4 nodes with adaptive gas enabled
- [x] Stress test: 200 txs, 4 nodes in consensus, 0 errors

### Phase 3: Execution Modes (completed)
- [x] Standard mode: full EVM compatibility (default)
- [x] Fast mode: skip redundant checks for verified contracts
- [x] Contract verifier: bytecode analysis for SELFDESTRUCT, DELEGATECALL, CREATE
- [x] `ethernova_executionMode` / `ethernova_executionModeSet` RPC endpoints
- [x] Parallel mode: conservative speculative execution (simple transfers only)
- [x] Transaction classifier: separate parallel-safe txs from sequential
- [x] State snapshot + merge with conflict detection

### Phase 4: Runtime Optimization (completed)
- [x] Cache results for pure contract calls (same input = same output)
- [x] Dynamic bytecode analysis at deploy time (loop detection, opcode groups, cacheability)
- [x] `ethernova_callCache` / `ethernova_bytecodeAnalysis` RPC endpoints

### Phase 5: Polish & Infrastructure (completed)
- [x] Opcode sequence optimizer (detect PUSH+POP, DUP+POP, ISZERO+ISZERO, etc.)
- [x] Auto-tuning: adaptive gas percentages adjust based on real network data
- [x] Devnet dashboard and faucet
- [x] CI/CD: GitHub Actions (build, test core, test ethernova, go vet)
- [x] Security audit script and benchmark script
- [x] Public explorer at https://devexplorer.ethnova.net
- [x] Public HTTPS RPC at https://devrpc.ethnova.net
- [x] One-click binary with embedded genesis and auto-peer discovery

### Phase 6: DApp Validation & Public Testing (completed)
- [x] Public faucet at https://faucet.ethnova.net (10 NOVA per request)
- [x] Smart contract test suite: ERC-20 (NovaToken), ERC-721 (NovaNFT), DEX (NovaDEX), MultiSig
- [x] Custom precompiled contracts: `novaBatchHash` (0x20) and `novaBatchVerify` (0x21)
- [x] `ethernova_precompiles` RPC endpoint
- [x] Hardhat developer config with Ethernova Devnet network ready to use
- [x] Gas benchmark and stress test scripts

## Custom Precompiles

Ethernova Devnet includes two custom precompiled contracts not found on any other EVM chain:

| Address | Name | Description | Gas |
|---------|------|-------------|-----|
| `0x20` | `novaBatchHash` | Batch keccak256 - hash multiple 32-byte items in one call | 30 per item |
| `0x21` | `novaBatchVerify` | Batch ecrecover - verify multiple signatures in one call | 2,000 per sig (vs 3,000 standard) |

### Using novaBatchHash from Solidity

```solidity
// Hash 3 items in one call (costs 90 gas vs ~108 in pure Solidity)
(bool ok, bytes memory result) = address(0x20).staticcall(
    abi.encodePacked(item1, item2, item3)
);
// result contains 3 concatenated 32-byte hashes
```

### Using novaBatchVerify from Solidity

```solidity
// Verify 2 signatures in one call (costs 4,000 gas vs 6,000 with ecrecover)
bytes memory input = abi.encodePacked(hash1, r1, s1, v1, hash2, r2, s2, v2);
(bool ok, bytes memory result) = address(0x21).staticcall(input);
// result contains 2 left-padded 32-byte addresses
```

## Faucet

Get free NOVA tokens for testing: **https://faucet.ethnova.net**

- 10 NOVA per request
- 5-minute cooldown per address/IP
- Tokens are devnet-only with no real value

## Developer Quick Start (Hardhat)

```bash
# Clone and setup
git clone https://github.com/EthernovaDev/ethernova-devnet.git
cd ethernova-devnet/devnet

# Install Hardhat
npm init -y && npm install --save-dev hardhat @nomicfoundation/hardhat-toolbox

# Copy config
cp hardhat.config.js ../hardhat.config.js
cp .env.example ../.env

# Edit .env with your private key, then deploy
npx hardhat run scripts/deploy.js --network ethernova_devnet
```

## Upstream

Fork of [EthernovaDev/ethernova-coregeth](https://github.com/EthernovaDev/ethernova-coregeth), downstream of CoreGeth / go-ethereum.
