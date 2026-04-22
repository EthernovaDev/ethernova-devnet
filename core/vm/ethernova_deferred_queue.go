// Ethernova: Pending Effects Queue + Deferred Execution Precompile
// (NIP-0004 Phase 2)
//
// Address: 0x2A (novaDeferredQueue)
//
// Follows the exact same pattern as novaProtocolObjectRegistry (0x29):
// storage is held at a reserved system account (0xFF02) via vm.StateDB
// GetState/SetState. No new trie type, no change to block header, no change
// to the state root formula. Phase 2 is a pure storage+execution-phase
// feature gated by DeferredExecForkBlock.
//
// Function selectors (first byte of input):
//   0x01 - enqueueEffect(effectType: uint8, payload: bytes)          WRITE
//           returns uint64 sequence number (big-endian, 32-byte padded)
//   0x02 - getPendingEffect(seq: uint64)                             READ
//           returns RLP-encoded DeferredEffect, or error if cleared/absent
//   0x03 - getQueueStats()                                           READ
//           returns (head, tail, pending, enqueuesThisBlock, totalProcessed)
//           packed as 5 × uint256 (160 bytes)
//
// Storage layout (all at DeferredQueueAddr = 0xFF02):
//   keccak256("q_head")                       -> next seq to process
//   keccak256("q_tail")                       -> next seq to write (exclusive)
//   keccak256("q_frontier")                   -> pinned snapshot for Phase 0
//                                                of the current block; 0 if
//                                                Phase 0 is not mid-flight.
//   keccak256("q_global_seq")                 -> monotonic counter, == tail
//   keccak256("q_total_processed")            -> lifetime processed count
//   keccak256("q_enq_count_at_block")         -> enqueue count at current block
//   keccak256("q_enq_last_block")             -> last block that enqueued
//   keccak256("q_entry_marker", seq)          -> 0x01 if entry present
//   keccak256("q_entry_len",    seq)          -> RLP byte length
//   keccak256("q_entry_chunks", seq)          -> RLP chunk count
//   keccak256("q_entry_chunk",  seq, idx)     -> 32-byte RLP chunk
//
// Determinism invariants (CONSENSUS-CRITICAL):
//   - seq is assigned sequentially from a single monotonic counter backed by
//     state. No Go map iteration, no timestamp, no wall-clock input.
//   - Enqueue order == insertion order == tx_index order (because tx
//     execution in state_processor is single-threaded and sequential).
//   - Entry keys are derived from seq with fixed-width 8-byte big-endian
//     encoding — cross-platform identical.
//   - Phase 0 drain order == seq order (ascending from head to frontier).

package vm

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params/ethernova"
)

// Gas costs for the 0x2A precompile. These are conservative and match the
// shape of 0x29 pricing: writes cost ~10× reads, enqueue is a multi-slot
// write so it gets the full write price plus a per-chunk adder.
const (
	deferredQueueGasEnqueueBase   uint64 = 10000
	deferredQueueGasEnqueuePerChk uint64 = 200 // per 32-byte chunk of payload
	deferredQueueGasRead          uint64 = 2000
	deferredQueueGasStats         uint64 = 1000
)

// DeferredQueueAddr is the system address where the pending-effects queue
// lives. It is distinct from ProtocolObjectRegistryAddr (0xFF01) to isolate
// queue storage from object storage — each subsystem has its own reserved
// account so a bug in one cannot corrupt the other.
var DeferredQueueAddr = common.HexToAddress("0x000000000000000000000000000000000000FF02")

// --- Storage key builders ---

func dqKeyHead() common.Hash           { return crypto.Keccak256Hash([]byte("q_head")) }
func dqKeyTail() common.Hash           { return crypto.Keccak256Hash([]byte("q_tail")) }
func dqKeyFrontier() common.Hash       { return crypto.Keccak256Hash([]byte("q_frontier")) }
func dqKeyGlobalSeq() common.Hash      { return crypto.Keccak256Hash([]byte("q_global_seq")) }
func dqKeyTotalProcessed() common.Hash { return crypto.Keccak256Hash([]byte("q_total_processed")) }
func dqKeyEnqCountAtBlock() common.Hash {
	return crypto.Keccak256Hash([]byte("q_enq_count_at_block"))
}
func dqKeyEnqLastBlock() common.Hash { return crypto.Keccak256Hash([]byte("q_enq_last_block")) }

func dqSeqBytes(seq uint64) []byte {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], seq)
	return buf[:]
}

func dqKeyEntryMarker(seq uint64) common.Hash {
	return crypto.Keccak256Hash([]byte("q_entry_marker"), dqSeqBytes(seq))
}
func dqKeyEntryLen(seq uint64) common.Hash {
	return crypto.Keccak256Hash([]byte("q_entry_len"), dqSeqBytes(seq))
}
func dqKeyEntryChunks(seq uint64) common.Hash {
	return crypto.Keccak256Hash([]byte("q_entry_chunks"), dqSeqBytes(seq))
}
func dqKeyEntryChunk(seq uint64, idx uint64) common.Hash {
	var idxBuf [8]byte
	binary.BigEndian.PutUint64(idxBuf[:], idx)
	return crypto.Keccak256Hash([]byte("q_entry_chunk"), dqSeqBytes(seq), idxBuf[:])
}

// --- Low-level state helpers ---

func dqReadUint64(sdb StateDB, key common.Hash) uint64 {
	val := sdb.GetState(DeferredQueueAddr, key)
	return new(big.Int).SetBytes(val.Bytes()).Uint64()
}

func dqWriteUint64(sdb StateDB, key common.Hash, v uint64) {
	sdb.SetState(DeferredQueueAddr, key, common.BigToHash(new(big.Int).SetUint64(v)))
}

// dqEnsureExists guarantees the reserved system account at 0xFF02 is
// non-empty per EIP-161. Same rationale as poEnsureRegistryExists (0xFF01):
// without nonce=1, state.Finalise(true) treats an account with only storage
// writes as empty and deletes it along with all pending writes, silently
// breaking consensus. See ethernova_protocol_objects.go for the full
// post-mortem of that bug.
func dqEnsureExists(sdb StateDB) {
	if !sdb.Exist(DeferredQueueAddr) {
		sdb.CreateAccount(DeferredQueueAddr)
	}
	if sdb.GetNonce(DeferredQueueAddr) == 0 {
		sdb.SetNonce(DeferredQueueAddr, 1)
	}
}

// dqWriteEntry serialises an entry RLP and writes it to chunked storage.
// The entry is keyed by its sequence number. Entries are NEVER rewritten
// under the same seq (seq is monotonic and never reused), so we don't need
// the "zero out trailing chunks" dance that the Protocol Object registry
// does for in-place updates.
func dqWriteEntry(sdb StateDB, seq uint64, data []byte) {
	dataLen := uint64(len(data))
	chunks := (dataLen + 31) / 32
	sdb.SetState(DeferredQueueAddr, dqKeyEntryMarker(seq),
		common.BytesToHash([]byte{0x01}))
	dqWriteUint64(sdb, dqKeyEntryLen(seq), dataLen)
	dqWriteUint64(sdb, dqKeyEntryChunks(seq), chunks)
	for i := uint64(0); i < chunks; i++ {
		start := i * 32
		end := start + 32
		if end > dataLen {
			end = dataLen
		}
		var chunk [32]byte
		copy(chunk[:], data[start:end])
		sdb.SetState(DeferredQueueAddr, dqKeyEntryChunk(seq, i),
			common.BytesToHash(chunk[:]))
	}
}

// dqReadEntry reads and decodes an entry. Returns nil if the marker is absent
// (either never existed or already cleared by Phase 0).
func dqReadEntry(sdb StateDB, seq uint64) *types.DeferredEffect {
	marker := sdb.GetState(DeferredQueueAddr, dqKeyEntryMarker(seq))
	if marker == (common.Hash{}) {
		return nil
	}
	dataLen := dqReadUint64(sdb, dqKeyEntryLen(seq))
	if dataLen == 0 {
		return nil
	}
	chunks := dqReadUint64(sdb, dqKeyEntryChunks(seq))
	data := make([]byte, 0, dataLen)
	for i := uint64(0); i < chunks; i++ {
		chunk := sdb.GetState(DeferredQueueAddr, dqKeyEntryChunk(seq, i))
		remaining := dataLen - uint64(len(data))
		if remaining >= 32 {
			data = append(data, chunk[:]...)
		} else {
			data = append(data, chunk[:remaining]...)
		}
	}
	ef, err := types.DecodeDeferredEffect(data)
	if err != nil {
		return nil
	}
	return ef
}

// dqClearEntry tombstones an entry slot. Called by Phase 0 after the entry
// is processed. We zero marker + len + chunk_count AND every chunk slot so
// the underlying state is fully erased and GC-friendly (state expiry can
// collect these zero slots).
func dqClearEntry(sdb StateDB, seq uint64) {
	chunks := dqReadUint64(sdb, dqKeyEntryChunks(seq))
	for i := uint64(0); i < chunks; i++ {
		sdb.SetState(DeferredQueueAddr, dqKeyEntryChunk(seq, i), common.Hash{})
	}
	sdb.SetState(DeferredQueueAddr, dqKeyEntryMarker(seq), common.Hash{})
	dqWriteUint64(sdb, dqKeyEntryLen(seq), 0)
	dqWriteUint64(sdb, dqKeyEntryChunks(seq), 0)
}

// --- Exported accessors (used by state processor, RPC layer, tests) ---

// DqGetHead returns the next sequence number awaiting Phase 0 processing.
func DqGetHead(sdb StateDB) uint64 { return dqReadUint64(sdb, dqKeyHead()) }

// DqGetTail returns the next sequence number that will be assigned on the
// next successful enqueue. (tail - head) is the current pending count.
func DqGetTail(sdb StateDB) uint64 { return dqReadUint64(sdb, dqKeyTail()) }

// DqGetFrontier returns the frontier snapshot set by the Phase 0 prologue.
// During tx execution this may be 0 (Phase 0 has finished and the field was
// reset) or non-zero (Phase 0 is in-flight, which should not be observable
// from a tx under normal flow since we finalize before tx loop starts).
func DqGetFrontier(sdb StateDB) uint64 { return dqReadUint64(sdb, dqKeyFrontier()) }

// DqGetPendingCount returns tail - head, the number of entries currently
// in the queue waiting to be processed.
func DqGetPendingCount(sdb StateDB) uint64 {
	head := DqGetHead(sdb)
	tail := DqGetTail(sdb)
	if tail < head {
		return 0 // defensive; should never happen
	}
	return tail - head
}

// DqGetTotalProcessed returns the lifetime count of entries that Phase 0
// has successfully drained.
func DqGetTotalProcessed(sdb StateDB) uint64 {
	return dqReadUint64(sdb, dqKeyTotalProcessed())
}

// DqGetEnqueueCountAtBlock returns the number of enqueues that have happened
// at the given block. Returns 0 if the given block is not the current tracked
// block (in which case the counter has either been reset or not yet started).
func DqGetEnqueueCountAtBlock(sdb StateDB, blockNum uint64) uint64 {
	lastBlock := dqReadUint64(sdb, dqKeyEnqLastBlock())
	if lastBlock != blockNum {
		return 0
	}
	return dqReadUint64(sdb, dqKeyEnqCountAtBlock())
}

// DqGetEntry returns the raw DeferredEffect at the given sequence number,
// or nil if absent/cleared. Used by RPC.
func DqGetEntry(sdb StateDB, seq uint64) *types.DeferredEffect {
	return dqReadEntry(sdb, seq)
}

// DqListPending returns up to `limit` pending entries starting from head+offset.
// Used by RPC for debugging; iteration is by ascending sequence number which
// is the authoritative processing order.
func DqListPending(sdb StateDB, offset, limit uint64) []*types.DeferredEffect {
	if limit == 0 {
		return nil
	}
	if limit > 256 {
		limit = 256
	}
	head := DqGetHead(sdb)
	tail := DqGetTail(sdb)
	start := head + offset
	out := make([]*types.DeferredEffect, 0, limit)
	for seq := start; seq < tail && uint64(len(out)) < limit; seq++ {
		if ef := dqReadEntry(sdb, seq); ef != nil {
			out = append(out, ef)
		}
	}
	return out
}

// DqSetFrontier pins the Phase 0 frontier at the current tail. Called by
// ProcessDeferredEffects before draining. Exported so the processing phase
// in package core can call it without duplicating the key-building logic.
func DqSetFrontier(sdb StateDB, frontier uint64) {
	dqWriteUint64(sdb, dqKeyFrontier(), frontier)
}

// DqClearFrontier resets the frontier to 0 after Phase 0 finishes draining.
// Callers outside this file are expected to do this via ProcessDeferredEffects
// only — exposed here for test harnesses.
func DqClearFrontier(sdb StateDB) { dqWriteUint64(sdb, dqKeyFrontier(), 0) }

// DqAdvanceHead moves head forward after successful drain. Head never moves
// backward. Called only by ProcessDeferredEffects.
func DqAdvanceHead(sdb StateDB, newHead uint64) {
	cur := DqGetHead(sdb)
	if newHead > cur {
		dqWriteUint64(sdb, dqKeyHead(), newHead)
	}
}

// DqIncrementTotalProcessed is called by ProcessDeferredEffects after each
// successful entry drain. Exposed so the drain loop lives in package core
// (where it can access *state.StateDB and concrete types cleanly).
func DqIncrementTotalProcessed(sdb StateDB, delta uint64) {
	cur := DqGetTotalProcessed(sdb)
	dqWriteUint64(sdb, dqKeyTotalProcessed(), cur+delta)
}

// DqClearEntry exposes the internal clear operation to the processing phase.
func DqClearEntry(sdb StateDB, seq uint64) { dqClearEntry(sdb, seq) }

// DqEnsureExists exposes the system-account anchor (EIP-161 nonce bump).
func DqEnsureExists(sdb StateDB) { dqEnsureExists(sdb) }

// DqReadEntry exposes the decoded-entry reader for the processing phase.
func DqReadEntry(sdb StateDB, seq uint64) *types.DeferredEffect {
	return dqReadEntry(sdb, seq)
}

// === Precompile ===

type novaDeferredQueue struct{}

func (c *novaDeferredQueue) RequiredGas(input []byte) uint64 {
	if len(input) < 1 {
		return 0
	}
	switch input[0] {
	case 0x01: // enqueueEffect
		// Base plus per-chunk for payload RLP. We don't have the full RLP
		// size here (header length depends on encoding), so we approximate
		// by payload raw length rounded up to chunks. The enqueue handler
		// hard-rejects oversize payloads so this is bounded.
		if len(input) < 2 {
			return deferredQueueGasEnqueueBase
		}
		// input[1] is effectType, input[2:] is payload.
		payloadLen := uint64(len(input)) - 2
		chunks := (payloadLen + 31) / 32
		return deferredQueueGasEnqueueBase + chunks*deferredQueueGasEnqueuePerChk
	case 0x02:
		return deferredQueueGasRead
	case 0x03:
		return deferredQueueGasStats
	default:
		return 0
	}
}

func (c *novaDeferredQueue) Run(input []byte) ([]byte, error) {
	return nil, errors.New("novaDeferredQueue: requires stateful execution")
}

func (c *novaDeferredQueue) RunStateful(evm *EVM, caller common.Address, input []byte, readOnly bool) ([]byte, error) {
	if len(input) < 1 {
		return nil, errors.New("empty input")
	}
	// Fork gate: Phase 2 precompile is a no-op before DeferredExecForkBlock.
	// Reads still return zero (stats = 0/0/0/0/0), writes revert.
	blockNum := evm.Context.BlockNumber.Uint64()
	forkActive := blockNum >= ethernova.DeferredExecForkBlock

	switch input[0] {
	case 0x01: // enqueueEffect — WRITE
		if readOnly {
			return nil, ErrWriteProtection
		}
		if !forkActive {
			return nil, errors.New("enqueueEffect: DeferredExec fork not active")
		}
		return c.enqueueEffect(evm, caller, input[1:])
	case 0x02: // getPendingEffect — READ
		if !forkActive {
			return nil, errors.New("getPendingEffect: DeferredExec fork not active")
		}
		return c.getPendingEffect(evm, input[1:])
	case 0x03: // getQueueStats — READ (returns zero vector if fork inactive)
		return c.getQueueStats(evm)
	default:
		return nil, errors.New("unknown function selector")
	}
}

func (c *novaDeferredQueue) enqueueEffect(evm *EVM, caller common.Address, input []byte) ([]byte, error) {
	if len(input) < 1 {
		return nil, errors.New("enqueueEffect: missing effect type")
	}
	effectType := input[0]
	if !types.IsValidDeferredEffectType(effectType) {
		return nil, fmt.Errorf("enqueueEffect: invalid effect type 0x%02x", effectType)
	}
	var payload []byte
	if len(input) > 1 {
		payload = input[1:]
	}
	if uint64(len(payload)) > ethernova.MaxDeferredEffectPayloadBytes {
		return nil, fmt.Errorf("enqueueEffect: payload exceeds cap (%d > %d)",
			len(payload), ethernova.MaxDeferredEffectPayloadBytes)
	}

	sdb := evm.StateDB
	blockNum := evm.Context.BlockNumber.Uint64()

	// Per-block backpressure. Reset counter on block boundary. We compare the
	// stored "last enqueue block" to current block — if different, this is
	// the first enqueue at this block, so we reset the count to 0.
	lastBlock := dqReadUint64(sdb, dqKeyEnqLastBlock())
	var enqCount uint64
	if lastBlock == blockNum {
		enqCount = dqReadUint64(sdb, dqKeyEnqCountAtBlock())
	} else {
		enqCount = 0
	}
	if enqCount >= ethernova.MaxPendingEffectsPerBlock {
		return nil, fmt.Errorf("enqueueEffect: per-block cap reached (%d)",
			ethernova.MaxPendingEffectsPerBlock)
	}

	// Ensure the queue system account is non-empty (EIP-161 guard).
	dqEnsureExists(sdb)

	// Mint sequence number from the monotonic global counter. This is the
	// authoritative ordering key. seq == tail at this point.
	tail := DqGetTail(sdb)
	seq := tail

	// SourceTxHash is a deterministic traceability tag derived from
	// (caller, blockNumber, seq). It is NOT the actual tx hash — exposing
	// the real tx hash to a precompile would require widening the vm.StateDB
	// interface (the concrete *state.StateDB field `thash` is unexported).
	// For consensus correctness only determinism matters, and the derivation
	// here is fully deterministic and unique per (caller,block,seq) triple.
	// Future phases can tighten this to the real hash if we decide to
	// extend the interface.
	var seqBuf, blockBuf [8]byte
	binary.BigEndian.PutUint64(seqBuf[:], seq)
	binary.BigEndian.PutUint64(blockBuf[:], blockNum)
	txHash := crypto.Keccak256Hash(caller.Bytes(), blockBuf[:], seqBuf[:])

	ef := &types.DeferredEffect{
		SeqNum:       seq,
		EffectType:   effectType,
		SourceBlock:  blockNum,
		SourceCaller: caller,
		SourceTxHash: txHash,
		Payload:      payload,
	}

	data, err := ef.EncodeRLP()
	if err != nil {
		return nil, fmt.Errorf("enqueueEffect: RLP encode: %w", err)
	}

	dqWriteEntry(sdb, seq, data)

	// Advance the monotonic counters. We keep tail and global_seq as
	// separate named slots even though they carry the same value; this
	// makes future extensions (e.g. sparse queues) cheaper and makes the
	// RPC layer's semantics clearer.
	dqWriteUint64(sdb, dqKeyTail(), seq+1)
	dqWriteUint64(sdb, dqKeyGlobalSeq(), seq+1)
	dqWriteUint64(sdb, dqKeyEnqCountAtBlock(), enqCount+1)
	dqWriteUint64(sdb, dqKeyEnqLastBlock(), blockNum)

	// Return the assigned sequence as a 32-byte big-endian word.
	return common.BigToHash(new(big.Int).SetUint64(seq)).Bytes(), nil
}

func (c *novaDeferredQueue) getPendingEffect(evm *EVM, input []byte) ([]byte, error) {
	if len(input) < 8 {
		return nil, errors.New("getPendingEffect: seq argument too short")
	}
	// Accept either 8-byte or 32-byte encoding of the uint64 seq. Standard
	// Solidity ABI pads to 32 bytes, so we read the last 8 bytes.
	var seq uint64
	if len(input) >= 32 {
		seq = binary.BigEndian.Uint64(input[24:32])
	} else {
		seq = binary.BigEndian.Uint64(input[:8])
	}
	ef := dqReadEntry(evm.StateDB, seq)
	if ef == nil {
		return nil, fmt.Errorf("getPendingEffect: no entry at seq %d", seq)
	}
	return ef.EncodeRLP()
}

func (c *novaDeferredQueue) getQueueStats(evm *EVM) ([]byte, error) {
	sdb := evm.StateDB
	head := DqGetHead(sdb)
	tail := DqGetTail(sdb)
	var pending uint64
	if tail >= head {
		pending = tail - head
	}
	totalProcessed := DqGetTotalProcessed(sdb)
	blockNum := evm.Context.BlockNumber.Uint64()
	enqAtBlock := DqGetEnqueueCountAtBlock(sdb, blockNum)

	// Pack 5 × uint256 = 160 bytes: head, tail, pending, enqAtBlock, totalProcessed.
	out := make([]byte, 160)
	copy(out[0:32], common.BigToHash(new(big.Int).SetUint64(head)).Bytes())
	copy(out[32:64], common.BigToHash(new(big.Int).SetUint64(tail)).Bytes())
	copy(out[64:96], common.BigToHash(new(big.Int).SetUint64(pending)).Bytes())
	copy(out[96:128], common.BigToHash(new(big.Int).SetUint64(enqAtBlock)).Bytes())
	copy(out[128:160], common.BigToHash(new(big.Int).SetUint64(totalProcessed)).Bytes())
	return out, nil
}
