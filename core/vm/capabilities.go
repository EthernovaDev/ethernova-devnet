package vm

import "github.com/ethereum/go-ethereum/common"

// CapabilityMask is the Phase 6 call-chain permission set. Capabilities only
// narrow while moving down the call stack: a callee can never gain a capability
// the caller did not already have.
type CapabilityMask uint64

const (
	CapabilityProtocolObjects CapabilityMask = 1 << iota
	CapabilityDeferredQueue
	CapabilityContentRegistry
	CapabilityMailboxManager
	CapabilityStateWitness
	CapabilityMailboxOps
	CapabilitySessionArbiter
	CapabilityAppPrecompiles
)

const (
	CapabilityNone CapabilityMask = 0
	CapabilityNova CapabilityMask = CapabilityProtocolObjects |
		CapabilityDeferredQueue |
		CapabilityContentRegistry |
		CapabilityMailboxManager |
		CapabilityStateWitness |
		CapabilityMailboxOps |
		CapabilitySessionArbiter |
		CapabilityAppPrecompiles
)

type executionFrame struct {
	domain       ExecutionDomain
	capabilities CapabilityMask
}

func defaultCapabilitiesForDomain(domain ExecutionDomain) CapabilityMask {
	switch domain {
	case DomainNova, DomainChannel:
		return CapabilityNova
	default:
		return CapabilityNone
	}
}

// DefaultCapabilitiesForDomain exposes the Phase 6 default capability grant
// for RPC/tooling. Runtime enforcement continues through execution frames.
func DefaultCapabilitiesForDomain(domain ExecutionDomain) CapabilityMask {
	return defaultCapabilitiesForDomain(domain)
}

func requiredCapabilityForPrecompile(addr common.Address) CapabilityMask {
	switch addr[19] {
	case 0x29:
		return CapabilityProtocolObjects
	case 0x2A:
		return CapabilityDeferredQueue
	case 0x2B:
		return CapabilityContentRegistry
	case 0x2C:
		return CapabilityMailboxManager
	case 0x2D:
		return CapabilitySessionArbiter
	case 0x2F:
		return CapabilityStateWitness
	case 0x35:
		return CapabilityMailboxOps
	case 0x30, 0x31, 0x32, 0x33, 0x34, 0x36:
		return CapabilityAppPrecompiles
	default:
		return CapabilityNone
	}
}

// RequiredCapabilityForPrecompile returns the capability bit needed to call a
// Nova precompile. Non-Nova precompiles return CapabilityNone.
func RequiredCapabilityForPrecompile(addr common.Address) CapabilityMask {
	return requiredCapabilityForPrecompile(addr)
}

// CapabilityName renders a stable label for a single capability bit.
func CapabilityName(cap CapabilityMask) string {
	switch cap {
	case CapabilityProtocolObjects:
		return "protocolObjects"
	case CapabilityDeferredQueue:
		return "deferredQueue"
	case CapabilityContentRegistry:
		return "contentRegistry"
	case CapabilityMailboxManager:
		return "mailboxManager"
	case CapabilityStateWitness:
		return "stateWitness"
	case CapabilityMailboxOps:
		return "mailboxOps"
	case CapabilitySessionArbiter:
		return "sessionArbiter"
	case CapabilityAppPrecompiles:
		return "applicationPrecompiles"
	default:
		return "unknown"
	}
}

// CapabilityNames returns all labels enabled in the mask.
func CapabilityNames(mask CapabilityMask) []string {
	catalog := []CapabilityMask{
		CapabilityProtocolObjects,
		CapabilityDeferredQueue,
		CapabilityContentRegistry,
		CapabilityMailboxManager,
		CapabilityStateWitness,
		CapabilityMailboxOps,
		CapabilitySessionArbiter,
		CapabilityAppPrecompiles,
	}
	names := make([]string, 0, len(catalog))
	for _, cap := range catalog {
		if mask&cap != 0 {
			names = append(names, CapabilityName(cap))
		}
	}
	return names
}

func (evm *EVM) pushExecutionFrame(domain ExecutionDomain) {
	caps := defaultCapabilitiesForDomain(domain)
	if n := len(evm.executionFrames); n > 0 {
		caps &= evm.executionFrames[n-1].capabilities
	}
	evm.executionFrames = append(evm.executionFrames, executionFrame{
		domain:       domain,
		capabilities: caps,
	})
}

func (evm *EVM) popExecutionFrame() {
	if len(evm.executionFrames) == 0 {
		return
	}
	evm.executionFrames = evm.executionFrames[:len(evm.executionFrames)-1]
}

func (evm *EVM) currentExecutionDomain(fallback common.Address) ExecutionDomain {
	if n := len(evm.executionFrames); n > 0 {
		return evm.executionFrames[n-1].domain
	}
	return evm.domainOfAddress(fallback)
}

func (evm *EVM) currentCapabilities(caller common.Address) CapabilityMask {
	if n := len(evm.executionFrames); n > 0 {
		return evm.executionFrames[n-1].capabilities
	}
	domain := evm.domainOfAddress(caller)
	if domain == DomainLegacy && len(evm.StateDB.GetCode(caller)) == 0 {
		// EOAs are not Domain 0 contracts. Keep direct user/devnet RPC calls
		// to Nova precompiles working while contract calls remain gated.
		return CapabilityNova
	}
	return defaultCapabilitiesForDomain(domain)
}

func (evm *EVM) checkPrecompileCapabilities(caller common.Address, addr common.Address) error {
	required := requiredCapabilityForPrecompile(addr)
	if required == CapabilityNone {
		return nil
	}
	if evm.currentCapabilities(caller)&required == 0 {
		return ErrExecutionReverted
	}
	return nil
}
