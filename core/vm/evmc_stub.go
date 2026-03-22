//go:build !cgo
// +build !cgo

package vm

import "github.com/ethereum/go-ethereum/log"

func appendExternalInterpreters(evm *EVM) {
	if evm.Config.EWASMInterpreter != "" || evm.Config.EVMInterpreter != "" {
		log.Warn("EVMC support disabled (built without cgo); ignoring external VM configuration")
	}
}

func InitEVMCEVM(config string) {
	if config != "" {
		log.Warn("EVMC support disabled (built without cgo); --vm.evm ignored", "path", config)
	}
}

func InitEVMCEwasm(config string) {
	if config != "" {
		log.Warn("EVMC support disabled (built without cgo); --vm.ewasm ignored", "path", config)
	}
}
