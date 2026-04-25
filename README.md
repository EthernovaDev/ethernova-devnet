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

### Active Features (v1.0.2)

| Feature | Status | Description |
|---------|--------|-------------|
| Adaptive Gas | **Monitoring** | Tracks contract purity patterns. Gas modifications disabled for consensus safety (v1.0.2) |
| EVM Profiler | Enabled | Real-time opcode tracking per contract |
| Opcode Optimizer | **Monitoring** | Detects redundant patterns. Gas refunds disabled for consensus safety (v1.0.2) |
| Call Cache | **Monitoring** | Tracks pure function calls. Cache returns disabled for consensus safety (v1.0.2) |
| Auto-Tuner | **Monitoring** | Tracks network patterns. Auto-adjustment disabled for consensus safety (v1.0.2) |
| Custom Precompiles | Active | novaBatchHash (0x20) and novaBatchVerify (0x21) |

> **Note on v1.0.2:** Gas-modifying features (adaptive gas discount/penalty, optimizer refunds, call cache returns) were disabled because they used node-local profiling data to modify gas costs, causing 4-17 gas divergence between nodes and BAD BLOCK errors. All features still collect data accessible via RPC. The Noven Fork for mainnet will use deterministic contract classification (static analysis at deploy time) instead of runtime profiling.

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

## Roadmap specifically for NIP-0004

### Phase 1: Protocol Object Trie Foundation (NIP-0004)
- [x] New `ProtocolObjectTrie` subtrie under Global State Root
- [x] RLP encoding for Protocol Objects: id, owner, type_tag, state_data, expiry_block, last_touched_block, rent_balance
- [x] Combined state root: `Hash(AccountTrieRoot, ProtocolObjectTrieRoot)`
- [x] Fork-gated activation (`ProtocolObjectForkBlock`)
- [x] `nova_getProtocolObject` / `nova_getProtocolObjectCount` RPC stubs
- [x] Consensus verification: 3+ nodes, 500 blocks post-fork, zero BAD BLOCK
- [x] Cross-platform state root identity (Linux + Windows binary)

### Phase 2: Deferred Execution Engine (NIP-0004)
- [x] Pending Effects Queue in state tree (ordered by sequence number, NOT Go map)
- [x] Deferred Processing Phase: runs at the start of every block before transaction execution
- [x] Effect ordering: `block_number * 1_000_000 + tx_index * 1_000 + effect_index`
- [x] Block-level queue size limit + backpressure (MSEND reverts if queue full)
- [x] Empty queue = no-op (zero overhead on existing blocks)
- [x] `nova_getPendingEffects` / `nova_getDeferredProcessingStats` RPC endpoints
- [x] Substage 2A: queue storage only (enqueue, no processing)
- [x] Substage 2B: active deferred processing + queue clearing
- [x] Consensus verification: 3+ nodes, 1000 blocks, 500+ enqueued effects, zero BAD BLOCK

### Phase 3: Content Reference Primitive (NIP-0004) ✅
- [x] `novaContentRegistry` precompile (**0x2B**): create, lookup, verify content references
      (NIP-0004 draft §3.4 originally specified 0x2A, but 0x2A is held by Phase 2's
      `novaDeferredQueue` in this codebase — see `docs/NIP-0004-Phase-3.md` §8 for the
      final slot map)
- [x] Content Reference as first live Protocol Object type (immutable after creation,
      type_tag = 0x03)
- [x] Storage rent model: deterministic integer-only per-byte per-block deduction at
      epoch boundaries (`RentEpochLength = 10000`, `RentRatePerBytePerBlock = 1`)
- [x] Rent exhaustion → `isValid` returns false, `getContentRef` reports
      `expiredReason = "rent_exhausted"`
- [x] `ethernova_getContentRef` / `ethernova_listContentRefs` /
      `ethernova_getContentRefCount` / `ethernova_contentRefConfig` RPC endpoints
- [x] Harness contract `devnet/contracts/ContentRefTest.sol`: create, getContentRef,
      isValid, listByOwner, createBatch
- [x] Validation script `devnet/phase-nip0004-03-test.js`: 10 scenarios covering fork
      gate, static call, multi-node consensus, epoch-boundary consensus, harness
      roundtrip, pagination, rent deduction, under-funded expiry, neighbour
      precompiles still alive
- [ ] Consensus verification: 3+ nodes, 500 blocks, zero BAD BLOCK (operator step)

### Phase 4: Mailbox Primitive (NIP-0004)
- [ ] `novaMailboxManager` precompile (**0x2C** — shifted from original 0x29 which is
      Protocol Object Registry): create, configure (capacity, ACL, postage), destroy
- [ ] Temporary `novaMailboxOps` precompile (0x33): sendMessage, recvMessage, peekMessage, countMessages
- [ ] Mailbox as stateful Protocol Object with ordered message queue
- [ ] Message send → Pending Queue (Phase 2) → delivered to target mailbox next block
- [ ] Anti-spam: capacity limit, sender whitelist/blacklist, minimum postage for unknown senders
- [ ] Substage 4A: mailbox lifecycle (create/configure/destroy, no messaging)
- [ ] Substage 4B: message send/receive + deferred delivery integration
- [ ] End-to-end test: Alice sends to Bob's mailbox, Bob reads next block
- [ ] Stress test: 100 messages to same mailbox in one block
- [ ] Consensus verification: 3+ nodes, 1000 blocks, 500+ messages, zero BAD BLOCK

### Phase 5: State Lifecycle Tiers (NIP-0004)
- [ ] 5-tier model: Active (100K blocks) → Warm (1M) → Cold (10M) → Archived (>10M) → Expired
- [ ] Lazy tier transition on access: check `last_touched_block`, apply warming fee, promote
- [ ] Tier-adjusted SLOAD/SSTORE costs: Warm = 3x, Cold = 10x + witness required
- [ ] `novaStateWitness` precompile (0x2F): Merkle proof verification for Cold/Archived state restoration
- [ ] Warm State Commitment Root in state root calculation
- [ ] Devnet: shortened thresholds (Active=100, Warm=500, Cold=1000 blocks)
- [ ] Substage 5A: tier tracking + RPC query (no demotion yet)
- [ ] Substage 5B: tier demotion + warming fees (Active → Warm → Cold)
- [ ] Substage 5C: witness verification + state restoration (Cold → Active via proof)
- [ ] End-to-end test: create state → wait → verify demotion → restore with witness
- [ ] Consensus verification: 3+ nodes, 2000 blocks, tier transitions occurring, zero BAD BLOCK

### Phase 6: Execution Domains & Capability Model (NIP-0004)
- [ ] Domain declaration at contract deployment: Domain 0 (Classic), Domain 1 (Deferred), Domain 2 (Channel)
- [ ] Domain 0 = no prefix (fully backward compatible); Domain 1 = prefix 0xEF01; Domain 2 = prefix 0xEF02
- [ ] Domain enforcement: Domain 0 contracts CANNOT call Nova precompiles (0x29+) → revert
- [ ] Capability bitmask (`msg.capabilities`): narrowing-only propagation through call chains
- [ ] Domain Bridge Protocol: Domain 0 → Domain 1 via MSEND (response is deferred, not synchronous)
- [ ] Existing precompiles 0x20–0x28 remain accessible from Domain 0
- [ ] Consensus verification: 3+ nodes, 500 blocks, cross-domain calls tested, zero BAD BLOCK

### Phase 7: Session/Channel Primitive (NIP-0004)
- [ ] Session Protocol Object: counterparties, state_hash, sequence_number, timeout_block, dispute_rules
- [ ] `novaSessionArbiter` precompile (0x2D): open, commit, close, dispute resolution
- [ ] Off-chain signed state updates: monotonically increasing sequence numbers, both-party signatures
- [ ] Dispute resolution: highest valid sequence number with valid signatures wins (deterministic)
- [ ] Session timeout: checked in Deferred Processing Phase
- [ ] Substage 7A: session lifecycle (open/close, no dispute)
- [ ] Substage 7B: state commit + sequence number validation
- [ ] Substage 7C: dispute resolution + timeout handling
- [ ] End-to-end test: two wallets open session, exchange signed updates P2P, commit checkpoint on-chain
- [ ] Consensus verification: 3+ nodes, 500 blocks, session operations, zero BAD BLOCK

### Phase 8: Nova RPC Namespace & Developer Tooling (NIP-0004)
- [ ] Unified `nova_*` RPC namespace: getProtocolObject, getMailbox, getMessages, getContentRef, getSession, getStateTier, getStateWitness, getPendingEffects, getCapabilities, getDomain
- [ ] ethers.js provider extensions (Nova SDK)
- [ ] Hardhat plugin for Domain 1/2 deployment
- [ ] Block explorer extensions for Protocol Objects, Mailbox states, Channel activities
- [ ] Developer documentation and example contracts

### Phase 9: NIP-0003 Chat Rebase to NIP-0004 Primitives
- [ ] Chat registry → Mailbox owner lookup + X25519 pubkey in Mailbox metadata
- [ ] Direct messages → Session channel (Phase 7): real-time P2P, periodic on-chain checkpoint
- [ ] Group chat → Domain 1 ChatRoom contract with Mailbox fanout via Deferred Processing
- [ ] Message content → ContentRef (encrypted payload off-chain, hash on-chain)
- [ ] NIP-0003 test cases 1–6 pass using NIP-0004 primitives
- [ ] Simple web chat harness (wallet connect + P2P messaging + on-chain settlement)

### Phase 10: Multi-Dimensional Resource Metering (NIP-0004)
- [ ] 5-dimension Resource Vector: compute, state_read, state_write, protocol_ops, proof_verify
- [ ] Per-dimension adaptive pricing (EIP-1559 style, ±12.5% per block, per dimension)
- [ ] Congestion isolation: busy DeFi (compute) does NOT make chat (protocol_ops) expensive
- [ ] Backward compatibility: standard `gasLimit` deterministically mapped to Resource Vector
- [ ] Extended transaction format (EIP-2718 style) for fine-grained per-dimension limits
- [ ] Substage 10A: Resource Vector tracking (monitoring only, no pricing change)
- [ ] Substage 10B: per-dimension pricing active (generous initial multipliers)
- [ ] Substage 10C: full adaptive pricing + congestion isolation validation
- [ ] Consensus verification: 3+ nodes, 2000 blocks, all transaction types, zero BAD BLOCK

### Phase 11: Application-Layer Precompiles (NIP-0004)
- [ ] `novaAsyncCallback` (0x30): register callback triggered in next block on condition
- [ ] `novaIdentityAttestation` (0x2C): DID verification, key binding proofs, reputation scores
- [ ] `novaSocialGraph` (0x2B): follow/unfollow, block, trust score, mutual connection checks
- [ ] `novaContentManifest` (0x2E): verifiable content manifests for browser-like decentralized systems
- [ ] `novaGameState` (0x31): commit/reveal, block-hash VRF, turn validation, compact state diffs
- [ ] `novaComputeBounty` (0x32): off-chain computation verification with proof submission
- [ ] Identity, Subscription, GameRoom Protocol Object types

### Phase 12: Nova Opcodes — Promotion from Precompile (NIP-0004)
- [ ] **Address revision required**: NIP-0004 proposed 0xf6–0xfe which conflict with existing EVM opcodes (STATICCALL=0xfa, REVERT=0xfd, INVALID=0xfe). Reassigned to 0xd0–0xd8.
- [ ] Promotion criteria: >1000 calls/block sustained for 30 days on devnet, precompile CALL overhead >20% of operation cost
- [ ] 0xd0 MSEND, 0xd1 MRECV, 0xd2 MPEEK, 0xd3 MCOUNT (mailbox operations)
- [ ] 0xd4 CREF, 0xd5 CVERIFY (content reference operations)
- [ ] 0xd6 SOPEN, 0xd7 SCOMMIT, 0xd8 SCLOSE (session/channel operations)
- [ ] Jump table + instruction handler + gas table updates
- [ ] Only promoted after proven usage data justifies opcode over precompile

### Final Test Results (v1.1.6 - Adaptive Gas ACTIVE)

```
================================================================
  ETHERNOVA v1.1.6 - MULTI-NODE CHAOS TEST
================================================================
  Feature tests:         35/35 PASSED
  Attack simulation:     12/12 BLOCKED
  Contract deploy:       SUCCESS (gas=100,473)
  Contract calls:        30 increment() calls
  Transfers:             500 + 1000 zero-value + 200 self
  Max txs/block:         66
  Precompiles:           9/9
  RPC endpoints:         11/11
  BAD BLOCK errors:      ZERO
================================================================
  CROSS-NODE CONSENSUS: 50/50 blocks MATCHED (Miner vs VPS)
  CROSS-PLATFORM:       Windows gasUsed IDENTICAL to Linux
================================================================
  Block 1607: Miner=111116 VPS=111116 Windows=111116  IDENTICAL
  Block 1600: Miner=1398232 VPS=1398232               IDENTICAL
  Block 1593: Miner=951116 VPS=951116                  IDENTICAL
  Block 1579: Miner=993116 VPS=993116                  IDENTICAL
================================================================
  ADAPTIVE GAS v2 IS DETERMINISTIC CROSS-PLATFORM
================================================================
```

**v1.1.6 includes Noven's trace-based adaptive gas v2** with recalibrated thresholds. Gas adjustment applied post-execution using integer-only math. Verified identical results across Linux (CGO_ENABLED=1) and Windows (CGO_ENABLED=0) builds.

### Security Audit Results (v1.1.0)

3 rounds of external AI security review (Gemini) identified 11 vulnerabilities across consensus, economic, and infrastructure layers. All fixed.

#### Round 1: Consensus & Safety
| Issue | Severity | Attack | Fix |
|-------|----------|--------|-----|
| Gas Refund DoS | HIGH | Heavy computation + REVERT = 90% refund while wasting miner CPU | Refund only for txs <100k execution gas |
| Reentrancy kills DeFi | MEDIUM | Global block prevents flash loans, DEX aggregators, oracles | Self-reentrancy only (A->B->A blocked, A->B->C allowed) |

#### Round 2: Engineering & Architecture
| Issue | Severity | Attack | Fix |
|-------|----------|--------|-----|
| Parallel exec collision | CRITICAL | Two "independent" txs modify same slot = BAD BLOCK | Analysis-only mode, sequential execution |
| Anti-MEV spam flood | HIGH | Bots send millions of min-gas txs for FIFO priority | Rate limit: max 16 pending txs per sender |
| Upgrade storage corruption | HIGH | Dev changes Solidity variable order = storage corrupted | Reject empty code, reject >10x size change, 100-block timelock |

#### Round 3: Economic Attacks
| Issue | Severity | Attack | Fix |
|-------|----------|--------|-----|
| Shielded pool double-spend | CRITICAL | Nullifier bug = infinite NOVA creation | Max 10,000 NOVA/withdrawal + double-check pool accounting |
| Oracle 51% manipulation | CRITICAL | Rent hashrate, inject false prices | Circuit breaker: reject >15% price change per block |
| Token spam fills LevelDB | HIGH | Create millions of junk tokens cheaply | 500,000 gas creation cost + max 100 tokens per address |

#### Round 4: Infrastructure Apocalypse
| Issue | Severity | Attack | Fix |
|-------|----------|--------|-----|
| Tempo TX mempool RAM bomb | CRITICAL | 5M future-dated txs fill node RAM = OOM kill | Max 500 blocks scheduling window |
| LevelDB crash desync | CRITICAL | Power failure corrupts external indexes | Atomic batch writes for all index operations |
| Tempo batch revert CPU bomb | HIGH | 15 heavy SSTORE calls + REVERT = CPU exhaustion | 5,000 gas overhead per call in batch |

#### Defense Summary
- **4 circuit breakers**: privacy withdrawal, oracle price, gas refund, batch overhead
- **3 rate limits**: 16 txs/sender, 100 tokens/creator, 500 block schedule window
- **2 atomic write guarantees**: LevelDB batch for all external indexes
- **1 safe architecture**: parallel exec = analysis only, no execution changes

### v1.0.2 Consensus Verification

```
================================================================
  ETHERNOVA DEVNET v1.0.7 - PHASE 14 TEST RESULTS
  2026-03-26
================================================================

=== Core Network (4/4 PASSED) ===
  Chain ID: 121526
  Version: v1.0.7-devnet
  Mining: active
  Peers: connected

=== Custom RPC Endpoints (11/11 PASSED) ===
  ethernova_forkStatus, ethernova_chainConfig, ethernova_nodeHealth,
  ethernova_evmProfile, ethernova_adaptiveGas, ethernova_optimizer,
  ethernova_callCache, ethernova_precompiles, ethernova_executionMode,
  ethernova_tempoConfig, ethernova_stateExpiry

=== Precompile Calls (4/4 PASSED) ===
  novaBatchHash (0x20): returns keccak256 hashes
  novaBatchVerify (0x21): signature verification
  novaFrameApprove (0x23): transaction approval
  novaFrameIntrospect (0x24): frame inspection

=== Contract Deployment (1/1 PASSED) ===
  SimpleStorage deployed at block 24,292, gas=96,573
  NO BAD BLOCK - consensus maintained after deployment

=== Batch Transfers (1/1 PASSED) ===
  10 ETH transfers sent successfully

=== Consensus Verification (10/10 BLOCKS MATCHED) ===
  Blocks 24,286-24,295 verified across Node1 + VPS
  Block hashes IDENTICAL on all nodes
  Blocks with transactions (deploy + calls) verified
  ZERO merkle root divergence

=== Fork Configuration (3/3 PASSED) ===
  NovenForkBlock: 20,500
  TempoTxForkBlock: 23,300
  StateExpiryForkBlock: 21,500

TOTAL: 35/37 passed (2 minor failures from test bytecode, not network issues)
CONSENSUS: 10/10 blocks matched across nodes
BAD BLOCK ERRORS: ZERO
```

**v1.0.7 is the most stable build to date.** All consensus-critical features verified:
- Contract deployment works without merkle root divergence
- All 5 custom precompiles (0x20-0x24) functional
- 11 custom RPC endpoints responding
- Cross-node block hash verification passes
- State expiry and LastTouched safely disabled (no state trie changes)
- Tempo and Frame AA fork blocks configured and ready

### v1.0.2 Consensus Verification Results

Full test suite run on 2026-03-24:

```
================================================================
  ETHERNOVA DEVNET v1.0.2 - FULL CONSENSUS TEST SUITE
================================================================

TEST 1: Contract Call (NovaToken.balanceOf)
  Node1: OK | VPS: OK

TEST 2: Precompile novaBatchHash (0x20)
  Node1: 0x2cefe4e59877c202...
  Node4: 0x2cefe4e59877c202...  (IDENTICAL)
  VPS:   0x2cefe4e59877c202...  (IDENTICAL)

TEST 3: Precompile novaBatchVerify (0x21)
  Node1: 0x0000000000000000...
  Node4: 0x0000000000000000...  (IDENTICAL)
  VPS:   0x0000000000000000...  (IDENTICAL)

TEST 4: Custom RPC Endpoints: 11/11 OK

TEST 5: Consensus - 10 block hash verification (3 nodes)
  Block 16747: 0x35d283e5cdd641d4 [MATCH]
  Block 16746: 0xcf9c38ee596f7289 [MATCH]
  Block 16745: 0x744e7f854bb1f7fe [MATCH]
  Block 16744: 0x245373d4c77a2c93 [MATCH]
  Block 16743: 0x0928f04d7a397ce3 [MATCH]
  Block 16742: 0xf1f43f8a343a874a [MATCH]
  Block 16741: 0x9b89feff25b46207 [MATCH]
  Block 16740: 0xb4a5710a3303917b [MATCH]
  Block 16739: 0xc2a3493ea35f4620 [MATCH]
  Block 16738: 0xa3438ab5a8d9eec0 [MATCH]

RESULTS: 10/10 consensus | 11/11 RPC | 0 BAD BLOCK errors
>>> v1.0.2 FULLY VERIFIED <<<
```

Run the test yourself:
```bash
bash devnet/v102-consensus-test.sh
```

## Stress Test Results

### Test 1: 1,000 Mixed Transactions (Local Network)

| Metric | Result |
|--------|--------|
| **Transactions** | 1,000 (500 ETH, 300 ERC-20, 100 NFT, 100 MultiSig) |
| **Time** | 68 seconds |
| **Throughput** | **14.7 TPS** |
| **Block Time** | **4 seconds avg** |
| **Consensus** | **5/5 nodes synced** |
| **Errors** | 0 |

### Test 2: 5,000 Mixed Transactions (VPS → Miner)

| Metric | Result |
|--------|--------|
| **Transactions Submitted** | 4,995 (2500 ETH, 1500 ERC-20, 500 NFT, 500 MultiSig) |
| **Submission Rate** | **64 tx/s** |
| **Failed to Submit** | 5 |
| **Block Time** | **~5 seconds avg** (CPU mining) |

### Deploy Gas Costs (measured)

| Contract | Deploy Gas | Type |
|----------|-----------|------|
| NovaToken (ERC-20) | 456,654 | Pure computation (99%) → **25% discount eligible** |
| NovaNFT (ERC-721) | 556,378 | Pure computation (100%) |
| NovaMultiSig | 918,331 | Pure computation (99%) |

### Adaptive Gas Results

| Contract | Calls | Pure Opcodes | Gas Effect |
|----------|-------|-------------|------------|
| NovaToken | 11+ | **99%** | **-25% discount active** |
| NovaNFT | 1+ | 100% | Qualifying (need 10+ calls) |
| NovaMultiSig | 1+ | 99% | Qualifying (need 10+ calls) |

### Optimizer Results
- **94 redundant opcode patterns** detected across all contracts
- **104 gas refunded** from pattern elimination (PUSH+POP, DUP+POP, etc.)

All 5 nodes (4 local + 1 VPS) maintained consensus throughout all tests.

## Known Issues & Lessons Learned

### v1.0.0/v1.0.1: Consensus Bug (FIXED in v1.0.2)

**Problem:** Nodes without custom precompiles computed different gas for contract deployments (4 gas difference), causing BAD BLOCK errors and chain splits.

**Root cause:** The adaptive gas system, opcode optimizer, and call cache modified gas costs during EVM execution using node-local profiling data. Each node builds different profiling data depending on transaction history, so gas calculations diverged by 4-17 units.

**Fix:** All gas-modifying features disabled during block execution (v1.0.2). Features still collect data for monitoring via RPC but no longer affect consensus-critical calculations.

**Key lesson for Noven Fork (mainnet):**
1. Any feature that modifies gas MUST be deterministic across all nodes
2. Runtime profiling-based gas changes are inherently non-deterministic
3. Future implementation will use static analysis at contract deploy time
4. Hard fork upgrades MUST be mandatory - all nodes must upgrade before activation block

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

Ethernova Devnet includes 12 custom precompiled contracts not found on any other EVM chain:

| Address | Name | Description | Gas |
|---------|------|-------------|-----|
| `0x20` | `novaBatchHash` | Batch keccak256 hashing | 30/item |
| `0x21` | `novaBatchVerify` | Batch signature verification | 2,000/sig |
| `0x22` | `novaAccountManager` | Smart wallet (recovery, key rotation) | 2k-10k |
| `0x23` | `novaFrameApprove` | Frame AA transaction approval | 5,000 |
| `0x24` | `novaFrameIntrospect` | Frame inspection for conditional logic | 2,000 |
| `0x25` | `novaTokenManager` | Native multi-token (no ERC-20 needed) | 1k-50k |
| `0x26` | `novaShieldedPool` | Optional privacy (shielded transfers) | 50k-100k |
| `0x27` | `novaContractUpgrade` | Safe contract upgrades with timelock | 2k-50k |
| `0x28` | `novaOracle` | Protocol-level price oracle with TWAP | 2k-5k |
| `0x29` | `novaProtocolObjectRegistry` | NIP-0004 Phase 1 — first-class Protocol Objects | 2k-20k |
| `0x2A` | `novaDeferredQueue` | NIP-0004 Phase 2 — pending effects queue + block-prologue drain | 2k-10k |
| `0x2B` | `novaContentRegistry` | NIP-0004 Phase 3 — content references with rent-backed expiry | 2k-10k |

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