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
	// Set to 0 on devnet (active from genesis). On mainnet this will be a future block.
	NovenForkBlock uint64 = 0
)
