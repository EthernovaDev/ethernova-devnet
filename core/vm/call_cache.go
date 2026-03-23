package vm

import (
	"crypto/sha256"
	"sync"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/common"
)

// CallCache caches results of pure contract calls.
// A "pure" call is one where the result depends only on the input data
// and the contract code — not on storage, block context, or external state.
// If the same contract is called with the same input and the contract is
// verified as deterministic, the cached result is returned without re-executing.
type CallCache struct {
	mu      sync.RWMutex
	entries map[common.Hash]*CacheEntry
	enabled atomic.Bool
	maxSize int

	// Stats
	hits   atomic.Uint64
	misses atomic.Uint64
	evicts atomic.Uint64
}

// CacheEntry holds a cached call result.
type CacheEntry struct {
	Result  []byte
	GasUsed uint64
}

var GlobalCallCache = &CallCache{
	entries: make(map[common.Hash]*CacheEntry),
	maxSize: 10000, // max 10k cached results
}

func init() {
	GlobalCallCache.enabled.Store(false) // disabled by default
}

// MakeCacheKey creates a unique key from contract address + input data.
func MakeCacheKey(addr common.Address, input []byte) common.Hash {
	h := sha256.New()
	h.Write(addr.Bytes())
	h.Write(input)
	var result common.Hash
	copy(result[:], h.Sum(nil))
	return result
}

// Get returns a cached result if available.
func (cc *CallCache) Get(addr common.Address, input []byte) (*CacheEntry, bool) {
	if !cc.enabled.Load() {
		return nil, false
	}

	key := MakeCacheKey(addr, input)

	cc.mu.RLock()
	entry, ok := cc.entries[key]
	cc.mu.RUnlock()

	if ok {
		cc.hits.Add(1)
		return entry, true
	}

	cc.misses.Add(1)
	return nil, false
}

// Put stores a call result in the cache.
func (cc *CallCache) Put(addr common.Address, input []byte, result []byte, gasUsed uint64) {
	if !cc.enabled.Load() {
		return
	}

	key := MakeCacheKey(addr, input)

	cc.mu.Lock()
	defer cc.mu.Unlock()

	// Evict if at capacity
	if len(cc.entries) >= cc.maxSize {
		// Simple eviction: remove first entry found
		for k := range cc.entries {
			delete(cc.entries, k)
			cc.evicts.Add(1)
			break
		}
	}

	// Copy result to avoid referencing external memory
	resultCopy := make([]byte, len(result))
	copy(resultCopy, result)

	cc.entries[key] = &CacheEntry{
		Result:  resultCopy,
		GasUsed: gasUsed,
	}
}

// SetEnabled enables or disables the cache.
func (cc *CallCache) SetEnabled(v bool) {
	cc.enabled.Store(v)
}

// IsEnabled returns whether caching is active.
func (cc *CallCache) IsEnabled() bool {
	return cc.enabled.Load()
}

// Reset clears all cached entries.
func (cc *CallCache) Reset() {
	cc.mu.Lock()
	cc.entries = make(map[common.Hash]*CacheEntry)
	cc.mu.Unlock()
	cc.hits.Store(0)
	cc.misses.Store(0)
	cc.evicts.Store(0)
}

// CacheStats holds cache statistics for RPC reporting.
type CacheStats struct {
	Enabled bool   `json:"enabled"`
	Size    int    `json:"size"`
	MaxSize int    `json:"maxSize"`
	Hits    uint64 `json:"hits"`
	Misses  uint64 `json:"misses"`
	Evicts  uint64 `json:"evicts"`
	HitRate float64 `json:"hitRate"`
}

// Stats returns current cache statistics.
func (cc *CallCache) Stats() CacheStats {
	cc.mu.RLock()
	size := len(cc.entries)
	cc.mu.RUnlock()

	hits := cc.hits.Load()
	misses := cc.misses.Load()
	total := hits + misses
	var hitRate float64
	if total > 0 {
		hitRate = float64(hits) / float64(total) * 100
	}

	return CacheStats{
		Enabled: cc.enabled.Load(),
		Size:    size,
		MaxSize: cc.maxSize,
		Hits:    hits,
		Misses:  misses,
		Evicts:  cc.evicts.Load(),
		HitRate: hitRate,
	}
}

// BytecodeAnalyzer performs static analysis on contract bytecode at deploy time.
type BytecodeAnalyzer struct {
	mu       sync.RWMutex
	analysis map[common.Address]*BytecodeAnalysis
}

// BytecodeAnalysis holds the results of static bytecode analysis.
type BytecodeAnalysis struct {
	TotalOpcodes    int      `json:"totalOpcodes"`
	UniqueOpcodes   int      `json:"uniqueOpcodes"`
	HasLoops        bool     `json:"hasLoops"`       // backward JUMPs detected
	MaxStackDepth   int      `json:"maxStackDepth"`
	EstimatedGas    uint64   `json:"estimatedGas"`
	IsCacheable     bool     `json:"isCacheable"`    // safe to cache results
	OpcodeGroups    map[string]int `json:"opcodeGroups"` // arithmetic, storage, memory, etc.
}

var GlobalBytecodeAnalyzer = &BytecodeAnalyzer{
	analysis: make(map[common.Address]*BytecodeAnalysis),
}

// Analyze performs static analysis on contract bytecode.
func (ba *BytecodeAnalyzer) Analyze(addr common.Address, code []byte) *BytecodeAnalysis {
	ba.mu.RLock()
	if existing, ok := ba.analysis[addr]; ok {
		ba.mu.RUnlock()
		return existing
	}
	ba.mu.RUnlock()

	groups := map[string]int{
		"arithmetic": 0,
		"comparison": 0,
		"bitwise":    0,
		"stack":      0,
		"memory":     0,
		"storage":    0,
		"control":    0,
		"system":     0,
	}

	seen := make(map[OpCode]bool)
	totalOps := 0
	hasLoops := false
	hasSStore := false
	hasCall := false
	hasCreate := false
	var estimatedGas uint64

	jumpDests := make(map[uint64]bool)

	// First pass: find all JUMPDEST locations
	for i := 0; i < len(code); i++ {
		op := OpCode(code[i])
		if op == JUMPDEST {
			jumpDests[uint64(i)] = true
		}
		if op >= PUSH1 && op <= PUSH32 {
			i += int(op - PUSH1 + 1)
		}
	}

	// Second pass: analyze opcodes
	for i := 0; i < len(code); i++ {
		op := OpCode(code[i])
		totalOps++
		seen[op] = true

		switch {
		case op >= ADD && op <= SIGNEXTEND:
			groups["arithmetic"]++
			estimatedGas += 3
		case op >= LT && op <= SAR:
			groups["comparison"]++
			estimatedGas += 3
		case op >= AND && op <= NOT:
			groups["bitwise"]++
			estimatedGas += 3
		case op >= POP && op <= PUSH32, op >= DUP1 && op <= SWAP16:
			groups["stack"]++
			estimatedGas += 3
		case op >= MLOAD && op <= MSTORE8:
			groups["memory"]++
			estimatedGas += 3
		case op == SLOAD:
			groups["storage"]++
			estimatedGas += 200
		case op == SSTORE:
			groups["storage"]++
			estimatedGas += 5000
			hasSStore = true
		case op >= JUMP && op <= JUMPDEST:
			groups["control"]++
			estimatedGas += 8
			// Detect backward jumps (loops)
			if op == JUMP || op == JUMPI {
				if i > 0 && OpCode(code[i-1]) >= PUSH1 && OpCode(code[i-1]) <= PUSH32 {
					pushOp := OpCode(code[i-1])
					pushSize := int(pushOp - PUSH1 + 1)
					if i-1-pushSize >= 0 {
						// Read the push value
						var target uint64
						for j := 0; j < pushSize && j < 8; j++ {
							target = (target << 8) | uint64(code[i-pushSize+j])
						}
						if target < uint64(i) {
							hasLoops = true
						}
					}
				}
			}
		case op == CALL || op == STATICCALL || op == DELEGATECALL || op == CALLCODE:
			groups["system"]++
			estimatedGas += 700
			hasCall = true
		case op == CREATE || op == CREATE2:
			groups["system"]++
			estimatedGas += 32000
			hasCreate = true
		}

		if op >= PUSH1 && op <= PUSH32 {
			i += int(op - PUSH1 + 1)
		}
	}

	// A contract is cacheable if it doesn't write storage, create contracts,
	// or make external calls
	isCacheable := !hasSStore && !hasCall && !hasCreate

	result := &BytecodeAnalysis{
		TotalOpcodes:  totalOps,
		UniqueOpcodes: len(seen),
		HasLoops:      hasLoops,
		EstimatedGas:  estimatedGas,
		IsCacheable:   isCacheable,
		OpcodeGroups:  groups,
	}

	ba.mu.Lock()
	ba.analysis[addr] = result
	ba.mu.Unlock()

	return result
}

// GetAnalysis returns the analysis for a contract, or nil if not analyzed.
func (ba *BytecodeAnalyzer) GetAnalysis(addr common.Address) *BytecodeAnalysis {
	ba.mu.RLock()
	defer ba.mu.RUnlock()
	return ba.analysis[addr]
}

// GetAllAnalysis returns all analyzed contracts.
func (ba *BytecodeAnalyzer) GetAllAnalysis() map[string]*BytecodeAnalysis {
	ba.mu.RLock()
	defer ba.mu.RUnlock()

	result := make(map[string]*BytecodeAnalysis)
	for addr, a := range ba.analysis {
		result[addr.Hex()] = a
	}
	return result
}

// Reset clears all analysis data.
func (ba *BytecodeAnalyzer) Reset() {
	ba.mu.Lock()
	ba.analysis = make(map[common.Address]*BytecodeAnalysis)
	ba.mu.Unlock()
}
