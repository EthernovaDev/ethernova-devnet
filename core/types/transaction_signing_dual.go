package types

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
)

var dualSignerFallbackCounter = metrics.NewRegisteredCounter("ethernova/dualsigner/fallback", nil)

type dualSigner struct {
	primary  Signer
	fallback Signer
}

func newDualSigner(primary, fallback Signer) Signer {
	return dualSigner{primary: primary, fallback: fallback}
}

func (s dualSigner) Sender(tx *Transaction) (common.Address, error) {
	addr, err := s.primary.Sender(tx)
	if err == nil || !errors.Is(err, ErrInvalidChainId) {
		return addr, err
	}
	dualSignerFallbackCounter.Inc(1)
	log.Debug("DualSigner fallback to legacy chain ID", "tx", tx.Hash())
	return s.fallback.Sender(tx)
}

func (s dualSigner) SignatureValues(tx *Transaction, sig []byte) (r, s2, v *big.Int, err error) {
	return s.primary.SignatureValues(tx, sig)
}

func (s dualSigner) ChainID() *big.Int {
	return s.primary.ChainID()
}

func (s dualSigner) Hash(tx *Transaction) common.Hash {
	return s.primary.Hash(tx)
}

func (s dualSigner) Equal(other Signer) bool {
	switch o := other.(type) {
	case dualSigner:
		return s.primary.Equal(o.primary) && s.fallback.Equal(o.fallback)
	case *dualSigner:
		return s.primary.Equal(o.primary) && s.fallback.Equal(o.fallback)
	default:
		return false
	}
}
