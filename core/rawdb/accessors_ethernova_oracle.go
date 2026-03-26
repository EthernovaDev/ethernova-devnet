// Ethernova: Native Oracle Storage (Phase 22)
// Persistent price feed storage in LevelDB.
//
// Schema:
//   "OP" + pairID(32) -> price(32 bytes)                     = Latest price
//   "OH" + pairID(32) + blockNumber(8) -> price(32 bytes)    = Price history

package rawdb

import (
	"encoding/binary"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
)

var (
	oraclePricePrefix   = []byte("OP")
	oracleHistoryPrefix = []byte("OH")
)

func oraclePriceKey(pairID common.Hash) []byte {
	return append(oraclePricePrefix, pairID.Bytes()...)
}

func oracleHistoryKey(pairID common.Hash, block uint64) []byte {
	key := make([]byte, 2+32+8)
	copy(key[:2], oracleHistoryPrefix)
	copy(key[2:34], pairID.Bytes())
	binary.BigEndian.PutUint64(key[34:], block)
	return key
}

// WriteOraclePrice stores the latest price for a pair.
func WriteOraclePrice(db ethdb.KeyValueWriter, pairID common.Hash, price *big.Int) {
	val := common.LeftPadBytes(price.Bytes(), 32)
	if err := db.Put(oraclePriceKey(pairID), val); err != nil {
		log.Crit("Failed to write oracle price", "err", err)
	}
}

// ReadOraclePrice returns the latest price for a pair.
func ReadOraclePrice(db ethdb.KeyValueReader, pairID common.Hash) *big.Int {
	data, err := db.Get(oraclePriceKey(pairID))
	if err != nil || len(data) == 0 {
		return new(big.Int)
	}
	return new(big.Int).SetBytes(data)
}

// WriteOraclePriceHistory stores a price at a specific block.
func WriteOraclePriceHistory(db ethdb.KeyValueWriter, pairID common.Hash, block uint64, price *big.Int) {
	val := common.LeftPadBytes(price.Bytes(), 32)
	if err := db.Put(oracleHistoryKey(pairID, block), val); err != nil {
		log.Crit("Failed to write oracle price history", "err", err)
	}
}

// ReadOraclePriceHistory returns the price at a specific block.
func ReadOraclePriceHistory(db ethdb.KeyValueReader, pairID common.Hash, block uint64) *big.Int {
	data, err := db.Get(oracleHistoryKey(pairID, block))
	if err != nil || len(data) == 0 {
		return new(big.Int)
	}
	return new(big.Int).SetBytes(data)
}
