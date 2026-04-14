package vm

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// novaAccountManager is a stateful precompiled contract at address 0x22.
// It provides native key rotation and guardian recovery for smart wallets.
//
// Function selectors (first byte of input):
//   0x01 - setGuardians(threshold, ...addresses)    -> sets guardian list
//   0x02 - getGuardians(targetAddr)                  -> returns guardians and threshold
//   0x03 - initiateRecovery(targetAddr, newOwner)    -> start recovery (guardian only)
//   0x04 - approveRecovery(targetAddr)               -> approve recovery (guardian only)
//   0x05 - finalizeRecovery(targetAddr)              -> execute after threshold + timelock
//   0x06 - getRecoveryStatus(targetAddr)             -> check recovery state
//   0x07 - initiateKeyRotation(newKeyHash)           -> start key rotation with timelock
//   0x08 - getKeyRotation(addr)                      -> check rotation status
//
// Storage layout (all at system address 0x000...nova22):
//   keccak256(addr, "guardianCount")           -> count
//   keccak256(addr, "guardianThreshold")       -> threshold
//   keccak256(addr, "guardian", index)          -> guardian address
//   keccak256(addr, "recovery.newOwner")        -> proposed new owner
//   keccak256(addr, "recovery.approvals")       -> approval count
//   keccak256(addr, "recovery.initiatedBlock")  -> block when recovery started
//   keccak256(addr, "recovery.approvedBy", idx) -> guardian who approved
//   keccak256(addr, "keyRotation.newKeyHash")   -> hash of new key
//   keccak256(addr, "keyRotation.block")        -> block when rotation initiated

const (
	accountManagerGasRead  uint64 = 2000
	accountManagerGasWrite uint64 = 10000
	recoveryTimelockBlocks uint64 = 100 // ~15 minutes on devnet
)

// System address where account manager stores its data
var accountManagerSystemAddr = common.HexToAddress("0x000000000000000000000000000000000000AA22")

type novaAccountManager struct{}

func (c *novaAccountManager) RequiredGas(input []byte) uint64 {
	if len(input) < 1 {
		return 0
	}
	switch input[0] {
	case 0x01: // setGuardians (write)
		return accountManagerGasWrite
	case 0x02: // getGuardians (read)
		return accountManagerGasRead
	case 0x03: // initiateRecovery (write)
		return accountManagerGasWrite
	case 0x04: // approveRecovery (write)
		return accountManagerGasWrite
	case 0x05: // finalizeRecovery (write)
		return accountManagerGasWrite
	case 0x06: // getRecoveryStatus (read)
		return accountManagerGasRead
	case 0x07: // initiateKeyRotation (write)
		return accountManagerGasWrite
	case 0x08: // getKeyRotation (read)
		return accountManagerGasRead
	default:
		return 0
	}
}

// RunStateful executes the account manager with access to EVM state.
// readOnly is true when called via STATICCALL — write ops MUST be rejected.
func (c *novaAccountManager) RunStateful(evm *EVM, caller common.Address, input []byte, readOnly bool) ([]byte, error) {
	if len(input) < 1 {
		return nil, errors.New("empty input")
	}

	switch input[0] {
	case 0x01: // setGuardians — WRITE
		if readOnly {
			return nil, ErrWriteProtection
		}
		return c.setGuardians(evm, caller, input[1:])
	case 0x02: // getGuardians — READ
		return c.getGuardians(evm, input[1:])
	case 0x03: // initiateRecovery — WRITE
		if readOnly {
			return nil, ErrWriteProtection
		}
		return c.initiateRecovery(evm, caller, input[1:])
	case 0x04: // approveRecovery — WRITE
		if readOnly {
			return nil, ErrWriteProtection
		}
		return c.approveRecovery(evm, caller, input[1:])
	case 0x05: // finalizeRecovery — WRITE
		if readOnly {
			return nil, ErrWriteProtection
		}
		return c.finalizeRecovery(evm, caller, input[1:])
	case 0x06: // getRecoveryStatus — READ
		return c.getRecoveryStatus(evm, input[1:])
	case 0x07: // initiateKeyRotation — WRITE
		if readOnly {
			return nil, ErrWriteProtection
		}
		return c.initiateKeyRotation(evm, caller, input[1:])
	case 0x08: // getKeyRotation — READ
		return c.getKeyRotation(evm, input[1:])
	default:
		return nil, errors.New("unknown function selector")
	}
}

// Run implements PrecompiledContract for non-stateful calls (read-only queries)
func (c *novaAccountManager) Run(input []byte) ([]byte, error) {
	return nil, errors.New("novaAccountManager requires stateful execution")
}

// Storage key helpers - deterministic hashing
func storageKey(parts ...[]byte) common.Hash {
	data := make([]byte, 0, 128)
	for _, p := range parts {
		data = append(data, p...)
	}
	return crypto.Keccak256Hash(data)
}

func addrBytes(a common.Address) []byte { return a.Bytes() }

func uint64ToHash(v uint64) common.Hash {
	return common.BigToHash(new(big.Int).SetUint64(v))
}

func hashToUint64(h common.Hash) uint64 {
	return new(big.Int).SetBytes(h.Bytes()).Uint64()
}

// setGuardians(threshold, addr1, addr2, ...)
// Input: 1 byte threshold + N*20 bytes addresses
func (c *novaAccountManager) setGuardians(evm *EVM, caller common.Address, data []byte) ([]byte, error) {
	if len(data) < 1 {
		return nil, errors.New("missing threshold")
	}
	threshold := uint64(data[0])
	addrs := data[1:]
	if len(addrs)%20 != 0 {
		return nil, errors.New("invalid guardian addresses")
	}
	count := uint64(len(addrs) / 20)
	if count == 0 || count > 10 {
		return nil, errors.New("guardian count must be 1-10")
	}
	if threshold == 0 || threshold > count {
		return nil, errors.New("invalid threshold")
	}

	sys := accountManagerSystemAddr

	// Store count and threshold
	evm.StateDB.SetState(sys, storageKey(addrBytes(caller), []byte("guardianCount")), uint64ToHash(count))
	evm.StateDB.SetState(sys, storageKey(addrBytes(caller), []byte("guardianThreshold")), uint64ToHash(threshold))

	// Store each guardian
	for i := uint64(0); i < count; i++ {
		start := i * 20
		var addr common.Address
		copy(addr[:], addrs[start:start+20])
		padded := common.BytesToHash(addr.Bytes())
		evm.StateDB.SetState(sys, storageKey(addrBytes(caller), []byte("guardian"), uint64ToHash(i).Bytes()), padded)
	}

	return uint64ToHash(count).Bytes(), nil
}

// getGuardians(targetAddr) - returns threshold + count + guardian addresses
func (c *novaAccountManager) getGuardians(evm *EVM, data []byte) ([]byte, error) {
	if len(data) < 20 {
		return nil, errors.New("missing target address")
	}
	var target common.Address
	copy(target[:], data[:20])
	sys := accountManagerSystemAddr

	count := hashToUint64(evm.StateDB.GetState(sys, storageKey(addrBytes(target), []byte("guardianCount"))))
	threshold := hashToUint64(evm.StateDB.GetState(sys, storageKey(addrBytes(target), []byte("guardianThreshold"))))

	result := make([]byte, 0, 64+count*32)
	result = append(result, uint64ToHash(threshold).Bytes()...)
	result = append(result, uint64ToHash(count).Bytes()...)

	for i := uint64(0); i < count; i++ {
		guardian := evm.StateDB.GetState(sys, storageKey(addrBytes(target), []byte("guardian"), uint64ToHash(i).Bytes()))
		result = append(result, guardian.Bytes()...)
	}

	return result, nil
}

// initiateRecovery(targetAddr, newOwnerAddr) - guardian starts recovery
func (c *novaAccountManager) initiateRecovery(evm *EVM, caller common.Address, data []byte) ([]byte, error) {
	if len(data) < 40 {
		return nil, errors.New("need target(20) + newOwner(20)")
	}
	var target, newOwner common.Address
	copy(target[:], data[:20])
	copy(newOwner[:], data[20:40])
	sys := accountManagerSystemAddr

	// Verify caller is a guardian
	if !c.isGuardian(evm, target, caller) {
		return nil, errors.New("caller is not a guardian of target")
	}

	// Store recovery request
	evm.StateDB.SetState(sys, storageKey(addrBytes(target), []byte("recovery.newOwner")), common.BytesToHash(newOwner.Bytes()))
	evm.StateDB.SetState(sys, storageKey(addrBytes(target), []byte("recovery.approvals")), uint64ToHash(1))
	evm.StateDB.SetState(sys, storageKey(addrBytes(target), []byte("recovery.initiatedBlock")), uint64ToHash(evm.Context.BlockNumber.Uint64()))
	evm.StateDB.SetState(sys, storageKey(addrBytes(target), []byte("recovery.approvedBy"), uint64ToHash(0).Bytes()), common.BytesToHash(caller.Bytes()))

	return uint64ToHash(1).Bytes(), nil
}

// approveRecovery(targetAddr) - guardian approves existing recovery
func (c *novaAccountManager) approveRecovery(evm *EVM, caller common.Address, data []byte) ([]byte, error) {
	if len(data) < 20 {
		return nil, errors.New("missing target address")
	}
	var target common.Address
	copy(target[:], data[:20])
	sys := accountManagerSystemAddr

	if !c.isGuardian(evm, target, caller) {
		return nil, errors.New("caller is not a guardian")
	}

	// Check recovery exists
	approvals := hashToUint64(evm.StateDB.GetState(sys, storageKey(addrBytes(target), []byte("recovery.approvals"))))
	if approvals == 0 {
		return nil, errors.New("no active recovery")
	}

	// Check not already approved
	for i := uint64(0); i < approvals; i++ {
		approved := evm.StateDB.GetState(sys, storageKey(addrBytes(target), []byte("recovery.approvedBy"), uint64ToHash(i).Bytes()))
		if common.BytesToAddress(approved.Bytes()) == caller {
			return nil, errors.New("already approved")
		}
	}

	// Add approval
	evm.StateDB.SetState(sys, storageKey(addrBytes(target), []byte("recovery.approvedBy"), uint64ToHash(approvals).Bytes()), common.BytesToHash(caller.Bytes()))
	approvals++
	evm.StateDB.SetState(sys, storageKey(addrBytes(target), []byte("recovery.approvals")), uint64ToHash(approvals))

	return uint64ToHash(approvals).Bytes(), nil
}

// finalizeRecovery(targetAddr) - execute recovery after threshold + timelock
func (c *novaAccountManager) finalizeRecovery(evm *EVM, caller common.Address, data []byte) ([]byte, error) {
	if len(data) < 20 {
		return nil, errors.New("missing target address")
	}
	var target common.Address
	copy(target[:], data[:20])
	sys := accountManagerSystemAddr

	approvals := hashToUint64(evm.StateDB.GetState(sys, storageKey(addrBytes(target), []byte("recovery.approvals"))))
	threshold := hashToUint64(evm.StateDB.GetState(sys, storageKey(addrBytes(target), []byte("guardianThreshold"))))
	initiatedBlock := hashToUint64(evm.StateDB.GetState(sys, storageKey(addrBytes(target), []byte("recovery.initiatedBlock"))))

	if approvals < threshold {
		return nil, errors.New("not enough approvals")
	}

	currentBlock := evm.Context.BlockNumber.Uint64()
	if currentBlock < initiatedBlock+recoveryTimelockBlocks {
		return nil, errors.New("timelock not expired")
	}

	// Get new owner
	newOwner := common.BytesToAddress(evm.StateDB.GetState(sys, storageKey(addrBytes(target), []byte("recovery.newOwner"))).Bytes())

	// Store key rotation result
	evm.StateDB.SetState(sys, storageKey(addrBytes(target), []byte("keyRotation.newKeyHash")), common.BytesToHash(newOwner.Bytes()))
	evm.StateDB.SetState(sys, storageKey(addrBytes(target), []byte("keyRotation.block")), uint64ToHash(currentBlock))

	// Clear recovery state
	evm.StateDB.SetState(sys, storageKey(addrBytes(target), []byte("recovery.approvals")), common.Hash{})
	evm.StateDB.SetState(sys, storageKey(addrBytes(target), []byte("recovery.newOwner")), common.Hash{})
	evm.StateDB.SetState(sys, storageKey(addrBytes(target), []byte("recovery.initiatedBlock")), common.Hash{})

	return common.BytesToHash(newOwner.Bytes()).Bytes(), nil
}

// getRecoveryStatus(targetAddr) - check recovery state
func (c *novaAccountManager) getRecoveryStatus(evm *EVM, data []byte) ([]byte, error) {
	if len(data) < 20 {
		return nil, errors.New("missing target address")
	}
	var target common.Address
	copy(target[:], data[:20])
	sys := accountManagerSystemAddr

	approvals := evm.StateDB.GetState(sys, storageKey(addrBytes(target), []byte("recovery.approvals")))
	newOwner := evm.StateDB.GetState(sys, storageKey(addrBytes(target), []byte("recovery.newOwner")))
	initiatedBlock := evm.StateDB.GetState(sys, storageKey(addrBytes(target), []byte("recovery.initiatedBlock")))
	threshold := evm.StateDB.GetState(sys, storageKey(addrBytes(target), []byte("guardianThreshold")))

	result := make([]byte, 0, 128)
	result = append(result, approvals.Bytes()...)
	result = append(result, threshold.Bytes()...)
	result = append(result, newOwner.Bytes()...)
	result = append(result, initiatedBlock.Bytes()...)

	return result, nil
}

// initiateKeyRotation(newKeyHash) - start key rotation with timelock
func (c *novaAccountManager) initiateKeyRotation(evm *EVM, caller common.Address, data []byte) ([]byte, error) {
	if len(data) < 32 {
		return nil, errors.New("need 32-byte new key hash")
	}
	sys := accountManagerSystemAddr
	var newKeyHash common.Hash
	copy(newKeyHash[:], data[:32])

	evm.StateDB.SetState(sys, storageKey(addrBytes(caller), []byte("keyRotation.newKeyHash")), newKeyHash)
	evm.StateDB.SetState(sys, storageKey(addrBytes(caller), []byte("keyRotation.block")), uint64ToHash(evm.Context.BlockNumber.Uint64()))

	return newKeyHash.Bytes(), nil
}

// getKeyRotation(addr) - check key rotation status
func (c *novaAccountManager) getKeyRotation(evm *EVM, data []byte) ([]byte, error) {
	if len(data) < 20 {
		return nil, errors.New("missing address")
	}
	var target common.Address
	copy(target[:], data[:20])
	sys := accountManagerSystemAddr

	newKeyHash := evm.StateDB.GetState(sys, storageKey(addrBytes(target), []byte("keyRotation.newKeyHash")))
	rotationBlock := evm.StateDB.GetState(sys, storageKey(addrBytes(target), []byte("keyRotation.block")))

	result := make([]byte, 0, 64)
	result = append(result, newKeyHash.Bytes()...)
	result = append(result, rotationBlock.Bytes()...)

	return result, nil
}

// isGuardian checks if addr is a guardian of target
func (c *novaAccountManager) isGuardian(evm *EVM, target, addr common.Address) bool {
	sys := accountManagerSystemAddr
	count := hashToUint64(evm.StateDB.GetState(sys, storageKey(addrBytes(target), []byte("guardianCount"))))
	for i := uint64(0); i < count; i++ {
		guardian := evm.StateDB.GetState(sys, storageKey(addrBytes(target), []byte("guardian"), uint64ToHash(i).Bytes()))
		if common.BytesToAddress(guardian.Bytes()) == addr {
			return true
		}
	}
	return false
}

// StatefulPrecompiledContract is the interface for precompiles that need EVM state access.
// readOnly MUST be true when called via STATICCALL (EIP-214). Implementations
// MUST reject any state-modifying operation when readOnly is true by returning
// ErrWriteProtection.
type StatefulPrecompiledContract interface {
	PrecompiledContract
	RunStateful(evm *EVM, caller common.Address, input []byte, readOnly bool) ([]byte, error)
}