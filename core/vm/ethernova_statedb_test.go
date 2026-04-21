// Regression tests for the Phase-20-24 StateDB migration and the EIP-161
// account-manager guard. These tests lock in three invariants we otherwise
// have no automated protection against:
//
//   1. System-address accounts (0xAA22, 0xAA25-AA28) survive Finalise(true).
//      Without the *EnsureSystemAccount helpers, EIP-161 would delete them
//      at the first tx boundary and wipe every storage slot they hold.
//   2. Revert semantics. A snapshot+RevertToSnapshot covering a
//      token/shield/oracle/upgrade write must erase that write, matching
//      the rest of the EVM. The previous rawdb-backed versions violated this.
//   3. Read-modify-read roundtrip through the state trie (not just in-memory
//      journal) works for every per-precompile storage key scheme.

package vm

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
)

// newTestEVM wires up an in-memory StateDB and an EVM suitable for invoking
// a stateful precompile's RunStateful directly. BlockNumber is set high
// enough to clear every Ethernova fork block.
func newTestEVM(t *testing.T) (*EVM, *state.StateDB) {
	t.Helper()
	sdb, err := state.New(types.EmptyRootHash, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
	if err != nil {
		t.Fatalf("state.New: %v", err)
	}
	ctx := BlockContext{
		BlockNumber: big.NewInt(1_000_000),
		Transfer:    func(StateDB, common.Address, common.Address, *uint256.Int) {},
	}
	evm := NewEVM(ctx, TxContext{}, sdb, params.AllEthashProtocolChanges, Config{})
	return evm, sdb
}

// ---------------------------------------------------------------------------
// Token manager (0xAA25)
// ---------------------------------------------------------------------------

func TestTokenManager_EIP161_GuardSurvivesFinalise(t *testing.T) {
	evm, sdb := newTestEVM(t)
	caller := common.HexToAddress("0x1111111111111111111111111111111111111111")

	// Give the creator some NOVA so the account isn't empty itself.
	sdb.CreateAccount(caller)
	sdb.SetNonce(caller, 1)

	// createToken(0x01) with 32 bytes of metadata and a nonzero supply tail.
	payload := make([]byte, 1+32)
	payload[0] = 0x01
	for i := 0; i < 31; i++ {
		payload[1+i] = byte(i + 1)
	}
	payload[1+31] = 0x64 // supply = 100 in last byte

	tm := &novaTokenManager{}
	tokenIDBytes, err := tm.RunStateful(evm, caller, payload, false)
	if err != nil {
		t.Fatalf("createToken: %v", err)
	}
	tokenID := common.BytesToHash(tokenIDBytes)

	// Finalise(true) is what triggers EIP-161 empty-account deletion. If our
	// amEnsureSystemAccount-equivalent guard is wrong, the whole 0xAA25
	// state blob disappears here.
	sdb.Finalise(true)

	if !sdb.Exist(tokenManagerSystemAddr) {
		t.Fatal("tokenManagerSystemAddr deleted by Finalise(true) — EIP-161 guard missing")
	}
	if got := sdb.GetNonce(tokenManagerSystemAddr); got == 0 {
		t.Fatalf("tokenManagerSystemAddr nonce=0 after Finalise; expected >=1, got %d", got)
	}
	if tmReadUint64(sdb, tmKeyMetaLen(tokenID)) == 0 {
		t.Fatal("token metadata length gone after Finalise — storage wiped")
	}
}

func TestTokenManager_RevertRollsBackCreate(t *testing.T) {
	evm, sdb := newTestEVM(t)
	caller := common.HexToAddress("0x2222222222222222222222222222222222222222")
	sdb.CreateAccount(caller)
	sdb.SetNonce(caller, 1)

	snap := sdb.Snapshot()

	payload := append([]byte{0x01}, bytes.Repeat([]byte{0x42}, 32)...)
	tm := &novaTokenManager{}
	tokenIDBytes, err := tm.RunStateful(evm, caller, payload, false)
	if err != nil {
		t.Fatalf("createToken: %v", err)
	}
	tokenID := common.BytesToHash(tokenIDBytes)
	if tmReadUint64(sdb, tmKeyMetaLen(tokenID)) == 0 {
		t.Fatal("pre-revert: token should exist")
	}

	// Outer tx reverts. With the old rawdb implementation the token would
	// persist forever, denying anyone else from ever minting the same ID.
	sdb.RevertToSnapshot(snap)

	if tmReadUint64(sdb, tmKeyMetaLen(tokenID)) != 0 {
		t.Fatal("post-revert: token metadata persisted through RevertToSnapshot — rawdb-style leak regressed")
	}
	if got := tmReadUint64(sdb, tmKeyCount(caller)); got != 0 {
		t.Fatalf("post-revert: per-creator count=%d, expected 0", got)
	}
}

// ---------------------------------------------------------------------------
// Shielded pool (0xAA26 meta + 0xdEaD balance)
// ---------------------------------------------------------------------------

func TestShieldedPool_RevertRollsBackCommitment(t *testing.T) {
	evm, sdb := newTestEVM(t)
	caller := common.HexToAddress("0x3333333333333333333333333333333333333333")
	sdb.CreateAccount(caller)
	sdb.SetNonce(caller, 1)
	amt := uint256.NewInt(1_000_000_000_000_000_000) // 1 NOVA
	sdb.AddBalance(caller, amt)

	snap := sdb.Snapshot()

	commitment := common.HexToHash("0xabcdef0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d")
	input := make([]byte, 1+32+32)
	input[0] = 0x01
	copy(input[1:33], commitment.Bytes())
	copy(input[33:65], common.BigToHash(amt.ToBig()).Bytes())

	pool := &novaShieldedPool{}
	if _, err := pool.RunStateful(evm, caller, input, false); err != nil {
		t.Fatalf("shield: %v", err)
	}
	if !spHasCommitment(sdb, commitment) {
		t.Fatal("pre-revert: commitment should be recorded")
	}

	sdb.RevertToSnapshot(snap)

	if spHasCommitment(sdb, commitment) {
		t.Fatal("post-revert: commitment leaked through revert — original DOS vector regressed")
	}
	if spReadTotal(sdb).Sign() != 0 {
		t.Fatalf("post-revert: total=%s, expected 0", spReadTotal(sdb))
	}
}

func TestShieldedPool_EIP161_GuardSurvivesFinalise(t *testing.T) {
	evm, sdb := newTestEVM(t)
	caller := common.HexToAddress("0x4444444444444444444444444444444444444444")
	sdb.CreateAccount(caller)
	sdb.SetNonce(caller, 1)
	amt := uint256.NewInt(5_000_000_000_000_000_000)
	sdb.AddBalance(caller, amt)

	input := make([]byte, 1+32+32)
	input[0] = 0x01
	commitment := common.HexToHash("0xdead0000000000000000000000000000000000000000000000000000beef0001")
	copy(input[1:33], commitment.Bytes())
	copy(input[33:65], common.BigToHash(amt.ToBig()).Bytes())

	if _, err := (&novaShieldedPool{}).RunStateful(evm, caller, input, false); err != nil {
		t.Fatalf("shield: %v", err)
	}

	sdb.Finalise(true)

	if !sdb.Exist(shieldMetaAddr) {
		t.Fatal("shieldMetaAddr deleted by Finalise(true)")
	}
	if !spHasCommitment(sdb, commitment) {
		t.Fatal("commitment gone after Finalise — storage wiped by EIP-161")
	}
}

// ---------------------------------------------------------------------------
// Oracle (0xAA28)
// ---------------------------------------------------------------------------

func TestOracle_PriceRoundtripAcrossFinalise(t *testing.T) {
	evm, sdb := newTestEVM(t)
	caller := common.HexToAddress("0x5555555555555555555555555555555555555555")
	sdb.CreateAccount(caller)
	sdb.SetNonce(caller, 1)

	pairID := PairID("NOVA", "USD")
	price := big.NewInt(2_500_000) // 2.5 USD (6 decimals)

	input := make([]byte, 1+32+32+8)
	input[0] = 0x03
	copy(input[1:33], pairID.Bytes())
	copy(input[33:65], common.BigToHash(price).Bytes())
	// block at offset 65 is 8 bytes — leave zero.

	if _, err := (&novaOracle{}).RunStateful(evm, caller, input, false); err != nil {
		t.Fatalf("submitPrice: %v", err)
	}

	sdb.Finalise(true)

	if !sdb.Exist(oracleSystemAddr) {
		t.Fatal("oracleSystemAddr deleted by Finalise(true)")
	}

	// Read via getPrice(0x01) path.
	readIn := make([]byte, 1+32)
	readIn[0] = 0x01
	copy(readIn[1:33], pairID.Bytes())
	out, err := (&novaOracle{}).RunStateful(evm, caller, readIn, true)
	if err != nil {
		t.Fatalf("getPrice: %v", err)
	}
	if got := new(big.Int).SetBytes(out); got.Cmp(price) != 0 {
		t.Fatalf("getPrice round-trip: want %s, got %s", price, got)
	}
}

func TestOracle_CircuitBreakerRejectsLargeSwing(t *testing.T) {
	evm, sdb := newTestEVM(t)
	caller := common.HexToAddress("0x5555555555555555555555555555555555555556")
	sdb.CreateAccount(caller)
	sdb.SetNonce(caller, 1)

	pairID := PairID("NOVA", "USD")

	mkInput := func(price *big.Int) []byte {
		in := make([]byte, 1+32+32+8)
		in[0] = 0x03
		copy(in[1:33], pairID.Bytes())
		copy(in[33:65], common.BigToHash(price).Bytes())
		return in
	}

	or := &novaOracle{}
	if _, err := or.RunStateful(evm, caller, mkInput(big.NewInt(1000)), false); err != nil {
		t.Fatalf("first submit: %v", err)
	}
	// +50%: far beyond the 15% circuit-breaker threshold.
	_, err := or.RunStateful(evm, caller, mkInput(big.NewInt(1500)), false)
	if err == nil {
		t.Fatal("expected circuit breaker to reject 50% swing, got nil error")
	}
}

// ---------------------------------------------------------------------------
// Contract upgrade (0xAA27)
// ---------------------------------------------------------------------------

func TestUpgrade_CancelClearsRequest(t *testing.T) {
	evm, sdb := newTestEVM(t)
	caller := common.HexToAddress("0x6666666666666666666666666666666666666666")
	target := common.HexToAddress("0x7777777777777777777777777777777777777777")

	// initiateUpgrade
	code := bytes.Repeat([]byte{0x60, 0x01, 0x60, 0x02}, 40) // 160 bytes = 5 chunks
	initIn := append([]byte{0x01}, target.Bytes()...)
	initIn = append(initIn, code...)
	up := &novaContractUpgrade{}
	if _, err := up.RunStateful(evm, caller, initIn, false); err != nil {
		t.Fatalf("initiate: %v", err)
	}
	if !ugHasPendingUpgrade(sdb, target) {
		t.Fatal("after initiate: pending should be true")
	}

	// cancelUpgrade
	cancelIn := append([]byte{0x02}, target.Bytes()...)
	if _, err := up.RunStateful(evm, caller, cancelIn, false); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	if ugHasPendingUpgrade(sdb, target) {
		t.Fatal("after cancel: pending should be false")
	}
	// Verify the code chunks were zeroed too, not just the len.
	for i := uint64(0); i < 5; i++ {
		if (sdb.GetState(upgradeSystemAddr, ugKeyCodeChunk(target, i)) != common.Hash{}) {
			t.Fatalf("chunk %d not zeroed by cancelUpgrade", i)
		}
	}
}

func TestUpgrade_RevertRollsBackInitiate(t *testing.T) {
	evm, sdb := newTestEVM(t)
	caller := common.HexToAddress("0x8888888888888888888888888888888888888888")
	target := common.HexToAddress("0x9999999999999999999999999999999999999999")

	snap := sdb.Snapshot()

	code := bytes.Repeat([]byte{0xAA}, 100)
	initIn := append([]byte{0x01}, target.Bytes()...)
	initIn = append(initIn, code...)
	if _, err := (&novaContractUpgrade{}).RunStateful(evm, caller, initIn, false); err != nil {
		t.Fatalf("initiate: %v", err)
	}
	if !ugHasPendingUpgrade(sdb, target) {
		t.Fatal("pre-revert: pending should be true")
	}

	sdb.RevertToSnapshot(snap)

	if ugHasPendingUpgrade(sdb, target) {
		t.Fatal("post-revert: pending upgrade leaked through revert")
	}
}

// ---------------------------------------------------------------------------
// AccountManager (0xAA22) — Fix #1 EIP-161 guard
// ---------------------------------------------------------------------------

func TestAccountManager_EIP161_SetGuardiansSurvivesFinalise(t *testing.T) {
	evm, sdb := newTestEVM(t)
	owner := common.HexToAddress("0xaaaa000000000000000000000000000000000001")
	sdb.CreateAccount(owner)
	sdb.SetNonce(owner, 1)

	// setGuardians (0x01): threshold=1, one guardian.
	guardian := common.HexToAddress("0xbbbb000000000000000000000000000000000001")
	input := []byte{0x01}
	input = append(input, 0x01) // threshold
	input = append(input, guardian.Bytes()...)

	am := &novaAccountManager{}
	if _, err := am.RunStateful(evm, owner, input, false); err != nil {
		t.Fatalf("setGuardians: %v", err)
	}

	sdb.Finalise(true)

	if !sdb.Exist(accountManagerSystemAddr) {
		t.Fatal("accountManagerSystemAddr deleted by Finalise(true) — missing EIP-161 guard means setGuardians is ephemeral")
	}
	if sdb.GetNonce(accountManagerSystemAddr) == 0 {
		t.Fatal("accountManagerSystemAddr nonce zeroed — guard did not set nonce")
	}

	// getGuardians must still see our guardian after Finalise.
	readIn := append([]byte{0x02}, owner.Bytes()...)
	out, err := am.RunStateful(evm, owner, readIn, true)
	if err != nil {
		t.Fatalf("getGuardians: %v", err)
	}
	// Result format: threshold(32) + count(32) + guardian(32)... — find the guardian bytes.
	if !bytes.Contains(out, guardian.Bytes()) {
		t.Fatalf("getGuardians after Finalise missing guardian; out=%x", out)
	}
}
