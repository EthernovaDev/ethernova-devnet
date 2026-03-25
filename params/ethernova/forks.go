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
	StateExpiryForkBlock uint64 = 21500
	// StateExpiryPeriod is the number of blocks of inactivity before a contract is archived.
	StateExpiryPeriod uint64 = 1000
	// TempoTxForkBlock activates Tempo-style smart transactions.
	// Enables: atomic batching, fee delegation, scheduled transactions.
	// Gas is always paid in NOVA (no ERC-20 gas payments).
	TempoTxForkBlock uint64 = 23300
)
