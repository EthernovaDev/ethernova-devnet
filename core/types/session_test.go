package types

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
)

func TestSessionStateRLPRoundTrip(t *testing.T) {
	in := &SessionState{
		Initiator:          common.HexToAddress("0x1111111111111111111111111111111111111111"),
		Counterparty:       common.HexToAddress("0x2222222222222222222222222222222222222222"),
		SessionType:        SessionTypeChat,
		Status:             SessionStatusDisputed,
		StateHash:          common.HexToHash("0x1234"),
		SequenceNumber:     42,
		TimeoutBlock:       1000,
		DisputeDeadline:    1010,
		DisputeRules:       common.HexToHash("0xabcd"),
		OpenedBlock:        900,
		ClosedBlock:        0,
		InitiatorSigner:    common.HexToAddress("0x1111111111111111111111111111111111111111"),
		CounterpartySigner: common.HexToAddress("0x2222222222222222222222222222222222222222"),
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

func TestSessionStateRLPRoundTripWithSigners(t *testing.T) {
	src := &SessionState{
		Initiator:          common.HexToAddress("0x1111111111111111111111111111111111111111"),
		Counterparty:       common.HexToAddress("0x2222222222222222222222222222222222222222"),
		SessionType:        SessionTypeChat,
		Status:             SessionStatusOpen,
		StateHash:          common.HexToHash("0xabc1"),
		SequenceNumber:     42,
		TimeoutBlock:       1000,
		DisputeDeadline:    1100,
		DisputeRules:       common.HexToHash("0xdef0"),
		OpenedBlock:        900,
		ClosedBlock:        0,
		InitiatorSigner:    common.HexToAddress("0xAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"),
		CounterpartySigner: common.HexToAddress("0xBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"),
	}
	data, err := src.EncodeRLP()
	if err != nil {
		t.Fatalf("EncodeRLP: %v", err)
	}
	got, err := DecodeSessionState(data)
	if err != nil {
		t.Fatalf("DecodeSessionState: %v", err)
	}
	if got.InitiatorSigner != src.InitiatorSigner {
		t.Errorf("InitiatorSigner mismatch: got %s, want %s", got.InitiatorSigner, src.InitiatorSigner)
	}
	if got.CounterpartySigner != src.CounterpartySigner {
		t.Errorf("CounterpartySigner mismatch: got %s, want %s", got.CounterpartySigner, src.CounterpartySigner)
	}
}

func TestSessionStateRLPDecodesLegacyEncoding(t *testing.T) {
	initiator := common.HexToAddress("0x1111111111111111111111111111111111111111")
	counterparty := common.HexToAddress("0x2222222222222222222222222222222222222222")
	legacy, err := rlp.EncodeToBytes([]interface{}{
		initiator, counterparty,
		SessionTypeChat,
		SessionStatusOpen,
		common.HexToHash("0xabc1"),
		uint64(7),
		uint64(800),
		uint64(0),
		common.HexToHash("0xdef0"),
		uint64(700),
		uint64(0),
	})
	if err != nil {
		t.Fatalf("encode legacy: %v", err)
	}
	got, err := DecodeSessionState(legacy)
	if err != nil {
		t.Fatalf("decode legacy: %v", err)
	}
	if got.InitiatorSigner != initiator {
		t.Errorf("legacy InitiatorSigner should mirror Initiator: got %s", got.InitiatorSigner)
	}
	if got.CounterpartySigner != counterparty {
		t.Errorf("legacy CounterpartySigner should mirror Counterparty: got %s", got.CounterpartySigner)
	}
}
