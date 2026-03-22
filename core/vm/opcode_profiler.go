package vm

import (
	"sort"
	"sync"
	"sync/atomic"
)

// OpcodeProfiler tracks execution counts and gas usage per opcode globally.
// It is safe for concurrent use by multiple EVM instances.
type OpcodeProfiler struct {
	counts   [256]atomic.Uint64
	gasUsed  [256]atomic.Uint64
	enabled  atomic.Bool
}

var GlobalProfiler = &OpcodeProfiler{}

func init() {
	GlobalProfiler.enabled.Store(true)
}

// Record records a single opcode execution with its gas cost.
func (p *OpcodeProfiler) Record(op OpCode, gas uint64) {
	if !p.enabled.Load() {
		return
	}
	p.counts[op].Add(1)
	p.gasUsed[op].Add(gas)
}

// SetEnabled enables or disables profiling.
func (p *OpcodeProfiler) SetEnabled(v bool) {
	p.enabled.Store(v)
}

// IsEnabled returns whether profiling is active.
func (p *OpcodeProfiler) IsEnabled() bool {
	return p.enabled.Load()
}

// OpcodeStats holds profiling data for a single opcode.
type OpcodeStats struct {
	Opcode     string  `json:"opcode"`
	Count      uint64  `json:"count"`
	GasUsed    uint64  `json:"gasUsed"`
	Percentage float64 `json:"percentage"`
}

// Snapshot returns profiling data for all opcodes that have been executed,
// sorted by execution count descending.
func (p *OpcodeProfiler) Snapshot() []OpcodeStats {
	var total uint64
	for i := 0; i < 256; i++ {
		total += p.counts[i].Load()
	}
	if total == 0 {
		return nil
	}

	var stats []OpcodeStats
	for i := 0; i < 256; i++ {
		c := p.counts[i].Load()
		if c == 0 {
			continue
		}
		stats = append(stats, OpcodeStats{
			Opcode:     OpCode(i).String(),
			Count:      c,
			GasUsed:    p.gasUsed[i].Load(),
			Percentage: float64(c) / float64(total) * 100,
		})
	}

	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Count > stats[j].Count
	})
	return stats
}

// TotalOps returns the total number of opcode executions recorded.
func (p *OpcodeProfiler) TotalOps() uint64 {
	var total uint64
	for i := 0; i < 256; i++ {
		total += p.counts[i].Load()
	}
	return total
}

// TotalGas returns the total gas consumed across all profiled opcodes.
func (p *OpcodeProfiler) TotalGas() uint64 {
	var total uint64
	for i := 0; i < 256; i++ {
		total += p.gasUsed[i].Load()
	}
	return total
}

// Reset clears all profiling data.
func (p *OpcodeProfiler) Reset() {
	for i := 0; i < 256; i++ {
		p.counts[i].Store(0)
		p.gasUsed[i].Store(0)
	}
}

// ContractProfiler tracks per-contract opcode execution.
type ContractProfiler struct {
	mu       sync.RWMutex
	contracts map[string]*contractProfile
	enabled  atomic.Bool
}

type contractProfile struct {
	counts  [256]uint64
	gasUsed [256]uint64
	calls   uint64
}

var GlobalContractProfiler = &ContractProfiler{
	contracts: make(map[string]*contractProfile),
}

// Record records an opcode execution for a specific contract address.
func (cp *ContractProfiler) Record(addr string, op OpCode, gas uint64) {
	if !cp.enabled.Load() {
		return
	}
	cp.mu.Lock()
	p, ok := cp.contracts[addr]
	if !ok {
		p = &contractProfile{}
		cp.contracts[addr] = p
	}
	p.counts[op]++
	p.gasUsed[op] += gas
	cp.mu.Unlock()
}

// RecordCall increments the call counter for a contract.
func (cp *ContractProfiler) RecordCall(addr string) {
	if !cp.enabled.Load() {
		return
	}
	cp.mu.Lock()
	p, ok := cp.contracts[addr]
	if !ok {
		p = &contractProfile{}
		cp.contracts[addr] = p
	}
	p.calls++
	cp.mu.Unlock()
}

// ContractStats holds profiling summary for a contract.
type ContractStats struct {
	Address  string `json:"address"`
	Calls    uint64 `json:"calls"`
	TotalOps uint64 `json:"totalOps"`
	TotalGas uint64 `json:"totalGas"`
}

// TopContracts returns the top N contracts by total operations.
func (cp *ContractProfiler) TopContracts(n int) []ContractStats {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	var stats []ContractStats
	for addr, p := range cp.contracts {
		var ops, gas uint64
		for i := 0; i < 256; i++ {
			ops += p.counts[i]
			gas += p.gasUsed[i]
		}
		stats = append(stats, ContractStats{
			Address:  addr,
			Calls:    p.calls,
			TotalOps: ops,
			TotalGas: gas,
		})
	}

	sort.Slice(stats, func(i, j int) bool {
		return stats[i].TotalOps > stats[j].TotalOps
	})

	if n > 0 && len(stats) > n {
		stats = stats[:n]
	}
	return stats
}

// Reset clears all contract profiling data.
func (cp *ContractProfiler) Reset() {
	cp.mu.Lock()
	cp.contracts = make(map[string]*contractProfile)
	cp.mu.Unlock()
}
