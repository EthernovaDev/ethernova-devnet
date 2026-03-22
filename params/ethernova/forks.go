package ethernova

const (
	// EVMCompatibilityForkBlock enables Constantinople + Petersburg + Istanbul.
	EVMCompatibilityForkBlock uint64 = 105000
	// EIP658ForkBlock enables receipt status (EIP-658).
	EIP658ForkBlock uint64 = 110500
	// MegaForkBlock enables missing historical EVM forks for compatibility.
	MegaForkBlock uint64 = 118200
)
