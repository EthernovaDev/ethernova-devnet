// Ethernova: Phase 5 Witness Verifier (NIP-0004 §5.5)
//
// This file is a thin, allocation-light wrapper around trie.VerifyProof
// that exposes a single deterministic entry point used by both the
// 0x2F novaStateWitness precompile and the StateLifecycleEngine
// restoration path.
//
// CONSENSUS-CRITICAL invariants:
//
//   1. The verifier is PURE: it never writes state, never returns a
//      mutable buffer, and depends only on (root, key, value, proof).
//      Identical inputs => identical outputs on every node.
//
//   2. The proof format is the same as the standard go-ethereum
//      eth_getProof RPC produces — a list of RLP-encoded trie nodes,
//      addressed by their keccak256 hash (the encoding produced by
//      Trie.Prove). We deserialize each node into an in-memory
//      KeyValueStore and feed it to trie.VerifyProof.
//
//   3. The "key" passed to VerifyProof is the keccak256 of the storage
//      slot (matching how go-ethereum hashes storage keys before
//      writing them into the storage trie — see core/state/state_object.go
//      where snap.Storage uses crypto.Keccak256Hash(key.Bytes())).
//      Phase 5 callers pass the raw 32-byte slot; this file does the
//      hashing internally so the precompile cannot accidentally pass a
//      pre-hashed key.
//
//   4. A "negative" proof — i.e. proof that the slot is unset — is
//      represented by VerifyStorageWitness returning value = zero hash
//      and ok = true. The Phase 5 restoration path only acts on
//      ok = true && value != zero, so a malformed proof producing zero
//      cannot inject a non-zero slot.

package state

import (
	"bytes"
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
)

// ErrEmptyProof is returned when the supplied proof byte slice contains
// zero nodes — the verifier rejects this rather than treating it as a
// trivially-valid empty trie, which would let any (root != EmptyRoot)
// caller bypass verification.
var ErrEmptyProof = errors.New("witness: empty proof payload")

// ErrProofTooLarge is returned when the encoded proof exceeds the cap
// passed by the caller. The precompile uses this to reject DoS-sized
// proofs before doing any RLP work.
var ErrProofTooLarge = errors.New("witness: proof exceeds size cap")

// ErrProofMalformed is returned when the proof byte stream cannot be
// parsed as a sequence of length-prefixed RLP node blobs.
var ErrProofMalformed = errors.New("witness: proof payload malformed")

// DecodeProofPayload parses the precompile-format proof payload into a
// list of RLP-encoded trie nodes. The on-the-wire format is:
//
//	count(uint32 BE) || (len(uint32 BE) || nodeBytes)*
//
// We use uint32 lengths to keep the encoding compact; a single trie
// node is always under 4 GiB. The total byte length is bounded by the
// caller via maxBytes — when it is exceeded we reject up-front.
func DecodeProofPayload(payload []byte, maxBytes uint64) ([][]byte, error) {
	if uint64(len(payload)) > maxBytes {
		return nil, ErrProofTooLarge
	}
	if len(payload) < 4 {
		return nil, ErrProofMalformed
	}
	count := beUint32(payload[0:4])
	cursor := 4
	if count == 0 {
		return nil, ErrEmptyProof
	}
	nodes := make([][]byte, 0, count)
	for i := uint32(0); i < count; i++ {
		if cursor+4 > len(payload) {
			return nil, ErrProofMalformed
		}
		nodeLen := beUint32(payload[cursor : cursor+4])
		cursor += 4
		if uint64(nodeLen) > maxBytes || cursor+int(nodeLen) > len(payload) {
			return nil, ErrProofMalformed
		}
		// Defensive copy so callers don't accidentally mutate the
		// payload backing array.
		node := make([]byte, nodeLen)
		copy(node, payload[cursor:cursor+int(nodeLen)])
		nodes = append(nodes, node)
		cursor += int(nodeLen)
	}
	if cursor != len(payload) {
		return nil, ErrProofMalformed
	}
	return nodes, nil
}

// EncodeProofPayload is the inverse of DecodeProofPayload. Used by the
// RPC layer (ethernova_getStateWitness) to serialize a freshly
// generated proof in the same format the precompile consumes. Callers
// should pass the proof nodes in the order produced by Trie.Prove.
func EncodeProofPayload(nodes [][]byte) []byte {
	totalLen := 4
	for _, n := range nodes {
		totalLen += 4 + len(n)
	}
	out := make([]byte, 0, totalLen)
	out = appendBeUint32(out, uint32(len(nodes)))
	for _, n := range nodes {
		out = appendBeUint32(out, uint32(len(n)))
		out = append(out, n...)
	}
	return out
}

// VerifyStorageWitness checks that `proof` is a valid Merkle proof,
// against `storageRoot`, that the storage key `slot` (raw, NOT
// pre-hashed) maps to the given `expected` value. Returns ok=true iff
// the proof is well-formed AND the trie value at slot equals expected.
//
// The function is pure: it does not write to any persistent store and
// holds the proof nodes only in an in-memory key/value db that is
// discarded on return.
func VerifyStorageWitness(storageRoot common.Hash, slot common.Hash, expected common.Hash, proofNodes [][]byte) (bool, error) {
	if len(proofNodes) == 0 {
		return false, ErrEmptyProof
	}
	memdb := rawdb.NewMemoryDatabase()
	for _, raw := range proofNodes {
		// Trie nodes are addressed by their keccak256 hash. Re-hashing
		// here ensures we never trust caller-supplied keys.
		nodeHash := crypto.Keccak256(raw)
		if err := memdb.Put(nodeHash, raw); err != nil {
			return false, err
		}
	}
	// Storage trie keys are the keccak256 of the raw slot bytes. This
	// matches core/state/state_object.go where reads go through
	// crypto.Keccak256Hash(key.Bytes()) before hitting the trie.
	hashedKey := crypto.Keccak256(slot.Bytes())
	value, err := trie.VerifyProof(storageRoot, hashedKey, memdb)
	if err != nil {
		return false, err
	}
	// VerifyProof returns nil for "key absent". A caller asserting
	// expected == 0 should accept that case; a caller asserting
	// expected != 0 must not.
	if value == nil {
		return expected == (common.Hash{}), nil
	}
	// The storage trie stores values RLP-encoded. We accept either the
	// raw 32-byte form (some trie variants leave it unwrapped) or the
	// canonical RLP-encoded uint256 form, comparing against expected
	// after normalizing.
	got, err := normalizeStorageProofValue(value)
	if err != nil {
		return false, err
	}
	return got == expected, nil
}

// normalizeStorageProofValue handles the two value-encodings that show
// up in storage proofs:
//
//   - raw 32-byte big-endian (used by some witness formats and by
//     account proofs at the leaf)
//   - RLP-encoded big.Int (the canonical EVM storage encoding)
//
// We try the RLP path first; if that fails or yields a too-long byte
// string we fall back to a raw 32-byte interpretation. Anything else
// is rejected.
func normalizeStorageProofValue(raw []byte) (common.Hash, error) {
	// RLP decode attempt
	var decoded []byte
	if err := rlp.DecodeBytes(raw, &decoded); err == nil {
		if len(decoded) > 32 {
			return common.Hash{}, errors.New("witness: storage value exceeds 32 bytes")
		}
		var h common.Hash
		copy(h[32-len(decoded):], decoded)
		return h, nil
	}
	// Raw 32-byte fallback
	if len(raw) > 32 {
		return common.Hash{}, errors.New("witness: storage value exceeds 32 bytes")
	}
	var h common.Hash
	copy(h[32-len(raw):], raw)
	return h, nil
}

// IsZeroProof reports whether the proof payload represents the
// canonical "no proof" sentinel — i.e. an explicitly-empty proof
// produced by an RPC client that doesn't have the data. The lifecycle
// engine never trusts such a payload; this helper exists so the
// precompile can reject it early with a stable error message.
func IsZeroProof(payload []byte) bool {
	return len(payload) == 0 || bytes.Equal(payload, make([]byte, len(payload)))
}

// --- tiny BE helpers (avoid pulling encoding/binary into every caller) ---

func beUint32(b []byte) uint32 {
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

func appendBeUint32(out []byte, v uint32) []byte {
	return append(out, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
}
