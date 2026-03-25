package vm

import (
	"sync/atomic"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
)

// StateRentConfig controls the storage rent surcharge on SSTORE operations.
// Contracts that use more storage slots pay proportionally more gas on writes.
// This creates an economic incentive to clean up unused storage.
//
// Design: deterministic surcharge based on contract's code size (proxy for storage usage).
// No per-account slot tracking needed - uses code presence and nonce as heuristic.
// Fully deterministic: same StateDB state = same gas on all nodes.
type StateRentConfig struct {
	enabled          atomic.Bool
	BaseRentPerSlot  uint64 // Extra gas per estimated storage unit (default: 5)
	MaxRentSurcharge uint64 // Cap to prevent excessive gas (default: 50000)
	FreeSlotThreshold uint64 // First N slots are free (default: 10)
}

// GlobalStateRent is the singleton state rent configuration.
var GlobalStateRent = &StateRentConfig{
	BaseRentPerSlot:  5,
	MaxRentSurcharge: 50000,
	FreeSlotThreshold: 10,
}

func init() {
	GlobalStateRent.enabled.Store(true)
}

// Enabled returns whether state rent is active.
func (sr *StateRentConfig) Enabled() bool {
	return sr.enabled.Load()
}

// SetEnabled toggles state rent on/off.
func (sr *StateRentConfig) SetEnabled(v bool) {
	sr.enabled.Store(v)
}

// CalculateSurcharge computes the extra gas cost for an SSTORE operation.
// It uses the contract's nonce as a proxy for how many storage operations it has done.
// This is deterministic because nonce is part of the account state on all nodes.
//
// The surcharge increases as the contract does more writes:
//   - First FreeSlotThreshold writes: 0 extra gas
//   - After that: BaseRentPerSlot * (nonce - FreeSlotThreshold)
//   - Capped at MaxRentSurcharge
//
// Deleting storage (value -> 0) gets a BONUS refund instead of surcharge,
// incentivizing state cleanup.
func (sr *StateRentConfig) CalculateSurcharge(evm *EVM, addr common.Address, isDelete bool) uint64 {
	if !sr.Enabled() {
		return 0
	}

	// Only charge contracts, not EOAs
	codeHash := evm.StateDB.GetCodeHash(addr)
	emptyCodeHash := common.HexToHash("c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470")
	if codeHash == emptyCodeHash || codeHash == (common.Hash{}) {
		return 0
	}

	// If deleting storage, no surcharge (we want to incentivize cleanup)
	if isDelete {
		return 0
	}

	// Use nonce as proxy for storage activity
	nonce := evm.StateDB.GetNonce(addr)
	if nonce <= sr.FreeSlotThreshold {
		return 0
	}

	surcharge := (nonce - sr.FreeSlotThreshold) * sr.BaseRentPerSlot
	if surcharge > sr.MaxRentSurcharge {
		surcharge = sr.MaxRentSurcharge
	}
	return surcharge
}

// gasSStoreWithRent wraps the standard SSTORE gas calculation and adds the rent surcharge.
// This replaces gasSStoreEIP2200 when the Noven fork is active.
func gasSStoreWithRent(evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	// Calculate standard SSTORE gas first
	baseCost, err := gasSStoreEIP2200(evm, contract, stack, mem, memorySize)
	if err != nil {
		return 0, err
	}

	// Determine if this is a delete operation (new value is zero)
	y := stack.Back(1)
	isDelete := y.IsZero()

	// Calculate rent surcharge
	surcharge := GlobalStateRent.CalculateSurcharge(evm, contract.Address(), isDelete)
	if surcharge == 0 {
		return baseCost, nil
	}

	total, overflow := math.SafeAdd(baseCost, surcharge)
	if overflow {
		return 0, ErrGasUintOverflow
	}
	return total, nil
}
