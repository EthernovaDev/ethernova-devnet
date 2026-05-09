// Package sessionrelay defines the deterministic wire payload used by the
// Phase 7 proving-ground relay. It intentionally stays transport-agnostic: the
// devnet can carry these bytes over any existing P2P path without changing
// consensus rules.
package sessionrelay

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

const MaxPayloadBytes = 1024

// Message is the relay envelope for one off-chain session update.
type Message struct {
	Update      types.SessionRelayUpdate
	Payload     []byte
	PayloadHash common.Hash
}

func NewMessage(update types.SessionRelayUpdate, payload []byte) (*Message, error) {
	if len(payload) > MaxPayloadBytes {
		return nil, fmt.Errorf("sessionrelay: payload exceeds cap (%d > %d)", len(payload), MaxPayloadBytes)
	}
	if update.SessionID == (common.Hash{}) {
		return nil, errors.New("sessionrelay: empty session id")
	}
	if update.SequenceNumber == 0 {
		return nil, errors.New("sessionrelay: zero sequence number")
	}
	if len(update.Signature) != 65 {
		return nil, fmt.Errorf("sessionrelay: invalid signature length %d", len(update.Signature))
	}
	msg := &Message{Update: update, Payload: append([]byte(nil), payload...), PayloadHash: update.PayloadHash}
	return msg, nil
}

func (m *Message) Encode() ([]byte, error) {
	if m == nil {
		return nil, errors.New("sessionrelay: nil message")
	}
	return m.Update.EncodeRLP()
}

func Decode(data []byte) (*Message, error) {
	update, err := types.DecodeSessionRelayUpdate(data)
	if err != nil {
		return nil, err
	}
	return NewMessage(*update, nil)
}
