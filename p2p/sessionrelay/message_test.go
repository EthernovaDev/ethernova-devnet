package sessionrelay

import (
	"bytes"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestSessionRelayMessageRoundTrip(t *testing.T) {
	update := types.SessionRelayUpdate{
		SessionID:      common.HexToHash("0x01"),
		SequenceNumber: 1,
		StateHash:      common.HexToHash("0x02"),
		PayloadHash:    common.HexToHash("0x03"),
		Signature:      bytes.Repeat([]byte{0x42}, 65),
	}
	msg, err := NewMessage(update, []byte("hello"))
	if err != nil {
		t.Fatalf("NewMessage: %v", err)
	}
	enc, err := msg.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	out, err := Decode(enc)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if out.Update.SessionID != update.SessionID || out.Update.SequenceNumber != update.SequenceNumber || out.Update.StateHash != update.StateHash || out.Update.PayloadHash != update.PayloadHash || !bytes.Equal(out.Update.Signature, update.Signature) {
		t.Fatalf("roundtrip mismatch: %#v", out.Update)
	}
}

func TestSessionRelayRejectsMalformed(t *testing.T) {
	_, err := NewMessage(types.SessionRelayUpdate{SessionID: common.HexToHash("0x01"), SequenceNumber: 1, Signature: []byte{1}}, nil)
	if err == nil {
		t.Fatal("expected signature length rejection")
	}
}
