//go:build cgo
// +build cgo

package vm

import "github.com/ethereum/evmc/v7/bindings/go/evmc"

func appendExternalInterpreters(evm *EVM) {
	if evm.Config.EWASMInterpreter != "" {
		evm.interpreters = append(evm.interpreters, &EVMC{ewasmModule, evm, evmc.CapabilityEWASM, false})
	}
	if evm.Config.EVMInterpreter != "" {
		evm.interpreters = append(evm.interpreters, &EVMC{evmModule, evm, evmc.CapabilityEVM1, false})
	}
}
