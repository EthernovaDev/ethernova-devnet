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

// sessionOpenInput builds openSession calldata. The two final signer args
// default to common.Address{} (zero) which the precompile auto-fills with
// caller/counterparty for backward-compatible behavior.
func sessionOpenInput(counterparty common.Address, sessionType uint8, timeoutBlocks uint64, signers ...common.Address) []byte {
	var initiatorSigner, counterpartySigner common.Address
	if len(signers) >= 1 {
		initiatorSigner = signers[0]
	}
	if len(signers) >= 2 {
		counterpartySigner = signers[1]
	}
	input := []byte{0x01}
	input = append(input, sessionAddressWord(counterparty)...)
	input = append(input, sessionU64Word(uint64(sessionType))...)
	input = append(input, sessionU64Word(timeoutBlocks)...)
	input = append(input, common.HexToHash("0x7777").Bytes()...)
	input = append(input, sessionU64Word(0)...)
	input = append(input, sessionAddressWord(initiatorSigner)...)
	input = append(input, sessionAddressWord(counterpartySigner)...)
	return input
}

func hasRevertReason(out []byte, want string) bool {
	return strings.Contains(string(out), want)
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
	out, err := arbiter.RunStateful(evm, initiator, sessionSignedInput(0x02, id, 1, state1, sigA1, sigB1), false)
	if err != ErrExecutionReverted || !hasRevertReason(out, "sequence must increase") {
		t.Fatalf("expected stale sequence rejection with reason, got err=%v out=%x", err, out)
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

// TestSessionArbiterContractCallerWithExplicitSigners proves that a Domain 2
// channel contract -- which by construction has no private key -- can open a
// session, delegate signing to two EOAs, and have the precompile accept their
// signatures on commitState.
func TestSessionArbiterContractCallerWithExplicitSigners(t *testing.T) {
	evm, sdb := newTestEVM(t)
	channel := common.HexToAddress("0xCAFEFEEDcafefEED00000000000000000000C001")
	sdb.CreateAccount(channel)
	sdb.SetNonce(channel, 1)
	sdb.SetCode(channel, []byte{0xEF, 0x02, 0x00})

	ka, signerA := func() (*ecdsa.PrivateKey, common.Address) {
		k, err := crypto.GenerateKey()
		if err != nil {
			t.Fatalf("GenerateKey signerA: %v", err)
		}
		return k, crypto.PubkeyToAddress(k.PublicKey)
	}()
	kb, signerB := func() (*ecdsa.PrivateKey, common.Address) {
		k, err := crypto.GenerateKey()
		if err != nil {
			t.Fatalf("GenerateKey signerB: %v", err)
		}
		return k, crypto.PubkeyToAddress(k.PublicKey)
	}()
	counterparty := common.HexToAddress("0xDEADBEEFdeadBEEF11111111111111111111B002")
	sdb.CreateAccount(counterparty)
	sdb.SetNonce(counterparty, 1)

	arbiter := &novaSessionArbiter{}
	idBytes, err := arbiter.RunStateful(evm, channel, sessionOpenInput(counterparty, types.SessionTypeChat, 40, signerA, signerB), false)
	if err != nil {
		t.Fatalf("openSession from Domain 2 contract: %v", err)
	}
	id := common.BytesToHash(idBytes)

	st := SessGetSessionState(sdb, id)
	if st == nil {
		t.Fatal("session not stored")
	}
	if st.Initiator != channel {
		t.Errorf("Initiator should be channel contract: got %s, want %s", st.Initiator, channel)
	}
	if st.InitiatorSigner != signerA {
		t.Errorf("InitiatorSigner should be signerA: got %s, want %s", st.InitiatorSigner, signerA)
	}
	if st.CounterpartySigner != signerB {
		t.Errorf("CounterpartySigner should be signerB: got %s, want %s", st.CounterpartySigner, signerB)
	}

	state1 := common.HexToHash("0xC0FFEE0001")
	sigA1 := sessionSign(t, ka, id, 1, state1)
	sigB1 := sessionSign(t, kb, id, 1, state1)
	if _, err := arbiter.RunStateful(evm, channel, sessionSignedInput(0x02, id, 1, state1, sigA1, sigB1), false); err != nil {
		t.Fatalf("commitState with explicit signers: %v", err)
	}

	after := SessGetSessionState(sdb, id)
	if after.SequenceNumber != 1 || after.StateHash != state1 {
		t.Fatalf("commit did not persist: %#v", after)
	}
}

// TestSessionArbiterRejectsNonSignerEvenIfParticipant locks in the new signer
// model: once explicit signer EOAs are configured, the nominal initiator/caller
// is not accepted unless it is also one of those signer EOAs.
func TestSessionArbiterRejectsNonSignerEvenIfParticipant(t *testing.T) {
	evm, sdb := newTestEVM(t)
	ka, signerA, kb, signerB := newSessionKeys(t)
	callerKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey caller: %v", err)
	}
	caller := crypto.PubkeyToAddress(callerKey.PublicKey)
	sdb.CreateAccount(caller)
	sdb.SetNonce(caller, 1)
	sdb.SetCode(caller, []byte{0xEF, 0x02})

	counterparty := common.HexToAddress("0xC0DEC0DEc0deC0DE22222222222222222222C002")

	arbiter := &novaSessionArbiter{}
	idBytes, err := arbiter.RunStateful(evm, caller, sessionOpenInput(counterparty, types.SessionTypeChat, 40, signerA, signerB), false)
	if err != nil {
		t.Fatalf("openSession: %v", err)
	}
	id := common.BytesToHash(idBytes)

	state1 := common.HexToHash("0xBADF00D0001")
	sigCaller := sessionSign(t, callerKey, id, 1, state1)
	sigB := sessionSign(t, kb, id, 1, state1)
	out, err := arbiter.RunStateful(evm, caller, sessionSignedInput(0x02, id, 1, state1, sigCaller, sigB), false)
	if err != ErrExecutionReverted || !hasRevertReason(out, "non-participant") {
		t.Fatalf("expected rejection of caller-as-signer when explicit signers configured, got err=%v out=%x", err, out)
	}

	sigA := sessionSign(t, ka, id, 1, state1)
	if _, err := arbiter.RunStateful(evm, caller, sessionSignedInput(0x02, id, 1, state1, sigA, sigB), false); err != nil {
		t.Fatalf("commit with explicit signers should succeed: %v", err)
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
	out, err := arbiter.RunStateful(evm, initiator, sessionSignedInput(0x02, id, 1, state, sigA, sigBad), false)
	if err != ErrExecutionReverted || !hasRevertReason(out, "non-participant") {
		t.Fatalf("expected non-participant signature rejection with reason, got err=%v out=%x", err, out)
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
