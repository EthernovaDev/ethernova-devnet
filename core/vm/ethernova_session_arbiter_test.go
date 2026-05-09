package vm

import (
	"crypto/ecdsa"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func sessionWord(v *big.Int) []byte              { return common.BigToHash(v).Bytes() }
func sessionU64Word(v uint64) []byte             { return sessionWord(new(big.Int).SetUint64(v)) }
func sessionAddressWord(a common.Address) []byte { return common.BytesToHash(a.Bytes()).Bytes() }

func sessionOpenInput(counterparty common.Address, sessionType uint8, timeoutBlocks uint64) []byte {
	input := []byte{0x01}
	input = append(input, sessionAddressWord(counterparty)...)
	input = append(input, sessionU64Word(uint64(sessionType))...)
	input = append(input, sessionU64Word(timeoutBlocks)...)
	input = append(input, common.HexToHash("0x7777").Bytes()...)
	input = append(input, sessionU64Word(0)...)
	return input
}

func sessionSignedInput(selector byte, id common.Hash, seq uint64, stateHash common.Hash, sigs ...[]byte) []byte {
	input := []byte{selector}
	input = append(input, id.Bytes()...)
	input = append(input, sessionU64Word(seq)...)
	input = append(input, stateHash.Bytes()...)
	input = append(input, sessionU64Word(uint64(len(sigs)))...)
	for _, sig := range sigs {
		input = append(input, sig...)
	}
	return input
}

func sessionSign(t *testing.T, key *ecdsa.PrivateKey, id common.Hash, seq uint64, stateHash common.Hash) []byte {
	t.Helper()
	digest := types.SessionCommitMessageHash(id, seq, stateHash)
	sig, err := crypto.Sign(digest.Bytes(), key)
	if err != nil {
		t.Fatalf("crypto.Sign: %v", err)
	}
	return sig
}

func newSessionKeys(t *testing.T) (*ecdsa.PrivateKey, common.Address, *ecdsa.PrivateKey, common.Address) {
	t.Helper()
	ka, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey A: %v", err)
	}
	kb, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey B: %v", err)
	}
	return ka, crypto.PubkeyToAddress(ka.PublicKey), kb, crypto.PubkeyToAddress(kb.PublicKey)
}

func TestSessionArbiterOpenCommitDisputeClose(t *testing.T) {
	evm, sdb := newTestEVM(t)
	ka, initiator, kb, counterparty := newSessionKeys(t)
	sdb.CreateAccount(initiator)
	sdb.SetNonce(initiator, 1)
	sdb.SetCode(initiator, []byte{0xEF, 0x02}) // Domain 2 channel contract for strict write gate.
	sdb.CreateAccount(counterparty)
	sdb.SetNonce(counterparty, 1)

	arbiter := &novaSessionArbiter{}
	idBytes, err := arbiter.RunStateful(evm, initiator, sessionOpenInput(counterparty, types.SessionTypeChat, 40), false)
	if err != nil {
		t.Fatalf("openSession: %v", err)
	}
	id := common.BytesToHash(idBytes)
	st := SessGetSessionState(sdb, id)
	if st == nil || st.Status != types.SessionStatusOpen || st.Counterparty != counterparty || st.TimeoutBlock != evm.Context.BlockNumber.Uint64()+40 {
		t.Fatalf("unexpected opened session: %#v", st)
	}

	state1 := common.HexToHash("0x1001")
	sigA1 := sessionSign(t, ka, id, 1, state1)
	sigB1 := sessionSign(t, kb, id, 1, state1)
	if _, err := arbiter.RunStateful(evm, initiator, sessionSignedInput(0x02, id, 1, state1, sigA1, sigB1), false); err != nil {
		t.Fatalf("commitState seq1: %v", err)
	}
	st = SessGetSessionState(sdb, id)
	if st.SequenceNumber != 1 || st.StateHash != state1 {
		t.Fatalf("commit did not persist: %#v", st)
	}
	_, err = arbiter.RunStateful(evm, initiator, sessionSignedInput(0x02, id, 1, state1, sigA1, sigB1), false)
	if err == nil || !strings.Contains(err.Error(), "sequence must increase") {
		t.Fatalf("expected stale sequence rejection, got %v", err)
	}

	state2 := common.HexToHash("0x2002")
	sigA2 := sessionSign(t, ka, id, 2, state2)
	sigB2 := sessionSign(t, kb, id, 2, state2)
	if _, err := arbiter.RunStateful(evm, initiator, sessionSignedInput(0x04, id, 2, state2, sigA2, sigB2), false); err != nil {
		t.Fatalf("disputeSession seq2: %v", err)
	}
	st = SessGetSessionState(sdb, id)
	if st.Status != types.SessionStatusDisputed || st.SequenceNumber != 2 || st.DisputeDeadline == 0 {
		t.Fatalf("dispute did not persist: %#v", st)
	}

	state3 := common.HexToHash("0x3003")
	sigA3 := sessionSign(t, ka, id, 3, state3)
	sigB3 := sessionSign(t, kb, id, 3, state3)
	if _, err := arbiter.RunStateful(evm, initiator, sessionSignedInput(0x03, id, 3, state3, sigA3, sigB3), false); err != nil {
		t.Fatalf("closeSession seq3: %v", err)
	}
	st = SessGetSessionState(sdb, id)
	if st.Status != types.SessionStatusClosed || st.SequenceNumber != 3 || st.StateHash != state3 || st.ClosedBlock == 0 {
		t.Fatalf("close did not persist: %#v", st)
	}
}

func TestSessionArbiterRejectsInvalidSignatures(t *testing.T) {
	evm, sdb := newTestEVM(t)
	ka, initiator, _, counterparty := newSessionKeys(t)
	badKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey bad: %v", err)
	}
	sdb.CreateAccount(initiator)
	sdb.SetNonce(initiator, 1)
	sdb.SetCode(initiator, []byte{0xEF, 0x02})

	arbiter := &novaSessionArbiter{}
	idBytes, err := arbiter.RunStateful(evm, initiator, sessionOpenInput(counterparty, types.SessionTypeGeneric, 10), false)
	if err != nil {
		t.Fatalf("openSession: %v", err)
	}
	id := common.BytesToHash(idBytes)
	state := common.HexToHash("0x9999")
	sigA := sessionSign(t, ka, id, 1, state)
	sigBad := sessionSign(t, badKey, id, 1, state)
	_, err = arbiter.RunStateful(evm, initiator, sessionSignedInput(0x02, id, 1, state, sigA, sigBad), false)
	if err == nil || !strings.Contains(err.Error(), "non-participant") {
		t.Fatalf("expected non-participant signature rejection, got %v", err)
	}
}

func TestSessionArbiterDomainGate(t *testing.T) {
	evm, sdb := newTestEVM(t)
	_, caller, _, counterparty := newSessionKeys(t)
	sdb.CreateAccount(caller)
	sdb.SetNonce(caller, 1)
	sdb.SetCode(caller, []byte{0xEF, 0x01}) // Domain 1 Nova, not Domain 2 Channel.

	_, err := (&novaSessionArbiter{}).RunStateful(evm, caller, sessionOpenInput(counterparty, types.SessionTypeGeneric, 10), false)
	if err != ErrExecutionReverted {
		t.Fatalf("expected Domain 1 write gate rejection, got %v", err)
	}
}

func TestSessionTimeoutSweepExpiresDueSession(t *testing.T) {
	evm, sdb := newTestEVM(t)
	_, initiator, _, counterparty := newSessionKeys(t)
	sdb.CreateAccount(initiator)
	sdb.SetNonce(initiator, 1)
	sdb.SetCode(initiator, []byte{0xEF, 0x02})

	arbiter := &novaSessionArbiter{}
	idBytes, err := arbiter.RunStateful(evm, initiator, sessionOpenInput(counterparty, types.SessionTypeGeneric, 2), false)
	if err != nil {
		t.Fatalf("openSession: %v", err)
	}
	id := common.BytesToHash(idBytes)
	due := evm.Context.BlockNumber.Uint64() + 2
	processed, expired := ProcessSessionTimeouts(sdb, due, 10)
	if processed != 1 || expired != 1 {
		t.Fatalf("timeout sweep processed=%d expired=%d, want 1/1", processed, expired)
	}
	st := SessGetSessionState(sdb, id)
	if st.Status != types.SessionStatusExpired || st.ClosedBlock != due {
		t.Fatalf("session not expired by sweep: %#v", st)
	}
}
