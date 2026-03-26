// Ethernova: Native Price Oracle (Phase 22)
// Protocol-level price feeds attested by miners.
// No more Chainlink dependency, no oracle manipulation via flash loans.
//
// How it works:
// - Miners include price attestations in block headers (extra data)
// - Precompile at 0x28 (novaOracle) provides price queries
// - TWAP (Time-Weighted Average Price) computed across multiple blocks
// - Manipulation requires controlling 51%+ of hashrate for N blocks
// - Much more secure than single-block spot prices
//
// For devnet: price data is simulated. Production would use miner attestations.

package vm

import (
	"errors"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
)

// PriceFeed stores price data for a trading pair.
type PriceFeed struct {
	PairID    common.Hash // keccak256("NOVA/USD") etc.
	Price     *big.Int    // price * 10^8 (8 decimal precision)
	Block     uint64
	Timestamp uint64
}

// OracleState holds all price feeds.
type OracleState struct {
	mu     sync.RWMutex
	feeds  map[common.Hash]*PriceFeed
	history map[common.Hash][]PriceFeed // for TWAP calculation
}

// GlobalOracle is the singleton oracle.
var GlobalOracle = &OracleState{
	feeds:   make(map[common.Hash]*PriceFeed),
	history: make(map[common.Hash][]PriceFeed),
}

// UpdatePrice updates a price feed (called by miner during block production).
func (o *OracleState) UpdatePrice(pairID common.Hash, price *big.Int, block, timestamp uint64) {
	o.mu.Lock()
	defer o.mu.Unlock()
	feed := &PriceFeed{PairID: pairID, Price: price, Block: block, Timestamp: timestamp}
	o.feeds[pairID] = feed
	o.history[pairID] = append(o.history[pairID], *feed)
	// Keep last 1000 entries
	if len(o.history[pairID]) > 1000 {
		o.history[pairID] = o.history[pairID][len(o.history[pairID])-1000:]
	}
}

// GetTWAP returns the time-weighted average price over N blocks.
func (o *OracleState) GetTWAP(pairID common.Hash, blocks int) *big.Int {
	o.mu.RLock()
	defer o.mu.RUnlock()
	history := o.history[pairID]
	if len(history) == 0 {
		return new(big.Int)
	}
	count := blocks
	if count > len(history) {
		count = len(history)
	}
	sum := new(big.Int)
	for i := len(history) - count; i < len(history); i++ {
		sum.Add(sum, history[i].Price)
	}
	return new(big.Int).Div(sum, big.NewInt(int64(count)))
}

// novaOracle is the precompile for price queries.
// Address: 0x28
type novaOracle struct{}

// 0x01 = getPrice(pairID) -> price (uint256)
// 0x02 = getTWAP(pairID, blocks) -> twap (uint256)
// 0x03 = listPairs() -> count

func (c *novaOracle) RequiredGas(input []byte) uint64 {
	if len(input) < 1 {
		return 0
	}
	switch input[0] {
	case 0x01:
		return 2000
	case 0x02:
		return 5000
	case 0x03:
		return 2000
	default:
		return 0
	}
}

func (c *novaOracle) Run(input []byte) ([]byte, error) {
	if len(input) < 1 {
		return nil, errors.New("novaOracle: empty input")
	}
	switch input[0] {
	case 0x01: // getPrice
		if len(input) < 33 {
			return nil, errors.New("getPrice: need pairID")
		}
		var pairID common.Hash
		copy(pairID[:], input[1:33])
		GlobalOracle.mu.RLock()
		feed := GlobalOracle.feeds[pairID]
		GlobalOracle.mu.RUnlock()
		if feed == nil {
			return make([]byte, 32), nil
		}
		return common.LeftPadBytes(feed.Price.Bytes(), 32), nil

	case 0x02: // getTWAP
		if len(input) < 65 {
			return nil, errors.New("getTWAP: need pairID + blocks")
		}
		var pairID common.Hash
		copy(pairID[:], input[1:33])
		blocks := new(big.Int).SetBytes(input[33:65]).Int64()
		if blocks <= 0 {
			blocks = 100
		}
		twap := GlobalOracle.GetTWAP(pairID, int(blocks))
		return common.LeftPadBytes(twap.Bytes(), 32), nil

	case 0x03: // listPairs
		GlobalOracle.mu.RLock()
		count := len(GlobalOracle.feeds)
		GlobalOracle.mu.RUnlock()
		return common.LeftPadBytes(big.NewInt(int64(count)).Bytes(), 32), nil

	default:
		return nil, errors.New("novaOracle: unknown function")
	}
}
