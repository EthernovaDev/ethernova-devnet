// Ethernova: Native Price Oracle (Phase 22)
// Protocol-level price feeds with persistent LevelDB storage.
// Precompile at 0x28 (novaOracle)

package vm

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/crypto"
)

type novaOracle struct{}

func (c *novaOracle) RequiredGas(input []byte) uint64 {
	if len(input) < 1 {
		return 0
	}
	switch input[0] {
	case 0x01: return 2000   // getPrice
	case 0x02: return 5000   // getTWAP
	case 0x03: return 50000  // submitPrice (miner only)
	default:   return 0
	}
}

func (c *novaOracle) Run(input []byte) ([]byte, error) {
	if len(input) < 1 {
		return nil, errors.New("novaOracle: empty input")
	}
	if GlobalChainDB == nil {
		return nil, errors.New("novaOracle: database not initialized")
	}

	switch input[0] {
	case 0x01: // getPrice(pairID32)
		if len(input) < 33 {
			return nil, errors.New("getPrice: need pairID")
		}
		pairID := common.BytesToHash(input[1:33])
		price := rawdb.ReadOraclePrice(GlobalChainDB, pairID)
		return common.LeftPadBytes(price.Bytes(), 32), nil

	case 0x02: // getTWAP(pairID32, startBlock8, endBlock8)
		if len(input) < 49 {
			return nil, errors.New("getTWAP: need pairID + startBlock + endBlock")
		}
		pairID := common.BytesToHash(input[1:33])
		startBlock := new(big.Int).SetBytes(input[33:41]).Uint64()
		endBlock := new(big.Int).SetBytes(input[41:49]).Uint64()

		if endBlock <= startBlock {
			return nil, errors.New("getTWAP: endBlock must be > startBlock")
		}

		// Calculate TWAP from history
		sum := new(big.Int)
		count := uint64(0)
		for block := startBlock; block <= endBlock; block++ {
			price := rawdb.ReadOraclePriceHistory(GlobalChainDB, pairID, block)
			if price.Sign() > 0 {
				sum.Add(sum, price)
				count++
			}
		}
		if count == 0 {
			return make([]byte, 32), nil
		}
		twap := new(big.Int).Div(sum, new(big.Int).SetUint64(count))
		return common.LeftPadBytes(twap.Bytes(), 32), nil

	case 0x03: // submitPrice(pairID32, price32, block8)
		if len(input) < 73 {
			return nil, errors.New("submitPrice: need pairID + price + block")
		}
		pairID := common.BytesToHash(input[1:33])
		price := new(big.Int).SetBytes(input[33:65])
		block := new(big.Int).SetBytes(input[65:73]).Uint64()

		// Store latest price
		rawdb.WriteOraclePrice(GlobalChainDB, pairID, price)
		// Store in history
		rawdb.WriteOraclePriceHistory(GlobalChainDB, pairID, block, price)
		return common.LeftPadBytes([]byte{1}, 32), nil

	default:
		return nil, errors.New("novaOracle: unknown function")
	}
}

// PairID generates a deterministic pair identifier from two token names.
func PairID(base, quote string) common.Hash {
	return crypto.Keccak256Hash([]byte(base + "/" + quote))
}
