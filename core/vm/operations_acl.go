// Copyright 2020 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package vm

import (
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/params/ethernova"
	"github.com/ethereum/go-ethereum/params/vars"
)

// applyLifecycleSurcharge adds the Phase 5 warming-fee surcharge to a
// base gas amount when the contract address is in a non-Active tier.
// Returns the (possibly-unchanged) gas amount and any overflow error.
//
// SCOPE NOTE (Phase 5 v1): the surcharge is applied to SLOAD only,
// not SSTORE. SSTORE behavior is to PROMOTE the touched account back
// to Active (the touch list updated by the consensus engine at end
// of block makes this automatic), so the very next SLOAD is cheap
// again. Charging on SSTORE as well would double-bill the same
// access and would require touching the SSTORE refund table, which
// is too risky to bundle into Phase 5.
//
// CONSENSUS-CRITICAL invariants:
//
//  1. Pre-fork (block < StateLifecycleForkBlock) this returns base
//     unchanged. The function MUST be a strict no-op before the
//     fork to preserve gas costs across the activation boundary.
//
//  2. The tier is computed from the external Phase 5 LevelDB index
//     via the state.StateLifecycleEngine. The engine reads only
//     LevelDB; it never touches the state trie. So this surcharge
//     cannot create a state-root divergence.
//
//  3. ComputeWarmingFee uses overflow-safe uint64 multiplication
//     and saturates at MaxUint64 on overflow, so ErrGasUintOverflow
//     arises only from the SafeAdd on top of base gas.
//
//  4. The chain DB used for the lookup comes from the package-global
//     registered at node startup via SetLifecycleDB. This avoids the
//     type-assertion-on-StateDB anti-pattern that was failing during
//     eth_estimateGas / eth_call simulation paths where the StateDB
//     is a copy or wrapper that doesn't expose the disk DB. The
//     simulation StateDB and the production StateDB read from the
//     SAME LevelDB now, so estimate gas matches mined gas.
//
//  5. If the global has not been set (test harness, very early init)
//     we fall through to the legacy type-assertion path, then to a
//     final no-op. The fallthrough is the conservative default —
//     consensus is preserved (every node that lacks the registration
//     applies the same zero surcharge), but estimate accuracy is
//     lost. Production startup always sets the global.
func applyLifecycleSurcharge(evm *EVM, contract *Contract, base uint64) (uint64, error) {
	// Cheap fork gate first — every SLOAD hits this path.
	if evm.Context.BlockNumber == nil ||
		evm.Context.BlockNumber.Uint64() < ethernova.LifecycleSloadSurchargeForkBlock {
		return base, nil
	}

	// Resolve the chain DB. Primary path: package-global registered at
	// node startup (works for ANY StateDB type — production, copy,
	// override, simulated). Fallback path: type-assert evm.StateDB to
	// *state.StateDB and reach DiskDB through it (works only for the
	// production StateDB; silently drops to no-op in simulations).
	disk := getLifecycleDB()
	if disk == nil {
		concrete, ok := evm.StateDB.(*state.StateDB)
		if !ok {
			return base, nil
		}
		if concrete.Database() == nil {
			return base, nil
		}
		disk = concrete.Database().DiskDB()
		if disk == nil {
			return base, nil
		}
	}

	cfg := state.LifecycleConfig{
		Thresholds: state.LifecycleThresholds{
			ActiveBlocks: ethernova.ActiveTierBlocks,
			WarmBlocks:   ethernova.WarmTierBlocks,
			ColdBlocks:   ethernova.ColdTierBlocks,
		},
		Fees: state.LifecycleFees{
			PerByte: ethernova.WarmingFeePerByte,
		},
		MaxSweepPerBlock: ethernova.MaxLifecycleSweepPerBlock,
	}
	engine := state.NewStateLifecycleEngine(disk, cfg)
	tier := engine.TierOf(contract.Address(), evm.Context.BlockNumber.Uint64())
	if tier == state.TierActive {
		return base, nil
	}
	surcharge := state.ComputeWarmingFee(tier, ethernova.LifecycleStorageSlotSize, cfg.Fees)
	if surcharge == 0 {
		return base, nil
	}
	out, overflow := math.SafeAdd(base, surcharge)
	if overflow {
		return 0, ErrGasUintOverflow
	}
	return out, nil
}

func makeGasSStoreFunc(clearingRefund uint64) gasFunc {
	return func(evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
		// If we fail the minimum gas availability invariant, fail (0)
		if contract.Gas <= vars.SstoreSentryGasEIP2200 {
			return 0, errors.New("not enough gas for reentrancy sentry")
		}
		// Gas sentry honoured, do the actual gas calculation based on the stored value
		var (
			y, x    = stack.Back(1), stack.peek()
			slot    = common.Hash(x.Bytes32())
			current = evm.StateDB.GetState(contract.Address(), slot)
			cost    = uint64(0)
		)
		// Check slot presence in the access list
		if addrPresent, slotPresent := evm.StateDB.SlotInAccessList(contract.Address(), slot); !slotPresent {
			cost = vars.ColdSloadCostEIP2929
			// If the caller cannot afford the cost, this change will be rolled back
			evm.StateDB.AddSlotToAccessList(contract.Address(), slot)
			if !addrPresent {
				// Once we're done with YOLOv2 and schedule this for mainnet, might
				// be good to remove this panic here, which is just really a
				// canary to have during testing
				panic("impossible case: address was not present in access list during sstore op")
			}
		}
		value := common.Hash(y.Bytes32())

		if current == value { // noop (1)
			// EIP 2200 original clause:
			//		return params.SloadGasEIP2200, nil
			return cost + vars.WarmStorageReadCostEIP2929, nil // SLOAD_GAS
		}
		original := evm.StateDB.GetCommittedState(contract.Address(), x.Bytes32())
		if original == current {
			if original == (common.Hash{}) { // create slot (2.1.1)
				return cost + vars.SstoreSetGasEIP2200, nil
			}
			if value == (common.Hash{}) { // delete slot (2.1.2b)
				evm.StateDB.AddRefund(clearingRefund)
			}
			// EIP-2200 original clause:
			//		return vars.SstoreResetGasEIP2200, nil // write existing slot (2.1.2)
			return cost + (vars.SstoreResetGasEIP2200 - vars.ColdSloadCostEIP2929), nil // write existing slot (2.1.2)
		}
		if original != (common.Hash{}) {
			if current == (common.Hash{}) { // recreate slot (2.2.1.1)
				evm.StateDB.SubRefund(clearingRefund)
			} else if value == (common.Hash{}) { // delete slot (2.2.1.2)
				evm.StateDB.AddRefund(clearingRefund)
			}
		}
		if original == value {
			if original == (common.Hash{}) { // reset to original inexistent slot (2.2.2.1)
				// EIP 2200 Original clause:
				// evm.StateDB.AddRefund(vars.SstoreSetGasEIP2200 - params.SloadGasEIP2200)
				evm.StateDB.AddRefund(vars.SstoreSetGasEIP2200 - vars.WarmStorageReadCostEIP2929)
			} else { // reset to original existing slot (2.2.2.2)
				// EIP 2200 Original clause:
				//	evm.StateDB.AddRefund(vars.SstoreResetGasEIP2200 - params.SloadGasEIP2200)
				// - SSTORE_RESET_GAS redefined as (5000 - COLD_SLOAD_COST)
				// - SLOAD_GAS redefined as WARM_STORAGE_READ_COST
				// Final: (5000 - COLD_SLOAD_COST) - WARM_STORAGE_READ_COST
				evm.StateDB.AddRefund((vars.SstoreResetGasEIP2200 - vars.ColdSloadCostEIP2929) - vars.WarmStorageReadCostEIP2929)
			}
		}
		// EIP-2200 original clause:
		// return params.SloadGasEIP2200, nil // dirty update (2.2)
		return cost + vars.WarmStorageReadCostEIP2929, nil // dirty update (2.2)
	}
}

// gasSLoadEIP2929 calculates dynamic gas for SLOAD according to EIP-2929
// For SLOAD, if the (address, storage_key) pair (where address is the address of the contract
// whose storage is being read) is not yet in accessed_storage_keys,
// charge 2100 gas and add the pair to accessed_storage_keys.
// If the pair is already in accessed_storage_keys, charge 100 gas.
func gasSLoadEIP2929(evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	loc := stack.peek()
	slot := common.Hash(loc.Bytes32())
	// Check slot presence in the access list
	if _, slotPresent := evm.StateDB.SlotInAccessList(contract.Address(), slot); !slotPresent {
		// If the caller cannot afford the cost, this change will be rolled back
		// If he does afford it, we can skip checking the same thing later on, during execution
		evm.StateDB.AddSlotToAccessList(contract.Address(), slot)
		// Phase 5: apply lifecycle warming-fee surcharge on the cold
		// access path. Pre-fork this returns the base unchanged.
		return applyLifecycleSurcharge(evm, contract, vars.ColdSloadCostEIP2929)
	}
	// Phase 5: apply lifecycle warming-fee surcharge on the warm
	// access path. Pre-fork this returns the base unchanged.
	return applyLifecycleSurcharge(evm, contract, vars.WarmStorageReadCostEIP2929)
}

// makeGasSLoadLifecycle wraps pre-EIP-2929 SLOAD pricing with the Phase 5
// warming-fee surcharge. On this chain EIP-2929 is not active, so SLOAD is
// normally a constant-gas opcode (800 after EIP-2200) and would otherwise never
// enter gasSLoadEIP2929.
func makeGasSLoadLifecycle(base uint64) gasFunc {
	return func(evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
		return applyLifecycleSurcharge(evm, contract, base)
	}
}

// gasExtCodeCopyEIP2929 implements extcodecopy according to EIP-2929
// EIP spec:
// > If the target is not in accessed_addresses,
// > charge COLD_ACCOUNT_ACCESS_COST gas, and add the address to accessed_addresses.
// > Otherwise, charge WARM_STORAGE_READ_COST gas.
func gasExtCodeCopyEIP2929(evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	// memory expansion first (dynamic part of pre-2929 implementation)
	gas, err := gasExtCodeCopy(evm, contract, stack, mem, memorySize)
	if err != nil {
		return 0, err
	}
	addr := common.Address(stack.peek().Bytes20())
	// Check slot presence in the access list
	if !evm.StateDB.AddressInAccessList(addr) {
		evm.StateDB.AddAddressToAccessList(addr)
		var overflow bool
		// We charge (cold-warm), since 'warm' is already charged as constantGas
		if gas, overflow = math.SafeAdd(gas, vars.ColdAccountAccessCostEIP2929-vars.WarmStorageReadCostEIP2929); overflow {
			return 0, ErrGasUintOverflow
		}
		return gas, nil
	}
	return gas, nil
}

// gasEip2929AccountCheck checks whether the first stack item (as address) is present in the access list.
// If it is, this method returns '0', otherwise 'cold-warm' gas, presuming that the opcode using it
// is also using 'warm' as constant factor.
// This method is used by:
// - extcodehash,
// - extcodesize,
// - (ext) balance
func gasEip2929AccountCheck(evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	addr := common.Address(stack.peek().Bytes20())
	// Check slot presence in the access list
	if !evm.StateDB.AddressInAccessList(addr) {
		// If the caller cannot afford the cost, this change will be rolled back
		evm.StateDB.AddAddressToAccessList(addr)
		// The warm storage read cost is already charged as constantGas
		return vars.ColdAccountAccessCostEIP2929 - vars.WarmStorageReadCostEIP2929, nil
	}
	return 0, nil
}

func makeCallVariantGasCallEIP2929(oldCalculator gasFunc) gasFunc {
	return func(evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
		addr := common.Address(stack.Back(1).Bytes20())
		// Check slot presence in the access list
		warmAccess := evm.StateDB.AddressInAccessList(addr)
		// The WarmStorageReadCostEIP2929 (100) is already deducted in the form of a constant cost, so
		// the cost to charge for cold access, if any, is Cold - Warm
		coldCost := vars.ColdAccountAccessCostEIP2929 - vars.WarmStorageReadCostEIP2929
		if !warmAccess {
			evm.StateDB.AddAddressToAccessList(addr)
			// Charge the remaining difference here already, to correctly calculate available
			// gas for call
			if !contract.UseGas(coldCost) {
				return 0, ErrOutOfGas
			}
		}
		// Now call the old calculator, which takes into account
		// - create new account
		// - transfer value
		// - memory expansion
		// - 63/64ths rule
		gas, err := oldCalculator(evm, contract, stack, mem, memorySize)
		if warmAccess || err != nil {
			return gas, err
		}
		// In case of a cold access, we temporarily add the cold charge back, and also
		// add it to the returned gas. By adding it to the return, it will be charged
		// outside of this function, as part of the dynamic gas, and that will make it
		// also become correctly reported to tracers.
		contract.Gas += coldCost

		var overflow bool
		if gas, overflow = math.SafeAdd(gas, coldCost); overflow {
			return 0, ErrGasUintOverflow
		}
		return gas, nil
	}
}

var (
	gasCallEIP2929         = makeCallVariantGasCallEIP2929(gasCall)
	gasDelegateCallEIP2929 = makeCallVariantGasCallEIP2929(gasDelegateCall)
	gasStaticCallEIP2929   = makeCallVariantGasCallEIP2929(gasStaticCall)
	gasCallCodeEIP2929     = makeCallVariantGasCallEIP2929(gasCallCode)
	gasSelfdestructEIP2929 = makeSelfdestructGasFn(true)
	// gasSelfdestructEIP3529 implements the changes in EIP-3529 (no refunds)
	gasSelfdestructEIP3529 = makeSelfdestructGasFn(false)

	// gasSStoreEIP2929 implements gas cost for SSTORE according to EIP-2929
	//
	// When calling SSTORE, check if the (address, storage_key) pair is in accessed_storage_keys.
	// If it is not, charge an additional COLD_SLOAD_COST gas, and add the pair to accessed_storage_keys.
	// Additionally, modify the parameters defined in EIP 2200 as follows:
	//
	// Parameter 	Old value 	New value
	// SLOAD_GAS 	800 	= WARM_STORAGE_READ_COST
	// SSTORE_RESET_GAS 	5000 	5000 - COLD_SLOAD_COST
	//
	//The other parameters defined in EIP 2200 are unchanged.
	// see gasSStoreEIP2200(...) in core/vm/gas_table.go for more info about how EIP 2200 is specified
	gasSStoreEIP2929 = makeGasSStoreFunc(vars.SstoreClearsScheduleRefundEIP2200)

	// gasSStoreEIP3529 implements gas cost for SSTORE according to EIP-3529
	// Replace `SSTORE_CLEARS_SCHEDULE` with `SSTORE_RESET_GAS + ACCESS_LIST_STORAGE_KEY_COST` (4,800)
	gasSStoreEIP3529 = makeGasSStoreFunc(vars.SstoreClearsScheduleRefundEIP3529)
)

// makeSelfdestructGasFn can create the selfdestruct dynamic gas function for EIP-2929 and EIP-3529
func makeSelfdestructGasFn(refundsEnabled bool) gasFunc {
	gasFunc := func(evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
		var (
			gas     uint64
			address = common.Address(stack.peek().Bytes20())
		)
		if !evm.StateDB.AddressInAccessList(address) {
			// If the caller cannot afford the cost, this change will be rolled back
			evm.StateDB.AddAddressToAccessList(address)
			gas = vars.ColdAccountAccessCostEIP2929
		}
		// if empty and transfers value
		if evm.StateDB.Empty(address) && evm.StateDB.GetBalance(contract.Address()).Sign() != 0 {
			gas += vars.CreateBySelfdestructGas
		}
		if refundsEnabled && !evm.StateDB.HasSelfDestructed(contract.Address()) {
			evm.StateDB.AddRefund(vars.SelfdestructRefundGas)
		}
		return gas, nil
	}
	return gasFunc
}
