// Ethernova: Mailbox Manager Precompile (NIP-0004 Phase 4)
//
// Address: 0x2C (novaMailboxManager)
//
// Lifecycle precompile for Mailbox Protocol Objects: create, configure
// (re-key), destroy, and the read-only getConfig accessor.
//
// Function selectors (first byte of input):
//
//   0x01 createMailbox(...)                                      WRITE
//          Returns: 32-byte mailbox ID.
//          Input layout (all 32-byte words, fixed-width head + ACL tail):
//            [0..32]    capacityLimit       (uint64; <= MailboxAbsoluteCapacity)
//            [32..64]   retentionPolicy     (uint8;  see types.MailboxRetention*)
//            [64..96]   retentionBlocks     (uint64; <= MailboxMaxRetentionBlocks)
//            [96..128]  minPostageWei       (uint256)
//            [128..160] aclMode             (uint8;  see types.MailboxACLMode*)
//            [160..192] expiryBlock         (uint64; 0 = never)
//            [192..224] rentPrepayWei       (uint256)
//            [224..256] aclCount            (uint64; <= MailboxMaxACLEntries)
//            [256..]    ACL[i]              (each 32 bytes, low 20 = address)
//
//   0x02 configureMailbox(...)                                   WRITE
//          Same field layout as createMailbox, prefixed with the 32-byte
//          mailbox ID. Caller MUST be the mailbox owner. Recreates the
//          MailboxConfig RLP from scratch — partial updates not supported.
//          Input layout:
//            [0..32]    mailboxID           (bytes32)
//            [32..64]   capacityLimit
//            [64..96]   retentionPolicy
//            [96..128]  retentionBlocks
//            [128..160] minPostageWei
//            [160..192] aclMode
//            [192..224] aclCount
//            [224..]    ACL[i]
//          Returns: 32-byte (1 = ok).
//
//   0x03 destroyMailbox(id:32)                                   WRITE
//          Caller MUST be owner. Clears PO record, owner indices (Phase 1
//          + Mailbox-specific), and queue counters. Returns 32-byte 1.
//
//   0x04 getMailboxConfig(id:32)                                 READ
//          Returns the RLP-encoded MailboxConfig from the PO state_data.
//
// readOnly enforcement (EIP-214):
//
//   0x01/0x02/0x03 are write operations. They MUST return ErrWriteProtection
//   when invoked under STATICCALL (readOnly == true) — this guarantees the
//   readOnly bit propagates through the precompile boundary as it does
//   through normal contract calls.
//
//   0x04 is read-only and is permitted under STATICCALL.
//
// Fork gate: gated on MailboxForkBlock. Pre-fork all selectors revert.

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

// Gas costs for 0x2C. Mirrors the shape of 0x29 / 0x2B: writes are ~10x
// reads, create/configure get a per-ACL adder so a 64-entry ACL costs the
// caller appropriately. These are devnet-tunable and consensus-stable.
const (
	mailboxMgrGasCreateBase    uint64 = 30000
	mailboxMgrGasCreatePerACL  uint64 = 500
	mailboxMgrGasConfigureBase uint64 = 20000
	mailboxMgrGasConfigPerACL  uint64 = 500
	mailboxMgrGasDestroy       uint64 = 15000
	mailboxMgrGasGetConfig     uint64 = 2000
)

// =====================================================================
// Precompile struct
// =====================================================================

type novaMailboxManager struct{}

// novaMailboxManager statically asserts compliance with the
// StatefulPrecompiledContract interface (defined in
// core/vm/ethernova_account_manager.go). A compile failure here means a
// signature drift in the interface and is the early-warning we want.
var _ StatefulPrecompiledContract = (*novaMailboxManager)(nil)

func (c *novaMailboxManager) RequiredGas(input []byte) uint64 {
	if len(input) < 1 {
		return 0
	}
	switch input[0] {
	case 0x01: // createMailbox
		// Approximate per-ACL cost from the trailing ACL slice length.
		// Full validation of the count happens in createMailbox; here we
		// just want gas to scale with declared work.
		extra := uint64(0)
		if len(input) > 1+8*32 {
			extraBytes := uint64(len(input)) - (1 + 8*32)
			extra = (extraBytes / 32) * mailboxMgrGasCreatePerACL
		}
		return mailboxMgrGasCreateBase + extra
	case 0x02: // configureMailbox
		extra := uint64(0)
		if len(input) > 1+7*32 {
			extraBytes := uint64(len(input)) - (1 + 7*32)
			extra = (extraBytes / 32) * mailboxMgrGasConfigPerACL
		}
		return mailboxMgrGasConfigureBase + extra
	case 0x03: // destroyMailbox
		return mailboxMgrGasDestroy
	case 0x04: // getMailboxConfig
		return mailboxMgrGasGetConfig
	default:
		return 0
	}
}

func (c *novaMailboxManager) Run(input []byte) ([]byte, error) {
	return nil, errors.New("novaMailboxManager: requires stateful execution")
}

func (c *novaMailboxManager) RunStateful(evm *EVM, caller common.Address, input []byte, readOnly bool) ([]byte, error) {
	if len(input) < 1 {
		return nil, errors.New("novaMailboxManager: empty input")
	}
	// Fork gate: pre-fork, every selector reverts. Registered unconditionally
	// like 0x29/0x2A/0x2B, semantics gated here so there is no incidental
	// state touch on early blocks.
	if evm.Context.BlockNumber.Uint64() < ethernova.MailboxForkBlock {
		return nil, errors.New("novaMailboxManager: not yet active")
	}
	switch input[0] {
	case 0x01: // createMailbox — WRITE
		if readOnly {
			return nil, ErrWriteProtection
		}
		return c.createMailbox(evm, caller, input[1:])
	case 0x02: // configureMailbox — WRITE
		if readOnly {
			return nil, ErrWriteProtection
		}
		return c.configureMailbox(evm, caller, input[1:])
	case 0x03: // destroyMailbox — WRITE
		if readOnly {
			return nil, ErrWriteProtection
		}
		return c.destroyMailbox(evm, caller, input[1:])
	case 0x04: // getMailboxConfig — READ (allowed under STATICCALL)
		return c.getMailboxConfig(evm, input[1:])
	default:
		return nil, errors.New("novaMailboxManager: unknown selector")
	}
}

// --- shared input parsing -----------------------------------------------

// parseACLTail decodes a fixed-width 32-byte-per-entry ACL slice. Each
// entry's low 20 bytes form the address; the high 12 bytes are ignored
// (Solidity ABI left-pads addresses with zeros, so this is the standard
// shape). Returns ErrACL... on out-of-range / malformed inputs.
func parseACLTail(tail []byte, count uint64) ([]common.Address, error) {
	if count > MailboxMaxACLEntries {
		return nil, fmt.Errorf("acl exceeds MailboxMaxACLEntries (%d > %d)",
			count, MailboxMaxACLEntries)
	}
	need := count * 32
	if uint64(len(tail)) < need {
		return nil, errors.New("acl tail shorter than declared count")
	}
	out := make([]common.Address, count)
	for i := uint64(0); i < count; i++ {
		out[i] = common.BytesToAddress(tail[i*32 : i*32+32])
	}
	return out, nil
}

// validateConfigCommon enforces the field-level invariants shared by
// createMailbox and configureMailbox. Capacity == 0 is rejected — a
// zero-capacity mailbox is unusable and would fail every send.
func validateConfigCommon(cfg *types.MailboxConfig) error {
	if cfg.CapacityLimit == 0 {
		return errors.New("capacityLimit must be > 0")
	}
	if cfg.CapacityLimit > MailboxAbsoluteCapacity {
		return fmt.Errorf("capacityLimit exceeds MailboxAbsoluteCapacity (%d > %d)",
			cfg.CapacityLimit, MailboxAbsoluteCapacity)
	}
	if !types.IsValidMailboxRetention(cfg.RetentionPolicy) {
		return fmt.Errorf("invalid retentionPolicy 0x%02x", cfg.RetentionPolicy)
	}
	if cfg.RetentionPolicy == types.MailboxRetentionTTL &&
		cfg.RetentionBlocks > MailboxMaxRetentionBlocks {
		return fmt.Errorf("retentionBlocks exceeds MailboxMaxRetentionBlocks (%d > %d)",
			cfg.RetentionBlocks, MailboxMaxRetentionBlocks)
	}
	if !types.IsValidMailboxACLMode(cfg.ACLMode) {
		return fmt.Errorf("invalid aclMode 0x%02x", cfg.ACLMode)
	}
	if uint64(len(cfg.ACL)) > MailboxMaxACLEntries {
		return fmt.Errorf("acl exceeds MailboxMaxACLEntries (%d > %d)",
			len(cfg.ACL), MailboxMaxACLEntries)
	}
	if cfg.MinPostageWei == nil || cfg.MinPostageWei.Sign() < 0 {
		return errors.New("minPostageWei must be non-negative")
	}
	return nil
}

// --- 0x01 createMailbox -------------------------------------------------

func (c *novaMailboxManager) createMailbox(evm *EVM, caller common.Address, input []byte) ([]byte, error) {
	const headLen = 8 * 32 // 8 fixed-width words (see selector docs above)
	if len(input) < headLen {
		return nil, fmt.Errorf("createMailbox: input too short (need %d bytes head, got %d)",
			headLen, len(input))
	}
	capacityLimit := new(big.Int).SetBytes(input[0:32]).Uint64()
	retentionPolicy := uint8(new(big.Int).SetBytes(input[32:64]).Uint64())
	retentionBlocks := new(big.Int).SetBytes(input[64:96]).Uint64()
	minPostage := new(big.Int).SetBytes(input[96:128])
	aclMode := uint8(new(big.Int).SetBytes(input[128:160]).Uint64())
	expiryBlock := new(big.Int).SetBytes(input[160:192]).Uint64()
	rentPrepay := new(big.Int).SetBytes(input[192:224])
	aclCount := new(big.Int).SetBytes(input[224:256]).Uint64()

	acl, err := parseACLTail(input[headLen:], aclCount)
	if err != nil {
		return nil, fmt.Errorf("createMailbox: %w", err)
	}

	cfg := &types.MailboxConfig{
		CapacityLimit:   capacityLimit,
		RetentionPolicy: retentionPolicy,
		RetentionBlocks: retentionBlocks,
		MinPostageWei:   minPostage,
		ACLMode:         aclMode,
		ACL:             acl,
	}
	if err := validateConfigCommon(cfg); err != nil {
		return nil, fmt.Errorf("createMailbox: %w", err)
	}

	stateData, err := cfg.EncodeRLP()
	if err != nil {
		return nil, fmt.Errorf("createMailbox: encode config: %w", err)
	}

	sdb := evm.StateDB
	blockNum := evm.Context.BlockNumber.Uint64()

	// Make sure all three involved system accounts are non-empty BEFORE
	// any storage write — same EIP-161 rationale as Phase 1/2/3. The
	// Mailbox PO body lives at 0xFF01 (Phase 1 registry); the queue and
	// owner index live at 0xFF04 (this phase).
	poEnsureRegistryExists(sdb)
	mbEnsureExists(sdb)

	// Generate deterministic ID using the SHARED Phase 1 global nonce so
	// Mailbox IDs never collide with any other Protocol Object type.
	globalNonce := poReadUint64(sdb, poKeyGlobalNonce())
	var blockBuf, nonceBuf [8]byte
	binary.BigEndian.PutUint64(blockBuf[:], blockNum)
	binary.BigEndian.PutUint64(nonceBuf[:], globalNonce)
	idInput := make([]byte, 0, 20+8+8)
	idInput = append(idInput, caller.Bytes()...)
	idInput = append(idInput, blockBuf[:]...)
	idInput = append(idInput, nonceBuf[:]...)
	id := crypto.Keccak256Hash(idInput)
	poWriteUint64(sdb, poKeyGlobalNonce(), globalNonce+1)

	// Build the ProtocolObject (Phase 1 record) — Mailbox is type tag 0x01.
	obj := &types.ProtocolObject{
		ID:               id,
		Owner:            caller,
		TypeTag:          types.ProtoTypeMailbox,
		StateData:        stateData,
		ExpiryBlock:      expiryBlock,
		LastTouchedBlock: blockNum,
		RentBalance:      new(big.Int).Set(rentPrepay),
	}
	objData, err := obj.EncodeRLP()
	if err != nil {
		return nil, fmt.Errorf("createMailbox: encode object: %w", err)
	}

	// Phase 1 storage at 0xFF01: presence marker + RLP body.
	sdb.SetState(ProtocolObjectRegistryAddr, poKeyObject(id),
		common.BytesToHash([]byte{0x01}))
	poWriteRLP(sdb, id, objData)

	// Phase 1 global + per-type counts.
	totalCount := PoGetObjectCount(sdb)
	poWriteUint64(sdb, poKeyTotalCount(), totalCount+1)
	typeCount := PoGetTypeCount(sdb, types.ProtoTypeMailbox)
	poWriteUint64(sdb, poKeyTypeCount(types.ProtoTypeMailbox), typeCount+1)

	// Phase 1 owner index — keeps getProtocolObjectsByOwner working across
	// all object types, mirrors the dual-update pattern used by Phase 3.
	slotsUsedP1 := poReadUint64(sdb, poKeyOwnerSlotsUsed(caller))
	sdb.SetState(ProtocolObjectRegistryAddr, poKeyOwnerIndex(caller, slotsUsedP1), id)
	poWriteUint64(sdb, poKeyOwnerSlotOf(id), slotsUsedP1)
	poWriteUint64(sdb, poKeyOwnerSlotsUsed(caller), slotsUsedP1+1)
	ownerCountP1 := poReadUint64(sdb, poKeyOwnerCount(caller))
	poWriteUint64(sdb, poKeyOwnerCount(caller), ownerCountP1+1)

	// Phase 4 mailbox-specific owner index at 0xFF04.
	mbOwnerIndexAdd(sdb, caller, id)

	// Initialise queue counters at zero for clarity. Reads of these slots
	// would already return zero from a fresh trie, but writing them here
	// touches MailboxOpsAddr's storage so the EIP-161 anchor (nonce=1)
	// from mbEnsureExists definitely sticks even if no message is ever
	// sent. We pick mb_count specifically because it's the most-read slot.
	mbWriteUint64(sdb, mbKeyCount(id), 0)

	return id.Bytes(), nil
}

// --- 0x02 configureMailbox ----------------------------------------------

func (c *novaMailboxManager) configureMailbox(evm *EVM, caller common.Address, input []byte) ([]byte, error) {
	const headLen = 32 + 7*32 // id + 7 fixed-width config words
	if len(input) < headLen {
		return nil, fmt.Errorf("configureMailbox: input too short (need %d, got %d)",
			headLen, len(input))
	}
	id := common.BytesToHash(input[0:32])
	capacityLimit := new(big.Int).SetBytes(input[32:64]).Uint64()
	retentionPolicy := uint8(new(big.Int).SetBytes(input[64:96]).Uint64())
	retentionBlocks := new(big.Int).SetBytes(input[96:128]).Uint64()
	minPostage := new(big.Int).SetBytes(input[128:160])
	aclMode := uint8(new(big.Int).SetBytes(input[160:192]).Uint64())
	aclCount := new(big.Int).SetBytes(input[192:224]).Uint64()

	acl, err := parseACLTail(input[headLen:], aclCount)
	if err != nil {
		return nil, fmt.Errorf("configureMailbox: %w", err)
	}

	cfg := &types.MailboxConfig{
		CapacityLimit:   capacityLimit,
		RetentionPolicy: retentionPolicy,
		RetentionBlocks: retentionBlocks,
		MinPostageWei:   minPostage,
		ACLMode:         aclMode,
		ACL:             acl,
	}
	if err := validateConfigCommon(cfg); err != nil {
		return nil, fmt.Errorf("configureMailbox: %w", err)
	}

	sdb := evm.StateDB
	obj := MbGetMailbox(sdb, id)
	if obj == nil {
		return nil, errors.New("configureMailbox: mailbox not found")
	}
	if obj.Owner != caller {
		return nil, errors.New("configureMailbox: caller is not owner")
	}
	// Sanity guardrail: shrinking CapacityLimit below the current count
	// would make the mailbox unable to accept any new messages until
	// the queue drains below the new cap. That's actually acceptable
	// behavior, but we explicitly disallow shrinking BELOW current count
	// because it could leave stranded messages that callers can't fully
	// process if they expected the new cap immediately. Allow exactly-equal.
	curCount := mbReadUint64(sdb, mbKeyCount(id))
	if cfg.CapacityLimit < curCount {
		return nil, fmt.Errorf("configureMailbox: capacity %d < current count %d",
			cfg.CapacityLimit, curCount)
	}

	newStateData, err := cfg.EncodeRLP()
	if err != nil {
		return nil, fmt.Errorf("configureMailbox: encode config: %w", err)
	}

	obj.StateData = newStateData
	obj.LastTouchedBlock = evm.Context.BlockNumber.Uint64()
	objData, err := obj.EncodeRLP()
	if err != nil {
		return nil, fmt.Errorf("configureMailbox: encode object: %w", err)
	}
	poWriteRLP(sdb, id, objData)

	return common.BigToHash(big.NewInt(1)).Bytes(), nil
}

// --- 0x03 destroyMailbox ------------------------------------------------

func (c *novaMailboxManager) destroyMailbox(evm *EVM, caller common.Address, input []byte) ([]byte, error) {
	if len(input) < 32 {
		return nil, errors.New("destroyMailbox: input too short")
	}
	id := common.BytesToHash(input[0:32])
	sdb := evm.StateDB

	obj := MbGetMailbox(sdb, id)
	if obj == nil {
		// Idempotent on already-deleted: return 0 (matches the Phase 1
		// deleteObject behavior so callers can treat destroy as
		// "best-effort cleanup").
		return common.BigToHash(big.NewInt(0)).Bytes(), nil
	}
	if obj.Owner != caller {
		return nil, errors.New("destroyMailbox: caller is not owner")
	}

	// Phase 1: clear PO marker + RLP body.
	sdb.SetState(ProtocolObjectRegistryAddr, poKeyObject(id), common.Hash{})
	poClearRLP(sdb, id)

	// Phase 1 owner index tombstone (mirrors Phase 1 deleteObject).
	slotP1 := poReadUint64(sdb, poKeyOwnerSlotOf(id))
	sdb.SetState(ProtocolObjectRegistryAddr, poKeyOwnerIndex(obj.Owner, slotP1), common.Hash{})
	poWriteUint64(sdb, poKeyOwnerSlotOf(id), 0)

	// Phase 1 counts.
	total := PoGetObjectCount(sdb)
	if total > 0 {
		poWriteUint64(sdb, poKeyTotalCount(), total-1)
	}
	typeCount := PoGetTypeCount(sdb, obj.TypeTag)
	if typeCount > 0 {
		poWriteUint64(sdb, poKeyTypeCount(obj.TypeTag), typeCount-1)
	}
	ownerCountP1 := poReadUint64(sdb, poKeyOwnerCount(obj.Owner))
	if ownerCountP1 > 0 {
		poWriteUint64(sdb, poKeyOwnerCount(obj.Owner), ownerCountP1-1)
	}

	// Phase 4: mailbox-specific owner index tombstone + queue counters.
	// We do NOT iterate the queue to clear every message slot — see
	// mbResetCounters comment for the rationale (orphan slots are GC'd
	// by state expiry; the mailbox ID is unreachable post-destroy).
	mbOwnerIndexRemove(sdb, obj.Owner, id)
	mbResetCounters(sdb, id)

	return common.BigToHash(big.NewInt(1)).Bytes(), nil
}

// --- 0x04 getMailboxConfig (read-only) ----------------------------------

func (c *novaMailboxManager) getMailboxConfig(evm *EVM, input []byte) ([]byte, error) {
	if len(input) < 32 {
		return nil, errors.New("getMailboxConfig: input too short")
	}
	id := common.BytesToHash(input[0:32])
	obj := MbGetMailbox(evm.StateDB, id)
	if obj == nil {
		return nil, errors.New("getMailboxConfig: mailbox not found")
	}
	// Returning the raw config RLP gives Solidity callers a stable
	// shape they can decode either with abi.decode or via libraries.
	// The full Protocol Object body is queryable via the Phase 1
	// novaProtocolObjectRegistry (0x29) selector 0x02.
	return obj.StateData, nil
}
