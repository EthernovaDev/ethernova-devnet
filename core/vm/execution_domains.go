package vm

import "github.com/ethereum/go-ethereum/common"

// ExecutionDomain is the NIP-0004 Phase 6 contract execution tier.
//
// Domain 0 is the default for all legacy bytecode and has no prefix.
// Domain 1/2 opt in by returning runtime bytecode prefixed with EF01/EF02.
// The prefix is kept in code storage as deterministic metadata, but stripped
// before interpretation so existing EVM byte offsets remain normal.
type ExecutionDomain uint8

const (
	DomainLegacy ExecutionDomain = iota
	DomainNova
	DomainChannel
)

func parseExecutionDomain(code []byte) (ExecutionDomain, []byte) {
	if len(code) >= 2 && code[0] == 0xEF {
		switch code[1] {
		case 0x01:
			return DomainNova, code[2:]
		case 0x02:
			return DomainChannel, code[2:]
		}
	}
	return DomainLegacy, code
}

func hasExecutionDomainPrefix(code []byte) bool {
	if len(code) < 2 || code[0] != 0xEF {
		return false
	}
	return code[1] == 0x01 || code[1] == 0x02
}

func (evm *EVM) domainOfAddress(addr common.Address) ExecutionDomain {
	domain, _ := parseExecutionDomain(evm.StateDB.GetCode(addr))
	return domain
}

func (evm *EVM) callerIsContract(caller ContractRef) bool {
	if _, ok := caller.(*Contract); ok {
		return true
	}
	return len(evm.StateDB.GetCode(caller.Address())) > 0
}

func (evm *EVM) checkContractDomainCall(caller ContractRef, callee common.Address) error {
	if !evm.callerIsContract(caller) {
		return nil
	}
	callerDomain := evm.currentExecutionDomain(caller.Address())
	calleeDomain := evm.domainOfAddress(callee)
	if callerDomain < calleeDomain {
		return ErrExecutionReverted
	}
	return nil
}
