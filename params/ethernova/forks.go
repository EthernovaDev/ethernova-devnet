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
)
