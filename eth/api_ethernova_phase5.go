// Ethernova: Phase 5 RPC helpers (storage proof generation).
//
// Lives in package eth so it can sit next to api_ethernova.go and
// share the same private helpers without crossing package
// boundaries. The functions here are pure helpers — they hold no
// state and are safe to call concurrently from RPC handlers.

package eth

import (
	"encoding/hex"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/trie"
)

// generateStorageProof builds a Merkle proof for (addr, slot) against
// the current head state's storage trie. Returns the list of RLP-
// encoded trie nodes in the order produced by Trie.Prove — the
// caller wraps them with state.EncodeProofPayload before returning
// to the precompile.
//
// headRoot MUST be the canonical state root of the block the
// statedb was opened against — typically chain.CurrentBlock().Root.
// We do NOT call IntermediateRoot here because that would commit
// pending changes to the trie, which is exactly the wrong thing for
// a read-only RPC.
//
// Equivalent to the storage-proof half of internal/ethapi.GetProof,
// but takes a *state.StateDB directly so the lifecycle RPC layer
// can avoid pulling the full BlockChainAPI surface.
func generateStorageProof(statedb *state.StateDB, headRoot common.Hash, addr common.Address, slot common.Hash) ([][]byte, error) {
	storageRoot := statedb.GetStorageRoot(addr)
	if storageRoot == types.EmptyRootHash || storageRoot == (common.Hash{}) {
		// Empty storage trie. The proof for "absent" is the empty
		// list; the verifier will treat this as "value == zero",
		// which is the correct semantic answer.
		return [][]byte{}, nil
	}
	id := trie.StorageTrieID(headRoot, crypto.Keccak256Hash(addr.Bytes()), storageRoot)
	st, err := trie.NewStateTrie(id, statedb.Database().TrieDB())
	if err != nil {
		return nil, err
	}
	collector := newProofCollector()
	if err := st.Prove(crypto.Keccak256(slot.Bytes()), collector); err != nil {
		return nil, err
	}
	return collector.nodes, nil
}

// proofCollector implements ethdb.KeyValueWriter so it can be passed
// to Trie.Prove. It accumulates RLP-encoded trie nodes in
// insertion order (which is also the verification order).
type proofCollector struct {
	nodes [][]byte
}

func newProofCollector() *proofCollector {
	return &proofCollector{nodes: make([][]byte, 0, 8)}
}

// Put captures a proof node. The key is the node hash; we discard it
// and keep only the value because state.EncodeProofPayload
// re-derives the hash on the verification side.
func (p *proofCollector) Put(key []byte, value []byte) error {
	cp := make([]byte, len(value))
	copy(cp, value)
	p.nodes = append(p.nodes, cp)
	return nil
}

// Delete is required by ethdb.KeyValueWriter but is never called by
// Trie.Prove. We panic if anyone calls it, matching the pattern
// used by internal/ethapi.proofList.
func (p *proofCollector) Delete(key []byte) error {
	panic("eth: proofCollector.Delete unsupported")
}

// commonBytes2Hex is a tiny convenience around hex.EncodeToString.
// Local to api_ethernova.go callers; not exported.
func commonBytes2Hex(b []byte) string {
	return hex.EncodeToString(b)
}

// Compile-time assertion: proofCollector satisfies the writer
// interface trie.Prove expects.
var _ ethdb.KeyValueWriter = (*proofCollector)(nil)
