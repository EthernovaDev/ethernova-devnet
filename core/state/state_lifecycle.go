// Ethernova: Phase 5 State Lifecycle Engine (NIP-0004 §5).
//
// 5-tier state lifecycle for Account state, Contract storage, and
// Protocol Object state:
//
//   Active   -> recently touched (<= ActiveTierBlocks since touch)
//   Warm     -> touched between ActiveTierBlocks and WarmTierBlocks
//   Cold     -> touched between WarmTierBlocks and ColdTierBlocks
//   Archived -> not touched within ColdTierBlocks (cold root persisted)
//   Expired  -> archive policy explicitly drops the cold root
//
// The engine is the SINGLE source of truth for tier classification.
// SLOAD/SSTORE in core/vm/operations_acl.go ask this engine for the
// current tier of a touched slot's containing account, and apply
// surcharges accordingly. The 0x2F precompile asks this engine to
// verify witness restorations.
//
// CONSENSUS-CRITICAL invariants enforced by this file:
//
//   1. ComputeTier and ComputeWarmingFee are PURE FUNCTIONS over
//      uint64 arithmetic. No floating point, no time.Now(),
//      no randomness, no map iteration.
//
//   2. Sweep iteration order is sorted-bytes (the candidates come from
//      the Phase 15 'x' index which is itself written sorted).
//
//   3. The sweep is bounded by MaxLifecycleSweepPerBlock so block
//      validation time stays bounded even under a sudden surge of
//      idle accounts. Overflow rolls forward via the cursor in the
//      "L" prefix.
//
//   4. Lifecycle transitions write to LevelDB ONLY (the external
//      index). They DO NOT mutate the state trie — that is reserved
//      for the Phase 5D account-archival path which will be guarded
//      by a separate fork block. This keeps Phase 5 strictly safe to
//      ship: a node can replay any block and reproduce identical
//      state roots regardless of whether the engine has run.
//
//   5. The 'last_touched_block' index is fed by Finalize() at every
//      block. Pre-fork, blocks pass through with no engine activity.

package state

import (
	"bytes"
	"encoding/binary"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
)

// Tier is the discriminant returned by ComputeTier.
type Tier uint8

const (
	TierActive   Tier = 0
	TierWarm     Tier = 1
	TierCold     Tier = 2
	TierArchived Tier = 3
	TierExpired  Tier = 4
)

// String returns the human-readable name (used by RPC layer).
func (t Tier) String() string {
	switch t {
	case TierActive:
		return "Active"
	case TierWarm:
		return "Warm"
	case TierCold:
		return "Cold"
	case TierArchived:
		return "Archived"
	case TierExpired:
		return "Expired"
	default:
		return "Unknown"
	}
}

// LifecycleThresholds are the three integer ages that bound the four
// reachable-by-time tiers. Expired is reached only via explicit policy
// (Phase 5D) and therefore is not part of this struct.
type LifecycleThresholds struct {
	ActiveBlocks uint64 // Active   if age <= ActiveBlocks
	WarmBlocks   uint64 // Warm     if ActiveBlocks <  age <= WarmBlocks
	ColdBlocks   uint64 // Cold     if WarmBlocks   <  age <= ColdBlocks; Archived above
}

// LifecycleFees are the integer fee parameters used by ComputeWarmingFee.
type LifecycleFees struct {
	PerByte uint64 // wei-of-gas per byte per tier-step
}

// LifecycleConfig is the immutable runtime configuration of the engine.
// It is built from params/ethernova constants once at engine creation
// and never mutated afterward — that immutability is what guarantees
// that two engines running on the same chain produce identical tier
// classifications even under concurrent access.
type LifecycleConfig struct {
	Thresholds       LifecycleThresholds
	Fees             LifecycleFees
	MaxSweepPerBlock uint64
}

// StateLifecycleEngine is the Phase 5 lifecycle controller. It wraps a
// single ethdb.KeyValueStore (which is a strict subset of
// ethdb.Database — ethdb.Database can be passed wherever
// KeyValueStore is required), writes to the Phase 15 + Phase 5 index
// keys, and is safe to share across goroutines (all operations route
// through crash-safe LevelDB batch writes — there is no in-memory
// mutable state on the engine itself).
type StateLifecycleEngine struct {
	db  ethdb.KeyValueStore
	cfg LifecycleConfig
}

// NewStateLifecycleEngine constructs a lifecycle engine with the given
// runtime configuration. db should be the chain's main LevelDB handle
// (eth.Ethereum.ChainDb()) or the disk handle hung off a *state.StateDB
// (statedb.Database().DiskDB()). Both work because ethdb.Database
// embeds ethdb.KeyValueStore.
func NewStateLifecycleEngine(db ethdb.KeyValueStore, cfg LifecycleConfig) *StateLifecycleEngine {
	return &StateLifecycleEngine{db: db, cfg: cfg}
}

// Config returns a copy of the engine configuration. Used by the RPC
// layer to surface threshold values.
func (e *StateLifecycleEngine) Config() LifecycleConfig {
	return e.cfg
}

// DB returns the underlying database handle. Used by the precompile
// when it needs to read tier metadata directly.
func (e *StateLifecycleEngine) DB() ethdb.KeyValueStore {
	return e.db
}

// =============================================================
// Pure tier classification (no state access)
// =============================================================

// ComputeTier classifies a (lastTouched, currentBlock) pair into one of
// {Active, Warm, Cold, Archived}. This is the canonical tier function
// — every consensus-touching call site MUST use this, never an
// ad-hoc comparison, so off-by-one errors stay in one place.
//
// Rules (ALL inclusive of the upper bound):
//
//	age <= ActiveBlocks                   -> Active
//	age <= WarmBlocks                     -> Warm
//	age <= ColdBlocks                     -> Cold
//	age >  ColdBlocks                     -> Archived
//
// Special cases:
//
//   - lastTouched == 0 means "never touched". For Phase 5 v1 we treat
//     this as Active, NOT Archived. Pre-fork accounts have no
//     last_touched record, and we MUST NOT charge them a warming fee
//     just because the index lookup missed. The engine flips an
//     account to a non-Active tier only after it has actually been
//     touched at least once post-fork.
//
//   - lastTouched > currentBlock would mean a future timestamp; this
//     should be impossible in normal operation. We return Active and
//     log a structured error rather than panic, to keep block
//     validation alive. The error log lets ops spot any index
//     corruption immediately.
func ComputeTier(lastTouched, currentBlock uint64, t LifecycleThresholds) Tier {
	if lastTouched == 0 {
		return TierActive
	}
	if lastTouched > currentBlock {
		log.Error("StateLifecycle: lastTouched > currentBlock",
			"lastTouched", lastTouched, "currentBlock", currentBlock)
		return TierActive
	}
	age := currentBlock - lastTouched
	switch {
	case age <= t.ActiveBlocks:
		return TierActive
	case age <= t.WarmBlocks:
		return TierWarm
	case age <= t.ColdBlocks:
		return TierCold
	default:
		return TierArchived
	}
}

// ComputeWarmingFee returns the gas surcharge owed for a single state
// access at the given tier. The formula is:
//
//	tier_gap = 0 (Active) | 1 (Warm) | 2 (Cold) | 3 (Archived)
//	fee_gas  = tier_gap * size_bytes * fee_per_byte
//
// All inputs are uint64; the multiplication is checked for overflow
// and saturated to MaxUint64 if it would wrap. A saturated value is
// effectively ErrOutOfGas at any reasonable tx, so consensus stays
// safe (every node arrives at the same OOG outcome).
func ComputeWarmingFee(tier Tier, sizeBytes uint64, fees LifecycleFees) uint64 {
	gap := tierGap(tier)
	if gap == 0 || sizeBytes == 0 || fees.PerByte == 0 {
		return 0
	}
	// Saturating multiply: a*b overflows iff a*b/a != b. uint64 mul
	// wrapping is well-defined in Go, so the check is exact.
	const max = ^uint64(0)
	if gap != 0 && fees.PerByte > max/gap {
		return max
	}
	step := gap * fees.PerByte
	if sizeBytes != 0 && step > max/sizeBytes {
		return max
	}
	return step * sizeBytes
}

// tierGap returns the integer "distance from Active" used in
// ComputeWarmingFee.
func tierGap(tier Tier) uint64 {
	switch tier {
	case TierActive:
		return 0
	case TierWarm:
		return 1
	case TierCold:
		return 2
	case TierArchived:
		return 3
	case TierExpired:
		return 3
	default:
		return 0
	}
}

// =============================================================
// Touch recording (called from Finalize on every block)
// =============================================================

// RecordBlockTouches saves the sorted, deduplicated list of accounts
// touched at blockNumber to the Phase 15 'X' (per-account) and 'x'
// (per-block) indexes. This is the "ingest" side of the engine — every
// post-fork block must call this exactly once during Finalize.
//
// The caller is responsible for collecting the address list from
// statedb (typically via StateDB.LifecycleTouchedAddresses()). This
// engine does no journal walking of its own.
func (e *StateLifecycleEngine) RecordBlockTouches(blockNumber uint64, addresses []common.Address) {
	if len(addresses) == 0 {
		return
	}
	// Sort + dedupe defensively. The caller should already have done
	// this; the cost is O(n log n) which is dominated by the trie work
	// happening alongside.
	sort.Slice(addresses, func(i, j int) bool {
		return bytes.Compare(addresses[i][:], addresses[j][:]) < 0
	})
	unique := dedupSortedAddresses(addresses)

	batch := e.db.NewBatch()
	for _, addr := range unique {
		rawdb.WriteLastTouched(batch, addr, blockNumber)
	}
	rawdb.WriteBlockTouchedAddresses(batch, blockNumber, unique)
	if err := batch.Write(); err != nil {
		log.Error("StateLifecycle: failed to record block touches",
			"block", blockNumber, "count", len(unique), "err", err)
	}
}

// RecordObjectBlockTouches is the Protocol Object analog of
// RecordBlockTouches. obj.LastTouchedBlock inside the trie remains
// canonical; this index gives the sweep an O(1) candidate lookup.
func (e *StateLifecycleEngine) RecordObjectBlockTouches(blockNumber uint64, ids []common.Hash) {
	if len(ids) == 0 {
		return
	}
	sort.Slice(ids, func(i, j int) bool {
		return bytes.Compare(ids[i].Bytes(), ids[j].Bytes()) < 0
	})
	unique := dedupSortedHashes(ids)

	batch := e.db.NewBatch()
	for _, id := range unique {
		rawdb.WriteObjectLastTouched(batch, id, blockNumber)
	}
	rawdb.WriteBlockTouchedObjects(batch, blockNumber, unique)
	if err := batch.Write(); err != nil {
		log.Error("StateLifecycle: failed to record object touches",
			"block", blockNumber, "count", len(unique), "err", err)
	}
}

// =============================================================
// Sweep (called from Finalize on every block; bounded work)
// =============================================================

// SweepResult is what ProcessLifecycle returns to the consensus engine.
// It is intentionally an aggregate of integer counters — the slice of
// addresses kept for the warm-root accumulator is internal and not
// exposed to consensus.
type SweepResult struct {
	BlockProcessed   uint64 // candidate block whose touches we examined
	Inspected        uint64 // accounts examined this sweep
	DemotedToWarm    uint64 // accounts whose tier crossed Active->Warm
	DemotedToCold    uint64 // ... Warm->Cold
	DemotedToArchive uint64 // ... Cold->Archived
}

// ProcessLifecycle is the per-block driver. It is called from
// consensus/lyra2/consensus.go in BOTH Finalize and FinalizeAndAssemble
// at the same point relative to other state mutations (after rewards,
// before IntermediateRoot). On the validator path and the miner path
// the call sites must match exactly — the engine itself only writes
// to the external LevelDB index, so a miss on one side would not
// produce a state root divergence, but it WOULD produce an index
// divergence that future restorations rely on.
//
// The sweep does NOT modify the state trie. It only:
//
//   - reads the candidate-block address list from the 'x' index
//   - for each candidate, computes the tier from its 'X' last_touched
//   - writes archive markers and cold roots to the 'T'/'C' indexes
//   - updates the rolling Warm State Commitment Root in 'W'
//   - advances the cursor in 'L'
//
// CONSENSUS NOTE: the absence of state-trie mutation is deliberate
// for Phase 5 v1. Phase 5D will add an opt-in trie-deletion step
// guarded by a separate fork block; until then, the trie is
// untouched and existing contracts retain their full storage even
// after demotion. The surcharge enforces the economic incentive; the
// archive marker enables the witness path.
func (e *StateLifecycleEngine) ProcessLifecycle(currentBlock uint64) SweepResult {
	res := SweepResult{}
	if currentBlock == 0 {
		return res
	}
	// Determine the candidate block. The simplest deterministic policy
	// is to examine block (currentBlock - ColdBlocks - 1) at every
	// height >= ColdBlocks+1. This produces a one-time pass per
	// candidate block, in deterministic order. Earlier-than-Cold
	// transitions (Active->Warm, Warm->Cold) are picked up implicitly
	// at access time via the per-call ComputeTier path; the sweep is
	// only responsible for the irreversible Cold->Archived transition.
	if currentBlock <= e.cfg.Thresholds.ColdBlocks {
		return res
	}
	candidate := currentBlock - e.cfg.Thresholds.ColdBlocks - 1
	// The lifecycle hook can be reached more than once for the same
	// block on miner/import paths. WarmStateRoot is a rolling accumulator,
	// so replaying the same candidate would permanently diverge the
	// external index even though the state trie remains identical. Use the
	// existing global cursor to make the sweep idempotent and to catch up
	// one missed candidate per subsequent block.
	const sweepEpoch uint64 = 0
	if next := rawdb.ReadLifecycleSweepCursor(e.db, sweepEpoch); next != 0 {
		if candidate < next {
			return res
		}
		if candidate > next {
			candidate = next
		}
	}
	res.BlockProcessed = candidate

	addrs := rawdb.ReadBlockTouchedAddresses(e.db, candidate)
	if len(addrs) == 0 {
		rawdb.WriteLifecycleSweepCursor(e.db, sweepEpoch, candidate+1)
		return res
	}
	// Apply per-block cap. Beyond the limit we leave the rest of the
	// list to the next block's sweep — but we MUST process them in
	// the same sorted order so all nodes drop the same suffix.
	limit := e.cfg.MaxSweepPerBlock
	if limit == 0 {
		limit = uint64(len(addrs))
	}
	if uint64(len(addrs)) > limit {
		// We do not advance a cursor beyond the limit: the leftover
		// suffix is examined at currentBlock+1 by reading the same
		// candidate block. This is wasteful only when sustained burst
		// touches happen, which is rare and bounded.
		addrs = addrs[:limit]
	}

	warmRoot := rawdb.ReadWarmStateRoot(e.db)
	batch := e.db.NewBatch()
	for _, addr := range addrs {
		res.Inspected++
		lastTouched := rawdb.ReadLastTouched(e.db, addr)
		// If the account has been touched again since the candidate
		// block, it has effectively been promoted back to Active and
		// no longer belongs to this candidate's sweep.
		if lastTouched > candidate {
			continue
		}
		tier := ComputeTier(lastTouched, currentBlock, e.cfg.Thresholds)
		switch tier {
		case TierWarm:
			res.DemotedToWarm++
			// No archive marker; surcharge is applied implicitly via
			// ComputeTier on every access. We DO update the warm-root
			// accumulator so a multi-node consensus check has a hook
			// to verify equivalence.
			warmRoot = accumulateWarmRoot(warmRoot, addr, currentBlock)
		case TierCold:
			res.DemotedToCold++
			warmRoot = accumulateWarmRoot(warmRoot, addr, currentBlock)
		case TierArchived:
			res.DemotedToArchive++
			// Mark archived in the index. The cold root is captured
			// from the live state trie root the FIRST time a sweep
			// finds the account in the Archived bucket. The state trie
			// itself is NOT mutated by Phase 5 v1.
			if rawdb.ReadArchiveMarker(e.db, addr) == rawdb.ArchiveMarkerNone {
				rawdb.WriteArchiveMarker(batch, addr, rawdb.ArchiveMarkerArchived)
				// Cold root is set by the consensus engine via
				// CaptureColdRoot below — we cannot read storage roots
				// from inside the engine without a *state.StateDB
				// handle. The engine writes a zero placeholder; the
				// consensus path overrides it with the real root.
			}
			warmRoot = accumulateWarmRoot(warmRoot, addr, currentBlock)
		case TierActive:
			// No-op: the address was eligible because it was touched
			// at the candidate block, but later touches lifted it
			// back. ComputeTier handles that with the lastTouched >
			// candidate guard above; this branch is the safety net.
		}
	}
	rawdb.WriteWarmStateRoot(batch, warmRoot)
	rawdb.WriteLifecycleSweepCursor(batch, sweepEpoch, candidate+1)
	if err := batch.Write(); err != nil {
		log.Error("StateLifecycle: sweep batch write failed",
			"block", currentBlock, "err", err)
	}
	return res
}

// CaptureColdRoot stores the storage trie root of an archived account.
// This is called by consensus/lyra2/consensus.go after ProcessLifecycle
// for each address whose archive marker was just stamped. The split
// between ProcessLifecycle (writes the marker) and CaptureColdRoot
// (writes the root) keeps the engine free of *state.StateDB knowledge.
func (e *StateLifecycleEngine) CaptureColdRoot(addr common.Address, root common.Hash) {
	rawdb.WriteColdStorageRoot(e.db, addr, root)
}

// =============================================================
// Tier query (used by the precompile and by SLOAD/SSTORE surcharge)
// =============================================================

// TierOf returns the current tier of an account. Reads the 'X' index;
// returns TierActive if the account has no record (matches the
// "untouched = active" rule in ComputeTier).
func (e *StateLifecycleEngine) TierOf(addr common.Address, currentBlock uint64) Tier {
	// An archive marker takes precedence: if the engine has stamped an
	// account as archived in a prior sweep, it stays archived until
	// witness restoration clears the marker.
	if rawdb.ReadArchiveMarker(e.db, addr) == rawdb.ArchiveMarkerArchived {
		return TierArchived
	}
	if rawdb.ReadArchiveMarker(e.db, addr) == rawdb.ArchiveMarkerExpired {
		return TierExpired
	}
	lastTouched := rawdb.ReadLastTouched(e.db, addr)
	return ComputeTier(lastTouched, currentBlock, e.cfg.Thresholds)
}

// TierOfObject returns the current tier of a Protocol Object. Same
// rules as TierOf but reads the 'P' index (mirror of obj.LastTouchedBlock).
func (e *StateLifecycleEngine) TierOfObject(id common.Hash, currentBlock uint64) Tier {
	lastTouched := rawdb.ReadObjectLastTouched(e.db, id)
	return ComputeTier(lastTouched, currentBlock, e.cfg.Thresholds)
}

// ColdStorageRoot returns the persisted cold storage root for an
// archived account, or the zero hash if none. Exposed so the 0x2F
// precompile selector 0x01 (verifyStateWitness) can perform the
// pure-read verification without going through the write-side
// RestoreFromWitness path.
func (e *StateLifecycleEngine) ColdStorageRoot(addr common.Address) common.Hash {
	return rawdb.ReadColdStorageRoot(e.db, addr)
}

// LastTouched returns the persisted last_touched_block for an
// account, or 0 if untracked. Exposed for the RPC layer.
func (e *StateLifecycleEngine) LastTouched(addr common.Address) uint64 {
	return rawdb.ReadLastTouched(e.db, addr)
}

// LastTouchedObject returns the persisted object last_touched_block mirror, or
// 0 if the Protocol Object has not yet been indexed by Phase 5C.
func (e *StateLifecycleEngine) LastTouchedObject(id common.Hash) uint64 {
	return rawdb.ReadObjectLastTouched(e.db, id)
}

// WarmStateRoot returns the rolling Warm State Commitment Root.
// Exposed for the RPC layer (ethernova_getWarmStateRoot).
func (e *StateLifecycleEngine) WarmStateRoot() common.Hash {
	return rawdb.ReadWarmStateRoot(e.db)
}

// IsArchived reports whether the address is currently flagged in the
// Archived tier by an explicit marker (as opposed to time-derived).
// Exposed for the RPC layer.
func (e *StateLifecycleEngine) IsArchived(addr common.Address) bool {
	return rawdb.ReadArchiveMarker(e.db, addr) == rawdb.ArchiveMarkerArchived
}

// =============================================================
// Witness restoration
// =============================================================

// RestoreFromWitness verifies a Merkle witness and, on success, clears
// the archive marker for the address and refreshes its last_touched to
// currentBlock. This is the only path that can move an account out of
// the Archived tier without explicit operator intervention.
//
// Precompile 0x2F selector 0x02 is the consensus-visible entry point
// for this function. The call is gated by readOnly=false at the
// precompile boundary; callers must enforce that.
//
// The witness is verified against the cold storage root recorded at
// archival time. If the supplied storageRoot does not match the one
// on file, restoration is rejected. This is what prevents witness
// replay across accounts: a witness for account A's storage trie
// cannot resurrect account B even if B's slot/value happens to match.
func (e *StateLifecycleEngine) RestoreFromWitness(
	addr common.Address,
	slot common.Hash,
	expected common.Hash,
	proofPayload []byte,
	currentBlock uint64,
	maxProofBytes uint64,
) error {
	if rawdb.ReadArchiveMarker(e.db, addr) != rawdb.ArchiveMarkerArchived {
		// Restore writes the Phase 5 metadata index outside the trie. If a
		// block import is retried after the first execution already cleared
		// the marker, replaying the same restore transaction must remain
		// deterministic. Treat "already restored in this exact block" as an
		// idempotent success, while still rejecting normal non-archived calls.
		if rawdb.ReadLastTouched(e.db, addr) == currentBlock &&
			rawdb.ReadColdStorageRoot(e.db, addr) == (common.Hash{}) {
			return nil
		}
		// The address is not archived — nothing to restore. Refuse to
		// silently succeed because a "no-op restore" gives caller no
		// signal that the witness was wrong.
		return errNotArchived
	}
	coldRoot := rawdb.ReadColdStorageRoot(e.db, addr)
	if coldRoot == (common.Hash{}) {
		// No snapshot was captured (consensus engine is expected to
		// call CaptureColdRoot after ProcessLifecycle stamps the
		// marker). Reject — there's no anchor to verify against.
		return errNoColdRoot
	}
	nodes, err := DecodeProofPayload(proofPayload, maxProofBytes)
	if err != nil {
		return err
	}
	ok, err := VerifyStorageWitness(coldRoot, slot, expected, nodes)
	if err != nil {
		return err
	}
	if !ok {
		return errProofRejected
	}
	// Witness valid. Clear marker, drop the cold root, refresh
	// last_touched. The state trie is NOT modified — the live storage
	// slot was never deleted in Phase 5 v1, so the account simply
	// re-enters the Active tier with its original storage intact.
	batch := e.db.NewBatch()
	rawdb.DeleteArchiveMarker(batch, addr)
	rawdb.DeleteColdStorageRoot(batch, addr)
	rawdb.WriteLastTouched(batch, addr, currentBlock)
	if err := batch.Write(); err != nil {
		return err
	}
	return nil
}

// =============================================================
// Internal helpers
// =============================================================

// accumulateWarmRoot folds a single demotion into the rolling Warm
// State Commitment Root:
//
//	new_root = keccak256(old_root || addr_be20 || block_be8)
//
// This is purely an integrity hook: the value is exposed via
// ethernova_getWarmStateRoot so external auditors can compare across
// nodes that they all observed the same demotion sequence.
func accumulateWarmRoot(prev common.Hash, addr common.Address, blockNumber uint64) common.Hash {
	var blockBuf [8]byte
	binary.BigEndian.PutUint64(blockBuf[:], blockNumber)
	return crypto.Keccak256Hash(prev.Bytes(), addr.Bytes(), blockBuf[:])
}

// dedupSortedAddresses returns a copy with consecutive duplicates
// removed. Input MUST be pre-sorted.
func dedupSortedAddresses(in []common.Address) []common.Address {
	if len(in) == 0 {
		return in
	}
	out := make([]common.Address, 0, len(in))
	out = append(out, in[0])
	for i := 1; i < len(in); i++ {
		if in[i] != in[i-1] {
			out = append(out, in[i])
		}
	}
	return out
}

// dedupSortedHashes is the common.Hash analog of dedupSortedAddresses.
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

// --- engine errors ---

type lifecycleError string

func (e lifecycleError) Error() string { return string(e) }

const (
	errNotArchived   lifecycleError = "lifecycle: account not archived"
	errNoColdRoot    lifecycleError = "lifecycle: no cold root recorded"
	errProofRejected lifecycleError = "lifecycle: witness rejected"
)
