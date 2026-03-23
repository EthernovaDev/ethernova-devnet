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
| **Faucet** | https://faucet.ethnova.net |
| **RPC (HTTP)** | http://localhost:8545 (local node) |

## MetaMask Setup

| Field | Value |
|-------|-------|
| Network Name | Ethernova Devnet |
| RPC URL | https://devrpc.ethnova.net |
| Chain ID | 121526 |
| Currency Symbol | NOVA |
| Block Explorer | https://devexplorer.ethnova.net |

## Network Status

The devnet is actively mined and maintained with the following infrastructure:

- **5 nodes** (4 local ESXi VMs + 1 public VPS)
- **GPU mining** (RTX 3080 Ti) + CPU mining
- **~5s average block time**
- **3,000+ blocks** mined
- **Archive node** on VPS for full state history

### Active Features

| Feature | Status | Description |
|---------|--------|-------------|
| Adaptive Gas | Enabled | 25% discount for pure contracts, 10% penalty for storage-heavy |
| EVM Profiler | Enabled | Real-time opcode tracking per contract |
| Opcode Optimizer | Enabled | Detects redundant patterns (PUSH+POP, DUP+POP, etc.) |
| Call Cache | Enabled | Caches pure function results (10,000 entry LRU) |
| Auto-Tuner | Enabled | Adjusts gas parameters every 100 blocks based on network data |
| Custom Precompiles | Active | novaBatchHash (0x20) and novaBatchVerify (0x21) |

### Live Benchmark Results

Real data from deployed contracts on the devnet:

| Contract | Address | Deploy Gas | Calls | Pure % | Gas Effect |
|----------|---------|-----------|-------|--------|------------|
| NovaToken (ERC-20) | `0xd6Dc5b3E...` | 456,654 | 11+ | **99%** | **-25% discount** |
| NovaNFT (ERC-721) | `0xa407ABC4...` | 556,378 | 1+ | 100% | qualifying... |
| NovaMultiSig | `0x24fcDc40...` | 918,331 | 1+ | 99% | qualifying... |

**Optimizer Performance:**
- 94 redundant opcode patterns detected
- 104 gas refunded from pattern elimination
- Patterns: PUSH+POP, DUP1+POP, ISZERO+ISZERO, duplicate PUSHes

**Profiling Stats:**
- 2,569+ opcodes executed and tracked
- 18,216+ gas tracked across all contracts
- Real-time per-contract opcode classification

### Gas Savings Model

| Contract Type | Pattern | Gas Effect | Example |
|--------------|---------|------------|---------|
| Math/Pure Logic | ≥70% pure opcodes | **-25% gas** | ERC-20 transfers, hash computation |
| Mixed Operations | 30-70% pure | Standard gas | Token mints, typical DeFi |
| Storage Heavy | <30% pure opcodes | **+10% gas** | DEX swaps, heavy SSTORE patterns |
| Batch Hash (precompile) | Native | **30 gas/item** vs ~36 in Solidity | Multi-item hashing |
| Batch Verify (precompile) | Native | **2,000 gas/sig** vs 3,000 standard | Multi-sig verification |

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
- [x] **Full feature validation: 47/47 tests passed** (Noven Fork readiness confirmed)

## Stress Test Results

1,000 mixed transactions across 5 synchronized nodes:

| Metric | Result |
|--------|--------|
| **Total Transactions** | 1,000 |
| **Transaction Mix** | 500 ETH transfers, 300 ERC-20, 100 NFT mints, 100 MultiSig |
| **Time to Process** | 68 seconds |
| **Throughput** | **14.7 TPS** |
| **Average Block Time** | **4 seconds** |
| **Blocks Used** | 14 |
| **Consensus** | **5/5 nodes synced** (4 local + 1 VPS) |
| **Errors** | 0 |
| **Optimizer Patterns Found** | 94 (104 gas refunded) |
| **NovaToken Gas Pattern** | 99% pure → **25% discount active** |

All nodes (including public VPS) maintained perfect consensus throughout the stress test.

## Noven Fork Readiness

All features have been validated on the devnet and are ready for mainnet deployment via the **Noven Fork** - a planned hard fork that will activate these features on the Ethernova mainnet (chainId 121525) without a chain reset.

### Test Results (47/47 PASSED)

```
=== Core Network ===          3/3  PASSED  (chainId, sync, version)
=== EVM Profiler ===          5/5  PASSED  (enable, disable, reset, toggle)
=== Adaptive Gas ===          6/6  PASSED  (enable, discount, penalty, set/restore)
=== Execution Modes ===       6/6  PASSED  (standard, fast, parallel, switch)
=== Call Cache ===             4/4  PASSED  (enable, size, reset)
=== Opcode Optimizer ===      4/4  PASSED  (enable, patterns, gas refunds)
=== Auto-Tuner ===            2/2  PASSED  (enable, status)
=== Bytecode Analysis ===     1/1  PASSED  (static analysis)
=== Custom Precompiles ===    5/5  PASSED  (list, novaBatchHash, novaBatchVerify)
=== Deployed Contracts ===    3/3  PASSED  (NovaToken, NovaNFT, NovaMultiSig)
=== Node Health ===           5/5  PASSED  (version, block, peers, uptime, memory)
───────────────────────────────────────────
TOTAL                        47/47 PASSED
```

Run the test yourself:
```bash
./devnet/phase6-full-test.sh https://devrpc.ethnova.net
```

## Deployed Contracts

Live contracts on the devnet for testing and benchmarking:

| Contract | Type | Address | Deploy Gas | Gas Pattern |
|----------|------|---------|-----------|-------------|
| NovaToken | ERC-20 | [`0xd6Dc5b3E9CEF3c4117fFd32F138717bBc0f8d91c`](https://devexplorer.ethnova.net/address/0xd6Dc5b3E9CEF3c4117fFd32F138717bBc0f8d91c) | 456,654 | 99% pure → **25% discount** |
| NovaNFT | ERC-721 | [`0xa407ABC46D71A56fb4fAc2Ae9CA1F599A2270C2a`](https://devexplorer.ethnova.net/address/0xa407ABC46D71A56fb4fAc2Ae9CA1F599A2270C2a) | 556,378 | 100% pure |
| NovaMultiSig | MultiSig Wallet | [`0x24fcDc40BFa6e8Fce87ACF50da1e69a36019083f`](https://devexplorer.ethnova.net/address/0x24fcDc40BFa6e8Fce87ACF50da1e69a36019083f) | 918,331 | 99% pure |

### Source Code

| Contract | Description | File |
|----------|-------------|------|
| `NovaToken` | Standard ERC-20 token (1M supply) | `devnet/contracts/NovaToken.sol` |
| `NovaNFT` | Minimal ERC-721 with mint/transfer | `devnet/contracts/NovaNFT.sol` |
| `NovaDEX` | Constant-product AMM swap pool | `devnet/contracts/NovaDEX.sol` |
| `NovaMultiSig` | Multi-owner transaction wallet | `devnet/contracts/NovaMultiSig.sol` |
| `TestProfiler` | Configurable opcode generator | `devnet/contracts/TestProfiler.sol` |

These contracts demonstrate how adaptive gas treats different execution patterns — pure computation gets cheaper, storage-heavy operations pay more.

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
