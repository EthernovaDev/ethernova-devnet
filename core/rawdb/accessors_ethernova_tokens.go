// Ethernova: Native Token Storage (Phase 20)
// Stores token balances and metadata in LevelDB outside the state trie.
//
// Schema:
//   "T" + tokenID(32) -> RLP(TokenMeta)                  = Token metadata
//   "B" + tokenID(32) + address(20) -> amount(32 bytes)   = Token balance
//   "C" + address(20) -> count(8 bytes)                   = Token creation count per address

package rawdb

import (
	"encoding/binary"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
)

var (
	tokenMetaPrefix    = []byte("T")
	tokenBalancePrefix = []byte("B")
	tokenCountPrefix   = []byte("C")
)

func tokenMetaKey(tokenID common.Hash) []byte {
	return append(tokenMetaPrefix, tokenID.Bytes()...)
}

func tokenBalanceKey(tokenID common.Hash, addr common.Address) []byte {
	key := make([]byte, 1+32+20)
	key[0] = tokenBalancePrefix[0]
	copy(key[1:33], tokenID.Bytes())
	copy(key[33:53], addr.Bytes())
	return key
}

func tokenCountKey(addr common.Address) []byte {
	return append(tokenCountPrefix, addr.Bytes()...)
}

// WriteTokenMeta stores token metadata.
func WriteTokenMeta(db ethdb.KeyValueWriter, tokenID common.Hash, data []byte) {
	if err := db.Put(tokenMetaKey(tokenID), data); err != nil {
		log.Crit("Failed to write token meta", "tokenID", tokenID, "err", err)
	}
}

// ReadTokenMeta returns token metadata.
func ReadTokenMeta(db ethdb.KeyValueReader, tokenID common.Hash) []byte {
	data, err := db.Get(tokenMetaKey(tokenID))
	if err != nil {
		return nil
	}
	return data
}

// WriteTokenBalance stores a token balance for an address.
func WriteTokenBalance(db ethdb.KeyValueWriter, tokenID common.Hash, addr common.Address, amount *big.Int) {
	val := common.LeftPadBytes(amount.Bytes(), 32)
	if err := db.Put(tokenBalanceKey(tokenID, addr), val); err != nil {
		log.Crit("Failed to write token balance", "err", err)
	}
}

// ReadTokenBalance returns a token balance for an address.
func ReadTokenBalance(db ethdb.KeyValueReader, tokenID common.Hash, addr common.Address) *big.Int {
	data, err := db.Get(tokenBalanceKey(tokenID, addr))
	if err != nil || len(data) == 0 {
		return new(big.Int)
	}
	return new(big.Int).SetBytes(data)
}

// WriteTokenCount stores the number of tokens created by an address.
func WriteTokenCount(db ethdb.KeyValueWriter, addr common.Address, count uint64) {
	val := make([]byte, 8)
	binary.BigEndian.PutUint64(val, count)
	if err := db.Put(tokenCountKey(addr), val); err != nil {
		log.Crit("Failed to write token count", "err", err)
	}
}

// ReadTokenCount returns the number of tokens created by an address.
func ReadTokenCount(db ethdb.KeyValueReader, addr common.Address) uint64 {
	data, err := db.Get(tokenCountKey(addr))
	if err != nil || len(data) < 8 {
		return 0
	}
	return binary.BigEndian.Uint64(data)
}
