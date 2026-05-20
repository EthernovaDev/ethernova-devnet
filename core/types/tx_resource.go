// Copyright 2026 The Ethernova Authors
//
// NIP-0004 Phase 10D — Multi-Dimensional Resource Metering.
//
// ResourceTx is a typed transaction (envelope byte 0x05) that carries an
// explicit per-dimension resource-limits vector in addition to the standard
// EIP-1559 gas fields. It is OPT-IN: legacy tx (type 0x00/0x01/0x02) and
// EIP-1559 tx (0x02) continue to work unchanged. Senders use ResourceTx
// when they want fine-grained per-dimension caps — for example a chat
// client that wants tight compute/state_write limits but a generous
// protocol_ops limit.
//
// Wire shape after the 0x05 envelope byte:
//
//   rlp_list(
//     ChainID,
//     Nonce,
//     GasTipCap,            // a.k.a. maxPriorityFeePerGas (legacy field)
//     GasFeeCap,            // a.k.a. maxFeePerGas (legacy field)
//     Gas,                  // total gas budget (still a hard ceiling)
//     To,                   // nil for contract creation
//     Value,
//     Data,
//     AccessList,
//     ResourceLimits,       // 5-uint64 list — the new Phase 10D field
//     V, R, S,
//   )
//
// Signing is identical to DynamicFeeTx except the prefix byte is 0x05 and
// the signed payload includes the ResourceLimits tuple after AccessList.

package types

import (
	"bytes"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
)

// ResourceTxType is the EIP-2718 envelope byte for ResourceTx. Type 0x04 is
// reserved for TempoTx; 0x03 is BlobTx; 0x05 is the next available slot.
const ResourceTxType = 0x05

// ResourceTx represents a transaction that explicitly carries a per-dimension
// resource limit vector. The semantics of the existing EIP-1559 fields are
// unchanged — gas accounting still uses gas * effectiveGasPrice. Phase 10D
// enforcement runs in addition, not in place of, gas accounting.
type ResourceTx struct {
	ChainID        *big.Int
	Nonce          uint64
	GasTipCap      *big.Int
	GasFeeCap      *big.Int
	Gas            uint64
	To             *common.Address `rlp:"nil"`
	Value          *big.Int
	Data           []byte
	AccessList     AccessList
	ResourceLimits ResourceLimits

	// Signature
	V *big.Int `json:"v" gencodec:"required"`
	R *big.Int `json:"r" gencodec:"required"`
	S *big.Int `json:"s" gencodec:"required"`
}

// copy creates a deep copy of the transaction data. Required by TxData so
// that NewTx can hand back an isolated, mutation-safe Transaction.
func (tx *ResourceTx) copy() TxData {
	cpy := &ResourceTx{
		Nonce:          tx.Nonce,
		To:             copyAddressPtr(tx.To),
		Data:           common.CopyBytes(tx.Data),
		Gas:            tx.Gas,
		AccessList:     make(AccessList, len(tx.AccessList)),
		Value:          new(big.Int),
		ChainID:        new(big.Int),
		GasTipCap:      new(big.Int),
		GasFeeCap:      new(big.Int),
		ResourceLimits: tx.ResourceLimits,
		V:              new(big.Int),
		R:              new(big.Int),
		S:              new(big.Int),
	}
	copy(cpy.AccessList, tx.AccessList)
	if tx.Value != nil {
		cpy.Value.Set(tx.Value)
	}
	if tx.ChainID != nil {
		cpy.ChainID.Set(tx.ChainID)
	}
	if tx.GasTipCap != nil {
		cpy.GasTipCap.Set(tx.GasTipCap)
	}
	if tx.GasFeeCap != nil {
		cpy.GasFeeCap.Set(tx.GasFeeCap)
	}
	if tx.V != nil {
		cpy.V.Set(tx.V)
	}
	if tx.R != nil {
		cpy.R.Set(tx.R)
	}
	if tx.S != nil {
		cpy.S.Set(tx.S)
	}
	return cpy
}

// accessors for innerTx — must match the TxData interface defined in
// core/types/transaction.go.
func (tx *ResourceTx) txType() byte           { return ResourceTxType }
func (tx *ResourceTx) chainID() *big.Int      { return tx.ChainID }
func (tx *ResourceTx) accessList() AccessList { return tx.AccessList }
func (tx *ResourceTx) data() []byte           { return tx.Data }
func (tx *ResourceTx) gas() uint64            { return tx.Gas }
func (tx *ResourceTx) gasFeeCap() *big.Int    { return tx.GasFeeCap }
func (tx *ResourceTx) gasTipCap() *big.Int    { return tx.GasTipCap }
func (tx *ResourceTx) gasPrice() *big.Int     { return tx.GasFeeCap }
func (tx *ResourceTx) value() *big.Int        { return tx.Value }
func (tx *ResourceTx) nonce() uint64          { return tx.Nonce }
func (tx *ResourceTx) to() *common.Address    { return tx.To }

// effectiveGasPrice is identical to DynamicFeeTx — the per-dimension Phase
// 10D check is a separate consensus rule, not a fee replacement.
func (tx *ResourceTx) effectiveGasPrice(dst *big.Int, baseFee *big.Int) *big.Int {
	if baseFee == nil {
		return dst.Set(tx.GasFeeCap)
	}
	tip := dst.Sub(tx.GasFeeCap, baseFee)
	if tip.Cmp(tx.GasTipCap) > 0 {
		tip.Set(tx.GasTipCap)
	}
	return tip.Add(tip, baseFee)
}

func (tx *ResourceTx) rawSignatureValues() (v, r, s *big.Int) {
	return tx.V, tx.R, tx.S
}

func (tx *ResourceTx) setSignatureValues(chainID, v, r, s *big.Int) {
	tx.ChainID, tx.V, tx.R, tx.S = chainID, v, r, s
}

func (tx *ResourceTx) encode(b *bytes.Buffer) error {
	return rlp.Encode(b, tx)
}

func (tx *ResourceTx) decode(input []byte) error {
	return rlp.DecodeBytes(input, tx)
}
