package vm

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func poLifecycleWord(v uint64) []byte {
	return common.BigToHash(new(big.Int).SetUint64(v)).Bytes()
}

func TestProtocolObjectRegistryRecordsLifecycleTouch(t *testing.T) {
	evm, sdb := newTestEVM(t)
	caller := common.HexToAddress("0x1111111111111111111111111111111111111111")
	sdb.CreateAccount(caller)
	sdb.SetNonce(caller, 1)

	input := []byte{0x01, types.ProtoTypeIdentity}
	input = append(input, poLifecycleWord(0)...) // expiryBlock
	input = append(input, poLifecycleWord(0)...) // rentPrepay
	input = append(input, []byte("phase5c-lifecycle")...)

	idBytes, err := (&novaProtocolObjectRegistry{}).RunStateful(evm, caller, input, false)
	if err != nil {
		t.Fatalf("createObject: %v", err)
	}
	id := common.BytesToHash(idBytes)
	touched := sdb.LifecycleTouchedObjects()
	if len(touched) != 1 || touched[0] != id {
		t.Fatalf("object lifecycle touch mismatch: got %#v want %s", touched, id)
	}
}
