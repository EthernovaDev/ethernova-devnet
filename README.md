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

| Node | Role     | IP             | P2P Port | RPC Port |
|------|----------|----------------|----------|----------|
| 1    | Miner    | 192.168.1.15   | 30301    | 8545     |
| 2    | Observer | 192.168.1.34   | 30302    | 8552     |
| 3    | Observer | 192.168.1.134  | 30303    | 8553     |
| 4    | Observer | 192.168.1.16   | 30304    | 8554     |

GPU mining via T-Rex (RTX 3080 Ti) through stratum proxy on port 8888.

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
| < 30% pure    | **5% gas surcharge** | Storage-heavy contracts |

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
- [x] Gas penalty (5%) for complex non-parallelizable workloads
- [x] Contract pattern tracker (pure vs impure opcode classification)
- [x] `ethernova_adaptiveGas` RPC endpoints (toggle, setDiscount, setPenalty, reset)
- [x] Validate consensus across all 4 nodes with adaptive gas enabled
- [x] Stress test: 200 txs, 4 nodes in consensus, 0 errors

### Phase 3: Execution Modes (current)
- [ ] Standard mode: full EVM compatibility (default)
- [ ] Fast mode: skip redundant checks for verified contracts
- [ ] Parallel mode: speculative parallel execution of independent txs
- [ ] Automatic rollback on state conflict in parallel mode

### Phase 4: Runtime Optimization
- [ ] Cache results for pure contract calls (same input = same output)
- [ ] Opcode sequence optimization (pre-compute common patterns)
- [ ] Dynamic bytecode analysis at deploy time

## Principles

1. **Determinism first** — Every optimization must produce identical state transitions on all nodes
2. **Devnet before mainnet** — Nothing goes to mainnet without full devnet validation
3. **Incremental** — Each phase builds on the previous
4. **Measure before optimize** — Profile first, then decide what to optimize
5. **Developer-friendly** — Make gas as cheap as possible for efficient code

## Upstream

Fork of [EthernovaDev/ethernova-coregeth](https://github.com/EthernovaDev/ethernova-coregeth), downstream of CoreGeth / go-ethereum.
