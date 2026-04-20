package vm

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
)

func newProtocolObjectTestEVM(t *testing.T, block uint64) *EVM {
	t.Helper()
	statedb, err := state.New(types.EmptyRootHash, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
	if err != nil {
		t.Fatalf("failed to create statedb: %v", err)
	}
	return &EVM{
		StateDB: statedb,
		Context: BlockContext{BlockNumber: new(big.Int).SetUint64(block)},
	}
}

func encodeABIWordU64(v uint64) []byte {
	return common.BigToHash(new(big.Int).SetUint64(v)).Bytes()
}

func encodeCreateABIInput(typeTag uint8, expiry uint64, rent *big.Int, stateData []byte) []byte {
	if rent == nil {
		rent = big.NewInt(0)
	}
	// selector + 4 static words + dynamic tail (len + data + padding)
	head := make([]byte, 4+32*4)
	copy(head[:4], poABISelectorCreate)
	head[4+31] = typeTag
	copy(head[4+32:4+64], encodeABIWordU64(expiry))
	copy(head[4+64:4+96], common.BigToHash(rent).Bytes())
	copy(head[4+96:4+128], encodeABIWordU64(32*4))

	padded := ((len(stateData) + 31) / 32) * 32
	tail := make([]byte, 32+padded)
	copy(tail[:32], encodeABIWordU64(uint64(len(stateData))))
	copy(tail[32:], stateData)
	return append(head, tail...)
}

func parseABIBytes32Array(ret []byte) []common.Hash {
	if len(ret) < 64 {
		return nil
	}
	n := new(big.Int).SetBytes(ret[32:64]).Uint64()
	ids := make([]common.Hash, 0, n)
	for i := uint64(0); i < n; i++ {
		start := 64 + i*32
		end := start + 32
		if end > uint64(len(ret)) {
			break
		}
		ids = append(ids, common.BytesToHash(ret[start:end]))
	}
	return ids
}

func TestProtocolObjectsABIEndToEnd(t *testing.T) {
	evm := newProtocolObjectTestEVM(t, 100)
	reg := &novaProtocolObjectRegistry{}
	owner := common.HexToAddress("0x0000000000000000000000000000000000000abc")

	// createObject(uint8,uint256,uint256,bytes)
	createIn := encodeCreateABIInput(types.ProtoTypeMailbox, 1000, big.NewInt(12345), []byte("hello-rlp"))
	idBytes, err := reg.RunStateful(evm, owner, createIn, false)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	id := common.BytesToHash(idBytes)
	if id == (common.Hash{}) {
		t.Fatalf("create returned zero id")
	}

	// getObjectCount()
	countOut, err := reg.RunStateful(evm, owner, poABISelectorGetObjectCount, true)
	if err != nil {
		t.Fatalf("getObjectCount failed: %v", err)
	}
	if got := new(big.Int).SetBytes(countOut).Uint64(); got != 1 {
		t.Fatalf("expected count=1 got=%d", got)
	}

	// getObject(bytes32) returns ABI-encoded bytes; decode and assert non-empty.
	getIn := append(append([]byte{}, poABISelectorGetObject...), id.Bytes()...)
	getOut, err := reg.RunStateful(evm, owner, getIn, true)
	if err != nil {
		t.Fatalf("getObject failed: %v", err)
	}
	if len(getOut) < 64 {
		t.Fatalf("invalid ABI bytes return")
	}
	rawLen := new(big.Int).SetBytes(getOut[32:64]).Uint64()
	if rawLen == 0 {
		t.Fatalf("expected non-empty object RLP")
	}

	// getObjectsByOwner(address,uint256,uint256)
	listIn := make([]byte, 4+32*3)
	copy(listIn[:4], poABISelectorGetObjectsByOwner)
	copy(listIn[4+12:4+32], owner.Bytes())
	copy(listIn[4+32:4+64], encodeABIWordU64(0))
	copy(listIn[4+64:4+96], encodeABIWordU64(10))
	listOut, err := reg.RunStateful(evm, owner, listIn, true)
	if err != nil {
		t.Fatalf("getObjectsByOwner failed: %v", err)
	}
	ids := parseABIBytes32Array(listOut)
	if len(ids) != 1 || ids[0] != id {
		t.Fatalf("unexpected owner list: %v", ids)
	}

	// vm helper/RPC source-of-truth must observe the same live state.
	if got := PoGetObjectCount(evm.StateDB); got != 1 {
		t.Fatalf("PoGetObjectCount mismatch: %d", got)
	}
	if obj := PoGetObject(evm.StateDB, id); obj == nil {
		t.Fatalf("PoGetObject returned nil")
	}

	// deleteObject(bytes32)
	delIn := append(append([]byte{}, poABISelectorDeleteObject...), id.Bytes()...)
	delOut, err := reg.RunStateful(evm, owner, delIn, false)
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if new(big.Int).SetBytes(delOut).Cmp(big.NewInt(1)) != 0 {
		t.Fatalf("delete should return true")
	}

	if got := PoGetObjectCount(evm.StateDB); got != 0 {
		t.Fatalf("expected count=0 after delete, got=%d", got)
	}
	if obj := PoGetObject(evm.StateDB, id); obj != nil {
		t.Fatalf("object should be deleted")
	}
	if ids := PoGetObjectsByOwner(evm.StateDB, owner, 0, 10); len(ids) != 0 {
		t.Fatalf("owner index should not contain ghost entry: %v", ids)
	}
}

func TestProtocolObjectsStaticcallWriteProtectionABI(t *testing.T) {
	evm := newProtocolObjectTestEVM(t, 1)
	reg := &novaProtocolObjectRegistry{}
	owner := common.HexToAddress("0x0000000000000000000000000000000000000def")

	_, err := reg.RunStateful(evm, owner, encodeCreateABIInput(types.ProtoTypeMailbox, 10, big.NewInt(1), []byte("x")), true)
	if err != ErrWriteProtection {
		t.Fatalf("expected ErrWriteProtection for ABI create in readonly mode, got %v", err)
	}
}
