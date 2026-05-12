package vm

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params/ethernova"
	"github.com/ethereum/go-ethereum/params/types/ctypes"
	"github.com/holiman/uint256"
)

func novaOpcodesActive(config ctypes.ChainConfigurator, bn *big.Int) bool {
	if config == nil || !ethernova.IsEthernovaChainID(config.GetChainID()) {
		return false
	}
	if ethernova.NovaOpcodeForkBlock == 0 {
		return true
	}
	return bn != nil && bn.Sign() >= 0 && bn.Uint64() >= ethernova.NovaOpcodeForkBlock
}

func enableNovaOpcodes(jt *JumpTable) {
	jt[MSEND] = &operation{execute: opMSend, constantGas: GasFastStep, minStack: minStack(3, 1), maxStack: maxStack(3, 1)}
	jt[MRECV] = &operation{execute: opMRecv, constantGas: GasFastStep, minStack: minStack(1, 1), maxStack: maxStack(1, 1)}
	jt[MPEEK] = &operation{execute: opMPeek, constantGas: GasFastStep, minStack: minStack(1, 1), maxStack: maxStack(1, 1)}
	jt[MCOUNT] = &operation{execute: opMCount, constantGas: GasFastStep, minStack: minStack(1, 1), maxStack: maxStack(1, 1)}
	jt[CREF] = &operation{execute: opCRef, constantGas: GasFastStep, minStack: minStack(4, 1), maxStack: maxStack(4, 1)}
	jt[CVERIFY] = &operation{execute: opCVerify, constantGas: GasFastStep, minStack: minStack(1, 1), maxStack: maxStack(1, 1)}
	jt[SOPEN] = &operation{execute: opSOpen, constantGas: GasFastStep, minStack: minStack(3, 1), maxStack: maxStack(3, 1)}
	jt[SCOMMIT] = &operation{execute: opSCommit, constantGas: GasFastStep, minStack: minStack(5, 1), maxStack: maxStack(5, 1)}
	jt[SCLOSE] = &operation{execute: opSClose, constantGas: GasFastStep, minStack: minStack(5, 1), maxStack: maxStack(5, 1)}
}

func opMSend(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	postage := novaStackWord(scope.Stack.pop())
	payloadHash := novaStackWord(scope.Stack.pop())
	mailboxID := novaStackWord(scope.Stack.pop())
	ret, err := novaOpcodePrecompile(interpreter, scope, 0x35, novaOpcodeInput(0x01, mailboxID, payloadHash, postage))
	if err != nil {
		return nil, err
	}
	novaPushFirstWord(scope, ret)
	return nil, nil
}

func opMRecv(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	mailboxID := novaStackWord(scope.Stack.pop())
	ret, err := novaOpcodePrecompile(interpreter, scope, 0x35, novaOpcodeInput(0x02, mailboxID))
	if err != nil {
		return nil, err
	}
	novaPushHash(scope, crypto.Keccak256Hash(ret))
	return nil, nil
}

func opMPeek(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	mailboxID := novaStackWord(scope.Stack.pop())
	ret, err := novaOpcodePrecompile(interpreter, scope, 0x35, novaOpcodeInput(0x03, mailboxID))
	if err != nil {
		return nil, err
	}
	novaPushHash(scope, crypto.Keccak256Hash(ret))
	return nil, nil
}

func opMCount(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	mailboxID := novaStackWord(scope.Stack.pop())
	ret, err := novaOpcodePrecompile(interpreter, scope, 0x35, novaOpcodeInput(0x04, mailboxID))
	if err != nil {
		return nil, err
	}
	novaPushFirstWord(scope, ret)
	return nil, nil
}

func opCRef(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	expiry := novaStackWord(scope.Stack.pop())
	rent := novaStackWord(scope.Stack.pop())
	size := novaStackWord(scope.Stack.pop())
	contentHash := novaStackWord(scope.Stack.pop())
	zero := make([]byte, 32)
	ret, err := novaOpcodePrecompile(interpreter, scope, 0x2B, novaOpcodeInput(0x01, contentHash, size, zero, zero, rent, expiry))
	if err != nil {
		return nil, err
	}
	novaPushFirstWord(scope, ret)
	return nil, nil
}

func opCVerify(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	contentRefID := novaStackWord(scope.Stack.pop())
	ret, err := novaOpcodePrecompile(interpreter, scope, 0x2B, novaOpcodeInput(0x03, contentRefID))
	if err != nil {
		return nil, err
	}
	novaPushFirstWord(scope, ret)
	return nil, nil
}

func opSOpen(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	timeout := novaStackWord(scope.Stack.pop())
	sessionType := novaStackWord(scope.Stack.pop())
	counterparty := novaStackWord(scope.Stack.pop())
	zero := make([]byte, 32)
	ret, err := novaOpcodePrecompile(interpreter, scope, 0x2D, novaOpcodeInput(0x01, counterparty, sessionType, timeout, zero, zero, zero, zero))
	if err != nil {
		return nil, err
	}
	novaPushFirstWord(scope, ret)
	return nil, nil
}

func opSCommit(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	return opSessionSignedTail(interpreter, scope, 0x02)
}

func opSClose(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	return opSessionSignedTail(interpreter, scope, 0x03)
}

func opSessionSignedTail(interpreter *EVMInterpreter, scope *ScopeContext, selector byte) ([]byte, error) {
	sigLen, err := novaPopUint64(scope.Stack)
	if err != nil {
		return nil, err
	}
	sigOffset, err := novaPopUint64(scope.Stack)
	if err != nil {
		return nil, err
	}
	stateHash := novaStackWord(scope.Stack.pop())
	seq := novaStackWord(scope.Stack.pop())
	sessionID := novaStackWord(scope.Stack.pop())
	tail := scope.Memory.GetCopy(int64(sigOffset), int64(sigLen))
	input := novaOpcodeInput(selector, sessionID, seq, stateHash)
	input = append(input, tail...)
	ret, err := novaOpcodePrecompile(interpreter, scope, 0x2D, input)
	if err != nil {
		return nil, err
	}
	novaPushFirstWord(scope, ret)
	return nil, nil
}

func novaOpcodePrecompile(interpreter *EVMInterpreter, scope *ScopeContext, addrByte byte, input []byte) ([]byte, error) {
	evm := interpreter.evm
	addr := common.BytesToAddress([]byte{addrByte})
	p, ok := evm.precompile(addr)
	if !ok {
		return nil, ErrExecutionReverted
	}
	caller := scope.Contract.Address()
	if err := evm.checkPrecompileCapabilities(caller, addr); err != nil {
		return nil, err
	}
	snapshot := evm.StateDB.Snapshot()
	ret, gas, err := runPrecompileOrStateful(p, evm, caller, addr, input, scope.Contract.Gas, interpreter.readOnly)
	if err != nil {
		evm.StateDB.RevertToSnapshot(snapshot)
		if err != ErrExecutionReverted {
			scope.Contract.Gas = 0
		}
		return nil, err
	}
	scope.Contract.Gas = gas
	return ret, nil
}

func novaOpcodeInput(selector byte, words ...[]byte) []byte {
	out := make([]byte, 1, 1+32*len(words))
	out[0] = selector
	for _, word := range words {
		out = append(out, common.LeftPadBytes(word, 32)...)
	}
	return out
}

func novaStackWord(v uint256.Int) []byte {
	h := common.Hash(v.Bytes32())
	return h.Bytes()
}

func novaPushHash(scope *ScopeContext, h common.Hash) {
	scope.Stack.push(new(uint256.Int).SetBytes(h.Bytes()))
}

func novaPushFirstWord(scope *ScopeContext, ret []byte) {
	novaPushHash(scope, common.BytesToHash(common.RightPadBytes(ret, 32)[:32]))
}

func novaPopUint64(stack *Stack) (uint64, error) {
	v := stack.pop()
	u, overflow := v.Uint64WithOverflow()
	if overflow {
		return 0, errors.New("nova opcode: uint64 stack overflow")
	}
	return u, nil
}
