// Copyright 2014 The go-ethereum Authors
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
	"fmt"
)

// List evm execution errors
var (
	ErrOutOfGas                 = errors.New("out of gas")
	ErrCodeStoreOutOfGas        = errors.New("contract creation code storage out of gas")
	ErrDepth                    = errors.New("max call depth exceeded")
	ErrInsufficientBalance      = errors.New("insufficient balance for transfer")
	ErrContractAddressCollision = errors.New("contract address collision")
	ErrExecutionReverted        = errors.New("execution reverted")
	ErrMaxInitCodeSizeExceeded  = errors.New("max initcode size exceeded")
	ErrMaxCodeSizeExceeded      = errors.New("max code size exceeded")
	ErrInvalidJump              = errors.New("invalid jump destination")
	ErrWriteProtection          = errors.New("write protection")
	ErrReturnDataOutOfBounds    = errors.New("return data out of bounds")
	ErrGasUintOverflow          = errors.New("gas uint64 overflow")
	ErrInvalidCode              = errors.New("invalid code: must not begin with 0xef")
	ErrNonceUintOverflow        = errors.New("nonce uint64 overflow")

	// NIP-0004 Phase 10D — per-dimension out-of-resource errors.
	// These are raised by the resource enforcer in core/state_transition.go
	// when a transaction exceeds its declared per-dimension limit. They
	// behave like ErrOutOfGas — the transaction is reverted, intrinsic gas
	// is still charged, and the receipt is marked failed.
	ErrOutOfResourceCompute     = errors.New("out of resource: compute dimension exhausted")
	ErrOutOfResourceStateRead   = errors.New("out of resource: state_read dimension exhausted")
	ErrOutOfResourceStateWrite  = errors.New("out of resource: state_write dimension exhausted")
	ErrOutOfResourceProtocolOps = errors.New("out of resource: protocol_ops dimension exhausted")
	ErrOutOfResourceProofVerify = errors.New("out of resource: proof_verify dimension exhausted")

	// errStopToken is an internal token indicating interpreter loop termination,
	// never returned to outside callers.
	errStopToken = errors.New("stop token")
)

// IsOutOfResourceError reports whether err is one of the per-dimension
// out-of-resource errors introduced by NIP-0004 Phase 10D.
func IsOutOfResourceError(err error) bool {
	switch err {
	case ErrOutOfResourceCompute,
		ErrOutOfResourceStateRead,
		ErrOutOfResourceStateWrite,
		ErrOutOfResourceProtocolOps,
		ErrOutOfResourceProofVerify:
		return true
	}
	return false
}

// ErrStackUnderflow wraps an evm error when the items on the stack less
// than the minimal requirement.
type ErrStackUnderflow struct {
	stackLen int
	required int
}

func (e *ErrStackUnderflow) Error() string {
	return fmt.Sprintf("stack underflow (%d <=> %d)", e.stackLen, e.required)
}

// ErrStackOverflow wraps an evm error when the items on the stack exceeds
// the maximum allowance.
type ErrStackOverflow struct {
	stackLen int
	limit    int
}

func (e *ErrStackOverflow) Error() string {
	return fmt.Sprintf("stack limit reached %d (%d)", e.stackLen, e.limit)
}

// ErrInvalidOpCode wraps an evm error when an invalid opcode is encountered.
type ErrInvalidOpCode struct {
	opcode OpCode
}

func (e *ErrInvalidOpCode) Error() string { return fmt.Sprintf("invalid opcode: %s", e.opcode) }

func NewErrInvalidOpCode(opcode OpCode) error {
	return &ErrInvalidOpCode{opcode}
}
