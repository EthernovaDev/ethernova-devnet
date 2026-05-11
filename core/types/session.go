// Ethernova: Session Protocol Object Types (NIP-0004 Phase 7)
//
// A Session is a bilateral Protocol Object for off-chain state exchange with
// on-chain checkpoints and dispute resolution. The canonical object body lives
// in ProtocolObject.StateData and is RLP encoded for consensus determinism.

package types

import (
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

const (
	// SessionTypeGeneric is the default bilateral state channel type.
	SessionTypeGeneric uint8 = 0x00
	// SessionTypeChat is reserved for the NIP-0003 proving-ground chat flow.
	SessionTypeChat uint8 = 0x01
	// SessionTypeGame is reserved for turn-based channel/game flows.
	SessionTypeGame uint8 = 0x02
)

const (
	// SessionStatusOpen means the session accepts checkpoints and disputes.
	SessionStatusOpen uint8 = 0x01
	// SessionStatusDisputed means at least one disputed checkpoint was posted.
	SessionStatusDisputed uint8 = 0x02
	// SessionStatusClosed is a cooperative close with valid participant sigs.
	SessionStatusClosed uint8 = 0x03
	// SessionStatusExpired is a deterministic timeout close by Phase 0 sweep.
	SessionStatusExpired uint8 = 0x04
)

// SessionState is the canonical RLP body stored in ProtocolObject.StateData.
// Field order is consensus-critical. DO NOT reorder existing fields without
// a hard fork. New fields MUST be added at the end with rlp:"optional" tags
// so old encodings remain decodable.
type SessionState struct {
	Initiator       common.Address
	Counterparty    common.Address
	SessionType     uint8
	Status          uint8
	StateHash       common.Hash
	SequenceNumber  uint64
	TimeoutBlock    uint64
	DisputeDeadline uint64
	DisputeRules    common.Hash
	OpenedBlock     uint64
	ClosedBlock     uint64

	// Phase 7.1: explicit signer EOAs whose ECDSA signatures authorize
	// commitState / closeSession / disputeSession. These are usually the
	// same as Initiator / Counterparty (and default to them when zero), but
	// when the session is opened from a Domain 2 channel contract the
	// contract address has no private key and must delegate signing to
	// designated EOAs via these fields.
	InitiatorSigner    common.Address `rlp:"optional"`
	CounterpartySigner common.Address `rlp:"optional"`
}

func (s *SessionState) EncodeRLP() ([]byte, error) {
	return rlp.EncodeToBytes([]interface{}{
		s.Initiator,
		s.Counterparty,
		s.SessionType,
		s.Status,
		s.StateHash,
		s.SequenceNumber,
		s.TimeoutBlock,
		s.DisputeDeadline,
		s.DisputeRules,
		s.OpenedBlock,
		s.ClosedBlock,
		s.InitiatorSigner,
		s.CounterpartySigner,
	})
}

type sessionStateRLP struct {
	Initiator          common.Address
	Counterparty       common.Address
	SessionType        uint8
	Status             uint8
	StateHash          common.Hash
	SequenceNumber     uint64
	TimeoutBlock       uint64
	DisputeDeadline    uint64
	DisputeRules       common.Hash
	OpenedBlock        uint64
	ClosedBlock        uint64
	InitiatorSigner    common.Address `rlp:"optional"`
	CounterpartySigner common.Address `rlp:"optional"`
}

func DecodeSessionState(data []byte) (*SessionState, error) {
	if len(data) == 0 {
		return nil, errors.New("DecodeSessionState: empty data")
	}
	var raw sessionStateRLP
	if err := rlp.DecodeBytes(data, &raw); err != nil {
		return nil, err
	}
	initiatorSigner := raw.InitiatorSigner
	if initiatorSigner == (common.Address{}) {
		initiatorSigner = raw.Initiator
	}
	counterpartySigner := raw.CounterpartySigner
	if counterpartySigner == (common.Address{}) {
		counterpartySigner = raw.Counterparty
	}
	return &SessionState{
		Initiator:          raw.Initiator,
		Counterparty:       raw.Counterparty,
		SessionType:        raw.SessionType,
		Status:             raw.Status,
		StateHash:          raw.StateHash,
		SequenceNumber:     raw.SequenceNumber,
		TimeoutBlock:       raw.TimeoutBlock,
		DisputeDeadline:    raw.DisputeDeadline,
		DisputeRules:       raw.DisputeRules,
		OpenedBlock:        raw.OpenedBlock,
		ClosedBlock:        raw.ClosedBlock,
		InitiatorSigner:    initiatorSigner,
		CounterpartySigner: counterpartySigner,
	}, nil
}

func IsValidSessionType(sessionType uint8) bool {
	switch sessionType {
	case SessionTypeGeneric, SessionTypeChat, SessionTypeGame:
		return true
	default:
		return false
	}
}

func IsLiveSessionStatus(status uint8) bool {
	return status == SessionStatusOpen || status == SessionStatusDisputed
}

// SessionCommitMessageHash is the canonical digest signed off-chain by both
// participants before commit/close/dispute can mutate session state.
func SessionCommitMessageHash(sessionID common.Hash, sequence uint64, stateHash common.Hash) common.Hash {
	return crypto.Keccak256Hash(
		[]byte("EthernovaSession:v1"),
		sessionID.Bytes(),
		encodeSessionU64(sequence),
		stateHash.Bytes(),
	)
}

func encodeSessionU64(v uint64) []byte {
	var out [8]byte
	out[0] = byte(v >> 56)
	out[1] = byte(v >> 48)
	out[2] = byte(v >> 40)
	out[3] = byte(v >> 32)
	out[4] = byte(v >> 24)
	out[5] = byte(v >> 16)
	out[6] = byte(v >> 8)
	out[7] = byte(v)
	return out[:]
}

// SessionRelayUpdate is the deterministic payload a P2P relay can carry for
// off-chain exchange before either party checkpoints on-chain.
type SessionRelayUpdate struct {
	SessionID      common.Hash
	SequenceNumber uint64
	StateHash      common.Hash
	PayloadHash    common.Hash
	Signature      []byte
}

func (u *SessionRelayUpdate) EncodeRLP() ([]byte, error) {
	return rlp.EncodeToBytes([]interface{}{
		u.SessionID,
		u.SequenceNumber,
		u.StateHash,
		u.PayloadHash,
		u.Signature,
	})
}

type sessionRelayUpdateRLP struct {
	SessionID      common.Hash
	SequenceNumber uint64
	StateHash      common.Hash
	PayloadHash    common.Hash
	Signature      []byte
}

func DecodeSessionRelayUpdate(data []byte) (*SessionRelayUpdate, error) {
	if len(data) == 0 {
		return nil, errors.New("DecodeSessionRelayUpdate: empty data")
	}
	var raw sessionRelayUpdateRLP
	if err := rlp.DecodeBytes(data, &raw); err != nil {
		return nil, err
	}
	return &SessionRelayUpdate{
		SessionID:      raw.SessionID,
		SequenceNumber: raw.SequenceNumber,
		StateHash:      raw.StateHash,
		PayloadHash:    raw.PayloadHash,
		Signature:      raw.Signature,
	}, nil
}
