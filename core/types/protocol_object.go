// Ethernova: Protocol Object Types (NIP-0004 Phase 1)
//
// Protocol Objects are first-class entities in the Ethernova state tree.
// They are stored as storage slots at the Protocol Object Registry system address.
// Each object has a unique bytes32 ID, an owner, a type tag, and type-specific data.
//
// This file defines the canonical struct, RLP encoding, and type constants.
// RLP encoding MUST be deterministic and platform-independent.

package types

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
)

// Protocol Object type tags (uint8 discriminator).
const (
	ProtoTypeMailbox          uint8 = 0x01
	ProtoTypeSession          uint8 = 0x02
	ProtoTypeContentReference uint8 = 0x03
	ProtoTypeIdentity         uint8 = 0x04
	ProtoTypeSubscription     uint8 = 0x05
	ProtoTypeGameRoom         uint8 = 0x06
)

// MaxProtocolObjectTypes is the hard cap from NIP-0004 §12.5.
// Adding a new type beyond this requires rigorous impact evaluation.
const MaxProtocolObjectTypes = 8

// ProtocolObject is the canonical structure for all protocol-level entities.
// Fields are ordered for deterministic RLP encoding.
// DO NOT reorder fields — RLP is position-dependent.
type ProtocolObject struct {
	ID               common.Hash    `json:"id"`               // Unique identifier (bytes32)
	Owner            common.Address `json:"owner"`            // Owner address
	TypeTag          uint8          `json:"typeTag"`          // Object type discriminator
	StateData        []byte         `json:"stateData"`        // Type-specific state (size-capped per type)
	ExpiryBlock      uint64         `json:"expiryBlock"`      // Block at which object archives if not touched
	LastTouchedBlock uint64         `json:"lastTouchedBlock"` // Last block this object was accessed
	RentBalance      *big.Int       `json:"rentBalance"`      // Available balance for storage rent
}

// EncodeRLP encodes the ProtocolObject to RLP deterministically.
// The field order is fixed and MUST match DecodeRLP exactly.
func (po *ProtocolObject) EncodeRLP() ([]byte, error) {
	rentBal := po.RentBalance
	if rentBal == nil {
		rentBal = new(big.Int)
	}
	return rlp.EncodeToBytes([]interface{}{
		po.ID,
		po.Owner,
		po.TypeTag,
		po.StateData,
		po.ExpiryBlock,
		po.LastTouchedBlock,
		rentBal,
	})
}

// protocolObjectRLP is the intermediate struct for RLP decoding.
type protocolObjectRLP struct {
	ID               common.Hash
	Owner            common.Address
	TypeTag          uint8
	StateData        []byte
	ExpiryBlock      uint64
	LastTouchedBlock uint64
	RentBalance      *big.Int
}

// DecodeProtocolObject decodes a ProtocolObject from RLP bytes.
func DecodeProtocolObject(data []byte) (*ProtocolObject, error) {
	var raw protocolObjectRLP
	if err := rlp.DecodeBytes(data, &raw); err != nil {
		return nil, err
	}
	rentBal := raw.RentBalance
	if rentBal == nil {
		rentBal = new(big.Int)
	}
	return &ProtocolObject{
		ID:               raw.ID,
		Owner:            raw.Owner,
		TypeTag:          raw.TypeTag,
		StateData:        raw.StateData,
		ExpiryBlock:      raw.ExpiryBlock,
		LastTouchedBlock: raw.LastTouchedBlock,
		RentBalance:      rentBal,
	}, nil
}

// IsValidType returns true if the type tag is a recognized Protocol Object type.
func IsValidProtocolObjectType(typeTag uint8) bool {
	return typeTag >= ProtoTypeMailbox && typeTag <= ProtoTypeGameRoom
}

// ProtocolObjectTypeName returns the human-readable name for a type tag.
func ProtocolObjectTypeName(typeTag uint8) string {
	switch typeTag {
	case ProtoTypeMailbox:
		return "Mailbox"
	case ProtoTypeSession:
		return "Session"
	case ProtoTypeContentReference:
		return "ContentReference"
	case ProtoTypeIdentity:
		return "Identity"
	case ProtoTypeSubscription:
		return "Subscription"
	case ProtoTypeGameRoom:
		return "GameRoom"
	default:
		return "Unknown"
	}
}