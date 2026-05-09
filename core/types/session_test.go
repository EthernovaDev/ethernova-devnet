package types

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestSessionStateRLPRoundTrip(t *testing.T) {
	in := &SessionState{
		Initiator:       common.HexToAddress("0x1111111111111111111111111111111111111111"),
		Counterparty:    common.HexToAddress("0x2222222222222222222222222222222222222222"),
		SessionType:     SessionTypeChat,
		Status:          SessionStatusDisputed,
		StateHash:       common.HexToHash("0x1234"),
		SequenceNumber:  42,
		TimeoutBlock:    1000,
		DisputeDeadline: 1010,
		DisputeRules:    common.HexToHash("0xabcd"),
		OpenedBlock:     900,
		ClosedBlock:     0,
	}
	enc, err := in.EncodeRLP()
	if err != nil {
		t.Fatalf("EncodeRLP: %v", err)
	}
	out, err := DecodeSessionState(enc)
	if err != nil {
		t.Fatalf("DecodeSessionState: %v", err)
	}
	if *out != *in {
		t.Fatalf("roundtrip mismatch\nin:  %#v\nout: %#v", in, out)
	}
}

func TestSessionCommitMessageHashDeterministic(t *testing.T) {
	id := common.HexToHash("0x01")
	state := common.HexToHash("0x02")
	a := SessionCommitMessageHash(id, 7, state)
	b := SessionCommitMessageHash(id, 7, state)
	c := SessionCommitMessageHash(id, 8, state)
	if a != b {
		t.Fatal("same session message produced different hashes")
	}
	if a == c {
		t.Fatal("sequence number did not affect message hash")
	}
}

func TestSessionRelayUpdateRLPRoundTrip(t *testing.T) {
	in := &SessionRelayUpdate{
		SessionID:      common.HexToHash("0x01"),
		SequenceNumber: 5,
		StateHash:      common.HexToHash("0x02"),
		PayloadHash:    common.HexToHash("0x03"),
		Signature:      []byte{1, 2, 3},
	}
	enc, err := in.EncodeRLP()
	if err != nil {
		t.Fatalf("EncodeRLP: %v", err)
	}
	out, err := DecodeSessionRelayUpdate(enc)
	if err != nil {
		t.Fatalf("DecodeSessionRelayUpdate: %v", err)
	}
	if out.SessionID != in.SessionID || out.SequenceNumber != in.SequenceNumber || out.StateHash != in.StateHash || out.PayloadHash != in.PayloadHash || string(out.Signature) != string(in.Signature) {
		t.Fatalf("roundtrip mismatch\nin:  %#v\nout: %#v", in, out)
	}
}
