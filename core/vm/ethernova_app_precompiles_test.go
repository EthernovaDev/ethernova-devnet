package vm

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/params/ethernova"
)

func appTestWord(v uint64) []byte                   { return common.BigToHash(new(big.Int).SetUint64(v)).Bytes() }
func appTestHash(hex string) []byte                 { return common.HexToHash(hex).Bytes() }
func appTestAddressWord(addr common.Address) []byte { return common.BytesToHash(addr.Bytes()).Bytes() }

func appTestInput(selector byte, words ...[]byte) []byte {
	out := []byte{selector}
	for _, word := range words {
		out = append(out, common.LeftPadBytes(word, 32)...)
	}
	return out
}

func appTestBool(out []byte) bool {
	return len(out) >= 32 && common.BytesToHash(out[:32]) != (common.Hash{})
}

func TestApplicationPrecompileIdentityLifecycle(t *testing.T) {
	evm, sdb := newTestEVM(t)
	issuer := common.HexToAddress("0x1111111111111111111111111111111111111111")
	subject := common.HexToAddress("0x2222222222222222222222222222222222222222")
	sdb.CreateAccount(issuer)
	sdb.SetNonce(issuer, 1)

	identity := &novaIdentityAttestation{}
	create := appTestInput(0x01, appTestAddressWord(subject), appTestHash("0xfeed01"), appTestWord(0))
	idBytes, err := identity.RunStateful(evm, issuer, create, false)
	if err != nil {
		t.Fatalf("attest: %v", err)
	}
	id := common.BytesToHash(idBytes)
	verify, err := identity.RunStateful(evm, issuer, appTestInput(0x02, id.Bytes()), true)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !appTestBool(verify) {
		t.Fatal("identity attestation should verify before revoke")
	}
	get, err := identity.RunStateful(evm, issuer, appTestInput(0x04, id.Bytes()), true)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got := common.BytesToAddress(get[:32]); got != subject {
		t.Fatalf("subject mismatch: got %s want %s", got, subject)
	}
	if _, err := identity.RunStateful(evm, issuer, appTestInput(0x03, id.Bytes()), true); err != ErrWriteProtection {
		t.Fatalf("static revoke should hit write protection, got %v", err)
	}
	if _, err := identity.RunStateful(evm, issuer, appTestInput(0x03, id.Bytes()), false); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	verify, err = identity.RunStateful(evm, issuer, appTestInput(0x02, id.Bytes()), true)
	if err != nil {
		t.Fatalf("verify after revoke: %v", err)
	}
	if appTestBool(verify) {
		t.Fatal("identity attestation should not verify after revoke")
	}
}

func TestApplicationPrecompileSocialManifestGameBounty(t *testing.T) {
	evm, sdb := newTestEVM(t)
	alice := common.HexToAddress("0x3333333333333333333333333333333333333333")
	bob := common.HexToAddress("0x4444444444444444444444444444444444444444")
	sdb.CreateAccount(alice)
	sdb.SetNonce(alice, 1)

	social := &novaSocialGraph{}
	if _, err := social.RunStateful(evm, alice, appTestInput(0x01, appTestAddressWord(bob)), false); err != nil {
		t.Fatalf("follow: %v", err)
	}
	isFollowing, err := social.RunStateful(evm, alice, appTestInput(0x03, appTestAddressWord(alice), appTestAddressWord(bob)), true)
	if err != nil || !appTestBool(isFollowing) {
		t.Fatalf("isFollowing err=%v out=%x", err, isFollowing)
	}
	trust, err := social.RunStateful(evm, alice, appTestInput(0x04, appTestAddressWord(alice), appTestAddressWord(bob)), true)
	if err != nil || new(big.Int).SetBytes(trust[:32]).Uint64() != 50 {
		t.Fatalf("trustScore err=%v out=%x", err, trust)
	}

	manifest := &novaContentManifest{}
	manifestIDBytes, err := manifest.RunStateful(evm, alice, appTestInput(0x01, appTestHash("0xaaaa"), appTestHash("0xbbbb"), appTestHash("0xcccc"), appTestWord(4096)), false)
	if err != nil {
		t.Fatalf("create manifest: %v", err)
	}
	manifestID := common.BytesToHash(manifestIDBytes)
	manifestOK, err := manifest.RunStateful(evm, alice, appTestInput(0x02, manifestID.Bytes(), appTestHash("0xaaaa")), true)
	if err != nil || !appTestBool(manifestOK) {
		t.Fatalf("verify manifest err=%v out=%x", err, manifestOK)
	}

	game := &novaGameState{}
	gameID := common.HexToHash("0x7777")
	commit, err := game.RunStateful(evm, alice, appTestInput(0x01, gameID.Bytes(), appTestHash("0x1234"), appTestWord(1)), false)
	if err != nil {
		t.Fatalf("game commit: %v", err)
	}
	if len(commit) != 32 {
		t.Fatalf("game commit returned %d bytes", len(commit))
	}
	if _, err := game.RunStateful(evm, alice, appTestInput(0x01, gameID.Bytes(), appTestHash("0x1235"), appTestWord(1)), false); err == nil {
		t.Fatal("stale game turn should fail")
	}

	bounty := &novaComputeBounty{}
	bountyIDBytes, err := bounty.RunStateful(evm, alice, appTestInput(0x01, appTestHash("0xbeef"), appTestWord(100), appTestWord(0)), false)
	if err != nil {
		t.Fatalf("create bounty: %v", err)
	}
	submissionBytes, err := bounty.RunStateful(evm, bob, appTestInput(0x02, bountyIDBytes, appTestHash("0x5151"), appTestHash("0x6161")), false)
	if err != nil {
		t.Fatalf("submit bounty: %v", err)
	}
	submissionOK, err := bounty.RunStateful(evm, alice, appTestInput(0x03, submissionBytes, appTestHash("0x5151")), true)
	if err != nil || !appTestBool(submissionOK) {
		t.Fatalf("verify submission err=%v out=%x", err, submissionOK)
	}
}

func TestApplicationPrecompileCapabilityGate(t *testing.T) {
	addr := common.BytesToAddress([]byte{0x30})
	if RequiredCapabilityForPrecompile(addr) != CapabilityAppPrecompiles {
		t.Fatalf("0x30 capability = %s", CapabilityName(RequiredCapabilityForPrecompile(addr)))
	}
	if DefaultCapabilitiesForDomain(DomainLegacy)&CapabilityAppPrecompiles != 0 {
		t.Fatal("Domain 0 should not have application precompile capability")
	}
	if DefaultCapabilitiesForDomain(DomainNova)&CapabilityAppPrecompiles == 0 {
		t.Fatal("Domain 1 should include application precompile capability")
	}
}

func TestNovaOpcodeActivationAndNames(t *testing.T) {
	cfg := *params.AllEthashProtocolChanges
	cfg.ChainID = ethernova.NewChainIDBig
	jt := instructionSetForConfig(&cfg, false, big.NewInt(0), nil)
	if jt[MSEND].minStack != minStack(3, 1) || jt[SCLOSE].minStack != minStack(5, 1) {
		t.Fatalf("Nova opcode jump table not enabled: MSEND=%#v SCLOSE=%#v", jt[MSEND], jt[SCLOSE])
	}
	cases := map[OpCode]string{
		MSEND: "MSEND", MRECV: "MRECV", MPEEK: "MPEEK", MCOUNT: "MCOUNT",
		CREF: "CREF", CVERIFY: "CVERIFY", SOPEN: "SOPEN", SCOMMIT: "SCOMMIT", SCLOSE: "SCLOSE",
	}
	for op, name := range cases {
		if op.String() != name {
			t.Fatalf("opcode %#x string = %q want %q", byte(op), op.String(), name)
		}
		if StringToOp(name) != op {
			t.Fatalf("StringToOp(%s) = %#x want %#x", name, StringToOp(name), op)
		}
	}
}
