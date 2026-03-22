# Ethernova Devnet Roadmap

## Goal
Build an adaptive, self-optimizing EVM execution environment that differentiates
Ethernova from other EVM chains.

## Phases

### Phase 1: Profiling (current)
- [x] EVM opcode profiler (global counters)
- [x] Per-contract execution profiling
- [x] `ethernova_evmProfile` RPC endpoint
- [x] `ethernova_evmProfileReset` RPC endpoint
- [x] `ethernova_evmProfileToggle` RPC endpoint
- [x] Devnet genesis (chainId 121526)
- [x] 4-node devnet scripts (2 miners, 2 observers)
- [ ] Deploy devnet on ESXi VMs
- [ ] Deploy test contracts and collect profiling data

### Phase 2: Adaptive Gas
- [ ] Analyze profiling data to identify optimization targets
- [ ] Implement gas discount for "optimized" opcode patterns
- [ ] Contracts with predictable execution paths cost less gas
- [ ] Complex/non-parallelizable workloads cost more
- [ ] Validate all 4 nodes maintain consensus with new gas rules

### Phase 3: Execution Modes
- [ ] Standard mode: full EVM compatibility (default)
- [ ] Fast mode: reduced overhead for verified contracts
- [ ] Parallel mode: speculative parallel execution of independent txs
- [ ] Automatic rollback on state conflict in parallel mode

### Phase 4: Runtime Optimization
- [ ] Cache execution results for pure contract calls (same input = same output)
- [ ] Opcode sequence optimization (pre-compute common patterns)
- [ ] Dynamic bytecode analysis at deploy time
- [ ] Strict determinism rules to ensure consensus safety

## Principles
1. **Determinism first**: Every optimization must produce identical state transitions on all nodes
2. **Devnet before mainnet**: Nothing goes to mainnet without full devnet validation
3. **Incremental**: Each phase builds on the previous, never skip ahead
4. **Measure before optimize**: Profile first, then decide what to optimize
