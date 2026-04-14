// Ethernova: Protocol Object Registry Precompile (NIP-0004 Phase 1)
//
// Address: 0x29 (novaProtocolObjectRegistry)
//
// Uses evm.StateDB (vm.StateDB interface) with GetState/SetState directly,
// same pattern as novaAccountManager (0x22). Does NOT depend on concrete
// *state.StateDB methods.
//
// Function selectors (first byte of input):
//   0x01 - createObject(typeTag, expiryBlock, rentPrepay, stateData) -> returns object ID
//   0x02 - getObject(id)                                             -> returns RLP-encoded object
//   0x03 - getObjectCount()                                          -> returns total count
//   0x04 - getObjectsByOwner(owner, offset, limit)                   -> returns list of IDs
//   0x05 - deleteObject(id)                                          -> deletes (owner only)
//
// Storage layout (all at system address 0xFF01):
//   keccak256("object", id)                  -> 0x01 marker (presence flag)
//   keccak256("data_len", id)                -> uint64 byte length of RLP data
//   keccak256("chunk_count", id)             -> number of 32-byte chunks
//   keccak256("chunk", id, chunkIndex)       -> 32-byte chunk of RLP data
//   keccak256("owner_count", owner)          -> count of objects owned
//   keccak256("owner_index", owner, index)   -> object ID
//   keccak256("total_count")                 -> global object count
//   keccak256("type_count", typeTag)         -> count per type

package vm

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	protoRegistryGasCreate uint64 = 20000
	protoRegistryGasRead   uint64 = 2000
	protoRegistryGasCount  uint64 = 1000
	protoRegistryGasList   uint64 = 2000
	protoRegistryGasDelete uint64 = 10000
)

// ProtocolObjectRegistryAddr is the system address where Protocol Objects live.
var ProtocolObjectRegistryAddr = common.HexToAddress("0x000000000000000000000000000000000000FF01")

// --- Storage key builders (deterministic, no map iteration) ---

func poKeyObject(id common.Hash) common.Hash {
	return crypto.Keccak256Hash([]byte("object"), id.Bytes())
}
func poKeyDataLen(id common.Hash) common.Hash {
	return crypto.Keccak256Hash([]byte("data_len"), id.Bytes())
}
func poKeyChunkCount(id common.Hash) common.Hash {
	return crypto.Keccak256Hash([]byte("chunk_count"), id.Bytes())
}
func poKeyChunk(id common.Hash, idx uint64) common.Hash {
	return crypto.Keccak256Hash([]byte("chunk"), id.Bytes(), new(big.Int).SetUint64(idx).Bytes())
}
func poKeyOwnerCount(owner common.Address) common.Hash {
	return crypto.Keccak256Hash([]byte("owner_count"), owner.Bytes())
}
func poKeyOwnerIndex(owner common.Address, idx uint64) common.Hash {
	return crypto.Keccak256Hash([]byte("owner_index"), owner.Bytes(), new(big.Int).SetUint64(idx).Bytes())
}
func poKeyTotalCount() common.Hash {
	return crypto.Keccak256Hash([]byte("total_count"))
}
func poKeyTypeCount(tag uint8) common.Hash {
	return crypto.Keccak256Hash([]byte("type_count"), []byte{tag})
}

// --- Low-level helpers using vm.StateDB interface ---

func poReadUint64(sdb StateDB, key common.Hash) uint64 {
	val := sdb.GetState(ProtocolObjectRegistryAddr, key)
	return new(big.Int).SetBytes(val.Bytes()).Uint64()
}

func poWriteUint64(sdb StateDB, key common.Hash, v uint64) {
	sdb.SetState(ProtocolObjectRegistryAddr, key, common.BigToHash(new(big.Int).SetUint64(v)))
}

func poWriteRLP(sdb StateDB, id common.Hash, data []byte) {
	sys := ProtocolObjectRegistryAddr
	dataLen := uint64(len(data))
	poWriteUint64(sdb, poKeyDataLen(id), dataLen)
	chunks := (dataLen + 31) / 32
	poWriteUint64(sdb, poKeyChunkCount(id), chunks)
	for i := uint64(0); i < chunks; i++ {
		start := i * 32
		end := start + 32
		if end > dataLen {
			end = dataLen
		}
		var chunk [32]byte
		copy(chunk[:], data[start:end])
		sdb.SetState(sys, poKeyChunk(id, i), common.BytesToHash(chunk[:]))
	}
}

func poReadRLP(sdb StateDB, id common.Hash) []byte {
	dataLen := poReadUint64(sdb, poKeyDataLen(id))
	if dataLen == 0 {
		return nil
	}
	chunks := poReadUint64(sdb, poKeyChunkCount(id))
	data := make([]byte, 0, dataLen)
	for i := uint64(0); i < chunks; i++ {
		chunk := sdb.GetState(ProtocolObjectRegistryAddr, poKeyChunk(id, i))
		remaining := dataLen - uint64(len(data))
		if remaining >= 32 {
			data = append(data, chunk[:]...)
		} else {
			data = append(data, chunk[:remaining]...)
		}
	}
	return data
}

func poClearRLP(sdb StateDB, id common.Hash) {
	chunks := poReadUint64(sdb, poKeyChunkCount(id))
	for i := uint64(0); i < chunks; i++ {
		sdb.SetState(ProtocolObjectRegistryAddr, poKeyChunk(id, i), common.Hash{})
	}
	poWriteUint64(sdb, poKeyDataLen(id), 0)
	poWriteUint64(sdb, poKeyChunkCount(id), 0)
}

func poEnsureRegistryExists(sdb StateDB) {
	if !sdb.Exist(ProtocolObjectRegistryAddr) {
		sdb.CreateAccount(ProtocolObjectRegistryAddr)
	}
}

// --- Exported read helpers (used by RPC layer via vm.StateDB) ---

// PoGetObject reads a Protocol Object by ID.
func PoGetObject(sdb StateDB, id common.Hash) *types.ProtocolObject {
	marker := sdb.GetState(ProtocolObjectRegistryAddr, poKeyObject(id))
	if marker == (common.Hash{}) {
		return nil
	}
	data := poReadRLP(sdb, id)
	if len(data) == 0 {
		return nil
	}
	obj, err := types.DecodeProtocolObject(data)
	if err != nil {
		return nil
	}
	return obj
}

// PoGetObjectCount returns total Protocol Object count.
func PoGetObjectCount(sdb StateDB) uint64 {
	return poReadUint64(sdb, poKeyTotalCount())
}

// PoGetTypeCount returns count for a specific type.
func PoGetTypeCount(sdb StateDB, tag uint8) uint64 {
	return poReadUint64(sdb, poKeyTypeCount(tag))
}

// PoGetObjectsByOwner returns object IDs for a given owner.
func PoGetObjectsByOwner(sdb StateDB, owner common.Address, offset, limit uint64) []common.Hash {
	total := poReadUint64(sdb, poKeyOwnerCount(owner))
	if offset >= total {
		return nil
	}
	var ids []common.Hash
	scanned := uint64(0)
	collected := uint64(0)
	for scanned < total+offset && collected < limit {
		val := sdb.GetState(ProtocolObjectRegistryAddr, poKeyOwnerIndex(owner, scanned))
		scanned++
		if val == (common.Hash{}) {
			continue
		}
		if scanned <= offset {
			continue
		}
		ids = append(ids, val)
		collected++
	}
	return ids
}

// === Precompile struct ===

type novaProtocolObjectRegistry struct{}

func (c *novaProtocolObjectRegistry) RequiredGas(input []byte) uint64 {
	if len(input) < 1 {
		return 0
	}
	switch input[0] {
	case 0x01:
		return protoRegistryGasCreate
	case 0x02:
		return protoRegistryGasRead
	case 0x03:
		return protoRegistryGasCount
	case 0x04:
		return protoRegistryGasList
	case 0x05:
		return protoRegistryGasDelete
	default:
		return 0
	}
}

func (c *novaProtocolObjectRegistry) Run(input []byte) ([]byte, error) {
	return nil, errors.New("novaProtocolObjectRegistry: requires stateful execution")
}

func (c *novaProtocolObjectRegistry) RunStateful(evm *EVM, caller common.Address, input []byte) ([]byte, error) {
	if len(input) < 1 {
		return nil, errors.New("empty input")
	}
	switch input[0] {
	case 0x01:
		return c.createObject(evm, caller, input[1:])
	case 0x02:
		return c.getObject(evm, input[1:])
	case 0x03:
		return c.getObjectCount(evm)
	case 0x04:
		return c.getObjectsByOwner(evm, input[1:])
	case 0x05:
		return c.deleteObject(evm, caller, input[1:])
	default:
		return nil, errors.New("unknown function selector")
	}
}

func (c *novaProtocolObjectRegistry) createObject(evm *EVM, caller common.Address, input []byte) ([]byte, error) {
	if len(input) < 65 {
		return nil, errors.New("createObject: input too short")
	}
	typeTag := input[0]
	if !types.IsValidProtocolObjectType(typeTag) {
		return nil, errors.New("createObject: invalid type tag")
	}
	expiryBlock := new(big.Int).SetBytes(input[1:33]).Uint64()
	rentPrepay := new(big.Int).SetBytes(input[33:65])

	var stateData []byte
	if len(input) > 65 {
		stateData = input[65:]
	}

	sdb := evm.StateDB
	blockNum := evm.Context.BlockNumber.Uint64()
	currentCount := PoGetObjectCount(sdb)

	// Deterministic ID: keccak256(caller, blockNumber, counter)
	idInput := append(caller.Bytes(), new(big.Int).SetUint64(blockNum).Bytes()...)
	idInput = append(idInput, new(big.Int).SetUint64(currentCount).Bytes()...)
	id := crypto.Keccak256Hash(idInput)

	obj := &types.ProtocolObject{
		ID:               id,
		Owner:            caller,
		TypeTag:          typeTag,
		StateData:        stateData,
		ExpiryBlock:      expiryBlock,
		LastTouchedBlock: blockNum,
		RentBalance:      rentPrepay,
	}

	data, err := obj.EncodeRLP()
	if err != nil {
		return nil, err
	}

	// Ensure registry account exists
	poEnsureRegistryExists(sdb)

	// Write presence marker + RLP data
	sdb.SetState(ProtocolObjectRegistryAddr, poKeyObject(id), common.BytesToHash([]byte{0x01}))
	poWriteRLP(sdb, id, data)

	// Update owner index
	ownerCount := poReadUint64(sdb, poKeyOwnerCount(caller))
	sdb.SetState(ProtocolObjectRegistryAddr, poKeyOwnerIndex(caller, ownerCount), id)
	poWriteUint64(sdb, poKeyOwnerCount(caller), ownerCount+1)

	// Update global + type counts
	poWriteUint64(sdb, poKeyTotalCount(), currentCount+1)
	typeCount := PoGetTypeCount(sdb, typeTag)
	poWriteUint64(sdb, poKeyTypeCount(typeTag), typeCount+1)

	return id.Bytes(), nil
}

func (c *novaProtocolObjectRegistry) getObject(evm *EVM, input []byte) ([]byte, error) {
	if len(input) < 32 {
		return nil, errors.New("getObject: input too short")
	}
	id := common.BytesToHash(input[:32])
	obj := PoGetObject(evm.StateDB, id)
	if obj == nil {
		return make([]byte, 32), nil
	}
	data, err := obj.EncodeRLP()
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (c *novaProtocolObjectRegistry) getObjectCount(evm *EVM) ([]byte, error) {
	count := PoGetObjectCount(evm.StateDB)
	return common.BigToHash(new(big.Int).SetUint64(count)).Bytes(), nil
}

func (c *novaProtocolObjectRegistry) getObjectsByOwner(evm *EVM, input []byte) ([]byte, error) {
	if len(input) < 84 {
		return nil, errors.New("getObjectsByOwner: input too short")
	}
	owner := common.BytesToAddress(input[:20])
	offset := new(big.Int).SetBytes(input[20:52]).Uint64()
	limit := new(big.Int).SetBytes(input[52:84]).Uint64()
	if limit > 100 {
		limit = 100
	}
	ids := PoGetObjectsByOwner(evm.StateDB, owner, offset, limit)
	result := make([]byte, 32+len(ids)*32)
	copy(result[:32], common.BigToHash(new(big.Int).SetUint64(uint64(len(ids)))).Bytes())
	for i, id := range ids {
		copy(result[32+i*32:32+(i+1)*32], id.Bytes())
	}
	return result, nil
}

func (c *novaProtocolObjectRegistry) deleteObject(evm *EVM, caller common.Address, input []byte) ([]byte, error) {
	if len(input) < 32 {
		return nil, errors.New("deleteObject: input too short")
	}
	id := common.BytesToHash(input[:32])
	obj := PoGetObject(evm.StateDB, id)
	if obj == nil {
		return common.BigToHash(big.NewInt(0)).Bytes(), nil
	}
	if obj.Owner != caller {
		return nil, errors.New("deleteObject: caller is not owner")
	}

	sdb := evm.StateDB

	// Clear presence + data
	sdb.SetState(ProtocolObjectRegistryAddr, poKeyObject(id), common.Hash{})
	poClearRLP(sdb, id)

	// Decrement counts
	total := PoGetObjectCount(sdb)
	if total > 0 {
		poWriteUint64(sdb, poKeyTotalCount(), total-1)
	}
	typeCount := PoGetTypeCount(sdb, obj.TypeTag)
	if typeCount > 0 {
		poWriteUint64(sdb, poKeyTypeCount(obj.TypeTag), typeCount-1)
	}
	ownerCount := poReadUint64(sdb, poKeyOwnerCount(obj.Owner))
	if ownerCount > 0 {
		poWriteUint64(sdb, poKeyOwnerCount(obj.Owner), ownerCount-1)
	}

	return common.BigToHash(big.NewInt(1)).Bytes(), nil
}