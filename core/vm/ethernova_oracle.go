// Ethernova: Native Price Oracle (Phase 22) — StateDB-BACKED
//
// Previous versions stored prices in LevelDB via core/rawdb, outside the
// state trie. That meant price reads could diverge silently across nodes
// (reorg survivors vs fresh syncers) — a node-local state leading to
// different submitPrice circuit-breaker outcomes and different consensus
// roots. All price state now lives at system address 0xAA28 via StateDB
// inside the Merkle Patricia Trie.
//
// SAFETY (retained from previous version):
//  1. TWAP uses the average of stored per-block prices in [start, end].
//  2. Circuit breaker: reject price if >15% change from previous block.
//  3. DeFi contracts should use TWAP, never spot price.

package vm

import (
	"encoding/binary"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// oracleSystemAddr holds all price state (latest and history).
var oracleSystemAddr = common.HexToAddress("0x000000000000000000000000000000000000AA28")

func orEnsureSystemAccount(sdb StateDB) {
	if !sdb.Exist(oracleSystemAddr) {
		sdb.CreateAccount(oracleSystemAddr)
	}
	if sdb.GetNonce(oracleSystemAddr) == 0 {
		sdb.SetNonce(oracleSystemAddr, 1)
	}
}

func orKeyPrice(pairID common.Hash) common.Hash {
	return crypto.Keccak256Hash([]byte("oracle.price"), pairID.Bytes())
}
func orKeyHistory(pairID common.Hash, block uint64) common.Hash {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], block)
	return crypto.Keccak256Hash([]byte("oracle.history"), pairID.Bytes(), buf[:])
}

func orReadBigInt(sdb StateDB, key common.Hash) *big.Int {
	return new(big.Int).SetBytes(sdb.GetState(oracleSystemAddr, key).Bytes())
}
func orWriteBigInt(sdb StateDB, key common.Hash, v *big.Int) {
	sdb.SetState(oracleSystemAddr, key, common.BigToHash(v))
}

type novaOracle struct{}

func (c *novaOracle) RequiredGas(input []byte) uint64 {
	if len(input) < 1 {
		return 0
	}
	switch input[0] {
	case 0x01:
		return 2000 // getPrice
	case 0x02:
		return 5000 // getTWAP
	case 0x03:
		return 50000 // submitPrice
	default:
		return 0
	}
}

func (c *novaOracle) Run(input []byte) ([]byte, error) {
	return nil, errors.New("novaOracle: use RunStateful")
}

func (c *novaOracle) RunStateful(evm *EVM, caller common.Address, input []byte, readOnly bool) ([]byte, error) {
	if len(input) < 1 {
		return nil, errors.New("novaOracle: empty input")
	}
	sdb := evm.StateDB

	switch input[0] {
	case 0x01: // getPrice(pairID32)
		if len(input) < 33 {
			return nil, errors.New("getPrice: need pairID")
		}
		pairID := common.BytesToHash(input[1:33])
		price := orReadBigInt(sdb, orKeyPrice(pairID))
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
		// Cap TWAP window to prevent unbounded gas use on a single query.
		// Each iteration is an SLOAD; keep the window reasonable.
		const maxTWAPWindow = 10000
		if endBlock-startBlock > maxTWAPWindow {
			return nil, errors.New("getTWAP: window too large (max 10000 blocks)")
		}

		sum := new(big.Int)
		count := uint64(0)
		for block := startBlock; block <= endBlock; block++ {
			price := orReadBigInt(sdb, orKeyHistory(pairID, block))
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
		if readOnly {
			return nil, ErrWriteProtection
		}
		if len(input) < 73 {
			return nil, errors.New("submitPrice: need pairID + price + block")
		}

		orEnsureSystemAccount(sdb)

		pairID := common.BytesToHash(input[1:33])
		price := new(big.Int).SetBytes(input[33:65])
		block := new(big.Int).SetBytes(input[65:73]).Uint64()

		// Circuit breaker against single-block manipulation: reject if the
		// new price differs by more than 15% from the last stored price.
		prevPrice := orReadBigInt(sdb, orKeyPrice(pairID))
		if prevPrice.Sign() > 0 && price.Sign() > 0 {
			diff := new(big.Int).Sub(price, prevPrice)
			if diff.Sign() < 0 {
				diff.Neg(diff)
			}
			threshold := new(big.Int).Mul(prevPrice, big.NewInt(15))
			threshold.Div(threshold, big.NewInt(100))
			if diff.Cmp(threshold) > 0 {
				return nil, errors.New("submitPrice: CIRCUIT BREAKER - price changed >15% from previous block")
			}
		}

		orWriteBigInt(sdb, orKeyPrice(pairID), price)
		orWriteBigInt(sdb, orKeyHistory(pairID, block), price)
		return common.LeftPadBytes([]byte{1}, 32), nil

	default:
		return nil, errors.New("novaOracle: unknown function")
	}
}

// PairID generates a deterministic pair identifier from two token names.
func PairID(base, quote string) common.Hash {
	return crypto.Keccak256Hash([]byte(base + "/" + quote))
}
