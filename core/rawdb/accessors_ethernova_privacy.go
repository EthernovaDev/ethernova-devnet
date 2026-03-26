// Ethernova: Shielded Pool Storage (Phase 24)
// Persistent storage for privacy commitments and nullifiers.
//
// Schema:
//   "SC" + commitment(32) -> 1 byte (exists)     = Active commitments
//   "SN" + nullifier(32)  -> 1 byte (exists)     = Spent nullifiers
//   "SP" -> amount(32 bytes)                      = Total shielded NOVA

package rawdb

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
)

var (
	shieldCommitmentPrefix = []byte("SC")
	shieldNullifierPrefix  = []byte("SN")
	shieldPoolTotalKey     = []byte("SP")
)

func commitmentKey(c common.Hash) []byte {
	return append(shieldCommitmentPrefix, c.Bytes()...)
}

func nullifierKey(n common.Hash) []byte {
	return append(shieldNullifierPrefix, n.Bytes()...)
}

// WriteCommitment marks a commitment as active.
func WriteCommitment(db ethdb.KeyValueWriter, commitment common.Hash) {
	if err := db.Put(commitmentKey(commitment), []byte{1}); err != nil {
		log.Crit("Failed to write commitment", "err", err)
	}
}

// HasCommitment returns true if the commitment exists.
func HasCommitment(db ethdb.KeyValueReader, commitment common.Hash) bool {
	data, err := db.Get(commitmentKey(commitment))
	return err == nil && len(data) > 0
}

// WriteNullifier marks a nullifier as spent (prevents double-spend).
func WriteNullifier(db ethdb.KeyValueWriter, nullifier common.Hash) {
	if err := db.Put(nullifierKey(nullifier), []byte{1}); err != nil {
		log.Crit("Failed to write nullifier", "err", err)
	}
}

// HasNullifier returns true if the nullifier has been spent.
func HasNullifier(db ethdb.KeyValueReader, nullifier common.Hash) bool {
	data, err := db.Get(nullifierKey(nullifier))
	return err == nil && len(data) > 0
}

// WriteShieldedTotal stores the total NOVA in the shielded pool.
func WriteShieldedTotal(db ethdb.KeyValueWriter, amount *big.Int) {
	val := common.LeftPadBytes(amount.Bytes(), 32)
	if err := db.Put(shieldPoolTotalKey, val); err != nil {
		log.Crit("Failed to write shielded total", "err", err)
	}
}

// ReadShieldedTotal returns the total NOVA in the shielded pool.
func ReadShieldedTotal(db ethdb.KeyValueReader) *big.Int {
	data, err := db.Get(shieldPoolTotalKey)
	if err != nil || len(data) == 0 {
		return new(big.Int)
	}
	return new(big.Int).SetBytes(data)
}
