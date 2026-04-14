// Ethernova: Protocol Object LevelDB accessors (NIP-0004 Phase 1)
//
// Persistent storage for Protocol Object metadata outside the state trie.
// Used for archival snapshots and off-chain indexing only.
// These do NOT affect state root or consensus.
//
// Key prefixes:
//   'P' + id (32 bytes)         -> archived Protocol Object RLP
//   'p' + owner (20 bytes)      -> list of object IDs (for index rebuilding)

package rawdb

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
)

var (
	protocolObjectPrefix      = []byte("P") // P + id -> RLP
	protocolObjectOwnerPrefix = []byte("p") // p + owner + index -> id
)

func protocolObjectKey(id common.Hash) []byte {
	return append(protocolObjectPrefix, id.Bytes()...)
}

func protocolObjectOwnerKey(owner common.Address, index uint64) []byte {
	key := append(protocolObjectOwnerPrefix, owner.Bytes()...)
	buf := make([]byte, 8)
	buf[0] = byte(index >> 56)
	buf[1] = byte(index >> 48)
	buf[2] = byte(index >> 40)
	buf[3] = byte(index >> 32)
	buf[4] = byte(index >> 24)
	buf[5] = byte(index >> 16)
	buf[6] = byte(index >> 8)
	buf[7] = byte(index)
	return append(key, buf...)
}

// WriteArchivedProtocolObject stores a Protocol Object RLP snapshot in LevelDB.
// This is for archival purposes only — does NOT affect state root.
func WriteArchivedProtocolObject(db ethdb.KeyValueWriter, id common.Hash, rlpData []byte) {
	if err := db.Put(protocolObjectKey(id), rlpData); err != nil {
		log.Error("Protocol Object: failed to write archive", "id", id.Hex(), "err", err)
	}
}

// ReadArchivedProtocolObject retrieves an archived Protocol Object.
func ReadArchivedProtocolObject(db ethdb.KeyValueReader, id common.Hash) []byte {
	data, err := db.Get(protocolObjectKey(id))
	if err != nil {
		return nil
	}
	return data
}

// DeleteArchivedProtocolObject removes an archived Protocol Object.
func DeleteArchivedProtocolObject(db ethdb.KeyValueWriter, id common.Hash) {
	if err := db.Delete(protocolObjectKey(id)); err != nil {
		log.Error("Protocol Object: failed to delete archive", "id", id.Hex(), "err", err)
	}
}

// HasArchivedProtocolObject checks if an archived Protocol Object exists.
func HasArchivedProtocolObject(db ethdb.KeyValueReader, id common.Hash) bool {
	has, _ := db.Has(protocolObjectKey(id))
	return has
}