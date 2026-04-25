// Ethernova: Content Reference Primitive Registry Precompile (NIP-0004 Phase 3)
//
// Address: 0x2B (novaContentRegistry)
//
// NOTE on address slot: NIP-0004 §3.4 originally drafted this precompile
// at 0x2A, but in this codebase 0x2A is already held by the Phase 2
// novaDeferredQueue. To preserve backward compatibility with Phase 2
// contracts (which have been live on devnet for multiple commits), the
// Content Registry takes the next free slot: 0x2B. The revised spec is
// in the Phase 3 document that ships with this commit. The chat-channel
// / 0x2A conflict described by NIP-0003 is therefore moot here — 0x2A
// is unambiguously the Deferred Queue.
//
// Function selectors (first byte of input):
//   0x01 createContentRef(contentHash:32, size:32, contentType:32, rentPrepay:32,
//                         expiryBlock:32, contentTypeBytes:bytes-padded?, proof:bytes)
//                                                                WRITE
//          Encoded input layout (fixed-width head + trailing tails):
//            [0x01]
//            [0..32]    contentHash            (32 bytes, keccak256 of content)
//            [32..64]   size                   (uint256, capped by params)
//            [64..96]   contentTypeLen         (uint256, <= MaxContentRefTypeBytes)
//            [96..128]  availabilityProofLen   (uint256, <= MaxContentRefAvailabilityProofBytes)
//            [128..160] rentPrepayWei          (uint256, >= MinRentPrepayWei)
//            [160..192] expiryBlock            (uint256, 0 = never)
//            [192..]    contentType bytes, then availability_proof bytes
//          Returns: 32-byte ContentRef ID.
//
//   0x02 getContentRef(id:32)                                     READ
//          Returns the RLP-encoded ProtocolObject body plus a trailing
//          byte (0x01 = valid, 0x00 = expired-by-rent-or-block).
//          Reverts if the object does not exist.
//
//   0x03 isValid(id:32)                                           READ
//          Returns a single 32-byte word: 0x01 if the ContentRef is
//          present, not past expiry_block, and has enough rent_balance
//          to cover at least one more epoch. 0x00 otherwise.
//
//   0x04 listContentRefsByOwner(owner:20 || padded32, offset:32, limit:32)  READ
//          Returns ABI-style: count(32) || id[0](32) || id[1](32) || ...
//          Limit is hard-capped at 100 per call.
//
//   0x05 getContentRefCount()                                     READ
//          Returns the global live (non-deleted) ContentRef count as a
//          32-byte uint256.
//
// Storage layout (all at ContentRegistryAddr = 0xFF03):
//   keccak256("cr_slots_used")                -> monotonic high-water mark (never decrements)
//   keccak256("cr_live_count")                -> live ContentRef count (decrements on delete)
//   keccak256("cr_rent_cursor")               -> next slot to process in the rent-epoch drain
//   keccak256("cr_slot", i)                   -> ContentRef ID at slot i (0 if tombstoned)
//   keccak256("cr_slot_of", id)               -> reverse slot index (for tombstone on delete)
//   keccak256("cr_owner_count", owner)        -> live count per owner
//   keccak256("cr_owner_slots_used", owner)   -> monotonic high-water per owner
//   keccak256("cr_owner_index", owner, i)     -> ID at per-owner slot i
//   keccak256("cr_owner_slot_of", id)         -> reverse per-owner slot
//
// The ContentRef BODY (content_hash, size, content_type, availability_proof,
// expiry_block, last_touched_block, rent_balance) is NOT stored here — it
// is written into the Phase 1 Protocol Object Registry at 0xFF01 via the
// existing PoGetObject / poWriteRLP helpers, tagged with ProtoTypeContentReference.
// This file only owns the ContentRef-specific INDICES and the rent engine.
//
// CONSENSUS-CRITICAL invariants:
//
//   1. ID generation is deterministic: keccak256(caller ++ blockNum_be8 ++
//      global_nonce_be8). The nonce is the shared Phase 1 nonce so that
//      ContentRef IDs never collide with any other Protocol Object ID.
//
//   2. Rent deduction runs ONLY at block boundaries where
//      block % RentEpochLength == 0 AND block > 0. Per-tx or per-read
//      deduction is forbidden (would depend on tx order, not block state).
//
//   3. Reads compute "effective" balance lazily: persisted balance minus
//      accrued-but-not-yet-persisted rent. This is a pure function of
//      (persisted_balance, size, last_touched_block, current_block,
//      rate, epoch_length), so all nodes produce identical answers.
//
//   4. Iteration over slots uses fixed-width big-endian encoding for the
//      slot index — cross-platform identical.
//
//   5. Expired ContentRefs are NOT auto-deleted. They are marked
//      effectively-dead by rent_balance == 0. A separate future phase
//      can introduce a sweep; for Phase 3 expired objects stay in the
//      trie but are filtered out by isValid() and return status=0x00
//      from getContentRef().

package vm

import (
	"encoding/binary"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params/ethernova"
	"github.com/ethereum/go-ethereum/rlp"
)

// Gas costs for 0x2B. Matches the shape of 0x29 / 0x2A pricing: writes are
// ~10x reads, create includes a per-payload-chunk adder because we RLP-
// encode the object and stripe it across 32-byte storage slots.
const (
	contentRegistryGasCreateBase   uint64 = 10000
	contentRegistryGasCreatePerChk uint64 = 200 // per 32-byte RLP chunk
	contentRegistryGasRead         uint64 = 2000
	contentRegistryGasIsValid      uint64 = 1000
	contentRegistryGasList         uint64 = 2000
	contentRegistryGasCount        uint64 = 500
)

// ContentRegistryAddr is the system address where ContentRef indices live.
// Distinct from 0xFF01 (Protocol Object Registry) and 0xFF02 (Deferred
// Queue) — each subsystem owns its own reserved account so a corruption
// in one cannot cross-contaminate another.
var ContentRegistryAddr = common.HexToAddress("0x000000000000000000000000000000000000FF03")

// --- Storage key builders (deterministic, fixed-width) ---

func crKeySlotsUsed() common.Hash  { return crypto.Keccak256Hash([]byte("cr_slots_used")) }
func crKeyLiveCount() common.Hash  { return crypto.Keccak256Hash([]byte("cr_live_count")) }
func crKeyRentCursor() common.Hash { return crypto.Keccak256Hash([]byte("cr_rent_cursor")) }
func crKeyLastEpochDone() common.Hash {
	return crypto.Keccak256Hash([]byte("cr_last_epoch_block"))
}

func crKeySlot(i uint64) common.Hash {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], i)
	return crypto.Keccak256Hash([]byte("cr_slot"), buf[:])
}
func crKeySlotOf(id common.Hash) common.Hash {
	return crypto.Keccak256Hash([]byte("cr_slot_of"), id.Bytes())
}
func crKeyOwnerCount(owner common.Address) common.Hash {
	return crypto.Keccak256Hash([]byte("cr_owner_count"), owner.Bytes())
}
func crKeyOwnerSlotsUsed(owner common.Address) common.Hash {
	return crypto.Keccak256Hash([]byte("cr_owner_slots_used"), owner.Bytes())
}
func crKeyOwnerIndex(owner common.Address, i uint64) common.Hash {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], i)
	return crypto.Keccak256Hash([]byte("cr_owner_index"), owner.Bytes(), buf[:])
}
func crKeyOwnerSlotOf(id common.Hash) common.Hash {
	return crypto.Keccak256Hash([]byte("cr_owner_slot_of"), id.Bytes())
}

// --- uint64 storage helpers (mirror the 0x29 pattern) ---

func crReadUint64(sdb StateDB, key common.Hash) uint64 {
	val := sdb.GetState(ContentRegistryAddr, key)
	return new(big.Int).SetBytes(val.Bytes()).Uint64()
}

func crWriteUint64(sdb StateDB, key common.Hash, v uint64) {
	sdb.SetState(ContentRegistryAddr, key, common.BigToHash(new(big.Int).SetUint64(v)))
}

// crEnsureRegistryExists is the ContentRef analog of poEnsureRegistryExists.
// Same EIP-161 rationale: a storage-only account would be wiped by
// Finalise(true) unless we mark it non-empty via nonce=1.
func crEnsureRegistryExists(sdb StateDB) {
	if !sdb.Exist(ContentRegistryAddr) {
		sdb.CreateAccount(ContentRegistryAddr)
	}
	if sdb.GetNonce(ContentRegistryAddr) == 0 {
		sdb.SetNonce(ContentRegistryAddr, 1)
	}
}

// --- ContentRef state_data (RLP body inside the ProtocolObject) ---

// ContentRefStateData is what goes into ProtocolObject.StateData for a
// type_tag = ProtoTypeContentReference object. Encoded as RLP list
// [contentHash, size, contentType, availabilityProof]. Fields are
// position-dependent — do NOT reorder without a fork.
type ContentRefStateData struct {
	ContentHash       common.Hash
	Size              uint64
	ContentType       []byte
	AvailabilityProof []byte
}

// EncodeContentRefStateData serializes a ContentRefStateData to RLP.
// Uses the rlp package the same way types/protocol_object.go does so
// encoding stays bit-identical cross-platform.
func EncodeContentRefStateData(d *ContentRefStateData) ([]byte, error) {
	return rlp.EncodeToBytes([]interface{}{
		d.ContentHash,
		d.Size,
		d.ContentType,
		d.AvailabilityProof,
	})
}

// DecodeContentRefStateData parses a StateData byte blob into a
// ContentRefStateData. Returns an error on malformed RLP.
func DecodeContentRefStateData(data []byte) (*ContentRefStateData, error) {
	var raw struct {
		ContentHash       common.Hash
		Size              uint64
		ContentType       []byte
		AvailabilityProof []byte
	}
	if err := rlp.DecodeBytes(data, &raw); err != nil {
		return nil, err
	}
	return &ContentRefStateData{
		ContentHash:       raw.ContentHash,
		Size:              raw.Size,
		ContentType:       raw.ContentType,
		AvailabilityProof: raw.AvailabilityProof,
	}, nil
}

// ----- Read-path helpers exposed for the RPC layer -----

// CrEffectiveRentBalance returns the effective (lazy) rent balance for a
// ContentRef at the given block. It subtracts rent that has accrued since
// last_touched_block but has not yet been persisted by Finalize(). This
// is the balance that determines isValid(). If the accrued rent exceeds
// the persisted balance, returns 0 (the object is effectively expired).
func CrEffectiveRentBalance(obj *types.ProtocolObject, currentBlock uint64) *big.Int {
	if obj == nil || obj.RentBalance == nil {
		return new(big.Int)
	}
	d, err := DecodeContentRefStateData(obj.StateData)
	if err != nil {
		return new(big.Int)
	}
	accrued := state.ComputeAccruedRentWei(
		ethernova.RentRatePerBytePerBlock,
		d.Size,
		ethernova.RentEpochLength,
		obj.LastTouchedBlock,
		currentBlock,
	)
	if accrued.Cmp(obj.RentBalance) >= 0 {
		return new(big.Int)
	}
	return new(big.Int).Sub(obj.RentBalance, accrued)
}

// CrIsValid returns true if the ContentRef exists, is not past its
// expiry_block, and has at least one epoch of rent left (effectively).
func CrIsValid(sdb StateDB, id common.Hash, currentBlock uint64) bool {
	obj := PoGetObject(sdb, id)
	if obj == nil || obj.TypeTag != types.ProtoTypeContentReference {
		return false
	}
	if obj.ExpiryBlock != 0 && currentBlock > obj.ExpiryBlock {
		return false
	}
	d, err := DecodeContentRefStateData(obj.StateData)
	if err != nil {
		return false
	}
	eff := CrEffectiveRentBalance(obj, currentBlock)
	need := state.ComputeEpochRentWei(
		ethernova.RentRatePerBytePerBlock,
		d.Size,
		ethernova.RentEpochLength,
	)
	return eff.Cmp(need) >= 0
}

// CrGetContentRef fetches the ProtocolObject for a ContentRef ID, or nil
// if absent or wrong type. Does NOT check expiry — caller decides.
func CrGetContentRef(sdb StateDB, id common.Hash) *types.ProtocolObject {
	obj := PoGetObject(sdb, id)
	if obj == nil || obj.TypeTag != types.ProtoTypeContentReference {
		return nil
	}
	return obj
}

// CrGetLiveCount returns the number of live (non-deleted) ContentRefs.
func CrGetLiveCount(sdb StateDB) uint64 {
	return crReadUint64(sdb, crKeyLiveCount())
}

// CrGetSlotsUsed returns the monotonic slot high-water mark.
func CrGetSlotsUsed(sdb StateDB) uint64 {
	return crReadUint64(sdb, crKeySlotsUsed())
}

// CrListByOwner returns ContentRef IDs for a given owner, paginated by
// (offset, limit) over LIVE (non-tombstoned) entries only.
func CrListByOwner(sdb StateDB, owner common.Address, offset, limit uint64) []common.Hash {
	if limit == 0 {
		return nil
	}
	slotsUsed := crReadUint64(sdb, crKeyOwnerSlotsUsed(owner))
	if slotsUsed == 0 {
		return nil
	}
	var ids []common.Hash
	skipped := uint64(0)
	for slot := uint64(0); slot < slotsUsed; slot++ {
		val := sdb.GetState(ContentRegistryAddr, crKeyOwnerIndex(owner, slot))
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

// =================================================================
// Precompile struct
// =================================================================

type novaContentRegistry struct{}

func (c *novaContentRegistry) RequiredGas(input []byte) uint64 {
	if len(input) < 1 {
		return 0
	}
	switch input[0] {
	case 0x01:
		// Create: base cost + per-chunk cost of the declared payload.
		// We price based on input length as a conservative proxy; the
		// actual RLP-encoded ProtocolObject is slightly larger than the
		// input payload but within the same order of magnitude.
		chunks := (uint64(len(input)) + 31) / 32
		return contentRegistryGasCreateBase + chunks*contentRegistryGasCreatePerChk
	case 0x02:
		return contentRegistryGasRead
	case 0x03:
		return contentRegistryGasIsValid
	case 0x04:
		return contentRegistryGasList
	case 0x05:
		return contentRegistryGasCount
	default:
		return 0
	}
}

func (c *novaContentRegistry) Run(input []byte) ([]byte, error) {
	return nil, errors.New("novaContentRegistry: requires stateful execution")
}

func (c *novaContentRegistry) RunStateful(evm *EVM, caller common.Address, input []byte, readOnly bool) ([]byte, error) {
	if len(input) < 1 {
		return nil, errors.New("novaContentRegistry: empty input")
	}
	// Fork gate: before ContentRefForkBlock, all selectors revert. The
	// precompile itself is registered unconditionally (same as 0x29/0x2A)
	// but its semantics are gated so there is no incidental state touch
	// on early blocks.
	if evm.Context.BlockNumber.Uint64() < ethernova.ContentRefForkBlock {
		return nil, errors.New("novaContentRegistry: not yet active")
	}
	switch input[0] {
	case 0x01:
		if readOnly {
			return nil, ErrWriteProtection
		}
		return c.createContentRef(evm, caller, input[1:])
	case 0x02:
		return c.getContentRef(evm, input[1:])
	case 0x03:
		return c.isValid(evm, input[1:])
	case 0x04:
		return c.listByOwner(evm, input[1:])
	case 0x05:
		return c.getCount(evm)
	default:
		return nil, errors.New("novaContentRegistry: unknown selector")
	}
}

// createContentRef writes a new ContentRef object into the Protocol Object
// Registry and registers its ID in the ContentRef indices.
func (c *novaContentRegistry) createContentRef(evm *EVM, caller common.Address, input []byte) ([]byte, error) {
	if len(input) < 192 {
		return nil, errors.New("createContentRef: input too short (need 192 bytes head)")
	}
	contentHash := common.BytesToHash(input[0:32])
	size := new(big.Int).SetBytes(input[32:64]).Uint64()
	contentTypeLen := new(big.Int).SetBytes(input[64:96]).Uint64()
	proofLen := new(big.Int).SetBytes(input[96:128]).Uint64()
	rentPrepay := new(big.Int).SetBytes(input[128:160])
	expiryBlock := new(big.Int).SetBytes(input[160:192]).Uint64()

	// Validate caps before touching state.
	if size > ethernova.MaxContentRefSize {
		return nil, errors.New("createContentRef: size exceeds MaxContentRefSize")
	}
	if contentTypeLen > ethernova.MaxContentRefTypeBytes {
		return nil, errors.New("createContentRef: contentType too long")
	}
	if proofLen > ethernova.MaxContentRefAvailabilityProofBytes {
		return nil, errors.New("createContentRef: availabilityProof too long")
	}
	if rentPrepay.Sign() < 0 || rentPrepay.Cmp(new(big.Int).SetUint64(ethernova.MinRentPrepayWei)) < 0 {
		return nil, errors.New("createContentRef: rentPrepay below MinRentPrepayWei")
	}

	// Tail parse: content_type bytes, then availability_proof bytes.
	needTail := contentTypeLen + proofLen
	if uint64(len(input))-192 < needTail {
		return nil, errors.New("createContentRef: tail shorter than declared lengths")
	}
	contentType := make([]byte, contentTypeLen)
	copy(contentType, input[192:192+contentTypeLen])
	availabilityProof := make([]byte, proofLen)
	copy(availabilityProof, input[192+contentTypeLen:192+contentTypeLen+proofLen])

	stateData, err := EncodeContentRefStateData(&ContentRefStateData{
		ContentHash:       contentHash,
		Size:              size,
		ContentType:       contentType,
		AvailabilityProof: availabilityProof,
	})
	if err != nil {
		return nil, err
	}

	sdb := evm.StateDB
	blockNum := evm.Context.BlockNumber.Uint64()

	// Generate deterministic ID using the SAME global nonce as
	// novaProtocolObjectRegistry (0x29). Sharing the nonce guarantees
	// zero ID collision across Protocol Object types.
	poEnsureRegistryExists(sdb)
	crEnsureRegistryExists(sdb)

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

	// Build the ProtocolObject and write it into the Phase 1 registry.
	obj := &types.ProtocolObject{
		ID:               id,
		Owner:            caller,
		TypeTag:          types.ProtoTypeContentReference,
		StateData:        stateData,
		ExpiryBlock:      expiryBlock,
		LastTouchedBlock: blockNum,
		RentBalance:      new(big.Int).Set(rentPrepay),
	}
	objData, err := obj.EncodeRLP()
	if err != nil {
		return nil, err
	}

	// Presence marker + RLP body (Phase 1 registry storage at 0xFF01)
	sdb.SetState(ProtocolObjectRegistryAddr, poKeyObject(id), common.BytesToHash([]byte{0x01}))
	poWriteRLP(sdb, id, objData)

	// Global Phase 1 counts
	totalCount := PoGetObjectCount(sdb)
	poWriteUint64(sdb, poKeyTotalCount(), totalCount+1)
	typeCount := PoGetTypeCount(sdb, types.ProtoTypeContentReference)
	poWriteUint64(sdb, poKeyTypeCount(types.ProtoTypeContentReference), typeCount+1)

	// Phase 1 owner index (so existing nova_getProtocolObjectsByOwner works too)
	slotsUsedP1 := poReadUint64(sdb, poKeyOwnerSlotsUsed(caller))
	sdb.SetState(ProtocolObjectRegistryAddr, poKeyOwnerIndex(caller, slotsUsedP1), id)
	poWriteUint64(sdb, poKeyOwnerSlotOf(id), slotsUsedP1)
	poWriteUint64(sdb, poKeyOwnerSlotsUsed(caller), slotsUsedP1+1)
	ownerCountP1 := poReadUint64(sdb, poKeyOwnerCount(caller))
	poWriteUint64(sdb, poKeyOwnerCount(caller), ownerCountP1+1)

	// ContentRef-specific indices (0xFF03)
	crSlotsUsed := crReadUint64(sdb, crKeySlotsUsed())
	sdb.SetState(ContentRegistryAddr, crKeySlot(crSlotsUsed), id)
	crWriteUint64(sdb, crKeySlotOf(id), crSlotsUsed)
	crWriteUint64(sdb, crKeySlotsUsed(), crSlotsUsed+1)
	crLive := crReadUint64(sdb, crKeyLiveCount())
	crWriteUint64(sdb, crKeyLiveCount(), crLive+1)

	// ContentRef per-owner index (separate from Phase 1 owner index —
	// this one lets RPC filter by type without scanning all types)
	ownerSlotsUsed := crReadUint64(sdb, crKeyOwnerSlotsUsed(caller))
	sdb.SetState(ContentRegistryAddr, crKeyOwnerIndex(caller, ownerSlotsUsed), id)
	crWriteUint64(sdb, crKeyOwnerSlotOf(id), ownerSlotsUsed)
	crWriteUint64(sdb, crKeyOwnerSlotsUsed(caller), ownerSlotsUsed+1)
	ownerCount := crReadUint64(sdb, crKeyOwnerCount(caller))
	crWriteUint64(sdb, crKeyOwnerCount(caller), ownerCount+1)

	return id.Bytes(), nil
}

// getContentRef returns the RLP-encoded ProtocolObject body followed by a
// single validity flag byte (0x01 valid, 0x00 expired).
func (c *novaContentRegistry) getContentRef(evm *EVM, input []byte) ([]byte, error) {
	if len(input) < 32 {
		return nil, errors.New("getContentRef: input too short")
	}
	id := common.BytesToHash(input[0:32])
	obj := CrGetContentRef(evm.StateDB, id)
	if obj == nil {
		return nil, errors.New("getContentRef: not found")
	}
	data, err := obj.EncodeRLP()
	if err != nil {
		return nil, err
	}
	valid := byte(0x00)
	if CrIsValid(evm.StateDB, id, evm.Context.BlockNumber.Uint64()) {
		valid = 0x01
	}
	out := make([]byte, len(data)+1)
	copy(out, data)
	out[len(data)] = valid
	return out, nil
}

// isValid returns 0x01 in the low byte of a 32-byte word if the ContentRef
// is live and has enough rent; 0x00 otherwise.
func (c *novaContentRegistry) isValid(evm *EVM, input []byte) ([]byte, error) {
	if len(input) < 32 {
		return nil, errors.New("isValid: input too short")
	}
	id := common.BytesToHash(input[0:32])
	out := make([]byte, 32)
	if CrIsValid(evm.StateDB, id, evm.Context.BlockNumber.Uint64()) {
		out[31] = 0x01
	}
	return out, nil
}

// listByOwner returns ABI-style count||ids packed into a single byte slice.
// Takes 84 bytes of input: owner(20 left-padded to 32) || offset(32) || limit(32).
// Accepts 20-byte owner OR 32-byte owner (takes low 20) to be permissive.
func (c *novaContentRegistry) listByOwner(evm *EVM, input []byte) ([]byte, error) {
	if len(input) < 84 {
		return nil, errors.New("listByOwner: input too short")
	}
	// Accept either (20||32||32) = 84 OR (32||32||32) = 96.
	// The first form is "raw" address padding; the second is ABI-encoded.
	var owner common.Address
	var offset, limit uint64
	if len(input) >= 96 && (input[0] == 0 && input[1] == 0) {
		// Looks like left-padded 32-byte address
		owner = common.BytesToAddress(input[12:32])
		offset = new(big.Int).SetBytes(input[32:64]).Uint64()
		limit = new(big.Int).SetBytes(input[64:96]).Uint64()
	} else {
		owner = common.BytesToAddress(input[0:20])
		offset = new(big.Int).SetBytes(input[20:52]).Uint64()
		limit = new(big.Int).SetBytes(input[52:84]).Uint64()
	}
	if limit == 0 || limit > 100 {
		limit = 100
	}
	ids := CrListByOwner(evm.StateDB, owner, offset, limit)
	out := make([]byte, 32+32*len(ids))
	copy(out[:32], common.BigToHash(new(big.Int).SetUint64(uint64(len(ids)))).Bytes())
	for i, id := range ids {
		copy(out[32+i*32:32+(i+1)*32], id.Bytes())
	}
	return out, nil
}

// getCount returns the global live ContentRef count.
func (c *novaContentRegistry) getCount(evm *EVM) ([]byte, error) {
	n := CrGetLiveCount(evm.StateDB)
	return common.BigToHash(new(big.Int).SetUint64(n)).Bytes(), nil
}

// =================================================================
// Rent engine — called from consensus Finalize() at epoch boundaries
// =================================================================

// CrProcessRentEpoch deducts one epoch of rent from every live ContentRef
// at an epoch boundary. Safe to call on every block — it is a no-op if
// blockNum is not a boundary or the fork is not yet active.
//
// CONSENSUS-CRITICAL: both validator (consensus.Finalize) and miner
// (consensus.FinalizeAndAssemble) MUST call this with identical
// arguments. The single entry point and the deterministic-iteration
// design below guarantee bit-identical state mutations on both paths.
//
// Work per call is bounded by params.MaxContentRefsPerRentEpoch. If the
// live population exceeds that, the remaining objects are processed by
// subsequent epoch-boundary blocks via the cr_rent_cursor slot.
func CrProcessRentEpoch(sdb StateDB, blockNum uint64) {
	if blockNum < ethernova.ContentRefForkBlock {
		return
	}
	if !state.IsRentEpochBoundary(blockNum, ethernova.RentEpochLength) {
		return
	}

	// Guard against double-processing: if we already ran at this block,
	// skip. This guards against e.g. consensus engines that call both
	// Finalize and FinalizeAndAssemble on the same header (the reward
	// path in consensus.go shows this pattern for block reward, which
	// uses a different primitive).
	lastEpoch := crReadUint64(sdb, crKeyLastEpochDone())
	if lastEpoch == blockNum {
		return
	}

	slotsUsed := crReadUint64(sdb, crKeySlotsUsed())
	if slotsUsed == 0 {
		// Empty-state fast path: the function is a no-op over no objects,
		// and writing cr_last_epoch_block here would be wiped by
		// Finalise(true) anyway (the ContentRegistry account doesn't
		// exist yet so it's still empty per EIP-161). Returning outright
		// is both cheaper and correct — the next epoch boundary will
		// re-enter this function and likewise be a no-op until the
		// first ContentRef is created.
		return
	}

	// Ensure the registry account sticks through Finalise(true) — required
	// BEFORE any state write to 0xFF03, otherwise the write would be
	// orphaned along with the still-empty account.
	crEnsureRegistryExists(sdb)

	cursor := crReadUint64(sdb, crKeyRentCursor())
	processed := uint64(0)
	max := ethernova.MaxContentRefsPerRentEpoch

	for cursor < slotsUsed && processed < max {
		idHash := sdb.GetState(ContentRegistryAddr, crKeySlot(cursor))
		cursor++
		if idHash == (common.Hash{}) {
			// Tombstoned slot — skip without counting against the cap,
			// tombstones are cheap pure reads.
			continue
		}
		id := idHash

		obj := PoGetObject(sdb, id)
		if obj == nil || obj.TypeTag != types.ProtoTypeContentReference {
			// Object gone / reclassified — drop the slot.
			continue
		}

		d, err := DecodeContentRefStateData(obj.StateData)
		if err != nil {
			// Corrupt state_data — skip. An explicit metric could count
			// these; for now silent-skip prevents a single bad object
			// from breaking consensus.
			processed++
			continue
		}

		// Charge one epoch of rent.
		per := state.ComputeEpochRentWei(
			ethernova.RentRatePerBytePerBlock,
			d.Size,
			ethernova.RentEpochLength,
		)

		var newBal *big.Int
		if obj.RentBalance == nil || obj.RentBalance.Cmp(per) < 0 {
			// Under-funded: rent fully absorbs the remaining balance
			// and the object becomes expired-by-rent. isValid() now
			// returns false because effective balance < next epoch.
			newBal = new(big.Int)
		} else {
			newBal = new(big.Int).Sub(obj.RentBalance, per)
		}

		// Hard expiry: if this block is past expiry_block, force balance
		// to zero so that no future re-use is possible. Block-expiry and
		// rent-expiry converge to the same dead state.
		if obj.ExpiryBlock != 0 && blockNum > obj.ExpiryBlock {
			newBal = new(big.Int)
		}

		obj.RentBalance = newBal
		obj.LastTouchedBlock = blockNum

		// Re-encode and overwrite the object.
		newData, err := obj.EncodeRLP()
		if err != nil {
			processed++
			continue
		}
		poWriteRLP(sdb, id, newData)

		processed++
	}

	if cursor >= slotsUsed {
		// Wrapped around — the next epoch starts from slot 0 again.
		crWriteUint64(sdb, crKeyRentCursor(), 0)
	} else {
		crWriteUint64(sdb, crKeyRentCursor(), cursor)
	}
	crWriteUint64(sdb, crKeyLastEpochDone(), blockNum)
}
