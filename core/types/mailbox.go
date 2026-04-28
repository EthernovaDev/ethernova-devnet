// Ethernova: Mailbox Protocol Object Types (NIP-0004 Phase 4)
//
// A Mailbox is a Protocol Object (type_tag = ProtoTypeMailbox = 0x01) that
// stores incoming messages as an ordered queue. The Phase 1 Protocol Object
// Registry (0xFF01) holds the canonical ProtocolObject record. The variable
// portion (config: capacity, retention, ACL, postage) is RLP-encoded into
// ProtocolObject.StateData. The actual message queue and per-mailbox counters
// live at MailboxOpsAddr (0xFF04) — see core/vm/ethernova_mailbox.go.
//
// Two RLP records are defined in this file:
//   - MailboxConfig : the static configuration written into PO.StateData.
//   - MailboxMessage: a single queue entry returned by recv/peek and stored
//                     in chunked storage at 0xFF04 keyed by (mailbox_id, idx).
//
// RLP encoding is deterministic, platform-independent, and field-ordered for
// consensus safety. Field order MUST NOT change without a hard fork.

package types

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
)

// Mailbox ACL modes. ACLMode determines how the ACL list is interpreted by
// the sendMessage path. The numeric values are stable on-chain consensus
// constants — DO NOT renumber.
const (
	// MailboxACLModeOpen accepts any sender. The ACL list is ignored.
	MailboxACLModeOpen uint8 = 0x00
	// MailboxACLModeWhitelist accepts only senders listed in ACL.
	MailboxACLModeWhitelist uint8 = 0x01
	// MailboxACLModeBlacklist rejects senders listed in ACL; accepts others.
	MailboxACLModeBlacklist uint8 = 0x02
)

// IsValidMailboxACLMode reports whether the tag is a recognized ACL mode.
func IsValidMailboxACLMode(mode uint8) bool {
	switch mode {
	case MailboxACLModeOpen, MailboxACLModeWhitelist, MailboxACLModeBlacklist:
		return true
	default:
		return false
	}
}

// MailboxRetentionPolicy values. Reserved for future use (NIP-0003 chat
// retention, time-bounded mailboxes). At Phase 4, the retention policy is
// stored but not yet enforced by the queue (capacity_limit is the only
// active backpressure). DO NOT renumber.
const (
	// MailboxRetentionNever keeps messages until explicitly recvd.
	MailboxRetentionNever uint8 = 0x00
	// MailboxRetentionTTL drops messages older than RetentionBlocks
	// (reserved — not enforced in Phase 4 to keep the queue purely FIFO).
	MailboxRetentionTTL uint8 = 0x01
)

// IsValidMailboxRetention reports whether the tag is a recognized retention
// policy. Validation happens at create/configure time.
func IsValidMailboxRetention(p uint8) bool {
	switch p {
	case MailboxRetentionNever, MailboxRetentionTTL:
		return true
	default:
		return false
	}
}

// MailboxConfig is the static, owner-configurable portion of a Mailbox
// Protocol Object. It is RLP-encoded into ProtocolObject.StateData by the
// 0x2C precompile (createMailbox / configureMailbox).
//
// Field order is fixed for deterministic RLP. DO NOT REORDER without a fork.
type MailboxConfig struct {
	CapacityLimit   uint64           // Max messages that may be in the queue at once
	RetentionPolicy uint8            // See MailboxRetention* constants
	RetentionBlocks uint64           // TTL in blocks if RetentionPolicy == TTL
	MinPostageWei   *big.Int         // Minimum NOVA the sender must attach per send
	ACLMode         uint8            // See MailboxACLMode* constants
	ACL             []common.Address // Whitelist or blacklist depending on ACLMode
}

// EncodeRLP encodes the MailboxConfig deterministically.
func (m *MailboxConfig) EncodeRLP() ([]byte, error) {
	postage := m.MinPostageWei
	if postage == nil {
		postage = new(big.Int)
	}
	return rlp.EncodeToBytes([]interface{}{
		m.CapacityLimit,
		m.RetentionPolicy,
		m.RetentionBlocks,
		postage,
		m.ACLMode,
		m.ACL,
	})
}

// mailboxConfigRLP is the intermediate RLP struct.
type mailboxConfigRLP struct {
	CapacityLimit   uint64
	RetentionPolicy uint8
	RetentionBlocks uint64
	MinPostageWei   *big.Int
	ACLMode         uint8
	ACL             []common.Address
}

// DecodeMailboxConfig parses a MailboxConfig from RLP bytes.
func DecodeMailboxConfig(data []byte) (*MailboxConfig, error) {
	if len(data) == 0 {
		return nil, errors.New("DecodeMailboxConfig: empty data")
	}
	var raw mailboxConfigRLP
	if err := rlp.DecodeBytes(data, &raw); err != nil {
		return nil, err
	}
	postage := raw.MinPostageWei
	if postage == nil {
		postage = new(big.Int)
	}
	return &MailboxConfig{
		CapacityLimit:   raw.CapacityLimit,
		RetentionPolicy: raw.RetentionPolicy,
		RetentionBlocks: raw.RetentionBlocks,
		MinPostageWei:   postage,
		ACLMode:         raw.ACLMode,
		ACL:             raw.ACL,
	}, nil
}

// IsSenderAllowed returns true if the sender is permitted to send to this
// mailbox under the current ACL configuration. The owner is always allowed
// to send to themselves regardless of ACL mode (useful for self-tests and
// owner-driven flows).
func (m *MailboxConfig) IsSenderAllowed(sender, owner common.Address) bool {
	if sender == owner {
		return true
	}
	switch m.ACLMode {
	case MailboxACLModeOpen:
		return true
	case MailboxACLModeWhitelist:
		for _, a := range m.ACL {
			if a == sender {
				return true
			}
		}
		return false
	case MailboxACLModeBlacklist:
		for _, a := range m.ACL {
			if a == sender {
				return false
			}
		}
		return true
	default:
		// Unknown mode — fail closed.
		return false
	}
}

// MailboxMessage is a single queue entry. Per NIP-0004 §3.5 a Mailbox queue
// entry carries {sender, payload_hash, timestamp, sequence_number}. The
// payload itself is intentionally NOT stored on-chain — sender posts a hash
// referencing off-chain content (typically via the Phase 3 ContentRef
// primitive) so the on-chain footprint per message stays bounded and small.
//
// SequenceNumber is sourced from the global Phase 2 deferred queue counter
// at enqueue time. It is monotonic across the entire chain (not just within
// a single mailbox) — using the global counter for free strict ordering and
// uniqueness, matching the determinism invariant in the deferred processing
// engine.
//
// Field order is fixed for RLP determinism. DO NOT reorder.
type MailboxMessage struct {
	Sender         common.Address // msg.sender of the originating sendMessage call
	PayloadHash    common.Hash    // Hash referring to off-chain content
	Timestamp      uint64         // Block number at which sendMessage was called
	SequenceNumber uint64         // Global deferred-queue seq number for this delivery
}

// EncodeRLP encodes a MailboxMessage deterministically.
func (m *MailboxMessage) EncodeRLP() ([]byte, error) {
	return rlp.EncodeToBytes([]interface{}{
		m.Sender,
		m.PayloadHash,
		m.Timestamp,
		m.SequenceNumber,
	})
}

// mailboxMessageRLP is the intermediate RLP struct.
type mailboxMessageRLP struct {
	Sender         common.Address
	PayloadHash    common.Hash
	Timestamp      uint64
	SequenceNumber uint64
}

// DecodeMailboxMessage parses a MailboxMessage from RLP bytes.
func DecodeMailboxMessage(data []byte) (*MailboxMessage, error) {
	if len(data) == 0 {
		return nil, errors.New("DecodeMailboxMessage: empty data")
	}
	var raw mailboxMessageRLP
	if err := rlp.DecodeBytes(data, &raw); err != nil {
		return nil, err
	}
	return &MailboxMessage{
		Sender:         raw.Sender,
		PayloadHash:    raw.PayloadHash,
		Timestamp:      raw.Timestamp,
		SequenceNumber: raw.SequenceNumber,
	}, nil
}

// MailboxSendEffect is the payload carried by an EffectTypeMailboxSend entry
// in the deferred queue. When sendMessage is called at block N, this struct
// is RLP-encoded and stored as the deferred effect payload. At block N+1 the
// deferred processing dispatcher decodes it and delivers a MailboxMessage to
// the target mailbox.
//
// Field order is fixed for RLP determinism. DO NOT reorder.
type MailboxSendEffect struct {
	MailboxID   common.Hash    // target Mailbox PO ID
	Sender      common.Address // original msg.sender (also stored on the deferred entry)
	PayloadHash common.Hash    // off-chain content hash
	SourceBlock uint64         // block N at which sendMessage was called
}

// EncodeRLP encodes a MailboxSendEffect deterministically.
func (m *MailboxSendEffect) EncodeRLP() ([]byte, error) {
	return rlp.EncodeToBytes([]interface{}{
		m.MailboxID,
		m.Sender,
		m.PayloadHash,
		m.SourceBlock,
	})
}

// mailboxSendEffectRLP is the intermediate RLP struct.
type mailboxSendEffectRLP struct {
	MailboxID   common.Hash
	Sender      common.Address
	PayloadHash common.Hash
	SourceBlock uint64
}

// DecodeMailboxSendEffect parses a MailboxSendEffect from RLP bytes.
func DecodeMailboxSendEffect(data []byte) (*MailboxSendEffect, error) {
	if len(data) == 0 {
		return nil, errors.New("DecodeMailboxSendEffect: empty data")
	}
	var raw mailboxSendEffectRLP
	if err := rlp.DecodeBytes(data, &raw); err != nil {
		return nil, err
	}
	return &MailboxSendEffect{
		MailboxID:   raw.MailboxID,
		Sender:      raw.Sender,
		PayloadHash: raw.PayloadHash,
		SourceBlock: raw.SourceBlock,
	}, nil
}
