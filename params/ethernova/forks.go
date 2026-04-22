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
)