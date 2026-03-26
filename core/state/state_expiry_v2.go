// Ethernova: State Expiry v2 (Phase 15)
// Uses external LevelDB index instead of modifying the state trie.
// This is deterministic because:
// 1. LastTouched is stored outside the trie (doesn't affect state root)
// 2. The sweep reads addresses from a block-indexed list (deterministic order)
// 3. Expired accounts are processed in the order they were originally touched
//
// The sweep runs at block N and looks up block (N - ExpiryPeriod).
// All contracts touched at that old block that haven't been touched since get archived.

package state

import (
	"bytes"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/holiman/uint256"
)

// StateExpiryEngine handles contract expiry using an external index.
type StateExpiryEngine struct {
	db ethdb.Database
}

// NewStateExpiryEngine creates a new expiry engine backed by the given database.
func NewStateExpiryEngine(db ethdb.Database) *StateExpiryEngine {
	return &StateExpiryEngine{db: db}
}

// TouchContract records that a contract was accessed at the given block.
// This writes to the external index only - does NOT modify the state trie.
func (e *StateExpiryEngine) TouchContract(addr common.Address, blockNumber uint64) {
	current := rawdb.ReadLastTouched(e.db, addr)
	if current == blockNumber {
		return // already touched at this block
	}
	rawdb.WriteLastTouched(e.db, addr, blockNumber)
}

// RecordBlockTouches saves the list of contracts touched during a block.
// Called at the end of block processing (in Finalize).
func (e *StateExpiryEngine) RecordBlockTouches(blockNumber uint64, addresses []common.Address) {
	if len(addresses) == 0 {
		return
	}
	// Sort for determinism
	sort.Slice(addresses, func(i, j int) bool {
		return bytes.Compare(addresses[i][:], addresses[j][:]) < 0
	})
	// Deduplicate
	unique := make([]common.Address, 0, len(addresses))
	for i, addr := range addresses {
		if i == 0 || addr != addresses[i-1] {
			unique = append(unique, addr)
		}
	}
	rawdb.WriteBlockTouchedAddresses(e.db, blockNumber, unique)
}

// SweepExpired checks contracts touched at (currentBlock - expiryPeriod) and
// archives any that haven't been touched since. Returns list of expired addresses.
// This modifies the state trie (deletes accounts) but in a DETERMINISTIC way
// because the input list comes from the sorted block index.
func (e *StateExpiryEngine) SweepExpired(
	statedb *StateDB,
	currentBlock uint64,
	expiryPeriod uint64,
) []common.Address {
	if currentBlock < expiryPeriod {
		return nil
	}
	targetBlock := currentBlock - expiryPeriod
	candidates := rawdb.ReadBlockTouchedAddresses(e.db, targetBlock)
	if len(candidates) == 0 {
		return nil
	}

	var expired []common.Address
	for _, addr := range candidates {
		// Check if the contract was touched again after targetBlock
		lastTouched := rawdb.ReadLastTouched(e.db, addr)
		if lastTouched > targetBlock {
			continue // still active
		}

		// Verify it's a contract (has code)
		obj := statedb.getStateObject(addr)
		if obj == nil || obj.deleted {
			continue
		}
		if bytes.Equal(obj.data.CodeHash, types.EmptyCodeHash.Bytes()) {
			continue // EOA - never expire
		}

		// Archive the account data
		receipt := types.SlimAccountRLP(obj.data)
		rawdb.WriteArchivedAccount(e.db, addr, receipt)

		// Delete from state trie
		obj.markSelfdestructed()
		obj.data.Balance = new(uint256.Int)
		obj.data.Nonce = 0
		obj.data.CodeHash = types.EmptyCodeHash.Bytes()
		obj.data.Root = types.EmptyRootHash

		expired = append(expired, addr)

		log.Info("State expiry: archived contract",
			"address", addr.Hex(),
			"lastTouched", lastTouched,
			"currentBlock", currentBlock,
			"inactive", currentBlock-lastTouched)
	}

	return expired
}
