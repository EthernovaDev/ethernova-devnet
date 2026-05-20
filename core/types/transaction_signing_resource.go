// Copyright 2026 The Ethernova Authors
//
// NIP-0004 Phase 10D — Multi-Dimensional Resource Metering.
//
// resourceSigner wraps the existing EIP-4844 (Cancun) signer and adds
// support for the ResourceTx envelope (type 0x05). Any other transaction
// type falls through to the eip4844Signer chain so existing legacy /
// AccessList / DynamicFee / Blob transactions continue to validate
// without behavior change.
//
// The signing hash for ResourceTx includes the per-dimension limits AFTER
// the AccessList field. Once a ResourceTx has been signed, the limits
// CANNOT be changed without breaking the signature — this is the consensus
// hook that makes per-dimension caps tamper-proof end-to-end.

package types

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// resourceSigner is the outermost signer in the Ethernova chain. It
// recognises ResourceTxType and falls back to eip4844Signer for everything
// else (which in turn falls back through eip1559 → eip2930 → eip155 →
// homestead → frontier).
type resourceSigner struct{ eip4844Signer }

// NewPhase10DSigner returns a signer that accepts ResourceTx (0x05) plus
// every other tx type understood by NewCancunSigner. Use this whenever the
// chain config has the Phase 10D fork either active or scheduled.
func NewPhase10DSigner(chainId *big.Int) Signer {
	return resourceSigner{eip4844Signer{eip1559Signer{eip2930Signer{NewEIP155Signer(chainId)}}}}
}

// Sender recovers the signer of tx. For ResourceTx it computes the Phase
// 10D signing hash (Hash below) and recovers the secp256k1 public key.
// All other tx types delegate to eip4844Signer.Sender so existing
// transactions are unaffected.
func (s resourceSigner) Sender(tx *Transaction) (common.Address, error) {
	if tx.Type() != ResourceTxType {
		return s.eip4844Signer.Sender(tx)
	}
	V, R, S := tx.RawSignatureValues()
	// ResourceTx uses 0/1 recovery ids like every other typed tx since
	// EIP-1559; add 27 so the recoverPlain path treats them identically
	// to legacy unprotected signatures.
	V = new(big.Int).Add(V, big.NewInt(27))
	if tx.ChainId().Cmp(s.chainId) != 0 {
		return common.Address{}, fmt.Errorf("%w: have %d want %d", ErrInvalidChainId, tx.ChainId(), s.chainId)
	}
	return recoverPlain(s.Hash(tx), R, S, V, true)
}

// Equal reports whether two signers are interchangeable. resourceSigner is
// considered equal only to another resourceSigner on the same chain id.
func (s resourceSigner) Equal(s2 Signer) bool {
	x, ok := s2.(resourceSigner)
	return ok && x.chainId.Cmp(s.chainId) == 0
}

// SignatureValues extracts (R, S, V) from a 65-byte secp256k1 signature.
// For ResourceTx the chain id is sanity-checked against the signer.
func (s resourceSigner) SignatureValues(tx *Transaction, sig []byte) (R, S, V *big.Int, err error) {
	txdata, ok := tx.inner.(*ResourceTx)
	if !ok {
		return s.eip4844Signer.SignatureValues(tx, sig)
	}
	if txdata.ChainID.Sign() != 0 && txdata.ChainID.Cmp(s.chainId) != 0 {
		return nil, nil, nil, fmt.Errorf("%w: have %d want %d", ErrInvalidChainId, txdata.ChainID, s.chainId)
	}
	R, S, _ = decodeSignature(sig)
	V = big.NewInt(int64(sig[64]))
	return R, S, V, nil
}

// Hash returns the keccak256 of the canonical Phase 10D signing payload.
// Fields appear in the same order as the wire format; the trailing V/R/S
// triple is naturally omitted from the signing payload since it has not
// been computed yet.
//
// Layout:
//
//	type_prefix(0x05) || rlp_list(
//	    chainId, nonce, gasTipCap, gasFeeCap, gas, to, value, data,
//	    accessList, resourceLimits,
//	)
func (s resourceSigner) Hash(tx *Transaction) common.Hash {
	if tx.Type() != ResourceTxType {
		return s.eip4844Signer.Hash(tx)
	}
	rtx := tx.inner.(*ResourceTx)
	return prefixedRlpHash(
		tx.Type(),
		[]interface{}{
			s.chainId,
			tx.Nonce(),
			tx.GasTipCap(),
			tx.GasFeeCap(),
			tx.Gas(),
			tx.To(),
			tx.Value(),
			tx.Data(),
			tx.AccessList(),
			rtx.ResourceLimits,
		})
}
