package ethernova

const (
	// EVMCompatibilityForkBlock enables Constantinople + Petersburg + Istanbul.
	EVMCompatibilityForkBlock uint64 = 0
	// EIP658ForkBlock enables receipt status (EIP-658).
	EIP658ForkBlock uint64 = 0
	// MegaForkBlock enables missing historical EVM forks for compatibility.
	MegaForkBlock uint64 = 0

	// ============================================================
	// NOVEN FORK — Ethernova 2.0 (devnet baseline)
	// Named after community developer Noven who built the adaptive
	// gas system and parallel execution classifier.
	// On devnet all Noven Fork features activate at block 0 so the
	// chain starts with the full v2.0.0 feature set enabled.
	// ============================================================

	// NovenForkBlock activates ALL Noven Fork features simultaneously:
	//   - Adaptive Gas V2 (trace-based post-execution adjustment)
	//   - Per-EVM reentrancy guard
	//   - Gas refund on revert (90% execution gas)
	//   - Native precompiles (0x20-0x28)
	//   - State expiry (contract garbage collection)
	//   - Tempo transactions (atomic batching)
	//   - Frame Account Abstraction
	NovenForkBlock uint64 = 0

	// AdaptiveGasV2ForkBlock activates trace-based adaptive gas pricing.
	// Replaces the v1 bytecode-based system that caused consensus splits.
	// Pure computation contracts get up to -25% gas discount.
	// Storage-heavy contracts get up to +10% gas penalty.
	// CONSENSUS RULE — all nodes MUST apply the same adjustment.
	AdaptiveGasV2ForkBlock uint64 = 0

	// StateExpiryForkBlock activates the state expiry garbage collector.
	// Contracts with no activity for StateExpiryPeriod blocks get archived.
	// EOA wallets are NEVER expired. Archived state can be restored.
	StateExpiryForkBlock uint64 = 0
	// StateExpiryPeriod is the number of blocks of inactivity before archival.
	// Short period for devnet testing (~3 hours at 11s/block). Mainnet uses 900000 (~115 days).
	StateExpiryPeriod uint64 = 1000

	// TempoTxForkBlock activates Tempo-style smart transactions.
	// Enables: atomic batching, fee delegation, scheduled transactions.
	TempoTxForkBlock uint64 = 0

	// FrameAAForkBlock activates Frame-style Account Abstraction.
	// Precompiles 0x23 (novaFrameApprove) and 0x24 (novaFrameIntrospect).
	FrameAAForkBlock uint64 = 0

	// ============================================================
	// NIP-0004 — Layered Deterministic Computer
	// Phase 1: Protocol Object Trie Foundation
	// Adds first-class Protocol Objects (Mailbox, Session, ContentRef,
	// Identity, Subscription, GameRoom) to the state tree.
	// Objects are stored at system address 0xFF01 in the account trie.
	// ============================================================

	// ProtocolObjectForkBlock activates Protocol Object support.
	// On devnet: block 0 (active from genesis, same as Noven Fork).
	// On mainnet: set to the agreed activation block.
	ProtocolObjectForkBlock uint64 = 0

	// ============================================================
	// NIP-0004 — Layered Deterministic Computer
	// Phase 2: Deferred Execution Engine
	// Introduces a Pending Effects Queue stored at system address
	// 0xFF02 and a Deferred Processing Phase that runs at the start
	// of each block (before regular transactions) once this fork is
	// active. Effects enqueued in block N are processed at the start
	// of block N+1 in strict insertion order (monotonic sequence).
	// ============================================================

	// DeferredExecForkBlock activates the Deferred Execution Engine.
	// On devnet: block 0 (active from genesis — queue starts empty so
	// this is a safe no-op on early blocks).
	// On mainnet: set to the agreed activation block. The Phase 0
	// Deferred Processing step is gated by block.Number() >=
	// DeferredExecForkBlock in BOTH the validator state_processor
	// path AND the miner worker path. Any asymmetry = consensus split.
	DeferredExecForkBlock uint64 = 0

	// MaxPendingEffectsPerBlock caps the number of enqueue operations
	// allowed in a single block. When the limit is reached, further
	// enqueueEffect precompile calls revert (backpressure). This is
	// the §9.1 queue-abuse mitigation from NIP-0004. Per-block, not
	// per-tx, so an attacker cannot split enqueues across txs to bypass.
	MaxPendingEffectsPerBlock uint64 = 1024

	// MaxDeferredProcessingPerBlock caps the number of entries the
	// Deferred Processing Phase will drain in a single block. If the
	// queue grows faster than drain, processing falls behind — this is
	// intentional: it bounds block validation time. Set equal to the
	// per-block enqueue cap so steady state matches steady-in.
	MaxDeferredProcessingPerBlock uint64 = 1024

	// MaxDeferredEffectPayloadBytes caps individual effect payload size.
	// Larger payloads must be stored externally and referenced by hash
	// (a pattern that will be introduced by ContentRef in Phase 3).
	MaxDeferredEffectPayloadBytes uint64 = 512

	// ============================================================
	// NIP-0004 — Layered Deterministic Computer
	// Phase 3: Content Reference Primitive
	//
	// First real, live Protocol Object type. A ContentRef is a
	// minimal immutable pointer to off-chain content plus a rent-
	// backed expiry. Storage lives at system address 0xFF03; the
	// object body itself is written into the Phase 1 Protocol Object
	// Registry (0xFF01) under type_tag = ProtoTypeContentReference.
	//
	// Precompile address 0x2B (NOT 0x2A as the original NIP-0004
	// §3.4 draft suggested — 0x2A is already occupied by the Phase 2
	// Deferred Queue in this codebase). The conflict and resolution
	// are documented in NIP-0004 Phase 3 spec §8.
	// ============================================================

	// ContentRefForkBlock activates the ContentRef precompile (0x2B),
	// its rent engine, and the nova_getContentRef / nova_listContentRefs
	// RPC surface. On devnet: block 0 (available from genesis so the
	// canary phase can start immediately).
	ContentRefForkBlock uint64 = 0

	// RentEpochLength is the number of blocks between rent deductions.
	// At every block where block_number % RentEpochLength == 0 and
	// block_number > 0, every live ContentRef has its rent_balance
	// decremented by RentRatePerBytePerBlock * size * RentEpochLength.
	// CONSENSUS-CRITICAL: fixed integer, no configuration at runtime.
	RentEpochLength uint64 = 10000

	// RentRatePerBytePerBlock is the rent rate in wei per byte per block.
	// Deduction per epoch per ContentRef = rate * size * RentEpochLength.
	// Example: 1024-byte ContentRef at 10000-block epoch costs
	//   1 * 1024 * 10000 = 10_240_000 wei per epoch (~0.00000001024 NOVA).
	// Intentionally cheap for devnet canary; mainnet sets this higher.
	RentRatePerBytePerBlock uint64 = 1

	// MinRentPrepayWei is the minimum rent prepay accepted by createContentRef.
	// Must cover at least one epoch of rent for a 1-byte object. Prevents
	// trivial ContentRef spam by rejecting zero-rent creations up front.
	MinRentPrepayWei uint64 = 10000

	// MaxContentRefSize caps the size field of any single ContentRef.
	// This is the declared off-chain size of the referenced content —
	// on-chain storage per object is fixed (hash + metadata), this cap
	// exists to prevent absurd rent-calculation inputs (e.g. size =
	// 2^63 would overflow the rent multiplication).
	MaxContentRefSize uint64 = 1 << 32 // 4 GiB

	// MaxContentRefTypeBytes caps the content_type field length (MIME-like).
	MaxContentRefTypeBytes uint64 = 64

	// MaxContentRefAvailabilityProofBytes caps the availability_proof field.
	// The proof is typically a single 32-byte commitment or a short CID.
	// A hard cap avoids unbounded Protocol Object payloads.
	MaxContentRefAvailabilityProofBytes uint64 = 256

	// MaxContentRefsPerRentEpoch caps how many ContentRefs are processed
	// in a single epoch-boundary Finalize() pass. If the live population
	// exceeds this, processing rolls over to subsequent epochs via the
	// cr_rent_cursor slot — bounded per-block work, no liveness loss.
	// The cap is generous because the work is simple arithmetic +
	// single-slot writes; 8192 objects = a few ms at most.
	MaxContentRefsPerRentEpoch uint64 = 8192

	// ============================================================
	// NIP-0004 — Layered Deterministic Computer
	// Phase 4: Mailbox Primitive
	//
	// Mailbox is the first stateful Protocol Object type with a queue
	// and mutation. It exposes two precompiles:
	//   - 0x2C novaMailboxManager : create / configure / destroy
	//   - 0x35 novaMailboxOps     : send / recv / peek / count
	//
	// Send messages enter the Phase 2 deferred queue with effectType =
	// EffectTypeMailboxSend (0x10). At block N+1 the deferred processing
	// dispatcher routes them to vm.HandleMailboxSendEffect for delivery
	// to the target mailbox's queue at MailboxOpsAddr (0xFF04).
	//
	// On devnet: block 0 (active from genesis, same as the rest of the
	// NIP-0004 stack). On mainnet: set to the agreed activation block.
	// Pre-fork, both 0x2C and 0x35 revert all selectors so there is no
	// incidental state touch on early blocks.
	// ============================================================

	// MailboxForkBlock activates the Phase 4 Mailbox primitive: the 0x2C
	// and 0x35 precompiles and the Mailbox handler in the deferred
	// processing dispatcher. Gating MUST be identical on both validator
	// and miner paths — the dispatch is reached from
	// core.ProcessDeferredEffects which is called by both
	// core/state_processor.go and miner/worker.go.
	MailboxForkBlock uint64 = 0

	// ============================================================
	// NIP-0004 — Layered Deterministic Computer
	// Phase 5: State Lifecycle Tiers
	//
	// 5-tier state lifecycle for Account state, Contract storage,
	// and Protocol Object state:
	//
	//   Active   -> recently touched (<= ActiveTierBlocks since touch)
	//   Warm     -> touched between ActiveTierBlocks and WarmTierBlocks
	//   Cold     -> touched between WarmTierBlocks and ColdTierBlocks
	//   Archived -> not touched within ColdTierBlocks (cold root persisted)
	//   Expired  -> archive policy explicitly drops the cold root
	//
	// CONSENSUS-CRITICAL invariants:
	//   1. Tier is a PURE FUNCTION of (lastTouched, currentBlock,
	//      thresholds). No wall-clock, no map iteration, no randomness.
	//   2. Sweep iteration order is sorted-bytes; the candidate list
	//      comes from the Phase 15 'x' index which is itself written
	//      sorted.
	//   3. Warming fee = tier_gap * size * fee_per_byte using uint64
	//      arithmetic with overflow-safe saturation.
	//   4. The 0x2F novaStateWitness precompile only verifies proofs
	//      and updates the external LevelDB index — it never mutates
	//      the state trie. State-root divergence cannot originate
	//      from Phase 5.
	// ============================================================

	// StateLifecycleForkBlock activates Phase 5 tier tracking, the
	// SLOAD warming-fee surcharge, the novaStateWitness precompile
	// (0x2F), and the ethernova_getStateTier / getStateWitness /
	// getWarmStateRoot / stateLifecycleConfig RPC surface.
	//
	// On devnet: block 0 (active from genesis, same as the rest of
	// the NIP-0004 stack). On mainnet: set to the agreed activation
	// block. Pre-fork the tier surcharge is exactly zero, the
	// precompile reverts on every selector, and the RPC methods
	// return tier=Active for any query — strict no-op on early
	// blocks.
	StateLifecycleForkBlock uint64 = 0

	// ActiveTierBlocks is the number of blocks since last_touched
	// within which an account/object is considered Active (no
	// surcharge). Devnet: 100 blocks (~17 minutes at ~10s/block).
	// Mainnet target: 100_000 blocks (~12.7 days at 11s/block).
	ActiveTierBlocks uint64 = 100

	// WarmTierBlocks is the upper bound on the Warm tier age in
	// blocks. Above ActiveTierBlocks and at-or-below WarmTierBlocks
	// => Warm. Devnet: 500 blocks. Mainnet target: 1_000_000.
	WarmTierBlocks uint64 = 500

	// ColdTierBlocks is the upper bound on the Cold tier age in
	// blocks. Above WarmTierBlocks and at-or-below ColdTierBlocks
	// => Cold. Above ColdTierBlocks => Archived.
	// Devnet: 1000 blocks. Mainnet target: 10_000_000.
	ColdTierBlocks uint64 = 1000

	// WarmingFeePerByte is the wei-of-gas surcharge applied per byte
	// of touched state when the tier is non-Active:
	//
	//     fee_gas = tier_gap * size_bytes * WarmingFeePerByte
	//
	// where tier_gap is 1 for Warm, 2 for Cold, 3 for Archived. For
	// a 32-byte slot at Warm: 32 * 1 * 5 = 160 extra gas. Cheap on
	// devnet for testability; mainnet target 50–500 depending on
	// calibration.
	WarmingFeePerByte uint64 = 5

	// MaxLifecycleSweepPerBlock caps the number of accounts the
	// lifecycle sweep examines at a single block. Beyond this cap
	// the remaining suffix rolls forward to the next block. The cap
	// is generous because the work is integer comparisons + a small
	// LevelDB batch.
	MaxLifecycleSweepPerBlock uint64 = 256

	// LifecycleStorageSlotSize is the canonical "size" used for the
	// tier surcharge on a single storage-slot operation. EVM storage
	// slots are always 32 bytes; this constant exists so the
	// surcharge formula reads identically in operations_acl.go and
	// in the lifecycle engine.
	LifecycleStorageSlotSize uint64 = 32

	// LifecycleAccountSize is the canonical "size" used for tier
	// surcharge on a whole-account access. Conservative ~96 bytes
	// covers the slim RLP account body. Phase 5 v1 only applies the
	// storage-slot surcharge inside SLOAD; the account-level
	// constant is reserved for the Phase 5D account-archival path
	// that follows.
	LifecycleAccountSize uint64 = 96

	// StateWitnessVerifyGas prices selector 0x01 (read-only Merkle
	// proof check). Comparable to one cold SLOAD plus a few hash
	// ops for proof traversal.
	StateWitnessVerifyGas uint64 = 5000

	// StateWitnessRestoreGas prices selector 0x02 (verify + write).
	// Verify cost plus an SSTORE-equivalent write surcharge.
	StateWitnessRestoreGas uint64 = 25000

	// StateWitnessGetTierGas prices selector 0x03 (single index
	// read). Cheap because it is one LevelDB lookup with no trie
	// traversal.
	StateWitnessGetTierGas uint64 = 1000

	// MaxStateWitnessProofBytes caps the proof payload accepted by
	// 0x2F. Typical storage proofs at depth 8 with 532-byte branch
	// nodes are ~4 KiB. 16 KiB leaves headroom while preventing
	// gigabyte-sized "proofs" that would exhaust RAM.
	MaxStateWitnessProofBytes uint64 = 16384
)
