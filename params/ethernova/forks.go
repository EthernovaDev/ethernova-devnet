package ethernova

const (
	// EVMCompatibilityForkBlock enables Constantinople + Petersburg + Istanbul.
	EVMCompatibilityForkBlock uint64 = 0
	// EIP658ForkBlock enables receipt status (EIP-658).
	EIP658ForkBlock uint64 = 0
	// MegaForkBlock enables missing historical EVM forks for compatibility.
	MegaForkBlock uint64 = 0
	// NovenForkBlock activates state rent surcharge and smart wallet features.
	// Named after community member Noven who proposed going public with the devnet.
	// Devnet activation: block 20,500 (~20 min from deployment)
	// Mainnet: TBD after devnet validation
	NovenForkBlock uint64 = 20500
	// StateExpiryForkBlock activates the state expiry garbage collector.
	// Contracts/tokens with no activity for StateExpiryPeriod blocks get archived.
	// EOA wallets are NEVER expired. Archived state can be restored with merkle proof.
	// Devnet activation: block 21,500
	StateExpiryForkBlock uint64 = 25200
	// StateExpiryPeriod is the number of blocks of inactivity before a contract is archived.
	StateExpiryPeriod uint64 = 1000
	// TempoTxForkBlock activates Tempo-style smart transactions.
	// Enables: atomic batching, fee delegation, scheduled transactions.
	// Gas is always paid in NOVA (no ERC-20 gas payments).
	TempoTxForkBlock uint64 = 23300
	// FrameAAForkBlock activates Frame-style Account Abstraction.
	// Smart contract wallets can validate and approve transactions.
	// Precompiles 0x23 (novaFrameApprove) and 0x24 (novaFrameIntrospect).
	// Inspired by EIP-8141 Frame Transactions.
	FrameAAForkBlock uint64 = 24000

	// AdaptiveGasV2ForkBlock activates trace-based adaptive gas pricing.
	// Replaces the v1 bytecode-based system that caused consensus splits.
	// Pure computation contracts get up to -25% gas discount.
	// Storage-heavy contracts get up to +10% gas penalty.
	// This is a CONSENSUS RULE — all nodes MUST apply the same adjustment.
	// Classification is deterministic: pure function of execution trace counters.
	// v1.1.5: Fork block set to FUTURE so ALL nodes must upgrade first.
	// Before: block 0 (active from genesis) caused BAD BLOCK because
	// nodes with old binary validated blocks without adaptive gas adjustment.
	// Now: all nodes update to v1.1.5, then adaptive gas activates at block 14400.
	AdaptiveGasV2ForkBlock uint64 = 14400
)