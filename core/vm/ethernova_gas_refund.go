// Ethernova: Gas Refund on Revert (Phase 18)
// When a transaction reverts, refund unused gas back to the sender.
// On Ethereum, users pay full gas even when transactions fail.
// On Ethernova, failed transactions only charge a minimal base fee.
//
// How it works:
// - Transaction executes normally
// - If it reverts, only charge 21,000 gas (base tx cost) + 10% of used gas
// - Refund the remaining 90% of gas consumed during the reverted execution
// - This protects users from losing money on failed transactions
//
// The 10% penalty prevents spam (can't submit infinite failing txs for free).

package vm

// CalculateRevertRefund computes how much gas to refund on a reverted transaction.
// baseGas = 21000 (intrinsic tx cost, always charged)
// gasUsed = total gas consumed during execution
// Returns the amount of gas to refund to the sender.
func CalculateRevertRefund(gasUsed uint64, baseGas uint64) uint64 {
	if gasUsed <= baseGas {
		return 0 // nothing to refund if only base gas was used
	}
	executionGas := gasUsed - baseGas
	// Refund 90% of execution gas, keep 10% as anti-spam penalty
	refund := executionGas * 90 / 100
	return refund
}

// RevertRefundEnabled controls whether revert refunds are active.
// Can be toggled via RPC for testing.
var RevertRefundEnabled = true
