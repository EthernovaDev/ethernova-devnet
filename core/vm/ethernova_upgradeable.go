// Ethernova: Native Contract Upgradeability (Phase 21) — StateDB-BACKED
//
// Previous versions stored pending upgrade requests and history in core/rawdb
// via GlobalChainDB. Same class of bug as the other rawdb-backed precompiles:
//   - Upgrade requests were NOT covered by the state root → reorgs could
//     leave an "executed" upgrade marker on one chain but not another.
//   - Execute-upgrade wrote the NEW CODE to the state trie via SetCode AND
//     deleted the rawdb request record. If the tx reverted after SetCode,
//     the trie move rolled back but the rawdb delete persisted — leaving a
//     contract whose pending-upgrade queue entry was silently gone.
//   - initiateUpgrade on a forked branch permanently reserved the upgrade
//     slot even after the fork was abandoned.
//
// All upgrade state now lives at system address 0xAA27 via StateDB. The
// variable-length code blob (up to ~24KB) is chunked into 32-byte slots in
// the same pattern as poWriteRLP / tmWriteMeta.
//
// SAFETY (retained):
//  1. 100-block timelock between initiate and execute.
//  2. New code cannot be empty.
//  3. New code cannot be >10x smaller than old (catches gross layout changes).

package vm

import (
	"encoding/binary"
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

const upgradeTimelock = 100

// upgradeSystemAddr holds all pending-upgrade requests and history.
var upgradeSystemAddr = common.HexToAddress("0x000000000000000000000000000000000000AA27")

func ugEnsureSystemAccount(sdb StateDB) {
	if !sdb.Exist(upgradeSystemAddr) {
		sdb.CreateAccount(upgradeSystemAddr)
	}
	if sdb.GetNonce(upgradeSystemAddr) == 0 {
		sdb.SetNonce(upgradeSystemAddr, 1)
	}
}

// Storage key builders. Each is namespaced so an attacker cannot craft a
// contract address that collides with another field for a different contract.
func ugKeyCaller(contract common.Address) common.Hash {
	return crypto.Keccak256Hash([]byte("upgrade.caller"), contract.Bytes())
}
func ugKeyReqBlock(contract common.Address) common.Hash {
	return crypto.Keccak256Hash([]byte("upgrade.reqBlock"), contract.Bytes())
}
func ugKeyActBlock(contract common.Address) common.Hash {
	return crypto.Keccak256Hash([]byte("upgrade.actBlock"), contract.Bytes())
}
func ugKeyCodeHash(contract common.Address) common.Hash {
	return crypto.Keccak256Hash([]byte("upgrade.codeHash"), contract.Bytes())
}
func ugKeyCodeLen(contract common.Address) common.Hash {
	return crypto.Keccak256Hash([]byte("upgrade.codeLen"), contract.Bytes())
}
func ugKeyCodeChunk(contract common.Address, idx uint64) common.Hash {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], idx)
	return crypto.Keccak256Hash([]byte("upgrade.codeChunk"), contract.Bytes(), buf[:])
}
func ugKeyHistoryBlock(contract common.Address) common.Hash {
	return crypto.Keccak256Hash([]byte("upgrade.historyBlock"), contract.Bytes())
}
func ugKeyHistoryHash(contract common.Address) common.Hash {
	return crypto.Keccak256Hash([]byte("upgrade.historyHash"), contract.Bytes())
}

func ugReadUint64(sdb StateDB, key common.Hash) uint64 {
	return binary.BigEndian.Uint64(sdb.GetState(upgradeSystemAddr, key).Bytes()[24:])
}
func ugWriteUint64(sdb StateDB, key common.Hash, v uint64) {
	var out [32]byte
	binary.BigEndian.PutUint64(out[24:], v)
	sdb.SetState(upgradeSystemAddr, key, common.BytesToHash(out[:]))
}

// ugHasPendingUpgrade uses codeLen as the existence marker. initiateUpgrade
// rejects empty code, so codeLen == 0 uniquely means "no pending upgrade".
func ugHasPendingUpgrade(sdb StateDB, contract common.Address) bool {
	return ugReadUint64(sdb, ugKeyCodeLen(contract)) > 0
}

func ugWriteCode(sdb StateDB, contract common.Address, code []byte) {
	dataLen := uint64(len(code))
	chunks := (dataLen + 31) / 32
	ugWriteUint64(sdb, ugKeyCodeLen(contract), dataLen)
	for i := uint64(0); i < chunks; i++ {
		start := i * 32
		end := start + 32
		if end > dataLen {
			end = dataLen
		}
		var chunk [32]byte
		copy(chunk[:], code[start:end])
		sdb.SetState(upgradeSystemAddr, ugKeyCodeChunk(contract, i), common.BytesToHash(chunk[:]))
	}
}

func ugReadCode(sdb StateDB, contract common.Address) []byte {
	dataLen := ugReadUint64(sdb, ugKeyCodeLen(contract))
	if dataLen == 0 {
		return nil
	}
	chunks := (dataLen + 31) / 32
	out := make([]byte, 0, dataLen)
	for i := uint64(0); i < chunks; i++ {
		chunk := sdb.GetState(upgradeSystemAddr, ugKeyCodeChunk(contract, i))
		remaining := dataLen - uint64(len(out))
		if remaining >= 32 {
			out = append(out, chunk[:]...)
		} else {
			out = append(out, chunk[:remaining]...)
		}
	}
	return out
}

// ugClearRequest zeroes every slot belonging to the pending request. Zeroing
// the code chunks keeps the state trie tidy so an abandoned upgrade doesn't
// leave witness-carrying slots behind.
func ugClearRequest(sdb StateDB, contract common.Address) {
	dataLen := ugReadUint64(sdb, ugKeyCodeLen(contract))
	chunks := (dataLen + 31) / 32
	for i := uint64(0); i < chunks; i++ {
		sdb.SetState(upgradeSystemAddr, ugKeyCodeChunk(contract, i), common.Hash{})
	}
	sdb.SetState(upgradeSystemAddr, ugKeyCodeLen(contract), common.Hash{})
	sdb.SetState(upgradeSystemAddr, ugKeyCaller(contract), common.Hash{})
	sdb.SetState(upgradeSystemAddr, ugKeyReqBlock(contract), common.Hash{})
	sdb.SetState(upgradeSystemAddr, ugKeyActBlock(contract), common.Hash{})
	sdb.SetState(upgradeSystemAddr, ugKeyCodeHash(contract), common.Hash{})
}

type novaContractUpgrade struct{}

func (c *novaContractUpgrade) RequiredGas(input []byte) uint64 {
	if len(input) < 1 {
		return 0
	}
	switch input[0] {
	case 0x01:
		return 50000
	case 0x02:
		return 10000
	case 0x03:
		return 2000
	case 0x04:
		return 50000
	default:
		return 0
	}
}

func (c *novaContractUpgrade) Run(input []byte) ([]byte, error) {
	return nil, errors.New("novaContractUpgrade: use RunStateful")
}

func (c *novaContractUpgrade) RunStateful(evm *EVM, caller common.Address, input []byte, readOnly bool) ([]byte, error) {
	if len(input) < 1 {
		return nil, errors.New("novaContractUpgrade: empty input")
	}
	sdb := evm.StateDB

	switch input[0] {
	case 0x01: // initiateUpgrade(contract20, newCode...)
		if readOnly {
			return nil, ErrWriteProtection
		}
		if len(input) < 22 {
			return nil, errors.New("initiateUpgrade: need contract(20) + code")
		}

		ugEnsureSystemAccount(sdb)

		contract := common.BytesToAddress(input[1:21])
		newCode := input[21:]

		if len(newCode) == 0 {
			return nil, errors.New("initiateUpgrade: new code is empty")
		}

		if ugHasPendingUpgrade(sdb, contract) {
			return nil, errors.New("initiateUpgrade: upgrade already pending")
		}

		currentBlock := evm.Context.BlockNumber.Uint64()
		activateBlock := currentBlock + upgradeTimelock
		newCodeHash := crypto.Keccak256Hash(newCode)

		sdb.SetState(upgradeSystemAddr, ugKeyCaller(contract), common.BytesToHash(caller.Bytes()))
		ugWriteUint64(sdb, ugKeyReqBlock(contract), currentBlock)
		ugWriteUint64(sdb, ugKeyActBlock(contract), activateBlock)
		sdb.SetState(upgradeSystemAddr, ugKeyCodeHash(contract), newCodeHash)
		ugWriteCode(sdb, contract, newCode)

		return newCodeHash.Bytes(), nil

	case 0x02: // cancelUpgrade(contract20)
		if readOnly {
			return nil, ErrWriteProtection
		}
		if len(input) < 21 {
			return nil, errors.New("cancelUpgrade: need contract address")
		}

		ugEnsureSystemAccount(sdb)

		contract := common.BytesToAddress(input[1:21])
		if !ugHasPendingUpgrade(sdb, contract) {
			return nil, errors.New("cancelUpgrade: no pending upgrade")
		}
		ugClearRequest(sdb, contract)
		return common.LeftPadBytes([]byte{1}, 32), nil

	case 0x03: // getUpgradeStatus(contract20) -> activateBlock(32) + codeHash(32)
		if len(input) < 21 {
			return nil, errors.New("getUpgradeStatus: need contract address")
		}
		contract := common.BytesToAddress(input[1:21])
		if !ugHasPendingUpgrade(sdb, contract) {
			return make([]byte, 64), nil
		}
		activateBlock := ugReadUint64(sdb, ugKeyActBlock(contract))
		codeHash := sdb.GetState(upgradeSystemAddr, ugKeyCodeHash(contract))
		result := make([]byte, 64)
		binary.BigEndian.PutUint64(result[24:32], activateBlock)
		copy(result[32:64], codeHash.Bytes())
		return result, nil

	case 0x04: // executeUpgrade(contract20)
		if readOnly {
			return nil, ErrWriteProtection
		}
		if len(input) < 21 {
			return nil, errors.New("executeUpgrade: need contract address")
		}

		ugEnsureSystemAccount(sdb)

		contract := common.BytesToAddress(input[1:21])
		if !ugHasPendingUpgrade(sdb, contract) {
			return nil, errors.New("executeUpgrade: no pending upgrade")
		}

		activateBlock := ugReadUint64(sdb, ugKeyActBlock(contract))
		currentBlock := evm.Context.BlockNumber.Uint64()
		if currentBlock < activateBlock {
			return nil, errors.New("executeUpgrade: timelock not expired yet")
		}

		newCode := ugReadCode(sdb, contract)
		newCodeHash := crypto.Keccak256Hash(newCode)

		// Defensive: the request was written with >0 bytes of code, but the
		// trie read could still produce nil if someone corrupted the slots
		// out-of-band. Keep the empty-check before touching StateDB.SetCode.
		if len(newCode) == 0 {
			return nil, errors.New("executeUpgrade: new code is empty")
		}
		oldCode := sdb.GetCode(contract)
		if len(oldCode) > 0 && len(newCode)*10 < len(oldCode) {
			return nil, errors.New("executeUpgrade: new code is >10x smaller than old code - possible storage layout corruption")
		}

		sdb.SetCode(contract, newCode)

		// Record latest upgrade in history, then clear the request queue.
		ugWriteUint64(sdb, ugKeyHistoryBlock(contract), currentBlock)
		sdb.SetState(upgradeSystemAddr, ugKeyHistoryHash(contract), newCodeHash)
		ugClearRequest(sdb, contract)

		return common.LeftPadBytes([]byte{1}, 32), nil

	default:
		return nil, errors.New("novaContractUpgrade: unknown function")
	}
}
