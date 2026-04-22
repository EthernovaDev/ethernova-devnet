// Ethernova: Deferred Effect Types (NIP-0004 Phase 2)
//
// A DeferredEffect is an entry in the Pending Effects Queue. It represents
// work scheduled at block N to be executed at the start of block N+1 by the
// Deferred Processing Phase. At Phase 2 the queue itself and its ordering
// are the consensus-critical artifact — the handlers that turn an effect
// into a side-effect (message delivery, callback execution) are the subject
// of later phases (Mailbox = Phase 4, Async Callback = Phase 7/11).
//
// RLP encoding is deterministic, platform-independent, and field-ordered
// for consensus safety. DO NOT reorder fields.

package types

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
)

// DeferredEffect type tags. These are discriminators the Phase 0 dispatcher
// will use in later phases to route effects to their handler. In Phase 2 all
// effect types drain to the no-op handler — they exist so that harnesses can
// emit distinguishable entries and we can verify ordering.
const (
	// EffectTypeNoop is a harness/test-only effect. Drains without side effect.
	EffectTypeNoop uint8 = 0x00
	// EffectTypePing is a harness effect that increments a state counter.
	// Used to prove that per-entry processing actually executes.
	EffectTypePing uint8 = 0x01
	// EffectTypeMailboxSend is reserved for Phase 4 (Mailbox).
	EffectTypeMailboxSend uint8 = 0x10
	// EffectTypeAsyncCallback is reserved for Phase 7/11.
	EffectTypeAsyncCallback uint8 = 0x20
	// EffectTypeSessionUpdate is reserved for Phase 7 (Session).
	EffectTypeSessionUpdate uint8 = 0x30
)

// IsValidDeferredEffectType reports whether a tag is recognised at the
// current phase. Unknown tags must be rejected at enqueue time, not
// silently dropped at processing time — otherwise we create non-determinism
// at the fork boundary for any client that ships a newer enum.
func IsValidDeferredEffectType(tag uint8) bool {
	switch tag {
	case EffectTypeNoop, EffectTypePing,
		EffectTypeMailboxSend, EffectTypeAsyncCallback, EffectTypeSessionUpdate:
		return true
	default:
		return false
	}
}

// DeferredEffectTypeName returns a human-readable tag name.
func DeferredEffectTypeName(tag uint8) string {
	switch tag {
	case EffectTypeNoop:
		return "Noop"
	case EffectTypePing:
		return "Ping"
	case EffectTypeMailboxSend:
		return "MailboxSend"
	case EffectTypeAsyncCallback:
		return "AsyncCallback"
	case EffectTypeSessionUpdate:
		return "SessionUpdate"
	default:
		return "Unknown"
	}
}

// DeferredEffect is a single pending-queue entry.
//
// Fields are ordered for deterministic RLP. The sequence number is the
// authoritative ordering key — it is a monotonic uint64 counter minted at
// enqueue time by the queue precompile, never decremented, and used directly
// as the storage key for O(1) lookup during Phase 0 processing.
//
// SourceBlock records the block during which the enqueue happened. This is
// informational only; Phase 0 does NOT decide "what to process" by filtering
// on SourceBlock — it drains [head, frontier) where frontier is snapshotted
// at the start of Phase 0. SourceBlock is still useful for RPC/debugging and
// for handlers that need to know "when was I scheduled".
type DeferredEffect struct {
	SeqNum       uint64         // Monotonic per-queue sequence. Assigned at enqueue.
	EffectType   uint8          // Discriminator; see EffectType* constants.
	SourceBlock  uint64         // Block number at which this was enqueued.
	SourceCaller common.Address // msg.sender of the tx that enqueued this.
	SourceTxHash common.Hash    // Hash of the source tx (for traceability).
	Payload      []byte         // Opaque, size-capped by MaxDeferredEffectPayloadBytes.
}

// EncodeRLP encodes the DeferredEffect deterministically.
// Field order is fixed and MUST match DecodeDeferredEffect.
func (e *DeferredEffect) EncodeRLP() ([]byte, error) {
	return rlp.EncodeToBytes([]interface{}{
		e.SeqNum,
		e.EffectType,
		e.SourceBlock,
		e.SourceCaller,
		e.SourceTxHash,
		e.Payload,
	})
}

// deferredEffectRLP is the intermediate struct for decoding.
type deferredEffectRLP struct {
	SeqNum       uint64
	EffectType   uint8
	SourceBlock  uint64
	SourceCaller common.Address
	SourceTxHash common.Hash
	Payload      []byte
}

// DecodeDeferredEffect decodes a DeferredEffect from RLP bytes.
func DecodeDeferredEffect(data []byte) (*DeferredEffect, error) {
	var raw deferredEffectRLP
	if err := rlp.DecodeBytes(data, &raw); err != nil {
		return nil, err
	}
	return &DeferredEffect{
		SeqNum:       raw.SeqNum,
		EffectType:   raw.EffectType,
		SourceBlock:  raw.SourceBlock,
		SourceCaller: raw.SourceCaller,
		SourceTxHash: raw.SourceTxHash,
		Payload:      raw.Payload,
	}, nil
}
