// Ethernova: Deferred Processing Phase (NIP-0004 Phase 2)
//
// This file implements the "Phase 0" prologue of block execution: before any
// transaction in block N+1 is applied, we drain the Pending Effects Queue
// that was populated during block N. Determinism is the absolute priority.
//
// Invariants (CONSENSUS-CRITICAL — any divergence here = BAD BLOCK):
//
//  1. This function runs on BOTH paths: validator (state_processor.Process)
//     and miner (worker.prepareWork / makeEnv). Any asymmetry between the
//     two paths is a consensus split. The SafeTuner precedent is documented
//     in state_processor.go — miners running a different hook set from
//     validators is the canonical way this chain has previously split.
//
//  2. Drain order == ascending sequence number. Sequence numbers are minted
//     monotonically by the 0x2A precompile's enqueue path, which runs inside
//     normal tx execution and is therefore already deterministic (tx order
//     is fixed by block body encoding). No Go map iteration is involved
//     anywhere in this path.
//
//  3. Drain window is [head, frontier), where frontier is pinned from
//     tail at the START of this function and held constant for the rest
//     of the drain. This prevents an effect enqueued DURING processing
//     (currently impossible because Phase 0 does not execute user code,
//     but kept as invariant for Phase 3+ when handlers may invoke
//     precompiles) from being processed in the same block.
//
//  4. Per-block drain cap = MaxDeferredProcessingPerBlock. If the queue
//     is larger than the cap, the drain stops and the remaining entries
//     wait for the next block. This bounds block validation wall time.
//
//  5. A handler that returns an error MUST NOT revert the whole block.
//     Per the NIP-0004 plan (§18 Failure Modes): "log error, skip entry,
//     jangan revert entire block." The entry is still cleared from the
//     queue (head advances) so the same entry cannot block the queue
//     forever — that would be a liveness bug worse than the failure.
//     The error is counted via a metric and logged but does NOT alter
//     state_root — both validator and miner agree on "cleared + counter++".
//
//  6. Empty queue == true no-op. The fast-path check reads head and tail
//     once and returns immediately if they are equal. This guarantees zero
//     regression on existing blocks where no contract has enqueued anything.

package core

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params/ethernova"
)

// errUnknownEffectType is returned by the drain dispatcher when it sees an
// effect tag it does not know. Enqueue-time validation should make this
// unreachable on canonical paths; reaching it means there is a client-level
// enum mismatch and the drain has elected to surface the error rather than
// silently skip the entry.
var errUnknownEffectType = errors.New("deferred: unknown effect type")

// DeferredProcessingResult captures what happened in one Phase 0 run. It is
// returned mostly for RPC/metrics; nothing in here feeds back into
// consensus beyond what has already been written to state by the drain.
type DeferredProcessingResult struct {
	BlockNumber      uint64
	Head             uint64 // head BEFORE drain
	Frontier         uint64 // tail snapshotted at start of drain
	NewHead          uint64 // head AFTER drain (== Head + Processed)
	Processed        uint64 // number of entries successfully handled
	FailedHandlers   uint64 // entries whose handler returned an error
	Skipped          uint64 // entries where marker was absent (already cleared)
	CapHit           bool   // true if drain stopped due to per-block cap
	NoOp             bool   // true if head == tail at entry (fast path)
}

// ProcessDeferredEffects runs the Phase 0 prologue for the given block
// header. It is gated by DeferredExecForkBlock — before the fork block the
// function is a pure no-op and MUST NOT touch state.
//
// The function is safe to call with nil header (it becomes a no-op), and is
// idempotent at the boundaries: calling it twice for the same block (which
// should never happen on the canonical paths) still produces the same final
// state because the second call sees head == tail and fast-paths.
func ProcessDeferredEffects(header *types.Header, statedb *state.StateDB) *DeferredProcessingResult {
	result := &DeferredProcessingResult{}
	if header == nil || statedb == nil {
		return result
	}
	blockNum := header.Number.Uint64()
	result.BlockNumber = blockNum

	// Fork gate. Before activation, this is a hard no-op — no state touches,
	// no account creation, nothing. Critical because we want empty-queue
	// regression to be literally zero-cost on pre-fork blocks.
	if blockNum < ethernova.DeferredExecForkBlock {
		result.NoOp = true
		return result
	}

	head := vm.DqGetHead(statedb)
	tail := vm.DqGetTail(statedb)
	result.Head = head
	result.Frontier = tail

	// Fast path: empty queue. No state writes. This matches the kriteria
	// selesai checklist item: "Jika queue kosong, Phase 0 = no-op
	// (no overhead pada existing blocks)."
	if head >= tail {
		result.NoOp = true
		return result
	}

	// Ensure the queue system account is materialised. This is a no-op on
	// the hot path after the first enqueue but guards the edge case where
	// an enqueue was reverted AFTER creating the account (in which case
	// Finalise would have deleted it). In practice this matters only for
	// the first-ever block with deferred effects.
	vm.DqEnsureExists(statedb)

	// Pin the frontier for this block's drain. If a handler in a future
	// phase invokes enqueueEffect (currently impossible — Phase 0 doesn't
	// run user code), the new entry lands beyond the frontier and is
	// deferred to the NEXT block. This is the single-write operation that
	// enforces "block N's queue is fully determined by end of block N-1".
	vm.DqSetFrontier(statedb, tail)

	drainCap := ethernova.MaxDeferredProcessingPerBlock
	drained := uint64(0)
	failed := uint64(0)
	skipped := uint64(0)

	for seq := head; seq < tail; seq++ {
		if drained >= drainCap {
			result.CapHit = true
			break
		}
		entry := vm.DqReadEntry(statedb, seq)
		if entry == nil {
			// Slot is already cleared (defensive — should not happen on
			// the canonical path). Count as skipped and continue; we
			// still advance head past it so the queue makes progress.
			skipped++
			drained++
			continue
		}

		// Dispatch to handler. Phase 2 ships with only the no-op/ping
		// handlers; Phase 4+ will bind real handlers here. A handler that
		// returns an error must NOT revert the block — we log, count,
		// clear, and move on.
		if err := dispatchDeferredEffect(entry, statedb, blockNum); err != nil {
			failed++
			log.Warn("deferred effect handler failed",
				"seq", entry.SeqNum,
				"type", types.DeferredEffectTypeName(entry.EffectType),
				"srcBlock", entry.SourceBlock,
				"err", err)
			// Intentional fallthrough: clear the entry even on handler
			// error. This is a design decision from the plan §18.
		}

		// Clear the entry's storage slots. This is a multi-slot write
		// (marker, len, chunk count, plus each payload chunk). After this
		// the seq slot is fully zero and GC-friendly for state expiry.
		vm.DqClearEntry(statedb, seq)
		drained++
	}

	// Advance head. Head moves by exactly `drained`, not by the distance
	// from head to the last seq — this is important if we hit the cap.
	newHead := head + drained
	vm.DqAdvanceHead(statedb, newHead)

	// Clear the frontier pin. Keeping a non-zero frontier outside of the
	// active drain would confuse RPC observers; the precompile's getStats
	// does not expose frontier directly but the invariant is
	// "frontier == 0 outside Phase 0".
	vm.DqClearFrontier(statedb)

	// Update lifetime metrics. NOTE: `failed` entries are counted in
	// `drained` AND in the totalProcessed bump — they exited the queue,
	// which is what "processed" means at queue level. This matters for
	// consensus: every node must agree on the final totalProcessed value.
	vm.DqIncrementTotalProcessed(statedb, drained)

	// Finalise so the drain's state writes land in the account before the
	// transaction loop starts. Without this, the first tx that happens to
	// read a queue slot could see a stale intermediate state. We pass
	// eip161d = true to match the per-tx finalise convention used by the
	// rest of state_processor.
	statedb.Finalise(true)

	result.NewHead = newHead
	result.Processed = drained - skipped - failed
	result.FailedHandlers = failed
	result.Skipped = skipped
	return result
}

// dispatchDeferredEffect routes a single entry to its handler. Phase 2 only
// implements the test-facing Noop and Ping handlers — all other types are
// accepted at enqueue time (for forward-compat with later phases) but
// treated as no-ops at drain time. This is safe because the real handlers
// (Mailbox, Session, AsyncCallback) are gated by their own fork blocks and
// will bind into this switch at the appropriate phase.
//
// Handlers MUST be deterministic: no wall clock, no random, no map
// iteration, no reliance on in-memory caches that may differ across nodes.
// All inputs come from `entry` (already in state) and `statedb` (the
// post-drain state). `blockNum` is the current block — the block doing the
// draining, which is `entry.SourceBlock + 1` on the normal path.
func dispatchDeferredEffect(entry *types.DeferredEffect, statedb *state.StateDB, blockNum uint64) error {
	switch entry.EffectType {
	case types.EffectTypeNoop:
		// True no-op. Used by harnesses to exercise queue ordering without
		// perturbing observable state.
		return nil
	case types.EffectTypePing:
		// Increments a deterministic counter slot on the queue system
		// account. Observable via RPC so tests can prove that a specific
		// entry's handler actually fired. Slot key is derived from
		// ("ping_counter", caller) so different callers see independent
		// counters — useful for multi-sender ordering tests.
		handlePing(entry, statedb)
		return nil
	case types.EffectTypeMailboxSend:
		// NIP-0004 Phase 4: Mailbox delivery. Pre-MailboxForkBlock the
		// handler is a no-op (matches Phase 2 behavior — entries enqueued
		// before the fork drain to nothing). Post-fork, the handler in
		// vm.HandleMailboxSendEffect performs the actual delivery into the
		// target mailbox's queue at MailboxOpsAddr (0xFF04).
		//
		// Per NIP-0004 §18, "expected" failures (mailbox missing, expired,
		// capacity raced) are silently dropped and return nil — the entry
		// is still cleared from the queue by the dispatcher caller, the
		// pending counter is released, and block validity is unaffected.
		// Only catastrophic failures (e.g. corrupt RLP — should be
		// unreachable) return an error, which the dispatcher logs and
		// counts as a failed handler without reverting the block.
		if blockNum < ethernova.MailboxForkBlock {
			return nil
		}
		return vm.HandleMailboxSendEffect(statedb, entry, blockNum)
	case types.EffectTypeAsyncCallback,
		types.EffectTypeSessionUpdate:
		// Reserved types — accepted at enqueue, no-op at drain until the
		// corresponding fork phase lands a real handler. Not an error.
		return nil
	default:
		// Unknown type. Should never happen because enqueueEffect
		// validates the tag — but we defend here to avoid silent drops
		// at drain time if the enum is ever extended inconsistently.
		return errUnknownEffectType
	}
}

// handlePing writes a deterministic counter into the queue system account,
// keyed by the enqueuing caller's address. The counter is readable from
// JS tests via eth_getStorageAt to prove that draining actually executed
// the handler. Storage at the queue address is otherwise used only for
// queue metadata; the ping counter lives in a distinct keyspace derived
// from the "ping_counter" prefix.
func handlePing(entry *types.DeferredEffect, statedb *state.StateDB) {
	key := pingCounterKey(entry.SourceCaller)
	cur := statedb.GetState(vm.DeferredQueueAddr, key)
	next := incHashUint256(cur)
	statedb.SetState(vm.DeferredQueueAddr, key, next)
}

// pingCounterKey computes the storage key for the per-caller ping counter.
// Keyspace is distinct from the queue's own metadata slots (those use
// prefixes like "q_head", "q_tail", etc.) so there is no aliasing.
func pingCounterKey(caller common.Address) common.Hash {
	return crypto.Keccak256Hash([]byte("ping_counter"), caller.Bytes())
}

// incHashUint256 treats the 32-byte hash as a big-endian uint256, adds 1,
// and returns the result as a hash. Overflow wraps to zero — in practice
// the per-caller ping counter will never get close to 2^256 so this is
// safe. Used by the Phase 2 ping handler; real Phase 4+ handlers will
// typically not need this helper.
func incHashUint256(h common.Hash) common.Hash {
	n := new(big.Int).SetBytes(h.Bytes())
	n.Add(n, big.NewInt(1))
	// Clamp to 256 bits. big.Int doesn't naturally wrap — we mask.
	mask := new(big.Int).Lsh(big.NewInt(1), 256)
	n.Mod(n, mask)
	return common.BigToHash(n)
}
