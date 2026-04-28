// Ethernova: Mailbox Primitive — Shared State Helpers (NIP-0004 Phase 4)
//
// This file owns the storage layout, queue mutation, and owner-index
// machinery for Mailbox Protocol Objects. The two Phase 4 precompiles
// (0x2C novaMailboxManager, 0x35 novaMailboxOps) and the deferred-effect
// dispatcher (core/ethernova_deferred_processing.go) all read and write
// through the helpers exposed here.
//
// The Mailbox PROTOCOL OBJECT BODY (id, owner, type_tag, state_data with
// MailboxConfig RLP, expiry_block, last_touched_block, rent_balance) lives
// in the Phase 1 Protocol Object Registry at 0xFF01. This file does NOT
// duplicate that body — it only owns:
//
//   1. The actual MESSAGE QUEUE — head/tail counters and chunked message
//      slots — keyed per mailbox at MailboxOpsAddr (0xFF04).
//
//   2. An IN-FLIGHT counter that tracks messages enqueued via sendMessage
//      but not yet delivered by the deferred processing phase. Together
//      with the live count this enforces capacity_limit at SEND time so
//      we never have to revert at delivery time (delivery never reverts;
//      see NIP-0004 §18).
//
//   3. A per-owner mailbox index — a separate index from the Phase 1
//      generic owner index — so type-filtered queries (getMailboxByOwner)
//      can run without scanning all Protocol Object types.
//
// Storage layout (all at MailboxOpsAddr = 0xFF04):
//
//   keccak256("mb_head", id)                  -> uint64 next message index to recv
//   keccak256("mb_tail", id)                  -> uint64 next message index to write
//   keccak256("mb_count", id)                 -> uint64 (tail - head, materialized)
//   keccak256("mb_pending", id)               -> uint64 in-flight (enqueued, not yet delivered)
//   keccak256("mb_msg_marker", id, idx)       -> 0x01 marker
//   keccak256("mb_msg_dlen",   id, idx)       -> uint64 RLP byte length
//   keccak256("mb_msg_chunks", id, idx)       -> uint64 chunk count
//   keccak256("mb_msg_chunk",  id, idx, c)    -> 32-byte chunk
//
//   keccak256("mb_owner_count",   owner)      -> uint64 live mailbox count
//   keccak256("mb_owner_slots",   owner)      -> uint64 monotonic high water mark
//   keccak256("mb_owner_idx",     owner, slot)-> mailbox_id (zero = tombstoned)
//   keccak256("mb_owner_slot_of", id)         -> uint64 reverse lookup
//
// Determinism invariants (CONSENSUS-CRITICAL):
//
//   - Queue ordering is by message index (uint64), never by Go map iteration.
//   - Indices are big-endian fixed-width 8-byte encoded so the storage keys
//     are bit-identical across Linux (CGO=1) and Windows (CGO=0) builds.
//   - Tail and head are monotonic counters; tail never decreases, head only
//     advances (recv) or is reset to 0 along with tail on destroy.
//   - Capacity check = (count + pending) < CapacityLimit. Both counters are
//     in state, no in-memory caches.
//   - All write paths are gated on readOnly=false at the precompile boundary
//     (EIP-214 protection); helpers in this file are pure state mutators.

package vm

import (
	"encoding/binary"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

// MailboxOpsAddr is the system address holding mailbox queue state.
// Distinct from 0xFF01 (Protocol Object Registry), 0xFF02 (Deferred Queue),
// and 0xFF03 (Content Registry). Each subsystem owns its own reserved
// account so a corruption in one cannot cross-contaminate another.
var MailboxOpsAddr = common.HexToAddress("0x000000000000000000000000000000000000FF04")

// Hard limits enforced by the manager precompile. These are conservative
// upper bounds that bound per-tx and per-block work; they are part of
// consensus and changing them requires a fork.
const (
	// MailboxAbsoluteCapacity is the hard upper bound on capacity_limit
	// the user can request. Sized so a fully-populated mailbox is still
	// bounded in disk footprint (~10k * ~120 bytes per message = ~1.2 MB).
	MailboxAbsoluteCapacity uint64 = 10000

	// MailboxMaxACLEntries caps how many addresses the user can place on
	// the ACL list. Sized so the RLP-encoded MailboxConfig stays small
	// (~64 * 20 = 1280 bytes plus framing).
	MailboxMaxACLEntries uint64 = 64

	// MailboxMaxRetentionBlocks caps the TTL field. Reserved for a future
	// retention enforcer; kept bounded now so configure cannot stash an
	// unreasonable value that we have to decode forever.
	MailboxMaxRetentionBlocks uint64 = 10_000_000

	// MailboxMaxRecvPerCall is the hard upper bound on the per-call recv
	// loop. recvMessage as currently shipped only dequeues ONE message per
	// call; this constant is reserved for a future batched-recv selector.
	MailboxMaxRecvPerCall uint64 = 1
)

// --- Storage key builders (deterministic, fixed-width) -------------------

func mbU64(idx uint64) []byte {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], idx)
	return b[:]
}

func mbKeyHead(id common.Hash) common.Hash {
	return crypto.Keccak256Hash([]byte("mb_head"), id.Bytes())
}
func mbKeyTail(id common.Hash) common.Hash {
	return crypto.Keccak256Hash([]byte("mb_tail"), id.Bytes())
}
func mbKeyCount(id common.Hash) common.Hash {
	return crypto.Keccak256Hash([]byte("mb_count"), id.Bytes())
}
func mbKeyPending(id common.Hash) common.Hash {
	return crypto.Keccak256Hash([]byte("mb_pending"), id.Bytes())
}
func mbKeyMsgMarker(id common.Hash, idx uint64) common.Hash {
	return crypto.Keccak256Hash([]byte("mb_msg_marker"), id.Bytes(), mbU64(idx))
}
func mbKeyMsgDataLen(id common.Hash, idx uint64) common.Hash {
	return crypto.Keccak256Hash([]byte("mb_msg_dlen"), id.Bytes(), mbU64(idx))
}
func mbKeyMsgChunkCount(id common.Hash, idx uint64) common.Hash {
	return crypto.Keccak256Hash([]byte("mb_msg_chunks"), id.Bytes(), mbU64(idx))
}
func mbKeyMsgChunk(id common.Hash, idx, chunkIdx uint64) common.Hash {
	return crypto.Keccak256Hash([]byte("mb_msg_chunk"), id.Bytes(), mbU64(idx), mbU64(chunkIdx))
}
func mbKeyOwnerCount(owner common.Address) common.Hash {
	return crypto.Keccak256Hash([]byte("mb_owner_count"), owner.Bytes())
}
func mbKeyOwnerSlots(owner common.Address) common.Hash {
	return crypto.Keccak256Hash([]byte("mb_owner_slots"), owner.Bytes())
}
func mbKeyOwnerIndex(owner common.Address, slot uint64) common.Hash {
	return crypto.Keccak256Hash([]byte("mb_owner_idx"), owner.Bytes(), mbU64(slot))
}
func mbKeyOwnerSlotOf(id common.Hash) common.Hash {
	return crypto.Keccak256Hash([]byte("mb_owner_slot_of"), id.Bytes())
}

// --- Low-level state helpers (mirror the 0x29 / 0x2A / 0x2B pattern) -----

func mbReadUint64(sdb StateDB, key common.Hash) uint64 {
	val := sdb.GetState(MailboxOpsAddr, key)
	return new(big.Int).SetBytes(val.Bytes()).Uint64()
}

func mbWriteUint64(sdb StateDB, key common.Hash, v uint64) {
	sdb.SetState(MailboxOpsAddr, key, common.BigToHash(new(big.Int).SetUint64(v)))
}

// mbEnsureExists makes sure MailboxOpsAddr (0xFF04) is non-empty per
// EIP-161. Same rationale as poEnsureRegistryExists / dqEnsureExists /
// crEnsureRegistryExists: a storage-only account would be wiped by
// state.Finalise(true) unless we mark it non-empty via nonce=1. Without
// this, the FIRST createMailbox or sendMessage in a chain's lifetime
// would silently lose all its writes.
func mbEnsureExists(sdb StateDB) {
	if !sdb.Exist(MailboxOpsAddr) {
		sdb.CreateAccount(MailboxOpsAddr)
	}
	if sdb.GetNonce(MailboxOpsAddr) == 0 {
		sdb.SetNonce(MailboxOpsAddr, 1)
	}
}

// mbWriteMessage RLP-encodes the message and stripes it across chunked
// storage at (id, idx). Caller is responsible for advancing tail and
// updating count. Per-message slots are NEVER overwritten at the same
// (id, idx) on the canonical path because tail is monotonic — recvd
// messages are tombstoned, not reused.
func mbWriteMessage(sdb StateDB, id common.Hash, idx uint64, data []byte) {
	dataLen := uint64(len(data))
	chunks := (dataLen + 31) / 32
	sdb.SetState(MailboxOpsAddr, mbKeyMsgMarker(id, idx),
		common.BytesToHash([]byte{0x01}))
	mbWriteUint64(sdb, mbKeyMsgDataLen(id, idx), dataLen)
	mbWriteUint64(sdb, mbKeyMsgChunkCount(id, idx), chunks)
	for i := uint64(0); i < chunks; i++ {
		start := i * 32
		end := start + 32
		if end > dataLen {
			end = dataLen
		}
		var chunk [32]byte
		copy(chunk[:], data[start:end])
		sdb.SetState(MailboxOpsAddr, mbKeyMsgChunk(id, idx, i),
			common.BytesToHash(chunk[:]))
	}
}

// mbReadMessage decodes the message at (id, idx). Returns nil if the marker
// is absent (already recvd, never written, or destroyed mailbox).
func mbReadMessage(sdb StateDB, id common.Hash, idx uint64) *types.MailboxMessage {
	marker := sdb.GetState(MailboxOpsAddr, mbKeyMsgMarker(id, idx))
	if marker == (common.Hash{}) {
		return nil
	}
	dataLen := mbReadUint64(sdb, mbKeyMsgDataLen(id, idx))
	if dataLen == 0 {
		return nil
	}
	chunks := mbReadUint64(sdb, mbKeyMsgChunkCount(id, idx))
	data := make([]byte, 0, dataLen)
	for i := uint64(0); i < chunks; i++ {
		chunk := sdb.GetState(MailboxOpsAddr, mbKeyMsgChunk(id, idx, i))
		remaining := dataLen - uint64(len(data))
		if remaining >= 32 {
			data = append(data, chunk[:]...)
		} else {
			data = append(data, chunk[:remaining]...)
		}
	}
	msg, err := types.DecodeMailboxMessage(data)
	if err != nil {
		return nil
	}
	return msg
}

// mbClearMessage tombstones a message slot. Called by recvMessage after
// the message has been returned to the caller. Zeroes marker + len +
// chunk_count AND every chunk so state expiry can collect the slots.
func mbClearMessage(sdb StateDB, id common.Hash, idx uint64) {
	chunks := mbReadUint64(sdb, mbKeyMsgChunkCount(id, idx))
	for i := uint64(0); i < chunks; i++ {
		sdb.SetState(MailboxOpsAddr, mbKeyMsgChunk(id, idx, i), common.Hash{})
	}
	sdb.SetState(MailboxOpsAddr, mbKeyMsgMarker(id, idx), common.Hash{})
	mbWriteUint64(sdb, mbKeyMsgDataLen(id, idx), 0)
	mbWriteUint64(sdb, mbKeyMsgChunkCount(id, idx), 0)
}

// --- Owner-index helpers -------------------------------------------------

// mbOwnerIndexAdd registers a new mailbox in the per-owner mailbox index.
// The slot allocation mirrors the Phase 1 / Phase 3 patterns: a monotonic
// high-water mark (mb_owner_slots) that never decrements, plus a live
// count (mb_owner_count). Tombstoned slots are zeroed but not reclaimed.
func mbOwnerIndexAdd(sdb StateDB, owner common.Address, id common.Hash) {
	slot := mbReadUint64(sdb, mbKeyOwnerSlots(owner))
	sdb.SetState(MailboxOpsAddr, mbKeyOwnerIndex(owner, slot), id)
	mbWriteUint64(sdb, mbKeyOwnerSlotOf(id), slot)
	mbWriteUint64(sdb, mbKeyOwnerSlots(owner), slot+1)
	cnt := mbReadUint64(sdb, mbKeyOwnerCount(owner))
	mbWriteUint64(sdb, mbKeyOwnerCount(owner), cnt+1)
}

// mbOwnerIndexRemove tombstones the slot for a destroyed mailbox using the
// reverse lookup. O(1), no scan.
func mbOwnerIndexRemove(sdb StateDB, owner common.Address, id common.Hash) {
	slot := mbReadUint64(sdb, mbKeyOwnerSlotOf(id))
	sdb.SetState(MailboxOpsAddr, mbKeyOwnerIndex(owner, slot), common.Hash{})
	mbWriteUint64(sdb, mbKeyOwnerSlotOf(id), 0)
	cnt := mbReadUint64(sdb, mbKeyOwnerCount(owner))
	if cnt > 0 {
		mbWriteUint64(sdb, mbKeyOwnerCount(owner), cnt-1)
	}
}

// MbListByOwner returns mailbox IDs for a given owner, paginated by
// (offset, limit) over LIVE (non-tombstoned) entries only. Used by RPC.
func MbListByOwner(sdb StateDB, owner common.Address, offset, limit uint64) []common.Hash {
	if limit == 0 {
		return nil
	}
	if limit > 100 {
		limit = 100
	}
	slots := mbReadUint64(sdb, mbKeyOwnerSlots(owner))
	if slots == 0 {
		return nil
	}
	var ids []common.Hash
	skipped := uint64(0)
	for slot := uint64(0); slot < slots; slot++ {
		val := sdb.GetState(MailboxOpsAddr, mbKeyOwnerIndex(owner, slot))
		if val == (common.Hash{}) {
			continue
		}
		if skipped < offset {
			skipped++
			continue
		}
		ids = append(ids, val)
		if uint64(len(ids)) >= limit {
			break
		}
	}
	return ids
}

// MbGetOwnerCount returns the live mailbox count for an owner.
func MbGetOwnerCount(sdb StateDB, owner common.Address) uint64 {
	return mbReadUint64(sdb, mbKeyOwnerCount(owner))
}

// --- Read-side accessors exposed for the RPC layer -----------------------

// MbGetCount returns the number of messages currently sitting in the queue
// (delivered and not yet recvd). Does NOT include in-flight (pending) ones.
func MbGetCount(sdb StateDB, id common.Hash) uint64 {
	return mbReadUint64(sdb, mbKeyCount(id))
}

// MbGetPending returns the number of messages in the deferred queue
// targeting this mailbox that have not yet been delivered. RPC exposes this
// for observability; on-chain capacity check uses count + pending.
func MbGetPending(sdb StateDB, id common.Hash) uint64 {
	return mbReadUint64(sdb, mbKeyPending(id))
}

// MbGetHead returns the next index to recv. Together with tail (= head +
// count) this lets callers iterate the queue without depending on internal
// storage shapes.
func MbGetHead(sdb StateDB, id common.Hash) uint64 {
	return mbReadUint64(sdb, mbKeyHead(id))
}

// MbGetTail returns the next index a delivery would write into.
func MbGetTail(sdb StateDB, id common.Hash) uint64 {
	return mbReadUint64(sdb, mbKeyTail(id))
}

// MbGetMessageAt returns the message at a specific index without dequeuing.
// Returns nil if the slot is empty (recvd or destroyed).
func MbGetMessageAt(sdb StateDB, id common.Hash, idx uint64) *types.MailboxMessage {
	return mbReadMessage(sdb, id, idx)
}

// MbGetMessages returns up to `limit` messages starting at head + offset.
// Used by RPC's ethernova_getMessages.
func MbGetMessages(sdb StateDB, id common.Hash, offset, limit uint64) []*types.MailboxMessage {
	if limit == 0 {
		return nil
	}
	if limit > 256 {
		limit = 256
	}
	head := mbReadUint64(sdb, mbKeyHead(id))
	tail := mbReadUint64(sdb, mbKeyTail(id))
	start := head + offset
	out := make([]*types.MailboxMessage, 0, limit)
	for idx := start; idx < tail && uint64(len(out)) < limit; idx++ {
		m := mbReadMessage(sdb, id, idx)
		if m != nil {
			out = append(out, m)
		}
	}
	return out
}

// MbGetMailbox is a thin convenience wrapper that returns the Phase 1
// ProtocolObject body filtered by type_tag = ProtoTypeMailbox. Returns nil
// if the ID does not exist or is the wrong type.
func MbGetMailbox(sdb StateDB, id common.Hash) *types.ProtocolObject {
	obj := PoGetObject(sdb, id)
	if obj == nil || obj.TypeTag != types.ProtoTypeMailbox {
		return nil
	}
	return obj
}

// --- Internal write helpers used by the precompiles + deferred handler ---

// mbResetCounters wipes head, tail, count, pending for a destroyed mailbox.
// The actual message slots are NOT zeroed here — they are tombstoned by
// virtue of the marker scheme. Doing a full sweep on destroy could fan out
// to thousands of writes; state expiry will collect the orphaned slots
// over time. The mailbox ID is unreachable after destroy (PO marker gone)
// so the orphans are not user-visible.
func mbResetCounters(sdb StateDB, id common.Hash) {
	mbWriteUint64(sdb, mbKeyHead(id), 0)
	mbWriteUint64(sdb, mbKeyTail(id), 0)
	mbWriteUint64(sdb, mbKeyCount(id), 0)
	mbWriteUint64(sdb, mbKeyPending(id), 0)
}

// mbReservePending increments the pending counter and is called from the
// 0x35 sendMessage path BEFORE enqueuing the deferred effect. Together
// with the count check it implements deterministic capacity backpressure
// at send time.
func mbReservePending(sdb StateDB, id common.Hash) {
	p := mbReadUint64(sdb, mbKeyPending(id))
	mbWriteUint64(sdb, mbKeyPending(id), p+1)
}

// mbReleasePending decrements the pending counter. Called by the deferred
// dispatcher after a delivery attempt — both on success (the message moved
// from pending to delivered) and on failure (mailbox vanished, capacity
// raced, etc.) so the counter cannot get stuck high.
func mbReleasePending(sdb StateDB, id common.Hash) {
	p := mbReadUint64(sdb, mbKeyPending(id))
	if p > 0 {
		mbWriteUint64(sdb, mbKeyPending(id), p-1)
	}
}

// mbAppend writes the message at the current tail, advances tail, and
// increments count. Caller has already verified capacity and decoded the
// MailboxConfig.
func mbAppend(sdb StateDB, id common.Hash, msg *types.MailboxMessage) error {
	data, err := msg.EncodeRLP()
	if err != nil {
		return err
	}
	tail := mbReadUint64(sdb, mbKeyTail(id))
	mbWriteMessage(sdb, id, tail, data)
	mbWriteUint64(sdb, mbKeyTail(id), tail+1)
	cnt := mbReadUint64(sdb, mbKeyCount(id))
	mbWriteUint64(sdb, mbKeyCount(id), cnt+1)
	return nil
}

// mbPopHead dequeues the message at head, returns it, advances head, and
// decrements count. Returns (nil, false) if the queue is empty.
func mbPopHead(sdb StateDB, id common.Hash) (*types.MailboxMessage, bool) {
	cnt := mbReadUint64(sdb, mbKeyCount(id))
	if cnt == 0 {
		return nil, false
	}
	head := mbReadUint64(sdb, mbKeyHead(id))
	msg := mbReadMessage(sdb, id, head)
	if msg == nil {
		// Defensive: a slot disappeared without a count update.
		// Re-sync: clear and advance, do NOT block the queue.
		mbClearMessage(sdb, id, head)
		mbWriteUint64(sdb, mbKeyHead(id), head+1)
		mbWriteUint64(sdb, mbKeyCount(id), cnt-1)
		return nil, false
	}
	mbClearMessage(sdb, id, head)
	mbWriteUint64(sdb, mbKeyHead(id), head+1)
	mbWriteUint64(sdb, mbKeyCount(id), cnt-1)
	return msg, true
}

// mbPeekHead reads the message at head WITHOUT mutating any state. Used by
// peekMessage (read-only path) and by the dispatcher's diagnostic logs.
func mbPeekHead(sdb StateDB, id common.Hash) *types.MailboxMessage {
	cnt := mbReadUint64(sdb, mbKeyCount(id))
	if cnt == 0 {
		return nil
	}
	head := mbReadUint64(sdb, mbKeyHead(id))
	return mbReadMessage(sdb, id, head)
}

// HandleMailboxSendEffect is invoked from the deferred processing
// dispatcher (package core) at block N+1 for each EffectTypeMailboxSend
// entry that was enqueued at block N. The contract:
//
//   - On success: append the message to the target mailbox, decrement
//     pending counter, return nil.
//   - On expected failure (mailbox missing, wrong type, capacity exceeded
//     by a race, expired): decrement pending, drop the message silently,
//     return nil. Per NIP-0004 §18, delivery NEVER causes a block to be
//     invalid; dropped messages are logged at the dispatcher level.
//   - On unrecoverable failure (corrupt RLP payload — should be impossible
//     because the enqueue path validates): return an error. The dispatcher
//     records the error metric and clears the entry; block validity is not
//     affected.
//
// CONSENSUS-CRITICAL: this function MUST be deterministic. All inputs are
// drawn from state (mailbox PO body, pending counter, count) or from the
// fixed deferred-effect entry. No wall clock, no random, no map iteration.
//
// `currentBlock` is the block at which delivery is happening (== sourceBlock
// + 1 on the canonical path). It is recorded as MailboxMessage.Timestamp
// only via the SourceBlock carried in the effect payload — the message's
// declared "timestamp" is the SEND block, not the DELIVERY block, matching
// NIP-0004 §3.5 semantics.
func HandleMailboxSendEffect(sdb StateDB, entry *types.DeferredEffect, currentBlock uint64) error {
	if entry == nil {
		return errors.New("HandleMailboxSendEffect: nil entry")
	}
	if entry.EffectType != types.EffectTypeMailboxSend {
		return errors.New("HandleMailboxSendEffect: wrong effect type")
	}
	payload, err := types.DecodeMailboxSendEffect(entry.Payload)
	if err != nil {
		// Corrupt payload — the enqueue path validates, so this should
		// be unreachable on canonical paths. We don't have a mailbox ID
		// to release pending against, so bail with an error.
		return err
	}

	// Always release pending no matter the outcome below — the message has
	// left the deferred queue, so its pending reservation must be released
	// regardless of whether delivery succeeds. We do this BEFORE other
	// checks so an early return from any branch still releases.
	defer mbReleasePending(sdb, payload.MailboxID)

	obj := MbGetMailbox(sdb, payload.MailboxID)
	if obj == nil {
		// Mailbox was destroyed, ID never existed, or wrong type. Drop.
		return nil
	}
	// Hard expiry check. Block-expiry is the same dead state regardless
	// of rent (Phase 4 mailboxes do not yet pay rent through the Phase 3
	// engine — the mechanism is plumbed for future use).
	if obj.ExpiryBlock != 0 && currentBlock > obj.ExpiryBlock {
		return nil
	}
	cfg, err := types.DecodeMailboxConfig(obj.StateData)
	if err != nil {
		// Corrupt mailbox config — drop the message. The mailbox itself
		// is unusable; users will see this via getMailbox returning a
		// decode error and can destroy + recreate.
		return nil
	}

	// Capacity check at delivery — defensive. The send path also checks
	// (count + pending) < capacity, so this should not normally trip.
	cnt := mbReadUint64(sdb, mbKeyCount(payload.MailboxID))
	if cnt >= cfg.CapacityLimit {
		return nil
	}

	msg := &types.MailboxMessage{
		Sender:         payload.Sender,
		PayloadHash:    payload.PayloadHash,
		Timestamp:      payload.SourceBlock,
		SequenceNumber: entry.SeqNum,
	}
	// mbAppend may fail only if RLP encoding of a known struct fails,
	// which is a programming error worth surfacing.
	return mbAppend(sdb, payload.MailboxID, msg)
}
