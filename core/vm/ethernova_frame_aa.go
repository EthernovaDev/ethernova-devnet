// Ethernova: Frame-Style Account Abstraction (Phase 12) — DEPRECATED STUBS
//
// DO NOT RE-ENABLE WITHOUT FULLY WIRING THE TRANSACTION PROCESSOR.
//
// The original implementation stored approvals and introspection state in
// package-level globals (GlobalFrameApprovals, FrameIntrospectionData) that
// were mutated by the precompiles but NEVER READ by the transaction
// processor. FrameIntrospectionData was never written either, so
// novaFrameIntrospect always returned "index out of range". The Reset path
// was defined but never called, which would have produced cross-transaction
// leakage even if the globals were wired up.
//
// The precompile addresses (0x23, 0x24) remain registered so gas accounting
// does not change for any caller — both entry points now return an explicit
// "not implemented" error. When Frame-AA is really shipped, the
// implementation must:
//   1. Consult the tx processor directly (no package-level globals).
//   2. Populate FrameIntrospectionData at the start of each frame.
//   3. Reset approval state between transactions.
//   4. Move the approval decision out of the precompile and into the
//      tx validation path where it can actually influence execution.
//
// Until then, these stubs fail loudly rather than silently succeed.

package vm

import (
	"errors"
)

var errFrameAANotImplemented = errors.New("frame-AA: not implemented on this network; precompile is a deprecated stub")

// novaFrameApprove (0x23) — originally mutated a package-level global that
// nothing read. Returns an explicit error so callers see the deprecation.
type novaFrameApprove struct{}

func (c *novaFrameApprove) RequiredGas(input []byte) uint64 {
	return 5000
}

func (c *novaFrameApprove) Run(input []byte) ([]byte, error) {
	return nil, errFrameAANotImplemented
}

// novaFrameIntrospect (0x24) — originally read a package-level slice that
// was never populated, so every call returned "frame index out of range".
// Returns the same explicit error now.
type novaFrameIntrospect struct{}

func (c *novaFrameIntrospect) RequiredGas(input []byte) uint64 {
	return 2000
}

func (c *novaFrameIntrospect) Run(input []byte) ([]byte, error) {
	return nil, errFrameAANotImplemented
}
