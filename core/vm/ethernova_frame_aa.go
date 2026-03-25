// Ethernova: Frame-Style Account Abstraction (Phase 12)
//
// Inspired by EIP-8141 Frame Transactions. Instead of adding a new EVM opcode
// (which breaks every tool), we implement the APPROVE and INTROSPECT mechanics
// as precompiled contracts:
//
// Address 0x23: novaFrameApprove - Smart contract wallets call this to approve/reject transactions
// Address 0x24: novaFrameIntrospect - Allows a frame to inspect other frames in the transaction
//
// This enables:
// - Smart contract wallets (passkeys, multisig, quantum-resistant signatures)
// - Conditional gas sponsorship
// - Privacy-compatible transactions (ZK proof validation)
// - Generalized permissions (delegate fine-grained tx execution rights)
//
// Gas is ALWAYS paid in NOVA. No ERC-20 gas payments.

package vm

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// Frame approval modes (matching EIP-8141 semantics)
const (
	FrameApproveTransaction = 0x00 // Approve sending the transaction
	FrameApproveGasPayment  = 0x01 // Approve paying gas for the transaction
	FrameApproveBoth        = 0x02 // Approve both sending and gas payment
)

// FrameApprovalStore tracks which contracts have approved the current transaction.
// This is reset for each new transaction and is consensus-critical.
type FrameApprovalStore struct {
	TransactionApprover common.Address // Contract that approved sending
	GasPayerApprover    common.Address // Contract that approved gas payment
	Approved            bool
	GasApproved         bool
}

// GlobalFrameApprovals is the per-transaction approval state.
// Reset at the start of each transaction processing.
var GlobalFrameApprovals = &FrameApprovalStore{}

// ResetApprovals clears all approvals for a new transaction.
func (fas *FrameApprovalStore) ResetApprovals() {
	fas.TransactionApprover = common.Address{}
	fas.GasPayerApprover = common.Address{}
	fas.Approved = false
	fas.GasApproved = false
}

// novaFrameApprove allows smart contracts to approve or reject transactions.
// Input: 1 byte mode (0x00=approve tx, 0x01=approve gas, 0x02=approve both)
// Output: 32 bytes (1 = success, 0 = failure)
// Gas: 5,000 per call
//
// This is the precompile equivalent of EIP-8141's APPROVE opcode.
// Smart contract wallets call this after validating the transaction
// (signature check, policy check, etc).
type novaFrameApprove struct{}

func (c *novaFrameApprove) RequiredGas(input []byte) uint64 {
	return 5000
}

func (c *novaFrameApprove) Run(input []byte) ([]byte, error) {
	if len(input) < 1 {
		return nil, errors.New("novaFrameApprove: input must be at least 1 byte (approval mode)")
	}

	mode := input[0]

	switch mode {
	case FrameApproveTransaction:
		GlobalFrameApprovals.Approved = true
	case FrameApproveGasPayment:
		GlobalFrameApprovals.GasApproved = true
	case FrameApproveBoth:
		GlobalFrameApprovals.Approved = true
		GlobalFrameApprovals.GasApproved = true
	default:
		return common.LeftPadBytes([]byte{0}, 32), errors.New("novaFrameApprove: invalid mode")
	}

	// Return success (1)
	return common.LeftPadBytes([]byte{1}, 32), nil
}

// novaFrameIntrospect allows a frame to inspect other frames in the transaction.
// This enables conditional logic like "only approve gas payment if the next frame
// transfers ERC20 tokens to my address".
//
// Input format:
//   - First 32 bytes: frame index to inspect (uint256)
//   - Next 4 bytes: field selector
//     0x01 = target address (returns 32 bytes, left-padded address)
//     0x02 = call data hash (returns 32 bytes, keccak256 of calldata)
//     0x03 = value (returns 32 bytes, uint256)
//     0x04 = gas limit (returns 32 bytes, uint256)
//     0x05 = call data first 4 bytes (function selector)
//
// Output: 32 bytes depending on field selector
// Gas: 2,000 per call
//
// Note: The actual frame data must be set by the transaction processor
// before executing each frame. This is stored in FrameIntrospectionData.
type novaFrameIntrospect struct{}

// FrameData holds the data for a single frame that can be introspected.
type FrameData struct {
	Target   common.Address
	Data     []byte
	Value    *big.Int
	GasLimit uint64
}

// FrameIntrospectionData holds all frames for the current transaction.
// Set by the transaction processor before execution.
var FrameIntrospectionData []FrameData

func (c *novaFrameIntrospect) RequiredGas(input []byte) uint64 {
	return 2000
}

func (c *novaFrameIntrospect) Run(input []byte) ([]byte, error) {
	if len(input) < 33 {
		return nil, errors.New("novaFrameIntrospect: need 32 bytes (index) + 1 byte (field)")
	}

	// Parse frame index (last byte of first 32 bytes for simplicity)
	idx := int(input[31])
	if idx >= len(FrameIntrospectionData) {
		return common.LeftPadBytes([]byte{0}, 32), errors.New("novaFrameIntrospect: frame index out of range")
	}

	frame := FrameIntrospectionData[idx]
	field := input[32]

	switch field {
	case 0x01: // target address
		return common.LeftPadBytes(frame.Target.Bytes(), 32), nil

	case 0x02: // call data hash
		if len(frame.Data) == 0 {
			return make([]byte, 32), nil
		}
		hash := crypto.Keccak256(frame.Data)
		return hash, nil

	case 0x03: // value
		if frame.Value == nil {
			return make([]byte, 32), nil
		}
		return common.LeftPadBytes(frame.Value.Bytes(), 32), nil

	case 0x04: // gas limit
		gasBytes := new(big.Int).SetUint64(frame.GasLimit).Bytes()
		return common.LeftPadBytes(gasBytes, 32), nil

	case 0x05: // function selector (first 4 bytes of calldata)
		result := make([]byte, 32)
		if len(frame.Data) >= 4 {
			copy(result[28:], frame.Data[:4])
		}
		return result, nil

	default:
		return common.LeftPadBytes([]byte{0}, 32), errors.New("novaFrameIntrospect: unknown field selector")
	}
}
