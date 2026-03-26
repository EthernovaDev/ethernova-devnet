// Ethernova: Contract Upgrade Queue Storage (Phase 21)
// Persistent storage for pending contract upgrades with timelock.
//
// Schema:
//   "UQ" + contract(20) -> RLP(UpgradeRequest)   = Pending upgrade
//   "UH" + contract(20) + block(8) -> codeHash   = Upgrade history

package rawdb

import (
	"encoding/binary"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
)

var (
	upgradeQueuePrefix   = []byte("UQ")
	upgradeHistoryPrefix = []byte("UH")
)

func upgradeQueueKey(contract common.Address) []byte {
	return append(upgradeQueuePrefix, contract.Bytes()...)
}

func upgradeHistoryKey(contract common.Address, block uint64) []byte {
	key := make([]byte, 2+20+8)
	copy(key[:2], upgradeHistoryPrefix)
	copy(key[2:22], contract.Bytes())
	binary.BigEndian.PutUint64(key[22:], block)
	return key
}

// UpgradeData is serialized as: owner(20) + requestBlock(8) + activateBlock(8) + newCodeHash(32) + newCode(variable)
// Total fixed: 68 bytes + code

// WriteUpgradeRequest stores a pending upgrade.
func WriteUpgradeRequest(db ethdb.KeyValueWriter, contract common.Address, data []byte) {
	if err := db.Put(upgradeQueueKey(contract), data); err != nil {
		log.Crit("Failed to write upgrade request", "contract", contract, "err", err)
	}
}

// ReadUpgradeRequest returns a pending upgrade, or nil if none.
func ReadUpgradeRequest(db ethdb.KeyValueReader, contract common.Address) []byte {
	data, err := db.Get(upgradeQueueKey(contract))
	if err != nil {
		return nil
	}
	return data
}

// DeleteUpgradeRequest removes a pending upgrade (after execution or cancel).
func DeleteUpgradeRequest(db ethdb.KeyValueWriter, contract common.Address) {
	if err := db.Delete(upgradeQueueKey(contract)); err != nil {
		log.Crit("Failed to delete upgrade request", "contract", contract, "err", err)
	}
}

// WriteUpgradeHistory records that an upgrade happened at a specific block.
func WriteUpgradeHistory(db ethdb.KeyValueWriter, contract common.Address, block uint64, newCodeHash common.Hash) {
	if err := db.Put(upgradeHistoryKey(contract, block), newCodeHash.Bytes()); err != nil {
		log.Crit("Failed to write upgrade history", "err", err)
	}
}

// ReadUpgradeHistory returns the code hash from an upgrade at a specific block.
func ReadUpgradeHistory(db ethdb.KeyValueReader, contract common.Address, block uint64) common.Hash {
	data, err := db.Get(upgradeHistoryKey(contract, block))
	if err != nil || len(data) < 32 {
		return common.Hash{}
	}
	return common.BytesToHash(data)
}
