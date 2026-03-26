// Ethernova: State Expiry Index (Phase 15)
// Stores LastTouched timestamps OUTSIDE the state trie in a separate LevelDB index.
// This allows tracking contract activity without changing the state root.
//
// Schema:
//   "X" + address(20 bytes) -> blockNumber(8 bytes big-endian)  = LastTouched per account
//   "x" + blockNumber(8 bytes) -> RLP([]address)                = Accounts touched at block N
//   "A" + address(20 bytes) -> RLP(SlimAccount)                 = Archived account data

package rawdb

import (
	"encoding/binary"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
)

var (
	// expiryLastTouchedPrefix stores per-account LastTouched block number
	expiryLastTouchedPrefix = []byte("X")
	// expiryBlockIndexPrefix stores list of addresses touched at a given block
	expiryBlockIndexPrefix = []byte("x")
	// expiryArchivedPrefix stores archived account data (slim RLP)
	expiryArchivedPrefix = []byte("A")
)

func lastTouchedKey(addr common.Address) []byte {
	return append(expiryLastTouchedPrefix, addr.Bytes()...)
}

func blockIndexKey(blockNumber uint64) []byte {
	key := make([]byte, len(expiryBlockIndexPrefix)+8)
	copy(key, expiryBlockIndexPrefix)
	binary.BigEndian.PutUint64(key[len(expiryBlockIndexPrefix):], blockNumber)
	return key
}

func archivedKey(addr common.Address) []byte {
	return append(expiryArchivedPrefix, addr.Bytes()...)
}

// WriteLastTouched stores the block number when an account was last accessed.
func WriteLastTouched(db ethdb.KeyValueWriter, addr common.Address, blockNumber uint64) {
	val := make([]byte, 8)
	binary.BigEndian.PutUint64(val, blockNumber)
	if err := db.Put(lastTouchedKey(addr), val); err != nil {
		log.Crit("Failed to write last touched", "addr", addr, "err", err)
	}
}

// ReadLastTouched returns the block number when an account was last accessed.
// Returns 0 if the account has never been tracked.
func ReadLastTouched(db ethdb.KeyValueReader, addr common.Address) uint64 {
	data, err := db.Get(lastTouchedKey(addr))
	if err != nil || len(data) < 8 {
		return 0
	}
	return binary.BigEndian.Uint64(data)
}

// WriteBlockTouchedAddresses stores the list of contract addresses touched at a block.
func WriteBlockTouchedAddresses(db ethdb.KeyValueWriter, blockNumber uint64, addresses []common.Address) {
	if len(addresses) == 0 {
		return
	}
	// Simple encoding: concatenate 20-byte addresses
	data := make([]byte, 0, len(addresses)*20)
	for _, addr := range addresses {
		data = append(data, addr.Bytes()...)
	}
	if err := db.Put(blockIndexKey(blockNumber), data); err != nil {
		log.Crit("Failed to write block touched addresses", "block", blockNumber, "err", err)
	}
}

// ReadBlockTouchedAddresses returns the list of contract addresses touched at a block.
func ReadBlockTouchedAddresses(db ethdb.KeyValueReader, blockNumber uint64) []common.Address {
	data, err := db.Get(blockIndexKey(blockNumber))
	if err != nil || len(data) == 0 {
		return nil
	}
	count := len(data) / 20
	addresses := make([]common.Address, 0, count)
	for i := 0; i < count; i++ {
		var addr common.Address
		copy(addr[:], data[i*20:(i+1)*20])
		addresses = append(addresses, addr)
	}
	return addresses
}

// WriteArchivedAccount stores archived contract data for future resurrection.
func WriteArchivedAccount(db ethdb.KeyValueWriter, addr common.Address, data []byte) {
	if err := db.Put(archivedKey(addr), data); err != nil {
		log.Crit("Failed to write archived account", "addr", addr, "err", err)
	}
}

// ReadArchivedAccount returns the archived account data, or nil if not archived.
func ReadArchivedAccount(db ethdb.KeyValueReader, addr common.Address) []byte {
	data, err := db.Get(archivedKey(addr))
	if err != nil {
		return nil
	}
	return data
}

// DeleteArchivedAccount removes archived data (used during resurrection).
func DeleteArchivedAccount(db ethdb.KeyValueWriter, addr common.Address) {
	if err := db.Delete(archivedKey(addr)); err != nil {
		log.Crit("Failed to delete archived account", "addr", addr, "err", err)
	}
}
