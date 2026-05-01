// Ethernova: Phase 5 State Lifecycle Index extension (NIP-0004 §5).
//
// This file extends the Phase 15 state expiry schema with prefixes
// that support the 5-tier lifecycle (Active / Warm / Cold / Archived /
// Expired) for both Account state and Protocol Object state.
//
// Existing prefixes (defined in accessors_ethernova_expiry.go) — DO
// NOT redefine here, this file consumes them as a stable contract:
//
//   "X" + addr(20)        -> uint64 last_touched_block         (Phase 15)
//   "x" + blockN(8)       -> []addr touched at block N         (Phase 15)
//   "A" + addr(20)        -> RLP(SlimAccount) archived body    (Phase 15)
//
// New prefixes (Phase 5):
//
//   "T" + addr(20)        -> uint8 archive marker (1 = archived storage)
//   "C" + addr(20)        -> 32-byte cold storage commitment root
//                            (snapshot of obj.Root at archival time)
//   "W"                    -> 32-byte rolling Warm State Commitment Root
//                             (single-key, accumulator over warm
//                             demotions in the current epoch)
//   "L" + blockN(8)       -> uint64 lifecycle sweep cursor (next
//                             candidate block index to process; advances
//                             monotonically toward currentBlock-ColdTier)
//   "P" + objectId(32)    -> uint64 last_touched_block for a Protocol
//                             Object. Mirror of obj.LastTouchedBlock for
//                             nodes that prefer reading from the index
//                             instead of decoding the full RLP body.
//   "p" + blockN(8)       -> []objectId(32) sorted, touched at block N
//
// Determinism rules (apply to every accessor in this file):
//   1. All keys are constant-prefix + fixed-width big-endian binary.
//      No host-byte-order, no map iteration, no time.Now().
//   2. All list values are written sorted by raw byte order; the writer
//      is responsible for sorting before calling Write*.
//   3. All writes are crash-safe: callers either pass a *batch and
//      commit elsewhere, or use the helpers that wrap a single Write.
//
// These accessors are pure I/O — they perform no validation, no
// archival policy, no tier classification. The lifecycle engine in
// core/state/state_lifecycle.go owns the policy.

package rawdb

import (
	"bytes"
	"encoding/binary"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
)

// Phase 5 prefixes. Single-byte distinct-from-Phase-15 to keep the
// LevelDB keyspace flat and human-greppable in dump tools.
var (
	lifecycleArchiveMarkerPrefix = []byte("T")
	lifecycleColdRootPrefix      = []byte("C")
	lifecycleWarmRootKey         = []byte("W")
	lifecycleSweepCursorPrefix   = []byte("L")
	lifecycleObjectTouchedPrefix = []byte("P")
	lifecycleObjectBlockPrefix   = []byte("p")
)

// ArchiveMarker values.
const (
	// ArchiveMarkerNone means the address has no Phase 5 archive record.
	// This is the default state for every account; the prefix is only
	// written when the engine archives storage.
	ArchiveMarkerNone uint8 = 0x00
	// ArchiveMarkerArchived means the storage trie of this account has
	// been replaced by a cold commitment root recorded under the "C"
	// prefix. Witness-based restoration is required for any access that
	// would touch a non-zero slot.
	ArchiveMarkerArchived uint8 = 0x01
	// ArchiveMarkerExpired means the cold commitment has been dropped
	// per archive policy and the account is no longer recoverable. This
	// is reserved for Phase 5D and is never written by the current code.
	ArchiveMarkerExpired uint8 = 0x02
)

// --- key builders ---

func lifecycleArchiveMarkerKey(addr common.Address) []byte {
	return append(lifecycleArchiveMarkerPrefix, addr.Bytes()...)
}

func lifecycleColdRootKey(addr common.Address) []byte {
	return append(lifecycleColdRootPrefix, addr.Bytes()...)
}

func lifecycleSweepCursorKey(epoch uint64) []byte {
	key := make([]byte, len(lifecycleSweepCursorPrefix)+8)
	copy(key, lifecycleSweepCursorPrefix)
	binary.BigEndian.PutUint64(key[len(lifecycleSweepCursorPrefix):], epoch)
	return key
}

func lifecycleObjectTouchedKey(id common.Hash) []byte {
	return append(lifecycleObjectTouchedPrefix, id.Bytes()...)
}

func lifecycleObjectBlockKey(blockNumber uint64) []byte {
	key := make([]byte, len(lifecycleObjectBlockPrefix)+8)
	copy(key, lifecycleObjectBlockPrefix)
	binary.BigEndian.PutUint64(key[len(lifecycleObjectBlockPrefix):], blockNumber)
	return key
}

// --- archive marker ---

// WriteArchiveMarker stamps the address with a tier marker byte.
func WriteArchiveMarker(db ethdb.KeyValueWriter, addr common.Address, marker uint8) {
	if err := db.Put(lifecycleArchiveMarkerKey(addr), []byte{marker}); err != nil {
		log.Crit("Failed to write archive marker", "addr", addr, "err", err)
	}
}

// ReadArchiveMarker returns the marker byte for the address, or
// ArchiveMarkerNone if no record exists.
func ReadArchiveMarker(db ethdb.KeyValueReader, addr common.Address) uint8 {
	data, err := db.Get(lifecycleArchiveMarkerKey(addr))
	if err != nil || len(data) < 1 {
		return ArchiveMarkerNone
	}
	return data[0]
}

// DeleteArchiveMarker clears the marker (used after successful witness
// restoration).
func DeleteArchiveMarker(db ethdb.KeyValueWriter, addr common.Address) {
	if err := db.Delete(lifecycleArchiveMarkerKey(addr)); err != nil {
		log.Crit("Failed to delete archive marker", "addr", addr, "err", err)
	}
}

// --- cold storage commitment root ---

// WriteColdStorageRoot persists the storage trie root that was alive at
// the moment the account moved to the Archived tier. Witness verification
// against this root is the only path that can resurrect the account.
func WriteColdStorageRoot(db ethdb.KeyValueWriter, addr common.Address, root common.Hash) {
	if err := db.Put(lifecycleColdRootKey(addr), root.Bytes()); err != nil {
		log.Crit("Failed to write cold storage root", "addr", addr, "err", err)
	}
}

// ReadColdStorageRoot returns the persisted cold storage root, or the
// zero hash if none.
func ReadColdStorageRoot(db ethdb.KeyValueReader, addr common.Address) common.Hash {
	data, err := db.Get(lifecycleColdRootKey(addr))
	if err != nil || len(data) != common.HashLength {
		return common.Hash{}
	}
	return common.BytesToHash(data)
}

// DeleteColdStorageRoot removes the snapshot (used after successful
// restoration, or by Phase 5D expiry policy).
func DeleteColdStorageRoot(db ethdb.KeyValueWriter, addr common.Address) {
	if err := db.Delete(lifecycleColdRootKey(addr)); err != nil {
		log.Crit("Failed to delete cold storage root", "addr", addr, "err", err)
	}
}

// --- rolling Warm State Commitment Root ---

// WriteWarmStateRoot stores the global Warm State Commitment Root.
// This is a 32-byte rolling accumulator: every time a non-trivial Warm
// demotion happens, the engine recomputes
//
//	new_root = keccak256(old_root || addr || demotion_block_be8)
//
// and persists it here. Both miner and validator paths write the same
// hash chain, so the value remains identical across nodes.
func WriteWarmStateRoot(db ethdb.KeyValueWriter, root common.Hash) {
	if err := db.Put(lifecycleWarmRootKey, root.Bytes()); err != nil {
		log.Crit("Failed to write warm state root", "err", err)
	}
}

// ReadWarmStateRoot returns the current Warm State Commitment Root, or
// the zero hash if none has ever been written.
func ReadWarmStateRoot(db ethdb.KeyValueReader) common.Hash {
	data, err := db.Get(lifecycleWarmRootKey)
	if err != nil || len(data) != common.HashLength {
		return common.Hash{}
	}
	return common.BytesToHash(data)
}

// --- sweep cursor (per-epoch resumable progress) ---

// WriteLifecycleSweepCursor records the next-to-process block index
// inside a given lifecycle epoch. epoch is just an arbitrary identifier
// chosen by the caller (today: 0 = single global cursor).
func WriteLifecycleSweepCursor(db ethdb.KeyValueWriter, epoch, nextBlock uint64) {
	val := make([]byte, 8)
	binary.BigEndian.PutUint64(val, nextBlock)
	if err := db.Put(lifecycleSweepCursorKey(epoch), val); err != nil {
		log.Crit("Failed to write lifecycle sweep cursor", "err", err)
	}
}

// ReadLifecycleSweepCursor returns the next-to-process block index, or
// 0 if none.
func ReadLifecycleSweepCursor(db ethdb.KeyValueReader, epoch uint64) uint64 {
	data, err := db.Get(lifecycleSweepCursorKey(epoch))
	if err != nil || len(data) < 8 {
		return 0
	}
	return binary.BigEndian.Uint64(data)
}

// --- protocol object last-touched index (mirror of obj.LastTouchedBlock) ---

// WriteObjectLastTouched records the block at which a Protocol Object
// was last touched. The body field obj.LastTouchedBlock is canonical;
// this index exists only as a fast lookup for the lifecycle sweep.
func WriteObjectLastTouched(db ethdb.KeyValueWriter, id common.Hash, blockNumber uint64) {
	val := make([]byte, 8)
	binary.BigEndian.PutUint64(val, blockNumber)
	if err := db.Put(lifecycleObjectTouchedKey(id), val); err != nil {
		log.Crit("Failed to write object last touched", "id", id, "err", err)
	}
}

// ReadObjectLastTouched returns the block at which a Protocol Object
// was last touched, or 0 if untracked.
func ReadObjectLastTouched(db ethdb.KeyValueReader, id common.Hash) uint64 {
	data, err := db.Get(lifecycleObjectTouchedKey(id))
	if err != nil || len(data) < 8 {
		return 0
	}
	return binary.BigEndian.Uint64(data)
}

// --- per-block touched protocol objects (mirror of Phase 15 'x' prefix) ---

// WriteBlockTouchedObjects stores the sorted list of object IDs touched
// at the given block. The caller is responsible for sorting and
// deduplicating before calling.
func WriteBlockTouchedObjects(db ethdb.KeyValueWriter, blockNumber uint64, ids []common.Hash) {
	if len(ids) == 0 {
		return
	}
	// Validate sorted-and-unique to make consensus mistakes loud at write
	// time rather than silent at read time. The cost is one O(n) pass;
	// callers should already have sorted, so the check is a tripwire.
	for i := 1; i < len(ids); i++ {
		if bytes.Compare(ids[i-1].Bytes(), ids[i].Bytes()) >= 0 {
			// Recover by sorting; this should never happen if callers
			// follow the contract, but we'd rather quietly correct than
			// brick the chain on a write-time bug.
			sorted := make([]common.Hash, len(ids))
			copy(sorted, ids)
			sort.Slice(sorted, func(a, b int) bool {
				return bytes.Compare(sorted[a].Bytes(), sorted[b].Bytes()) < 0
			})
			ids = dedupSortedHashes(sorted)
			break
		}
	}
	data := make([]byte, 0, len(ids)*common.HashLength)
	for _, id := range ids {
		data = append(data, id.Bytes()...)
	}
	if err := db.Put(lifecycleObjectBlockKey(blockNumber), data); err != nil {
		log.Crit("Failed to write block touched objects", "block", blockNumber, "err", err)
	}
}

// ReadBlockTouchedObjects returns the list of object IDs touched at the
// given block. Output is in the same sorted order as written.
func ReadBlockTouchedObjects(db ethdb.KeyValueReader, blockNumber uint64) []common.Hash {
	data, err := db.Get(lifecycleObjectBlockKey(blockNumber))
	if err != nil || len(data) == 0 {
		return nil
	}
	count := len(data) / common.HashLength
	ids := make([]common.Hash, 0, count)
	for i := 0; i < count; i++ {
		var id common.Hash
		copy(id[:], data[i*common.HashLength:(i+1)*common.HashLength])
		ids = append(ids, id)
	}
	return ids
}

// dedupSortedHashes returns a copy of the input with consecutive
// duplicates removed. Input MUST be pre-sorted; this function does not
// re-sort. Used as a defensive helper inside the block-write path.
func dedupSortedHashes(in []common.Hash) []common.Hash {
	if len(in) == 0 {
		return in
	}
	out := make([]common.Hash, 0, len(in))
	out = append(out, in[0])
	for i := 1; i < len(in); i++ {
		if in[i] != in[i-1] {
			out = append(out, in[i])
		}
	}
	return out
}
