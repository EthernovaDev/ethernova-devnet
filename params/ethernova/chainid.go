package ethernova

import "math/big"

const (
	NewChainID uint64 = 121526
)

var (
	NewChainIDBig = new(big.Int).SetUint64(NewChainID)
)

func ChainIDForBlock(number *big.Int) *big.Int {
	return new(big.Int).Set(NewChainIDBig)
}

func IsEthernovaChainID(chainID *big.Int) bool {
	if chainID == nil {
		return false
	}
	return chainID.Cmp(NewChainIDBig) == 0
}
