// Ethernova: Rent Engine math helpers (NIP-0004 Phase 3)
//
// This file contains ONLY pure arithmetic. No state access, no RLP, no
// crypto. That is deliberate — keeping the rent math in core/state (a
// package that both core/vm and consensus/* can import without cycles)
// means both the precompile (which reads current balances) and the
// consensus Finalize() hook (which deducts at epoch boundaries) compute
// deductions via the identical code path.
//
// CONSENSUS-CRITICAL invariants enforced here:
//
//   1. Integer-only. No floating point, no math/big when inputs are
//      uint64 — only big.Int for the final wei deduction where the
//      product can exceed 2^64 (size * epochLength * rate).
//
//   2. Deterministic. All inputs are uint64 from block state or
//      compile-time constants. No time.Now(), no map iteration, no
//      random source.
//
//   3. Saturating on overflow. Over-sized inputs are rejected by
//      params.MaxContentRefSize long before reaching here, but the
//      multiplication still uses big.Int so a malformed input can never
//      wrap around silently — it produces a well-defined huge number
//      that the caller treats as "exceeds balance" (expired).

package state

import "math/big"

// ComputeEpochRentWei returns the wei rent owed for a single epoch by an
// object of the given size at the given rate and epoch length.
//
//	deducted = rate * size * epochLength
//
// All three inputs are uint64. The product can exceed 2^64 for large
// sizes (e.g. size=2^32, epochLength=10000, rate=1 -> 4e13 wei = fits,
// but size=2^32, epochLength=2^32, rate=2^32 overflows), so the result
// is returned as *big.Int. Callers compare against a *big.Int balance.
func ComputeEpochRentWei(rate, size, epochLength uint64) *big.Int {
	r := new(big.Int).SetUint64(rate)
	s := new(big.Int).SetUint64(size)
	e := new(big.Int).SetUint64(epochLength)
	r.Mul(r, s)
	r.Mul(r, e)
	return r
}

// IsRentEpochBoundary reports whether the given block number is an epoch
// boundary where rent is deducted. Block 0 is deliberately excluded —
// genesis has no "previous epoch" to charge for, and an object created
// at block 0 should not be charged at block 0 itself.
func IsRentEpochBoundary(blockNum, epochLength uint64) bool {
	if blockNum == 0 || epochLength == 0 {
		return false
	}
	return blockNum%epochLength == 0
}

// EpochsElapsed returns how many full epoch boundaries lie in the
// half-open interval (fromBlock, toBlock]. If fromBlock >= toBlock, the
// result is 0. Used by read-path lazy-expiry checks to compute how much
// rent has accrued since an object was last persisted.
//
// Examples (epochLength = 10):
//
//	from=0,  to=10  -> 1   (boundaries: {10})
//	from=0,  to=20  -> 2   (boundaries: {10, 20})
//	from=5,  to=25  -> 2   (boundaries: {10, 20})
//	from=10, to=10  -> 0   (strictly greater than from)
//	from=10, to=20  -> 1   (boundaries: {20})
//	from=10, to=11  -> 0
//
// The half-open lower bound is consistent with the persist-at-boundary
// design: when Finalize() runs at block B and B is a boundary, it
// charges for the epoch ending at B and updates last_touched_block = B.
// The next boundary to charge is therefore strictly > B.
func EpochsElapsed(fromBlock, toBlock, epochLength uint64) uint64 {
	if epochLength == 0 || toBlock <= fromBlock {
		return 0
	}
	// Count multiples of epochLength in (fromBlock, toBlock].
	// = floor(toBlock/epochLength) - floor(fromBlock/epochLength)
	return (toBlock / epochLength) - (fromBlock / epochLength)
}

// ComputeAccruedRentWei returns the total wei of rent accrued by an
// object between fromBlock (exclusive) and toBlock (inclusive). This is
// the lazy-computation path used by read-only precompile selectors
// (getContentRef, isValid) so that a fresh read at block B reports the
// effective balance as of block B without requiring the Finalize()
// deduction to have run yet.
//
// The persisted (on-trie) rent_balance is only reduced at epoch
// boundaries by Finalize(); reads subtract additional accrued rent for
// any epoch boundaries that have passed since last_touched_block.
func ComputeAccruedRentWei(rate, size, epochLength, fromBlock, toBlock uint64) *big.Int {
	epochs := EpochsElapsed(fromBlock, toBlock, epochLength)
	if epochs == 0 {
		return new(big.Int)
	}
	per := ComputeEpochRentWei(rate, size, epochLength)
	return per.Mul(per, new(big.Int).SetUint64(epochs))
}
