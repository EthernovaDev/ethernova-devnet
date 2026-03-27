// Ethernova: Gas Refund on Revert (Phase 18)
// When a transaction reverts, refund unused gas back to the sender.
// On Ethereum, users pay full gas even when transactions fail.
// On Ethernova, failed transactions get a partial refund.
//
// ANTI-DoS PROTECTION (credit: Gemini review):
// A malicious contract could do heavy computation then REVERT at the end,
// getting 90% refund while wasting miner CPU. To prevent this:
// - Refund ONLY applies if execution gas < MaxRefundableGas (100,000)
// - Heavy transactions (>100k gas) that revert pay FULL gas (no refund)
// - This protects miners from computational DoS attacks
// - Simple failed txs (wrong nonce, insufficient balance, small reverts) still get refunds

package vm

const (
	// MaxRefundableGas is the maximum execution gas eligible for revert refund.
	// Transactions using more gas than this get NO refund on revert.
	// This prevents DoS attacks where attacker does heavy computation then reverts.
	MaxRefundableGas uint64 = 100000

	// RefundPercent is the percentage of execution gas refunded on revert.
	RefundPercent uint64 = 90
)

// CalculateRevertRefund computes how much gas to refund on a reverted transaction.
// Returns 0 if the transaction used too much gas (anti-DoS).
func CalculateRevertRefund(gasUsed uint64, baseGas uint64) uint64 {
	if gasUsed <= baseGas {
		return 0
	}
	executionGas := gasUsed - baseGas

	// ANTI-DoS: No refund for heavy transactions
	// This prevents attackers from doing expensive computation then reverting
	if executionGas > MaxRefundableGas {
		return 0
	}

	refund := executionGas * RefundPercent / 100
	return refund
}

// RevertRefundEnabled controls whether revert refunds are active.
// SAFETY: Changed from var to const. This is a consensus-critical
// parameter — if it differs between nodes, gasUsed diverges → BAD BLOCK.
// A var could be accidentally toggled via RPC; a const cannot.
const RevertRefundEnabled = true