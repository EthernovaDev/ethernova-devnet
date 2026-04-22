// Ethernova: DeferredEffect RLP round-trip and determinism tests
// (NIP-0004 Phase 2)
//
// These tests run fully offline — no statedb, no EVM, no network — and
// are the cheapest possible check that the on-the-wire encoding of queue
// entries is stable. If this file fails, the queue will almost certainly
// cause consensus splits: encoding is the core invariant.
//
// Run: go test -run TestDeferredEffect -v ./core/types/

package types

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestDeferredEffectRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		ef   DeferredEffect
	}{
		{
			name: "minimal_noop",
			ef: DeferredEffect{
				SeqNum:       0,
				EffectType:   EffectTypeNoop,
				SourceBlock:  1,
				SourceCaller: common.HexToAddress("0x1"),
				SourceTxHash: common.HexToHash("0x2"),
				Payload:      nil,
			},
		},
		{
			name: "ping_with_payload",
			ef: DeferredEffect{
				SeqNum:       42,
				EffectType:   EffectTypePing,
				SourceBlock:  1000,
				SourceCaller: common.HexToAddress("0xdeadbeef"),
				SourceTxHash: common.HexToHash("0xfeedface"),
				Payload:      []byte("hello world"),
			},
		},
		{
			name: "max_uint64_seq",
			ef: DeferredEffect{
				SeqNum:       ^uint64(0),
				EffectType:   EffectTypeMailboxSend,
				SourceBlock:  ^uint64(0),
				SourceCaller: common.HexToAddress("0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF"),
				SourceTxHash: common.BigToHash(new(big.Int).SetBytes(bytes.Repeat([]byte{0xff}, 32))),
				Payload:      bytes.Repeat([]byte{0xaa}, 512),
			},
		},
		{
			name: "empty_payload_explicit",
			ef: DeferredEffect{
				SeqNum:       7,
				EffectType:   EffectTypeAsyncCallback,
				SourceBlock:  99,
				SourceCaller: common.Address{},
				SourceTxHash: common.Hash{},
				Payload:      []byte{},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := tc.ef.EncodeRLP()
			if err != nil {
				t.Fatalf("EncodeRLP: %v", err)
			}
			if len(data) == 0 {
				t.Fatalf("empty encoding")
			}
			dec, err := DecodeDeferredEffect(data)
			if err != nil {
				t.Fatalf("DecodeDeferredEffect: %v", err)
			}
			if dec.SeqNum != tc.ef.SeqNum {
				t.Errorf("SeqNum: got %d want %d", dec.SeqNum, tc.ef.SeqNum)
			}
			if dec.EffectType != tc.ef.EffectType {
				t.Errorf("EffectType: got %d want %d", dec.EffectType, tc.ef.EffectType)
			}
			if dec.SourceBlock != tc.ef.SourceBlock {
				t.Errorf("SourceBlock: got %d want %d", dec.SourceBlock, tc.ef.SourceBlock)
			}
			if dec.SourceCaller != tc.ef.SourceCaller {
				t.Errorf("SourceCaller: got %v want %v", dec.SourceCaller, tc.ef.SourceCaller)
			}
			if dec.SourceTxHash != tc.ef.SourceTxHash {
				t.Errorf("SourceTxHash: got %v want %v", dec.SourceTxHash, tc.ef.SourceTxHash)
			}
			// nil vs empty []byte round-trip — RLP treats both as empty
			// so we compare by content, not by nil-ness.
			if !bytes.Equal(dec.Payload, tc.ef.Payload) {
				t.Errorf("Payload: got %x want %x", dec.Payload, tc.ef.Payload)
			}
		})
	}
}

// TestDeferredEffectDeterminism encodes the same logical entry twice and
// verifies byte-for-byte equality. If RLP ever becomes non-deterministic
// (e.g. someone adds a map somewhere) this is the canary.
func TestDeferredEffectDeterminism(t *testing.T) {
	ef := DeferredEffect{
		SeqNum:       12345,
		EffectType:   EffectTypePing,
		SourceBlock:  67890,
		SourceCaller: common.HexToAddress("0xabc"),
		SourceTxHash: common.HexToHash("0xdef"),
		Payload:      []byte("determinism matters"),
	}
	a, err := ef.EncodeRLP()
	if err != nil {
		t.Fatalf("encode 1: %v", err)
	}
	b, err := ef.EncodeRLP()
	if err != nil {
		t.Fatalf("encode 2: %v", err)
	}
	if !bytes.Equal(a, b) {
		t.Fatalf("non-deterministic encoding:\n a=%x\n b=%x", a, b)
	}
}

// TestDeferredEffectTypeValidation ensures every declared constant passes
// validation and no sentinel gets silently accepted.
func TestDeferredEffectTypeValidation(t *testing.T) {
	valid := []uint8{
		EffectTypeNoop,
		EffectTypePing,
		EffectTypeMailboxSend,
		EffectTypeAsyncCallback,
		EffectTypeSessionUpdate,
	}
	for _, tag := range valid {
		if !IsValidDeferredEffectType(tag) {
			t.Errorf("tag 0x%02x should be valid", tag)
		}
	}
	invalid := []uint8{0x02, 0x05, 0x0f, 0x11, 0x21, 0x31, 0xff}
	for _, tag := range invalid {
		if IsValidDeferredEffectType(tag) {
			t.Errorf("tag 0x%02x should be invalid", tag)
		}
	}
}

// TestDeferredEffectDecodeGarbage ensures bad bytes do not panic; they
// must return an error cleanly.
func TestDeferredEffectDecodeGarbage(t *testing.T) {
	bad := [][]byte{
		nil,
		{},
		{0x00},
		{0xff, 0xff, 0xff},
		bytes.Repeat([]byte{0x80}, 50),
	}
	for i, data := range bad {
		if _, err := DecodeDeferredEffect(data); err == nil {
			// empty RLP list might decode to zero-valued struct — that's OK.
			// Only actively malformed bytes must error.
			if len(data) > 1 {
				t.Errorf("case %d: garbage decoded without error", i)
			}
		}
	}
}
