package runtime

import (
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
)

func wordToUint64(t *testing.T, ret []byte) uint64 {
	t.Helper()
	if len(ret) != 32 {
		t.Fatalf("expected 32-byte return, got %d bytes: %x", len(ret), ret)
	}
	return new(big.Int).SetBytes(ret).Uint64()
}

func domainCode(domain byte, runtime []byte) []byte {
	out := []byte{0xEF, domain}
	return append(out, runtime...)
}

func returnInitcode(runtime []byte) []byte {
	const prefixLen = 12
	out := []byte{
		byte(vm.PUSH1), byte(len(runtime)),
		byte(vm.PUSH1), prefixLen,
		byte(vm.PUSH1), 0x00,
		byte(vm.CODECOPY),
		byte(vm.PUSH1), byte(len(runtime)),
		byte(vm.PUSH1), 0x00,
		byte(vm.RETURN),
	}
	return append(out, runtime...)
}

// callStateWitnessReturnSuccess performs CALL(0x2F, selector/address word=0)
// and returns the CALL success bit as a 32-byte word.
func callStateWitnessReturnSuccess() []byte {
	return []byte{
		byte(vm.PUSH1), 0x03, // selector getCurrentTier
		byte(vm.PUSH1), 0x00,
		byte(vm.MSTORE8),
		byte(vm.PUSH1), 0x20, // retSize
		byte(vm.PUSH1), 0x00, // retOffset
		byte(vm.PUSH1), 0x21, // inSize: selector + 32-byte address
		byte(vm.PUSH1), 0x00, // inOffset (zeroed memory => selector/address word)
		byte(vm.PUSH1), 0x00, // value
		byte(vm.PUSH1), 0x2F, // novaStateWitness
		byte(vm.GAS),
		byte(vm.CALL),
		byte(vm.PUSH1), 0x00,
		byte(vm.MSTORE),
		byte(vm.PUSH1), 0x20,
		byte(vm.PUSH1), 0x00,
		byte(vm.RETURN),
	}
}

func TestDomain0CannotCallNovaPrecompile(t *testing.T) {
	ret, _, err := Execute(callStateWitnessReturnSuccess(), nil, nil)
	if err != nil {
		t.Fatalf("domain 0 execution failed: %v", err)
	}
	if got := wordToUint64(t, ret); got != 0 {
		t.Fatalf("domain 0 call to 0x2F succeeded, want blocked success bit 0")
	}
}

func TestDomain1CanCallNovaPrecompile(t *testing.T) {
	ret, _, err := Execute(domainCode(0x01, callStateWitnessReturnSuccess()), nil, nil)
	if err != nil {
		t.Fatalf("domain 1 execution failed: %v", err)
	}
	if got := wordToUint64(t, ret); got != 1 {
		t.Fatalf("domain 1 call to 0x2F success bit = %d, want 1", got)
	}
}

func TestCreateDomain1PrefixedRuntime(t *testing.T) {
	sdb, _ := state.New(types.EmptyRootHash, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
	cfg := &Config{State: sdb}
	runtime := domainCode(0x01, callStateWitnessReturnSuccess())

	created, addr, _, err := Create(returnInitcode(runtime), cfg)
	if err != nil {
		t.Fatalf("domain 1 create failed: %v", err)
	}
	if string(created) != string(runtime) {
		t.Fatalf("created runtime mismatch: got %x want %x", created, runtime)
	}
	if code := sdb.GetCode(addr); string(code) != string(runtime) {
		t.Fatalf("stored runtime mismatch: got %x want %x", code, runtime)
	}

	ret, _, err := Call(addr, nil, cfg)
	if err != nil {
		t.Fatalf("domain 1 created contract call failed: %v", err)
	}
	if got := wordToUint64(t, ret); got != 1 {
		t.Fatalf("created domain 1 call to 0x2F success bit = %d, want 1", got)
	}
}

func TestEIP3541StillRejectsUnknownEFPrefix(t *testing.T) {
	_, _, _, err := Create(returnInitcode([]byte{0xEF, 0x03, byte(vm.STOP)}), nil)
	if !errors.Is(err, vm.ErrInvalidCode) {
		t.Fatalf("unknown 0xEF prefix create error = %v, want %v", err, vm.ErrInvalidCode)
	}
}

func TestCapabilitiesNarrowAcrossCall(t *testing.T) {
	sdb, _ := state.New(types.EmptyRootHash, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
	callee := common.HexToAddress("0xbb")
	sdb.SetCode(callee, callStateWitnessReturnSuccess())

	caller := domainCode(0x01, []byte{
		byte(vm.PUSH1), 0x20, // retSize from callee
		byte(vm.PUSH1), 0x00, // retOffset
		byte(vm.PUSH1), 0x00, // inSize
		byte(vm.PUSH1), 0x00, // inOffset
		byte(vm.PUSH1), 0x00, // value
		byte(vm.PUSH1), 0xbb, // callee
		byte(vm.GAS),
		byte(vm.CALL),
		byte(vm.POP), // ignore CALL success; return callee's result word
		byte(vm.PUSH1), 0x20,
		byte(vm.PUSH1), 0x00,
		byte(vm.RETURN),
	})

	ret, _, err := Execute(caller, nil, &Config{State: sdb})
	if err != nil {
		t.Fatalf("domain 1 -> domain 0 call failed: %v", err)
	}
	if got := wordToUint64(t, ret); got != 0 {
		t.Fatalf("domain 0 callee retained Nova capability, got success bit %d, want 0", got)
	}
}

func TestDomain0CannotRawCallDomain1(t *testing.T) {
	sdb, _ := state.New(types.EmptyRootHash, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
	callee := common.HexToAddress("0xcc")
	sdb.SetCode(callee, domainCode(0x01, []byte{
		byte(vm.PUSH1), 0x01,
		byte(vm.PUSH1), 0x00,
		byte(vm.MSTORE),
		byte(vm.PUSH1), 0x20,
		byte(vm.PUSH1), 0x00,
		byte(vm.RETURN),
	}))

	caller := []byte{
		byte(vm.PUSH1), 0x20, // retSize
		byte(vm.PUSH1), 0x00, // retOffset
		byte(vm.PUSH1), 0x00, // inSize
		byte(vm.PUSH1), 0x00, // inOffset
		byte(vm.PUSH1), 0x00, // value
		byte(vm.PUSH1), 0xcc, // domain 1 callee
		byte(vm.GAS),
		byte(vm.CALL),
		byte(vm.PUSH1), 0x00,
		byte(vm.MSTORE),
		byte(vm.PUSH1), 0x20,
		byte(vm.PUSH1), 0x00,
		byte(vm.RETURN),
	}

	ret, _, err := Execute(caller, nil, &Config{State: sdb})
	if err != nil {
		t.Fatalf("domain 0 caller failed: %v", err)
	}
	if got := wordToUint64(t, ret); got != 0 {
		t.Fatalf("domain 0 raw call to domain 1 succeeded, want blocked success bit 0")
	}
}
