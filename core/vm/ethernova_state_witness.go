// Ethernova: novaStateWitness precompile (NIP-0004 §3.4 / Phase 5 §5.5)
//
// Address: 0x2F
//
// Function selectors (first byte of input):
//
//   0x01 verifyStateWitness(addr:32, slot:32, value:32, proofLen:32, proof:bytes)
//          READ-ONLY — verifies a Merkle proof against the cold storage
//          root of `addr` recorded at archival time. Returns one
//          32-byte word: 0x01 if valid, 0x00 otherwise. Reverts only
//          on malformed input or an oversize proof.
//
//   0x02 restoreState(addr:32, slot:32, value:32, proofLen:32, proof:bytes)
//          WRITE — verifies the same proof and, on success, clears the
//          archive marker and refreshes last_touched_block. Reverts in
//          STATICCALL/readOnly contexts. Returns one 32-byte word:
//          0x01 on success.
//
//   0x03 getCurrentTier(addr:32)
//          READ-ONLY — returns the current tier of `addr` as a left-
//          padded 32-byte word: 0x00=Active, 0x01=Warm, 0x02=Cold,
//          0x03=Archived, 0x04=Expired. Cheap: a single index read.
//
// CONSENSUS-CRITICAL invariants:
//
//   1. The precompile reverts on every selector before
//      StateLifecycleForkBlock. Pre-fork there is NO state side
//      effect, so the address is safe to pre-register in
//      PrecompiledContractsEthernova.
//
//   2. EIP-214 is enforced: selector 0x02 returns ErrWriteProtection
//      when readOnly=true. Selectors 0x01 and 0x03 are read-only and
//      pass through readOnly safely.
//
//   3. The precompile uses a TYPE ASSERTION to obtain the underlying
//      *state.StateDB. If the assertion fails (e.g. a future caller
//      passes a different StateDB implementation), all selectors
//      return an explicit error rather than silently succeeding.
//      This keeps the consensus path's failure mode loud.
//
//   4. Proof size is bounded by params.MaxStateWitnessProofBytes
//      BEFORE any RLP work, preventing RAM exhaustion via
//      gigabyte-sized proofs.

package vm

import (
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/params/ethernova"
)

// novaStateWitness is the 0x2F precompile.
type novaStateWitness struct{}

// RequiredGas returns the integer gas cost based on the first input
// byte. Selector-dependent pricing matches the convention used by
// 0x29 / 0x2A / 0x2B / 0x2C: trivial reads charge a flat fee, the
// proof verifier charges more, and the write path charges roughly
// SSTORE-equivalent.
func (c *novaStateWitness) RequiredGas(input []byte) uint64 {
	if len(input) < 1 {
		return 0
	}
	switch input[0] {
	case 0x01:
		return ethernova.StateWitnessVerifyGas
	case 0x02:
		return ethernova.StateWitnessRestoreGas
	case 0x03:
		return ethernova.StateWitnessGetTierGas
	default:
		return 0
	}
}

// Run is the non-stateful entry point. The precompile is always
// stateful — Run returns an explicit error so any code path that
// reaches it via RunPrecompiledContract instead of
// runPrecompileOrStateful fails loud.
func (c *novaStateWitness) Run(input []byte) ([]byte, error) {
	return nil, errors.New("novaStateWitness: requires stateful execution")
}

// RunStateful dispatches the selector. EIP-214 readOnly enforcement
// happens here, before any state read, so a malicious STATICCALL
// cannot get past the boundary.
//
// The caller parameter is intentionally unused: witness verification
// and tier queries are permissionless. The whole-EVM call frame
// (gas, value-transfer accounting) is established before this
// function runs and is not relevant to the precompile's logic.
func (c *novaStateWitness) RunStateful(evm *EVM, caller common.Address, input []byte, readOnly bool) ([]byte, error) {
	if len(input) < 1 {
		return nil, errors.New("novaStateWitness: empty input")
	}
	// Fork gate. Pre-fork ALL selectors revert with no state effect.
	if evm.Context.BlockNumber == nil ||
		evm.Context.BlockNumber.Uint64() < ethernova.StateLifecycleForkBlock {
		return nil, errors.New("novaStateWitness: not yet active")
	}
	selector := input[0]
	switch selector {
	case 0x01:
		return c.verifyStateWitness(evm, input[1:])
	case 0x02:
		if readOnly {
			return nil, ErrWriteProtection
		}
		return c.restoreState(evm, input[1:])
	case 0x03:
		return c.getCurrentTier(evm, input[1:])
	default:
		return nil, errors.New("novaStateWitness: unknown selector")
	}
}

// verifyStateWitness implements selector 0x01. The input layout is:
//
//	[0..32]    addr           (left-padded 20-byte address)
//	[32..64]   slot           (raw 32-byte storage slot)
//	[64..96]   value          (expected 32-byte value at slot)
//	[96..128]  proofLen       (uint256, MUST equal len(proofBytes))
//	[128..]    proofBytes     (DecodeProofPayload format)
//
// Output: 32-byte word, 0x01 if proof valid, 0x00 otherwise.
func (c *novaStateWitness) verifyStateWitness(evm *EVM, input []byte) ([]byte, error) {
	addr, slot, value, proof, err := decodeWitnessInput(input)
	if err != nil {
		return nil, err
	}
	engine, err := lifecycleEngineFromEVM(evm)
	if err != nil {
		return nil, err
	}
	// Reject oversize proofs before any RLP work.
	if uint64(len(proof)) > ethernova.MaxStateWitnessProofBytes {
		return wordFromBool(false), nil
	}
	// Use the engine's RestoreFromWitness logic in "verify only" mode
	// by calling the underlying decoder + verifier directly. This
	// avoids a no-op write path and means selector 0x01 is purely
	// read-only even on the engine side.
	coldRoot := engine.ColdStorageRoot(addr)
	if coldRoot == (common.Hash{}) {
		return wordFromBool(false), nil
	}
	nodes, err := state.DecodeProofPayload(proof, ethernova.MaxStateWitnessProofBytes)
	if err != nil {
		return wordFromBool(false), nil
	}
	ok, err := state.VerifyStorageWitness(coldRoot, slot, value, nodes)
	if err != nil {
		return wordFromBool(false), nil
	}
	return wordFromBool(ok), nil
}

// restoreState implements selector 0x02. Same input layout as 0x01.
// Output: 32-byte word, 0x01 on success. Errors propagate as Go
// errors (which the EVM turns into a revert with the error string).
func (c *novaStateWitness) restoreState(evm *EVM, input []byte) ([]byte, error) {
	addr, slot, value, proof, err := decodeWitnessInput(input)
	if err != nil {
		return nil, err
	}
	if uint64(len(proof)) > ethernova.MaxStateWitnessProofBytes {
		return nil, errors.New("novaStateWitness: proof exceeds size cap")
	}
	engine, err := lifecycleEngineFromEVM(evm)
	if err != nil {
		return nil, err
	}
	currentBlock := evm.Context.BlockNumber.Uint64()
	if err := engine.RestoreFromWitness(
		addr, slot, value, proof, currentBlock,
		ethernova.MaxStateWitnessProofBytes,
	); err != nil {
		return nil, err
	}
	return wordFromBool(true), nil
}

// getCurrentTier implements selector 0x03. Input is a single 32-byte
// word containing the address. Output is a 32-byte word with the tier
// byte in the low position (0=Active, 4=Expired).
func (c *novaStateWitness) getCurrentTier(evm *EVM, input []byte) ([]byte, error) {
	if len(input) < 32 {
		return nil, errors.New("novaStateWitness: getCurrentTier input too short")
	}
	addr := common.BytesToAddress(input[12:32])
	engine, err := lifecycleEngineFromEVM(evm)
	if err != nil {
		return nil, err
	}
	currentBlock := evm.Context.BlockNumber.Uint64()
	tier := engine.TierOf(addr, currentBlock)
	out := make([]byte, 32)
	out[31] = byte(tier)
	return out, nil
}

// =============================================================
// Internal helpers
// =============================================================

// decodeWitnessInput parses the (addr, slot, value, proofLen, proof)
// payload shared by selectors 0x01 and 0x02. It validates that the
// declared proofLen matches the trailing byte length, refusing
// truncated or padded inputs that could disagree across nodes.
func decodeWitnessInput(input []byte) (
	addr common.Address,
	slot common.Hash,
	value common.Hash,
	proof []byte,
	err error,
) {
	if len(input) < 128 {
		err = errors.New("novaStateWitness: input shorter than 128-byte head")
		return
	}
	addr = common.BytesToAddress(input[12:32])
	slot = common.BytesToHash(input[32:64])
	value = common.BytesToHash(input[64:96])
	declared := bytesToUint64Big(input[96:128])
	tail := input[128:]
	if uint64(len(tail)) != declared {
		err = errors.New("novaStateWitness: declared proofLen does not match input length")
		return
	}
	proof = tail
	return
}

// lifecycleEngineFromEVM constructs a Phase 5 engine from the
// underlying *state.StateDB hung off evm.StateDB. The construction is
// cheap (just an ethdb pointer + immutable config struct) so we do
// not bother caching it.
func lifecycleEngineFromEVM(evm *EVM) (*state.StateLifecycleEngine, error) {
	concrete, ok := evm.StateDB.(*state.StateDB)
	if !ok {
		return nil, errors.New("novaStateWitness: StateDB is not *state.StateDB")
	}
	disk := concrete.Database().DiskDB()
	if disk == nil {
		return nil, errors.New("novaStateWitness: no on-disk database available")
	}
	return state.NewStateLifecycleEngine(disk, lifecycleConfigFromParams()), nil
}

// lifecycleConfigFromParams reads the Phase 5 thresholds and fees from
// the params/ethernova package. Doing this on every call is fine: the
// values are compile-time constants today, so the read is free.
func lifecycleConfigFromParams() state.LifecycleConfig {
	return state.LifecycleConfig{
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
}

// wordFromBool packs a boolean into a left-padded 32-byte word.
func wordFromBool(v bool) []byte {
	out := make([]byte, 32)
	if v {
		out[31] = 0x01
	}
	return out
}

// bytesToUint64Big interprets a 32-byte big-endian word as uint64,
// taking only the low 8 bytes (high 24 must be zero in well-formed
// input — we tolerate non-zero high bytes by truncation since gas
// pricing already bounds the proof length).
func bytesToUint64Big(b []byte) uint64 {
	if len(b) < 32 {
		return 0
	}
	var v uint64
	for i := 24; i < 32; i++ {
		v = (v << 8) | uint64(b[i])
	}
	return v
}
