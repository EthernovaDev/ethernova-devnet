# Ethernova Devnet

Private development environment for experimental EVM features. Based on Ethernova v1.3.2 (core-geth fork).

## What is this?

Ethernova mainnet is a PoW Ethash EVM chain (chainId 121525). This devnet (chainId 121526) is a sandbox to test protocol-level improvements that could make Ethernova's EVM execution faster, more predictable, and more efficient than standard EVM chains — without risking mainnet stability.

## Devnet Setup

| Node | Role     | P2P Port | RPC Port | WS Port |
|------|----------|----------|----------|---------|
| 1    | Miner    | 30301    | 8551     | 8561    |
| 2    | Miner    | 30302    | 8552     | 8562    |
| 3    | Observer | 30303    | 8553     | 8563    |
| 4    | Observer | 30304    | 8554     | 8564    |

### Quick Start

```bash
# Build
make geth

# Start all 4 nodes
./devnet/start-all.sh

# Check consensus
./devnet/check-consensus.sh

# Stop all
./devnet/stop-all.sh

# Reset chain data
./devnet/reset-all.sh
```

## Custom RPC Endpoints

### From mainnet (v1.3.2)
| Method | Description |
|--------|-------------|
| `ethernova_forkStatus` | Status of all forks |
| `ethernova_chainConfig` | Chain info (chainId, consensus, version) |
| `ethernova_nodeHealth` | Block, peers, sync, uptime, memory |

### Devnet experimental
| Method | Description |
|--------|-------------|
| `ethernova_evmProfile` | Opcode execution profiling data (top opcodes, top contracts) |
| `ethernova_evmProfileReset` | Clear all profiling data |
| `ethernova_evmProfileToggle` | Enable/disable profiling |

### Example

```bash
curl -s -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"ethernova_evmProfile","params":[],"id":1}' \
  http://localhost:8551
```

## Roadmap

### Phase 1: Profiling (current)
- [x] EVM opcode profiler (global + per-contract)
- [x] `ethernova_evmProfile` RPC endpoints
- [x] Devnet genesis, scripts, and topology
- [ ] Deploy on ESXi VMs (4 nodes)
- [ ] Deploy test contracts and collect profiling data

### Phase 2: Adaptive Gas
- [ ] Gas discounts for optimized/predictable execution patterns
- [ ] Complex non-parallelizable workloads cost more
- [ ] Validate consensus across all nodes with new gas rules

### Phase 3: Execution Modes
- [ ] Standard mode: full EVM compatibility (default)
- [ ] Fast mode: reduced overhead for verified contracts
- [ ] Parallel mode: speculative parallel execution of independent txs with rollback on conflict

### Phase 4: Runtime Optimization
- [ ] Cache results for pure contract calls (same input = same output)
- [ ] Opcode sequence optimization (pre-compute common patterns)
- [ ] Dynamic bytecode analysis at deploy time

## Principles

1. **Determinism first** — Every optimization must produce identical state transitions on all nodes
2. **Devnet before mainnet** — Nothing goes to mainnet without full devnet validation
3. **Incremental** — Each phase builds on the previous
4. **Measure before optimize** — Profile first, then decide what to optimize

## Upstream

Fork of [EthernovaDev/ethernova-coregeth](https://github.com/EthernovaDev/ethernova-coregeth), downstream of CoreGeth / go-ethereum.
