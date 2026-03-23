# Ethernova Devnet

Private development environment for experimental EVM features. Based on Ethernova v1.3.2 (core-geth fork).

## What is this?

Ethernova mainnet is a PoW Ethash EVM chain (chainId 121525). This devnet (chainId 121526) is a sandbox to test protocol-level improvements that could make Ethernova's EVM execution faster, more predictable, and more efficient than standard EVM chains — without risking mainnet stability.

The goal is to build an **adaptive, self-optimizing EVM** where:
- Efficient contracts pay **less gas** (25% discount)
- Heavy/complex contracts pay **slightly more** (5% surcharge)
- The protocol learns from execution patterns and rewards optimization

## Devnet Infrastructure

4 VMs on ESXi (Dell PowerEdge, 128GB RAM):

| Node | Role     | IP             | P2P Port | RPC Port | Extra |
|------|----------|----------------|----------|----------|-------|
| 1    | Miner    | 192.168.1.15   | 30301    | 9545     | Stratum proxy :8888 |
| 2    | Observer | 192.168.1.34   | 30302    | 8552     | Explorer :3000, API :4000 |
| 3    | Observer | 192.168.1.134  | 30303    | 8553     | |
| 4    | Observer | 192.168.1.16   | 30304    | 8554     | |

GPU mining via T-Rex (RTX 3080 Ti) through stratum proxy on port 8888.

### External Access (for collaborators)

Open these ports on your router to allow external access:

| Port | Service | Description |
|------|---------|-------------|
| 9545 | RPC HTTP | JSON-RPC endpoint (MetaMask, scripts) |
| 9546 | RPC WebSocket | Real-time subscriptions |
| 3000 | Explorer UI | Blockscout frontend |
| 4000 | Explorer API | Blockscout API |
| 8080 | Faucet | Get free devnet NOVA |
| 8081 | Dashboard | Devnet stats dashboard |

### MetaMask Setup

```
Network Name:    Ethernova Devnet
RPC URL:         http://<YOUR_PUBLIC_IP>:9545
Chain ID:        121526
Currency Symbol: NOVA
Explorer URL:    http://<YOUR_PUBLIC_IP>:3000
```

### Quick Start

```bash
# Build
make geth

# Start all 4 nodes
./devnet/start-all.sh

# Check consensus
./devnet/check-consensus.sh

# Run stress test
./devnet/stress-test.sh 200

# Stop all
./devnet/stop-all.sh
```

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

### Core (from mainnet v1.3.2)
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

### Runtime Optimization (Phase 4)
| Method | Description |
|--------|-------------|
| `ethernova_callCache` | Call cache stats (hits, misses, hit rate) |
| `ethernova_callCacheToggle(bool)` | Enable/disable pure call caching |
| `ethernova_callCacheReset` | Clear cached results |
| `ethernova_bytecodeAnalysis` | Static bytecode analysis for all contracts |

### Optimizer & Auto-Tuner (Phase 5)
| Method | Description |
|--------|-------------|
| `ethernova_optimizer` | Opcode sequence optimizer stats (redundant ops, gas refunded) |
| `ethernova_optimizerToggle(bool)` | Enable/disable sequence optimizer |
| `ethernova_optimizerReset` | Clear optimizer state |
| `ethernova_autoTuner` | Auto-tuner status (ranges, last tuned block) |
| `ethernova_autoTunerToggle(bool)` | Enable/disable auto-tuning of gas percentages |

### Example

```bash
# Check adaptive gas status and contract patterns
curl -s -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"ethernova_adaptiveGas","params":[],"id":1}' \
  http://localhost:8545

# Response:
# {
#   "enabled": true,
#   "discountPercent": 25,
#   "penaltyPercent": 5,
#   "contracts": [{
#     "address": "0x740D...",
#     "callCount": 118,
#     "totalOps": 4212,
#     "pureOps": 4212,
#     "purePercent": 100,
#     "patternScore": 100,
#     "discountPercent": 25,
#     "penaltyPercent": 0
#   }]
# }
```

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
- [x] Devnet dashboard (web UI on port 8081, auto-refreshes every 5s)
- [x] Faucet (web UI on port 8080, 10 ETH per request, 5min cooldown)
- [x] CI/CD: GitHub Actions (build, test core, test ethernova, go vet)
- [x] Security audit script (bounds checks, consensus, RPC health)
- [x] Benchmark script (gas savings vs standard EVM)
- [x] Devnet explorer (Blockscout on Node 2, port 3000/4000)
- [x] Public RPC endpoint on port 9545 for MetaMask / external access

## Principles

1. **Determinism first** — Every optimization must produce identical state transitions on all nodes
2. **Devnet before mainnet** — Nothing goes to mainnet without full devnet validation
3. **Incremental** — Each phase builds on the previous
4. **Measure before optimize** — Profile first, then decide what to optimize
5. **Developer-friendly** — Make gas as cheap as possible for efficient code

## Upstream

Fork of [EthernovaDev/ethernova-coregeth](https://github.com/EthernovaDev/ethernova-coregeth), downstream of CoreGeth / go-ethereum.
