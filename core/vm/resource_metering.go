package vm

import (
	"math"
	"sync"

	"github.com/ethereum/go-ethereum/common"
)

// ResourceVector is the NIP-0004 Phase 10 five-dimensional usage model.
//
// Phase 10A is monitoring-only: vectors are derived deterministically from
// execution counters and precompile dispatch, but they do not change gas,
// receipts, headers, state roots, or block validity.
type ResourceVector struct {
	Compute     uint64 `json:"compute"`
	StateRead   uint64 `json:"stateRead"`
	StateWrite  uint64 `json:"stateWrite"`
	ProtocolOps uint64 `json:"protocolOps"`
	ProofVerify uint64 `json:"proofVerify"`
}

func (v ResourceVector) Add(other ResourceVector) ResourceVector {
	v.Compute += other.Compute
	v.StateRead += other.StateRead
	v.StateWrite += other.StateWrite
	v.ProtocolOps += other.ProtocolOps
	v.ProofVerify += other.ProofVerify
	return v
}

// ResourcePrices are the Phase 10B per-dimension static multipliers.
//
// Phase 10B activates pricing as a deterministic quote/telemetry surface only.
// Consensus gas charging remains unchanged until the extended transaction and
// adaptive-pricing substages have completed their devnet soak.
type ResourcePrices struct {
	Compute     uint64 `json:"compute"`
	StateRead   uint64 `json:"stateRead"`
	StateWrite  uint64 `json:"stateWrite"`
	ProtocolOps uint64 `json:"protocolOps"`
	ProofVerify uint64 `json:"proofVerify"`
}

// ResourceCharge is a priced ResourceVector plus the saturated total.
type ResourceCharge struct {
	Compute     uint64 `json:"compute"`
	StateRead   uint64 `json:"stateRead"`
	StateWrite  uint64 `json:"stateWrite"`
	ProtocolOps uint64 `json:"protocolOps"`
	ProofVerify uint64 `json:"proofVerify"`
	Total       uint64 `json:"total"`
}

// Phase10BResourcePrices returns conservative devnet multipliers. Protocol ops
// intentionally stay at 1 so chat/mailbox traffic is not penalized just because
// storage-heavy DeFi activity has different cost characteristics.
func Phase10BResourcePrices() ResourcePrices {
	return ResourcePrices{
		Compute:     1,
		StateRead:   2,
		StateWrite:  4,
		ProtocolOps: 1,
		ProofVerify: 3,
	}
}

// PriceResourceVector applies per-dimension prices with saturating arithmetic
// so RPC quotation can never overflow into a misleading low number.
func PriceResourceVector(v ResourceVector, prices ResourcePrices) ResourceCharge {
	charge := ResourceCharge{
		Compute:     saturatingMul(v.Compute, prices.Compute),
		StateRead:   saturatingMul(v.StateRead, prices.StateRead),
		StateWrite:  saturatingMul(v.StateWrite, prices.StateWrite),
		ProtocolOps: saturatingMul(v.ProtocolOps, prices.ProtocolOps),
		ProofVerify: saturatingMul(v.ProofVerify, prices.ProofVerify),
	}
	charge.Total = saturatingAdd(charge.Compute, charge.StateRead)
	charge.Total = saturatingAdd(charge.Total, charge.StateWrite)
	charge.Total = saturatingAdd(charge.Total, charge.ProtocolOps)
	charge.Total = saturatingAdd(charge.Total, charge.ProofVerify)
	return charge
}

func saturatingMul(a, b uint64) uint64 {
	if a == 0 || b == 0 {
		return 0
	}
	if a > math.MaxUint64/b {
		return math.MaxUint64
	}
	return a * b
}

func saturatingAdd(a, b uint64) uint64 {
	if math.MaxUint64-a < b {
		return math.MaxUint64
	}
	return a + b
}

// ResourceMeter tracks non-opcode resource usage during one transaction.
type ResourceMeter struct {
	vector ResourceVector
}

func (m *ResourceMeter) Reset() {
	m.vector = ResourceVector{}
}

func (m *ResourceMeter) Vector() ResourceVector {
	return m.vector
}

func (m *ResourceMeter) RecordPrecompile(addr common.Address, input []byte, gasCost uint64) {
	switch {
	case IsProofVerifyPrecompile(addr, input):
		m.vector.ProofVerify += gasCost
	case IsProtocolPrecompile(addr):
		m.vector.ProtocolOps += gasCost
	}
}

// IsProtocolPrecompile reports whether addr belongs to the Nova protocol-object
// surface. The legacy 0x20-0x28 native helpers are intentionally excluded here:
// Phase 10 focuses on NIP-0004 primitives and keeps Domain 0 behavior untouched.
func IsProtocolPrecompile(addr common.Address) bool {
	b := addr.Bytes()
	if len(b) == 0 {
		return false
	}
	last := b[len(b)-1]
	switch last {
	case 0x29, 0x2a, 0x2b, 0x2c, 0x2d, 0x2f, 0x35:
		return addr == common.BytesToAddress([]byte{last})
	default:
		return false
	}
}

// IsProofVerifyPrecompile identifies proof-heavy Phase 10 accounting paths.
// For Phase 10A this is deliberately narrow: StateWitness (0x2F) and session
// commit/close/dispute selectors, which include signature/proof verification.
func IsProofVerifyPrecompile(addr common.Address, input []byte) bool {
	if addr == common.BytesToAddress([]byte{0x2f}) {
		return true
	}
	if addr == common.BytesToAddress([]byte{0x2d}) && len(input) > 0 {
		switch input[0] {
		case 0x02, 0x03, 0x04:
			return true
		}
	}
	return false
}

// ResourceVectorFromExecution converts observed execution counters into the
// Phase 10A monitoring vector. The formula is intentionally stable and simple
// so Linux/Windows and miner/validator paths produce identical diagnostics.
func ResourceVectorFromExecution(tc *TraceCounters, precompile ResourceVector, gasUsed, intrinsicGas uint64) ResourceVector {
	if tc == nil {
		return precompile
	}
	var executionGas uint64
	if gasUsed > intrinsicGas {
		executionGas = gasUsed - intrinsicGas
	}
	v := ResourceVector{
		Compute:     executionGas,
		StateRead:   tc.SloadCount*2100 + tc.ExtCodeCount*700,
		StateWrite:  tc.SstoreCount*20000 + (tc.CreateCount+tc.Create2Count)*32000 + tc.SelfDestructCount*5000,
		ProtocolOps: precompile.ProtocolOps,
		ProofVerify: precompile.ProofVerify,
	}
	return v
}

// LegacyGasToResourceLimits maps a standard Ethereum gasLimit into default
// resource limits using the ratios shown in NIP-0004 §6.1:
// 3,000,000 / 1,000,000 / 500,000 / 200,000 / 100,000.
func LegacyGasToResourceLimits(gasLimit uint64) ResourceVector {
	return ResourceVector{
		Compute:     gasLimit,
		StateRead:   gasLimit / 3,
		StateWrite:  gasLimit / 6,
		ProtocolOps: gasLimit / 15,
		ProofVerify: gasLimit / 30,
	}
}

type ResourceTxSample struct {
	TxHash      common.Hash    `json:"txHash"`
	BlockNumber uint64         `json:"blockNumber"`
	TxIndex     uint           `json:"txIndex"`
	Vector      ResourceVector `json:"vector"`
}

type resourceMonitor struct {
	mu     sync.RWMutex
	order  []common.Hash
	recent map[common.Hash]ResourceTxSample
	limit  int
}

var GlobalResourceMonitor = &resourceMonitor{
	recent: make(map[common.Hash]ResourceTxSample),
	limit:  2048,
}

func (m *resourceMonitor) RecordTx(hash common.Hash, blockNumber uint64, txIndex uint, vector ResourceVector) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.recent[hash]; !exists {
		m.order = append(m.order, hash)
	}
	m.recent[hash] = ResourceTxSample{
		TxHash:      hash,
		BlockNumber: blockNumber,
		TxIndex:     txIndex,
		Vector:      vector,
	}
	for len(m.order) > m.limit {
		old := m.order[0]
		m.order = m.order[1:]
		delete(m.recent, old)
	}
}

func (m *resourceMonitor) GetTx(hash common.Hash) (ResourceTxSample, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sample, ok := m.recent[hash]
	return sample, ok
}
