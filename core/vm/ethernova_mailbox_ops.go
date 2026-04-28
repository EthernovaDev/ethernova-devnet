// Ethernova: Mailbox Ops Precompile (NIP-0004 Phase 4)
//
// Address: 0x35 (novaMailboxOps)
//
// Per NIP-0004 §3.4 amendment: Mailbox lifecycle management lives at 0x2C
// (novaMailboxManager). Mailbox MESSAGE OPERATIONS — the high-frequency
// send/recv/peek/count surface — live at 0x35 as a separate precompile
// while the corresponding opcodes (MSEND/MRECV/MPEEK/MCOUNT, draft
// 0xf6-0xf9) are deferred to Phase 12. The "fold these selectors into
// 0x2C" alternative was considered and rejected at Phase 4A: keeping
// management and ops separate yields a smaller per-precompile gas table
// and lets us evolve the ops surface (batched recv, future opcode bridge)
// without churning the manager.
//
// Function selectors (first byte of input):
//
//   0x01 sendMessage(mailboxID:32, payloadHash:32, postageWei:32)   WRITE
//          Caller pays `postageWei` from their NOVA balance to the mailbox
//          owner. Must be >= mailbox.MinPostageWei. ACL is enforced. The
//          message is NOT delivered immediately — it is enqueued onto the
//          Phase 2 deferred queue with EffectType = MailboxSend, and is
//          delivered at the start of the NEXT block by the deferred
//          processing dispatcher. Returns the assigned global deferred-
//          queue sequence number as a 32-byte word.
//
//   0x02 recvMessage(mailboxID:32)                                  WRITE
//          Caller MUST be the mailbox owner. Dequeues the head message
//          (FIFO), tombstones its slot, and returns the RLP-encoded
//          MailboxMessage. Reverts if queue empty or caller not owner.
//
//   0x03 peekMessage(mailboxID:32)                                  READ
//          Returns the RLP-encoded MailboxMessage at head WITHOUT removing
//          it. Allowed under STATICCALL. Reverts if queue empty.
//
//   0x04 countMessages(mailboxID:32)                                READ
//          Returns the count of UNREAD (delivered, not yet recvd) messages
//          as a 32-byte word. Does not include in-flight messages.
//
// readOnly enforcement (EIP-214):
//   - 0x01 (sendMessage) and 0x02 (recvMessage) are write operations and
//     return ErrWriteProtection under STATICCALL.
//   - 0x03 (peekMessage) and 0x04 (countMessages) are read-only.
//
// Determinism notes:
//   - sendMessage assigns the message its global SequenceNumber from the
//     Phase 2 deferred queue. That counter is monotonic across all deferred
//     effects and is the authoritative ordering key for delivery at block
//     N+1. Two sendMessage calls in the same block to the same mailbox are
//     ordered by tx_index — same as any other state mutation.
//   - recvMessage / peekMessage iterate by uint64 head index, never by Go
//     map iteration.
//
// Postage flow:
//   Postage is debited from caller's NOVA balance and credited to the
//   mailbox owner's NOVA balance AT SEND TIME. This makes postage a real
//   anti-spam economic cost that cannot be reversed even if the deferred
//   delivery silently drops (capacity race, mailbox destroyed mid-flight).
//   Owners are encouraged to size MinPostageWei to make spam unprofitable.
//
// Fork gate: gated on MailboxForkBlock. Pre-fork all selectors revert.

package vm

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params/ethernova"
	"github.com/holiman/uint256"
)

// Gas costs for 0x35. These are conservative ballparks calibrated against
// the Phase 1/2/3 cost shape: the protocol_ops dimension is the relevant
// budget here, and per NIP-0004 §6.1 messaging is the canonical
// protocol_ops workload.
const (
	mailboxOpsGasSendBase    uint64 = 25000 // includes deferred-enqueue cost
	mailboxOpsGasSendPostage uint64 = 5000  // balance-touch surcharge
	mailboxOpsGasRecv        uint64 = 15000 // dequeue + tombstone
	mailboxOpsGasPeek        uint64 = 2000
	mailboxOpsGasCount       uint64 = 500
)

// =====================================================================
// Precompile struct
// =====================================================================

type novaMailboxOps struct{}

// Compile-time assertion: novaMailboxOps satisfies StatefulPrecompiledContract.
var _ StatefulPrecompiledContract = (*novaMailboxOps)(nil)

func (c *novaMailboxOps) RequiredGas(input []byte) uint64 {
	if len(input) < 1 {
		return 0
	}
	switch input[0] {
	case 0x01:
		return mailboxOpsGasSendBase + mailboxOpsGasSendPostage
	case 0x02:
		return mailboxOpsGasRecv
	case 0x03:
		return mailboxOpsGasPeek
	case 0x04:
		return mailboxOpsGasCount
	default:
		return 0
	}
}

func (c *novaMailboxOps) Run(input []byte) ([]byte, error) {
	return nil, errors.New("novaMailboxOps: requires stateful execution")
}

func (c *novaMailboxOps) RunStateful(evm *EVM, caller common.Address, input []byte, readOnly bool) ([]byte, error) {
	if len(input) < 1 {
		return nil, errors.New("novaMailboxOps: empty input")
	}
	if evm.Context.BlockNumber.Uint64() < ethernova.MailboxForkBlock {
		return nil, errors.New("novaMailboxOps: not yet active")
	}
	switch input[0] {
	case 0x01: // sendMessage — WRITE
		if readOnly {
			return nil, ErrWriteProtection
		}
		return c.sendMessage(evm, caller, input[1:])
	case 0x02: // recvMessage — WRITE (state mutation: dequeue + tombstone)
		if readOnly {
			return nil, ErrWriteProtection
		}
		return c.recvMessage(evm, caller, input[1:])
	case 0x03: // peekMessage — READ
		return c.peekMessage(evm, input[1:])
	case 0x04: // countMessages — READ
		return c.countMessages(evm, input[1:])
	default:
		return nil, errors.New("novaMailboxOps: unknown selector")
	}
}

// --- 0x01 sendMessage ---------------------------------------------------

func (c *novaMailboxOps) sendMessage(evm *EVM, caller common.Address, input []byte) ([]byte, error) {
	if len(input) < 96 {
		return nil, errors.New("sendMessage: input too short (need 96 bytes)")
	}
	mailboxID := common.BytesToHash(input[0:32])
	payloadHash := common.BytesToHash(input[32:64])
	postage := new(big.Int).SetBytes(input[64:96])

	if postage.Sign() < 0 {
		return nil, errors.New("sendMessage: postage must be non-negative")
	}

	sdb := evm.StateDB
	blockNum := evm.Context.BlockNumber.Uint64()

	// Resolve mailbox + config FIRST, before touching any balances. If the
	// mailbox is missing or invalid we revert without side-effects.
	obj := MbGetMailbox(sdb, mailboxID)
	if obj == nil {
		return nil, errors.New("sendMessage: mailbox not found")
	}
	// Block-expiry check: a mailbox past its expiry_block cannot accept
	// new messages. Already-queued messages can still be recvd; this only
	// affects new sends.
	if obj.ExpiryBlock != 0 && blockNum > obj.ExpiryBlock {
		return nil, errors.New("sendMessage: mailbox expired")
	}
	cfg, err := types.DecodeMailboxConfig(obj.StateData)
	if err != nil {
		return nil, fmt.Errorf("sendMessage: decode config: %w", err)
	}

	// ACL enforcement. Owner is always allowed (handled inside IsSenderAllowed).
	if !cfg.IsSenderAllowed(caller, obj.Owner) {
		return nil, errors.New("sendMessage: sender not permitted by ACL")
	}

	// Postage enforcement. Caller must attach >= MinPostageWei. The actual
	// postage paid is the value in input[64:96] — minimums above are
	// honored, callers can attach more to e.g. signal priority off-chain.
	minPostage := cfg.MinPostageWei
	if minPostage == nil {
		minPostage = new(big.Int)
	}
	if postage.Cmp(minPostage) < 0 {
		return nil, fmt.Errorf("sendMessage: postage %s below minimum %s",
			postage.String(), minPostage.String())
	}

	// Capacity backpressure: at send time, (count + pending) MUST be
	// strictly less than capacity_limit. This is the deterministic
	// enforcement of the NIP-0004 §16 "MSEND revert jika mailbox penuh"
	// criterion. Without the pending counter, two sends in the same block
	// could both pass this check and overflow at delivery.
	cnt := mbReadUint64(sdb, mbKeyCount(mailboxID))
	pending := mbReadUint64(sdb, mbKeyPending(mailboxID))
	if cnt >= cfg.CapacityLimit || pending >= cfg.CapacityLimit-cnt {
		return nil, fmt.Errorf("sendMessage: mailbox full (count=%d pending=%d cap=%d)",
			cnt, pending, cfg.CapacityLimit)
	}

	// Postage transfer. Pull from caller, credit to mailbox owner. We use
	// the standard SubBalance/AddBalance pair the rest of the codebase
	// uses (see ethernova_privacy.go for the reference).
	if postage.Sign() > 0 {
		postageU256, overflow := uint256.FromBig(postage)
		if overflow {
			return nil, errors.New("sendMessage: postage overflow")
		}
		callerBal := sdb.GetBalance(caller)
		if callerBal.Cmp(postageU256) < 0 {
			return nil, errors.New("sendMessage: insufficient NOVA balance for postage")
		}
		sdb.SubBalance(caller, postageU256)
		sdb.AddBalance(obj.Owner, postageU256)
	}

	// Build the deferred effect payload.
	effect := &types.MailboxSendEffect{
		MailboxID:   mailboxID,
		Sender:      caller,
		PayloadHash: payloadHash,
		SourceBlock: blockNum,
	}
	payload, err := effect.EncodeRLP()
	if err != nil {
		return nil, fmt.Errorf("sendMessage: encode effect: %w", err)
	}
	if uint64(len(payload)) > ethernova.MaxDeferredEffectPayloadBytes {
		// Should not happen — the encoded effect is fixed-shape (one hash
		// + one address + one hash + one uint64) which fits comfortably
		// inside MaxDeferredEffectPayloadBytes. Defended for safety.
		return nil, fmt.Errorf("sendMessage: payload exceeds cap (%d > %d)",
			len(payload), ethernova.MaxDeferredEffectPayloadBytes)
	}

	// Reserve pending capacity BEFORE enqueue so the pending counter and
	// the deferred queue tail advance under a single coherent state.
	mbReservePending(sdb, mailboxID)

	// Enqueue onto the Phase 2 deferred queue. This call performs the same
	// per-block enqueue cap, RLP framing, and tail advancement that the
	// 0x2A precompile's enqueueEffect selector does — see
	// DqEnqueueDirectly's contract.
	seq, err := DqEnqueueDirectly(sdb, blockNum, types.EffectTypeMailboxSend, caller, payload)
	if err != nil {
		// Roll back the pending reservation. If we left the counter
		// elevated after a failed enqueue the mailbox would slowly fill
		// up with phantom pending entries that never deliver.
		mbReleasePending(sdb, mailboxID)
		return nil, fmt.Errorf("sendMessage: deferred enqueue: %w", err)
	}

	// Refresh PO last_touched_block to keep state-expiry happy on busy
	// mailboxes. Cheap (one chunked write) and aligns with how Phase 3
	// touches its objects on each interaction.
	obj.LastTouchedBlock = blockNum
	objData, err := obj.EncodeRLP()
	if err == nil {
		poWriteRLP(sdb, mailboxID, objData)
	}

	return common.BigToHash(new(big.Int).SetUint64(seq)).Bytes(), nil
}

// --- 0x02 recvMessage ---------------------------------------------------

func (c *novaMailboxOps) recvMessage(evm *EVM, caller common.Address, input []byte) ([]byte, error) {
	if len(input) < 32 {
		return nil, errors.New("recvMessage: input too short")
	}
	mailboxID := common.BytesToHash(input[0:32])
	sdb := evm.StateDB

	obj := MbGetMailbox(sdb, mailboxID)
	if obj == nil {
		return nil, errors.New("recvMessage: mailbox not found")
	}
	if obj.Owner != caller {
		return nil, errors.New("recvMessage: caller is not owner")
	}

	msg, ok := mbPopHead(sdb, mailboxID)
	if !ok || msg == nil {
		return nil, errors.New("recvMessage: queue empty")
	}

	obj.LastTouchedBlock = evm.Context.BlockNumber.Uint64()
	objData, err := obj.EncodeRLP()
	if err == nil {
		poWriteRLP(sdb, mailboxID, objData)
	}

	return msg.EncodeRLP()
}

// --- 0x03 peekMessage ---------------------------------------------------

func (c *novaMailboxOps) peekMessage(evm *EVM, input []byte) ([]byte, error) {
	if len(input) < 32 {
		return nil, errors.New("peekMessage: input too short")
	}
	mailboxID := common.BytesToHash(input[0:32])
	sdb := evm.StateDB

	obj := MbGetMailbox(sdb, mailboxID)
	if obj == nil {
		return nil, errors.New("peekMessage: mailbox not found")
	}

	msg := mbPeekHead(sdb, mailboxID)
	if msg == nil {
		return nil, errors.New("peekMessage: queue empty")
	}
	return msg.EncodeRLP()
}

// --- 0x04 countMessages -------------------------------------------------

func (c *novaMailboxOps) countMessages(evm *EVM, input []byte) ([]byte, error) {
	if len(input) < 32 {
		return nil, errors.New("countMessages: input too short")
	}
	mailboxID := common.BytesToHash(input[0:32])
	// We do NOT check mailbox existence for countMessages — a missing
	// mailbox returns 0, which is the natural answer. This makes count
	// safe to call from contracts that may race with destroy.
	count := MbGetCount(evm.StateDB, mailboxID)
	return common.BigToHash(new(big.Int).SetUint64(count)).Bytes(), nil
}
