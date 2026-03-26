// Ethernova: Tempo-Style Smart Transactions (Phase 11)
// Inspired by Tempo Transactions - practical AA features:
// - Atomic batching: multiple calls in one transaction
// - Fee delegation: another wallet pays gas (always in NOVA)
// - Scheduling: valid_before/valid_after for time-bound execution
//
// Gas is ALWAYS paid in NOVA. No ERC-20 gas payments.

package types

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

const TempoTxType = 0x04

// TempoCall represents a single call within a Tempo transaction.
// Multiple calls are executed atomically - if any fails, all revert.
type TempoCall struct {
	Target   common.Address `json:"target"`
	Value    *big.Int       `json:"value"`
	GasLimit uint64         `json:"gasLimit"`
	Data     []byte         `json:"data"`
}

// TempoTransactionData holds the extra fields for a Tempo transaction.
// This is stored alongside the standard transaction fields.
type TempoTransactionData struct {
	// Calls is the list of contract calls to execute atomically.
	// If any call reverts, the entire transaction reverts.
	Calls []TempoCall `json:"calls"`

	// FeePayer is the address that pays for gas (optional).
	// If zero, the sender pays. Fee is always in NOVA.
	FeePayer common.Address `json:"feePayer,omitempty"`

	// FeePayerSignature is the ECDSA signature from the fee payer
	// authorizing gas payment for this transaction.
	FeePayerV *big.Int `json:"feePayerV,omitempty"`
	FeePayerR *big.Int `json:"feePayerR,omitempty"`
	FeePayerS *big.Int `json:"feePayerS,omitempty"`

	// ValidBefore is the block number before which this tx must be mined.
	// 0 means no expiry.
	ValidBefore uint64 `json:"validBefore,omitempty"`

	// ValidAfter is the block number after which this tx becomes valid.
	// 0 means immediately valid.
	ValidAfter uint64 `json:"validAfter,omitempty"`
}

// IsScheduled returns true if this transaction has time constraints.
// MaxScheduleWindow is the maximum number of blocks into the future
// that a scheduled transaction can target. Prevents mempool RAM bomb
// where attacker sends millions of far-future txs filling node memory.
// (Gemini security review - apocalyptic scenario 1)
const MaxScheduleWindow uint64 = 500

func (td *TempoTransactionData) IsScheduled() bool {
	return td.ValidBefore > 0 || td.ValidAfter > 0
}

// HasFeePayer returns true if a separate fee payer is specified.
func (td *TempoTransactionData) HasFeePayer() bool {
	return td.FeePayer != (common.Address{})
}

// IsValidAtBlock checks if this transaction is valid at the given block number.
func (td *TempoTransactionData) IsValidAtBlock(blockNumber uint64) bool {
	if td.ValidBefore > 0 && blockNumber >= td.ValidBefore {
		return false // expired
	}
	if td.ValidAfter > 0 && blockNumber < td.ValidAfter {
		return false // not yet valid
	}
	return true
}

// IsScheduleWindowValid checks that validAfter is not too far in the future.
// Rejects txs scheduled >500 blocks ahead to prevent mempool RAM exhaustion.
func (td *TempoTransactionData) IsScheduleWindowValid(currentBlock uint64) bool {
	if td.ValidAfter > 0 && td.ValidAfter > currentBlock+MaxScheduleWindow {
		return false // too far in the future - reject to protect mempool RAM
	}
	if td.ValidBefore > 0 && td.ValidBefore > currentBlock+MaxScheduleWindow {
		return false // expiry too far out
	}
	return true
}
