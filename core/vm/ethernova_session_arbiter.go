// Ethernova: Session Arbiter Precompile (NIP-0004 Phase 7)
//
// Address: 0x2D (novaSessionArbiter)
//
// The session arbiter anchors bilateral off-chain state channels. It stores a
// Session Protocol Object (type_tag = ProtoTypeSession) in the Phase 1 registry
// and exposes deterministic checkpoint, close, dispute, and timeout resolution.

package vm

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params/ethernova"
)

const (
	sessionGasOpen          uint64 = 45000
	sessionGasCheckpoint    uint64 = 60000
	sessionGasClose         uint64 = 60000
	sessionGasDispute       uint64 = 70000
	sessionGasGet           uint64 = 2500
	sessionGasResolve       uint64 = 30000
	sessionGasPerSignature  uint64 = 8000
	sessionSignatureLen     uint64 = 65
	sessionOpenInputWords          = 7
	sessionSignedInputWords        = 4
)

// encodeRevertReason returns ABI-encoded Error(string) revert data so Solidity,
// ethers.js, viem, and JSON-RPC eth_call can surface human-readable reasons.
func encodeRevertReason(msg string) []byte {
	const selector = "08c379a0"
	sel, _ := hex.DecodeString(selector)
	msgBytes := []byte(msg)
	msgLen := len(msgBytes)
	paddedLen := (msgLen + 31) / 32 * 32
	out := make([]byte, 0, 4+32+32+paddedLen)
	out = append(out, sel...)
	out = append(out, common.BigToHash(big.NewInt(0x20)).Bytes()...)
	out = append(out, common.BigToHash(big.NewInt(int64(msgLen))).Bytes()...)
	padded := make([]byte, paddedLen)
	copy(padded, msgBytes)
	out = append(out, padded...)
	return out
}

func sessRevert(format string, a ...interface{}) ([]byte, error) {
	return encodeRevertReason(fmt.Sprintf(format, a...)), ErrExecutionReverted
}

// SessionArbiterAddr is the Phase 7 system address for session indexes.
var SessionArbiterAddr = common.HexToAddress("0x000000000000000000000000000000000000FF05")

type novaSessionArbiter struct{}

var _ StatefulPrecompiledContract = (*novaSessionArbiter)(nil)

func (c *novaSessionArbiter) RequiredGas(input []byte) uint64 {
	if len(input) < 1 {
		return 0
	}
	sigGas := func(headBytes int) uint64 {
		if len(input) <= headBytes {
			return 0
		}
		return (uint64(len(input)-headBytes) / sessionSignatureLen) * sessionGasPerSignature
	}
	switch input[0] {
	case 0x01:
		return sessionGasOpen
	case 0x02:
		return sessionGasCheckpoint + sigGas(1+sessionSignedInputWords*32)
	case 0x03:
		return sessionGasClose + sigGas(1+sessionSignedInputWords*32)
	case 0x04:
		return sessionGasDispute + sigGas(1+sessionSignedInputWords*32)
	case 0x05:
		return sessionGasGet
	case 0x06:
		return sessionGasResolve
	default:
		return 0
	}
}

func (c *novaSessionArbiter) Run(input []byte) ([]byte, error) {
	return sessRevert("novaSessionArbiter: requires stateful execution")
}

func (c *novaSessionArbiter) RunStateful(evm *EVM, caller common.Address, input []byte, readOnly bool) ([]byte, error) {
	if len(input) < 1 {
		return sessRevert("novaSessionArbiter: empty input")
	}
	if evm.Context.BlockNumber.Uint64() < ethernova.SessionForkBlock {
		return sessRevert("novaSessionArbiter: not yet active")
	}
	switch input[0] {
	case 0x01: // openSession(counterparty, type, timeoutBlocks, disputeRules, rentPrepay)
		if readOnly {
			return nil, ErrWriteProtection
		}
		if err := requireSessionWriteDomain(evm, caller); err != nil {
			return nil, err
		}
		return c.openSession(evm, caller, input[1:])
	case 0x02: // commitState(sessionID, sequence, stateHash, sigCount, sigs...)
		if readOnly {
			return nil, ErrWriteProtection
		}
		if err := requireSessionWriteDomain(evm, caller); err != nil {
			return nil, err
		}
		return c.commitState(evm, input[1:])
	case 0x03: // closeSession(sessionID, sequence, finalStateHash, sigCount, sigs...)
		if readOnly {
			return nil, ErrWriteProtection
		}
		if err := requireSessionWriteDomain(evm, caller); err != nil {
			return nil, err
		}
		return c.closeSession(evm, input[1:])
	case 0x04: // disputeSession(sessionID, sequence, stateHash, sigCount, sigs...)
		if readOnly {
			return nil, ErrWriteProtection
		}
		if err := requireSessionWriteDomain(evm, caller); err != nil {
			return nil, err
		}
		return c.disputeSession(evm, input[1:])
	case 0x05: // getSession(sessionID)
		return c.getSession(evm, input[1:])
	case 0x06: // resolveTimeout(sessionID)
		if readOnly {
			return nil, ErrWriteProtection
		}
		if err := requireSessionWriteDomain(evm, caller); err != nil {
			return nil, err
		}
		return c.resolveTimeout(evm, input[1:])
	default:
		return sessRevert("novaSessionArbiter: unknown selector")
	}
}

func requireSessionWriteDomain(evm *EVM, caller common.Address) error {
	domain := evm.currentExecutionDomain(caller)
	if domain == DomainChannel {
		return nil
	}
	if len(evm.executionFrames) == 0 && len(evm.StateDB.GetCode(caller)) == 0 {
		// Direct EOA/RPC calls remain available for devnet tooling and tests.
		return nil
	}
	return ErrExecutionReverted
}

func (c *novaSessionArbiter) openSession(evm *EVM, caller common.Address, input []byte) ([]byte, error) {
	const headLen = sessionOpenInputWords * 32
	if len(input) < headLen {
		return sessRevert("openSession: input too short (need %d, got %d)", headLen, len(input))
	}
	counterparty := common.BytesToAddress(input[0:32])
	if counterparty == (common.Address{}) || counterparty == caller {
		return sessRevert("openSession: invalid counterparty")
	}
	sessionType, err := parseSessionUint8Word(input[32:64], "sessionType")
	if err != nil {
		return sessRevert("openSession: %v", err)
	}
	if !types.IsValidSessionType(sessionType) {
		return sessRevert("openSession: invalid sessionType 0x%02x", sessionType)
	}
	timeoutBlocks, err := parseSessionUint64Word(input[64:96], "timeoutBlocks")
	if err != nil {
		return sessRevert("openSession: %v", err)
	}
	if timeoutBlocks < ethernova.SessionMinTimeoutBlocks || timeoutBlocks > ethernova.SessionMaxTimeoutBlocks {
		return sessRevert("openSession: timeoutBlocks outside range (%d)", timeoutBlocks)
	}
	disputeRules := common.BytesToHash(input[96:128])
	rentPrepay := new(big.Int).SetBytes(input[128:160])

	initiatorSigner := common.BytesToAddress(input[160:192])
	counterpartySigner := common.BytesToAddress(input[192:224])
	if initiatorSigner == (common.Address{}) {
		initiatorSigner = caller
	}
	if counterpartySigner == (common.Address{}) {
		counterpartySigner = counterparty
	}
	if initiatorSigner == counterpartySigner {
		return sessRevert("openSession: initiator and counterparty signers must differ")
	}

	sdb := evm.StateDB
	blockNum := evm.Context.BlockNumber.Uint64()
	timeoutBlock := blockNum + timeoutBlocks
	if timeoutBlock < blockNum {
		return sessRevert("openSession: timeoutBlock overflow")
	}

	poEnsureRegistryExists(sdb)
	sessEnsureExists(sdb)

	globalNonce := poReadUint64(sdb, poKeyGlobalNonce())
	var blockBuf, nonceBuf [8]byte
	binary.BigEndian.PutUint64(blockBuf[:], blockNum)
	binary.BigEndian.PutUint64(nonceBuf[:], globalNonce)
	idInput := make([]byte, 0, 20+20+8+8)
	idInput = append(idInput, caller.Bytes()...)
	idInput = append(idInput, counterparty.Bytes()...)
	idInput = append(idInput, blockBuf[:]...)
	idInput = append(idInput, nonceBuf[:]...)
	id := crypto.Keccak256Hash(idInput)
	poWriteUint64(sdb, poKeyGlobalNonce(), globalNonce+1)

	state := &types.SessionState{
		Initiator:          caller,
		Counterparty:       counterparty,
		SessionType:        sessionType,
		Status:             types.SessionStatusOpen,
		TimeoutBlock:       timeoutBlock,
		DisputeRules:       disputeRules,
		OpenedBlock:        blockNum,
		SequenceNumber:     0,
		InitiatorSigner:    initiatorSigner,
		CounterpartySigner: counterpartySigner,
	}
	stateData, err := state.EncodeRLP()
	if err != nil {
		return sessRevert("openSession: encode state: %v", err)
	}
	if uint64(len(stateData)) > ethernova.MaxSessionStateBytes {
		return sessRevert("openSession: state data exceeds cap (%d > %d)", len(stateData), ethernova.MaxSessionStateBytes)
	}
	obj := &types.ProtocolObject{
		ID:               id,
		Owner:            caller,
		TypeTag:          types.ProtoTypeSession,
		StateData:        stateData,
		ExpiryBlock:      timeoutBlock,
		LastTouchedBlock: blockNum,
		RentBalance:      rentPrepay,
	}
	objData, err := obj.EncodeRLP()
	if err != nil {
		return sessRevert("openSession: encode object: %v", err)
	}

	sdb.SetState(ProtocolObjectRegistryAddr, poKeyObject(id), common.BytesToHash([]byte{0x01}))
	poWriteRLP(sdb, id, objData)
	poWriteUint64(sdb, poKeyTotalCount(), PoGetObjectCount(sdb)+1)
	poWriteUint64(sdb, poKeyTypeCount(types.ProtoTypeSession), PoGetTypeCount(sdb, types.ProtoTypeSession)+1)

	slotsUsedP1 := poReadUint64(sdb, poKeyOwnerSlotsUsed(caller))
	sdb.SetState(ProtocolObjectRegistryAddr, poKeyOwnerIndex(caller, slotsUsedP1), id)
	poWriteUint64(sdb, poKeyOwnerSlotOf(id), slotsUsedP1)
	poWriteUint64(sdb, poKeyOwnerSlotsUsed(caller), slotsUsedP1+1)
	poWriteUint64(sdb, poKeyOwnerCount(caller), poReadUint64(sdb, poKeyOwnerCount(caller))+1)

	sessOwnerIndexAdd(sdb, caller, id)
	sessParticipantIndexAdd(sdb, counterparty, id)
	sessDueIndexAdd(sdb, timeoutBlock, id)
	sessWriteUint64(sdb, sessKeyLiveCount(), sessReadUint64(sdb, sessKeyLiveCount())+1)

	return id.Bytes(), nil
}

func (c *novaSessionArbiter) commitState(evm *EVM, input []byte) ([]byte, error) {
	id, seq, stateHash, sigs, err := parseSessionSignedInput(input, "commitState")
	if err != nil {
		return encodeRevertReason(err.Error()), ErrExecutionReverted
	}
	obj, st, err := sessGetObjectAndState(evm.StateDB, id)
	if err != nil {
		return sessRevert("commitState: %v", err)
	}
	if !types.IsLiveSessionStatus(st.Status) {
		return sessRevert("commitState: session is not live")
	}
	if seq <= st.SequenceNumber {
		return sessRevert("commitState: sequence must increase (have %d, got %d)", st.SequenceNumber, seq)
	}
	if err := verifySessionSignatures(st, id, seq, stateHash, sigs); err != nil {
		return sessRevert("commitState: %v", err)
	}
	blockNum := evm.Context.BlockNumber.Uint64()
	st.StateHash = stateHash
	st.SequenceNumber = seq
	if err := sessWriteSessionState(evm.StateDB, obj, st, blockNum); err != nil {
		return sessRevert("commitState: %v", err)
	}
	return common.BigToHash(big.NewInt(1)).Bytes(), nil
}

func (c *novaSessionArbiter) closeSession(evm *EVM, input []byte) ([]byte, error) {
	id, seq, stateHash, sigs, err := parseSessionSignedInput(input, "closeSession")
	if err != nil {
		return encodeRevertReason(err.Error()), ErrExecutionReverted
	}
	obj, st, err := sessGetObjectAndState(evm.StateDB, id)
	if err != nil {
		return sessRevert("closeSession: %v", err)
	}
	if !types.IsLiveSessionStatus(st.Status) {
		return sessRevert("closeSession: session is not live")
	}
	if seq < st.SequenceNumber {
		return sessRevert("closeSession: sequence regresses")
	}
	if err := verifySessionSignatures(st, id, seq, stateHash, sigs); err != nil {
		return sessRevert("closeSession: %v", err)
	}
	blockNum := evm.Context.BlockNumber.Uint64()
	st.StateHash = stateHash
	st.SequenceNumber = seq
	st.Status = types.SessionStatusClosed
	st.ClosedBlock = blockNum
	if err := sessWriteSessionState(evm.StateDB, obj, st, blockNum); err != nil {
		return sessRevert("closeSession: %v", err)
	}
	sessDecrementLive(evm.StateDB)
	return common.BigToHash(big.NewInt(1)).Bytes(), nil
}

func (c *novaSessionArbiter) disputeSession(evm *EVM, input []byte) ([]byte, error) {
	id, seq, stateHash, sigs, err := parseSessionSignedInput(input, "disputeSession")
	if err != nil {
		return encodeRevertReason(err.Error()), ErrExecutionReverted
	}
	obj, st, err := sessGetObjectAndState(evm.StateDB, id)
	if err != nil {
		return sessRevert("disputeSession: %v", err)
	}
	if !types.IsLiveSessionStatus(st.Status) {
		return sessRevert("disputeSession: session is not live")
	}
	if seq <= st.SequenceNumber {
		return sessRevert("disputeSession: sequence must beat current")
	}
	if err := verifySessionSignatures(st, id, seq, stateHash, sigs); err != nil {
		return sessRevert("disputeSession: %v", err)
	}
	blockNum := evm.Context.BlockNumber.Uint64()
	deadline := blockNum + ethernova.SessionDisputeGraceBlocks
	if deadline < blockNum {
		return sessRevert("disputeSession: dispute deadline overflow")
	}
	st.StateHash = stateHash
	st.SequenceNumber = seq
	st.Status = types.SessionStatusDisputed
	st.DisputeDeadline = deadline
	if err := sessWriteSessionState(evm.StateDB, obj, st, blockNum); err != nil {
		return sessRevert("disputeSession: %v", err)
	}
	sessDueIndexAdd(evm.StateDB, deadline, id)
	return common.BigToHash(big.NewInt(1)).Bytes(), nil
}

func (c *novaSessionArbiter) getSession(evm *EVM, input []byte) ([]byte, error) {
	if len(input) < 32 {
		return sessRevert("getSession: input too short (need 32, got %d)", len(input))
	}
	_, st, err := sessGetObjectAndState(evm.StateDB, common.BytesToHash(input[:32]))
	if err != nil {
		return sessRevert("getSession: %v", err)
	}
	out, err := st.EncodeRLP()
	if err != nil {
		return sessRevert("getSession: encode state: %v", err)
	}
	return out, nil
}

func (c *novaSessionArbiter) resolveTimeout(evm *EVM, input []byte) ([]byte, error) {
	if len(input) < 32 {
		return sessRevert("resolveTimeout: input too short (need 32, got %d)", len(input))
	}
	id := common.BytesToHash(input[:32])
	obj, st, err := sessGetObjectAndState(evm.StateDB, id)
	if err != nil {
		return sessRevert("resolveTimeout: %v", err)
	}
	blockNum := evm.Context.BlockNumber.Uint64()
	if err := sessExpireIfDue(evm.StateDB, obj, st, blockNum); err != nil {
		return sessRevert("resolveTimeout: %v", err)
	}
	return common.BigToHash(big.NewInt(1)).Bytes(), nil
}

func parseSessionUint64Word(word []byte, field string) (uint64, error) {
	if len(word) != 32 {
		return 0, fmt.Errorf("%s: expected 32-byte word, got %d", field, len(word))
	}
	n := new(big.Int).SetBytes(word)
	if !n.IsUint64() {
		return 0, fmt.Errorf("%s exceeds uint64", field)
	}
	return n.Uint64(), nil
}

func parseSessionUint8Word(word []byte, field string) (uint8, error) {
	v, err := parseSessionUint64Word(word, field)
	if err != nil {
		return 0, err
	}
	if v > 255 {
		return 0, fmt.Errorf("%s exceeds uint8", field)
	}
	return uint8(v), nil
}

func parseSessionSignedInput(input []byte, op string) (common.Hash, uint64, common.Hash, [][]byte, error) {
	const headLen = sessionSignedInputWords * 32
	if len(input) < headLen {
		return common.Hash{}, 0, common.Hash{}, nil, fmt.Errorf("%s: input too short (need %d, got %d)", op, headLen, len(input))
	}
	id := common.BytesToHash(input[0:32])
	seq, err := parseSessionUint64Word(input[32:64], "sequence")
	if err != nil {
		return common.Hash{}, 0, common.Hash{}, nil, fmt.Errorf("%s: %w", op, err)
	}
	stateHash := common.BytesToHash(input[64:96])
	sigCount, err := parseSessionUint64Word(input[96:128], "sigCount")
	if err != nil {
		return common.Hash{}, 0, common.Hash{}, nil, fmt.Errorf("%s: %w", op, err)
	}
	if sigCount == 0 || sigCount > ethernova.MaxSessionSignatures {
		return common.Hash{}, 0, common.Hash{}, nil, fmt.Errorf("%s: invalid sigCount %d", op, sigCount)
	}
	need := headLen + int(sigCount*sessionSignatureLen)
	if len(input) < need {
		return common.Hash{}, 0, common.Hash{}, nil, fmt.Errorf("%s: signature tail shorter than declared count", op)
	}
	sigs := make([][]byte, sigCount)
	for i := uint64(0); i < sigCount; i++ {
		start := headLen + int(i*sessionSignatureLen)
		sig := make([]byte, sessionSignatureLen)
		copy(sig, input[start:start+int(sessionSignatureLen)])
		sigs[i] = sig
	}
	return id, seq, stateHash, sigs, nil
}

func verifySessionSignatures(st *types.SessionState, id common.Hash, seq uint64, stateHash common.Hash, sigs [][]byte) error {
	if len(sigs) != 2 {
		return errors.New("requires exactly two participant signatures")
	}
	digest := types.SessionCommitMessageHash(id, seq, stateHash)
	initiatorSigner := st.InitiatorSigner
	if initiatorSigner == (common.Address{}) {
		initiatorSigner = st.Initiator
	}
	counterpartySigner := st.CounterpartySigner
	if counterpartySigner == (common.Address{}) {
		counterpartySigner = st.Counterparty
	}
	seenInitiator := false
	seenCounterparty := false
	for _, sig := range sigs {
		addr, err := recoverSessionSigner(digest, sig)
		if err != nil {
			return err
		}
		switch addr {
		case initiatorSigner:
			seenInitiator = true
		case counterpartySigner:
			seenCounterparty = true
		default:
			return fmt.Errorf("signature from non-participant %s", addr.Hex())
		}
	}
	if !seenInitiator || !seenCounterparty {
		return errors.New("missing participant signature")
	}
	return nil
}

func recoverSessionSigner(digest common.Hash, sig []byte) (common.Address, error) {
	if len(sig) != int(sessionSignatureLen) {
		return common.Address{}, fmt.Errorf("invalid signature length %d", len(sig))
	}
	cp := make([]byte, len(sig))
	copy(cp, sig)
	if cp[64] >= 27 {
		cp[64] -= 27
	}
	pub, err := crypto.SigToPub(digest.Bytes(), cp)
	if err != nil {
		return common.Address{}, err
	}
	return crypto.PubkeyToAddress(*pub), nil
}

func sessKeyLiveCount() common.Hash { return crypto.Keccak256Hash([]byte("sess_live_count")) }
func sessKeyOwnerCount(owner common.Address) common.Hash {
	return crypto.Keccak256Hash([]byte("sess_owner_count"), owner.Bytes())
}
func sessKeyOwnerSlots(owner common.Address) common.Hash {
	return crypto.Keccak256Hash([]byte("sess_owner_slots"), owner.Bytes())
}
func sessKeyOwnerIndex(owner common.Address, slot uint64) common.Hash {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], slot)
	return crypto.Keccak256Hash([]byte("sess_owner_index"), owner.Bytes(), buf[:])
}
func sessKeyOwnerSlotOf(id common.Hash) common.Hash {
	return crypto.Keccak256Hash([]byte("sess_owner_slot_of"), id.Bytes())
}
func sessKeyParticipantCount(addr common.Address) common.Hash {
	return crypto.Keccak256Hash([]byte("sess_participant_count"), addr.Bytes())
}
func sessKeyParticipantSlots(addr common.Address) common.Hash {
	return crypto.Keccak256Hash([]byte("sess_participant_slots"), addr.Bytes())
}
func sessKeyParticipantIndex(addr common.Address, slot uint64) common.Hash {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], slot)
	return crypto.Keccak256Hash([]byte("sess_participant_index"), addr.Bytes(), buf[:])
}
func sessKeyDueCount(blockNum uint64) common.Hash {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], blockNum)
	return crypto.Keccak256Hash([]byte("sess_due_count"), buf[:])
}
func sessKeyDueCursor(blockNum uint64) common.Hash {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], blockNum)
	return crypto.Keccak256Hash([]byte("sess_due_cursor"), buf[:])
}
func sessKeyDueIndex(blockNum, slot uint64) common.Hash {
	var b, s [8]byte
	binary.BigEndian.PutUint64(b[:], blockNum)
	binary.BigEndian.PutUint64(s[:], slot)
	return crypto.Keccak256Hash([]byte("sess_due_index"), b[:], s[:])
}

func sessReadUint64(sdb StateDB, key common.Hash) uint64 {
	return new(big.Int).SetBytes(sdb.GetState(SessionArbiterAddr, key).Bytes()).Uint64()
}

func sessWriteUint64(sdb StateDB, key common.Hash, v uint64) {
	sdb.SetState(SessionArbiterAddr, key, common.BigToHash(new(big.Int).SetUint64(v)))
}

func sessEnsureExists(sdb StateDB) {
	if !sdb.Exist(SessionArbiterAddr) {
		sdb.CreateAccount(SessionArbiterAddr)
	}
	if sdb.GetNonce(SessionArbiterAddr) == 0 {
		sdb.SetNonce(SessionArbiterAddr, 1)
	}
}

func sessOwnerIndexAdd(sdb StateDB, owner common.Address, id common.Hash) {
	slot := sessReadUint64(sdb, sessKeyOwnerSlots(owner))
	sdb.SetState(SessionArbiterAddr, sessKeyOwnerIndex(owner, slot), id)
	sessWriteUint64(sdb, sessKeyOwnerSlotOf(id), slot)
	sessWriteUint64(sdb, sessKeyOwnerSlots(owner), slot+1)
	sessWriteUint64(sdb, sessKeyOwnerCount(owner), sessReadUint64(sdb, sessKeyOwnerCount(owner))+1)
}

func sessParticipantIndexAdd(sdb StateDB, addr common.Address, id common.Hash) {
	slot := sessReadUint64(sdb, sessKeyParticipantSlots(addr))
	sdb.SetState(SessionArbiterAddr, sessKeyParticipantIndex(addr, slot), id)
	sessWriteUint64(sdb, sessKeyParticipantSlots(addr), slot+1)
	sessWriteUint64(sdb, sessKeyParticipantCount(addr), sessReadUint64(sdb, sessKeyParticipantCount(addr))+1)
}

func sessDueIndexAdd(sdb StateDB, blockNum uint64, id common.Hash) {
	sessEnsureExists(sdb)
	slot := sessReadUint64(sdb, sessKeyDueCount(blockNum))
	sdb.SetState(SessionArbiterAddr, sessKeyDueIndex(blockNum, slot), id)
	sessWriteUint64(sdb, sessKeyDueCount(blockNum), slot+1)
}

func sessDecrementLive(sdb StateDB) {
	live := sessReadUint64(sdb, sessKeyLiveCount())
	if live > 0 {
		sessWriteUint64(sdb, sessKeyLiveCount(), live-1)
	}
}

func sessGetObjectAndState(sdb StateDB, id common.Hash) (*types.ProtocolObject, *types.SessionState, error) {
	obj := PoGetObject(sdb, id)
	if obj == nil {
		return nil, nil, errors.New("session not found")
	}
	if obj.TypeTag != types.ProtoTypeSession {
		return nil, nil, errors.New("object is not a session")
	}
	st, err := types.DecodeSessionState(obj.StateData)
	if err != nil {
		return nil, nil, err
	}
	return obj, st, nil
}

// SessGetSession reads a Session Protocol Object for RPC/tests.
func SessGetSession(sdb StateDB, id common.Hash) *types.ProtocolObject {
	obj := PoGetObject(sdb, id)
	if obj == nil || obj.TypeTag != types.ProtoTypeSession {
		return nil
	}
	return obj
}

func SessGetSessionState(sdb StateDB, id common.Hash) *types.SessionState {
	obj := SessGetSession(sdb, id)
	if obj == nil {
		return nil
	}
	st, err := types.DecodeSessionState(obj.StateData)
	if err != nil {
		return nil
	}
	return st
}

func sessWriteSessionState(sdb StateDB, obj *types.ProtocolObject, st *types.SessionState, blockNum uint64) error {
	stateData, err := st.EncodeRLP()
	if err != nil {
		return err
	}
	if uint64(len(stateData)) > ethernova.MaxSessionStateBytes {
		return fmt.Errorf("state data exceeds cap (%d > %d)", len(stateData), ethernova.MaxSessionStateBytes)
	}
	obj.StateData = stateData
	obj.LastTouchedBlock = blockNum
	objData, err := obj.EncodeRLP()
	if err != nil {
		return err
	}
	poWriteRLP(sdb, obj.ID, objData)
	return nil
}

func sessExpireIfDue(sdb StateDB, obj *types.ProtocolObject, st *types.SessionState, blockNum uint64) error {
	if !types.IsLiveSessionStatus(st.Status) {
		return nil
	}
	dueBlock := st.TimeoutBlock
	if st.Status == types.SessionStatusDisputed && st.DisputeDeadline != 0 {
		dueBlock = st.DisputeDeadline
	}
	if blockNum < dueBlock {
		return fmt.Errorf("session not due until block %d", dueBlock)
	}
	st.Status = types.SessionStatusExpired
	st.ClosedBlock = blockNum
	if err := sessWriteSessionState(sdb, obj, st, blockNum); err != nil {
		return err
	}
	sessDecrementLive(sdb)
	return nil
}

// ProcessSessionTimeouts is called from the Phase 0 deferred-processing hook.
// It handles only sessions indexed for the exact current block, with a cursor
// so work is bounded and deterministic if many sessions share one timeout.
func ProcessSessionTimeouts(sdb StateDB, blockNum, limit uint64) (processed uint64, expired uint64) {
	if sdb == nil || limit == 0 || !sdb.Exist(SessionArbiterAddr) {
		return 0, 0
	}
	count := sessReadUint64(sdb, sessKeyDueCount(blockNum))
	cursor := sessReadUint64(sdb, sessKeyDueCursor(blockNum))
	for cursor < count && processed < limit {
		id := sdb.GetState(SessionArbiterAddr, sessKeyDueIndex(blockNum, cursor))
		cursor++
		processed++
		if id == (common.Hash{}) {
			continue
		}
		obj, st, err := sessGetObjectAndState(sdb, id)
		if err != nil || !types.IsLiveSessionStatus(st.Status) {
			continue
		}
		if err := sessExpireIfDue(sdb, obj, st, blockNum); err == nil {
			expired++
		}
	}
	if processed > 0 {
		sessWriteUint64(sdb, sessKeyDueCursor(blockNum), cursor)
	}
	return processed, expired
}
