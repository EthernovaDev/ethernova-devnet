package vm

import (
	"math"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
)

func mailboxWord(v *big.Int) []byte {
	return common.BigToHash(v).Bytes()
}

func mailboxU64Word(v uint64) []byte {
	return mailboxWord(new(big.Int).SetUint64(v))
}

func mailboxManagerCreateInput(capacity, retentionPolicy, retentionBlocks, minPostage, aclMode, expiryBlock, rentPrepay, aclCount *big.Int) []byte {
	input := []byte{0x01}
	for _, word := range []*big.Int{
		capacity,
		retentionPolicy,
		retentionBlocks,
		minPostage,
		aclMode,
		expiryBlock,
		rentPrepay,
		aclCount,
	} {
		input = append(input, mailboxWord(word)...)
	}
	return input
}

func mailboxManagerConfigureInput(id common.Hash, capacity, retentionPolicy, retentionBlocks, minPostage, aclMode, expiryBlock, rentPrepay, aclCount *big.Int, acl ...common.Address) []byte {
	input := []byte{0x02}
	input = append(input, id.Bytes()...)
	for _, word := range []*big.Int{
		capacity,
		retentionPolicy,
		retentionBlocks,
		minPostage,
		aclMode,
		expiryBlock,
		rentPrepay,
		aclCount,
	} {
		input = append(input, mailboxWord(word)...)
	}
	for _, addr := range acl {
		input = append(input, common.BytesToHash(addr.Bytes()).Bytes()...)
	}
	return input
}

func createTestMailbox(t *testing.T, evm *EVM, owner common.Address, capacity uint64) common.Hash {
	t.Helper()
	input := mailboxManagerCreateInput(
		new(big.Int).SetUint64(capacity),
		big.NewInt(0),
		big.NewInt(0),
		big.NewInt(0),
		big.NewInt(0),
		big.NewInt(0),
		big.NewInt(0),
		big.NewInt(0),
	)
	idBytes, err := (&novaMailboxManager{}).RunStateful(evm, owner, input, false)
	if err != nil {
		t.Fatalf("createMailbox: %v", err)
	}
	return common.BytesToHash(idBytes)
}

func TestMailboxManagerRejectsOverflowingABIWords(t *testing.T) {
	evm, sdb := newTestEVM(t)
	caller := common.HexToAddress("0x1111111111111111111111111111111111111111")
	sdb.CreateAccount(caller)
	sdb.SetNonce(caller, 1)

	input := mailboxManagerCreateInput(
		big.NewInt(10),
		big.NewInt(256), // uint8 overflow; old code silently truncated to 0.
		big.NewInt(0),
		big.NewInt(0),
		big.NewInt(0),
		big.NewInt(0),
		big.NewInt(0),
		big.NewInt(0),
	)
	_, err := (&novaMailboxManager{}).RunStateful(evm, caller, input, false)
	if err == nil || !strings.Contains(err.Error(), "retentionPolicy exceeds uint8") {
		t.Fatalf("expected uint8 overflow rejection, got %v", err)
	}

	input = mailboxManagerCreateInput(
		new(big.Int).Lsh(big.NewInt(1), 64), // uint64 overflow.
		big.NewInt(0),
		big.NewInt(0),
		big.NewInt(0),
		big.NewInt(0),
		big.NewInt(0),
		big.NewInt(0),
		big.NewInt(0),
	)
	_, err = (&novaMailboxManager{}).RunStateful(evm, caller, input, false)
	if err == nil || !strings.Contains(err.Error(), "capacityLimit exceeds uint64") {
		t.Fatalf("expected uint64 overflow rejection, got %v", err)
	}
}

func TestMailboxConfigureRejectsCapacityBelowPendingUsage(t *testing.T) {
	evm, sdb := newTestEVM(t)
	owner := common.HexToAddress("0x2222222222222222222222222222222222222222")
	sdb.CreateAccount(owner)
	sdb.SetNonce(owner, 1)

	id := createTestMailbox(t, evm, owner, 3)
	mbWriteUint64(sdb, mbKeyCount(id), 2)
	mbWriteUint64(sdb, mbKeyPending(id), 1)

	input := mailboxManagerConfigureInput(
		id,
		big.NewInt(2),
		big.NewInt(0),
		big.NewInt(0),
		big.NewInt(0),
		big.NewInt(0),
		big.NewInt(0),
		big.NewInt(0),
		big.NewInt(0),
	)
	_, err := (&novaMailboxManager{}).RunStateful(evm, owner, input, false)
	if err == nil || !strings.Contains(err.Error(), "capacity 2 < current usage") {
		t.Fatalf("expected pending usage capacity rejection, got %v", err)
	}
}

func TestMailboxConfigurePersistsACL(t *testing.T) {
	evm, sdb := newTestEVM(t)
	owner := common.HexToAddress("0x5555555555555555555555555555555555555555")
	allowed := common.HexToAddress("0x6666666666666666666666666666666666666666")
	sdb.CreateAccount(owner)
	sdb.SetNonce(owner, 1)

	id := createTestMailbox(t, evm, owner, 3)
	input := mailboxManagerConfigureInput(
		id,
		big.NewInt(3),
		big.NewInt(0),
		big.NewInt(0),
		big.NewInt(0),
		big.NewInt(int64(types.MailboxACLModeWhitelist)),
		big.NewInt(0),
		big.NewInt(0),
		big.NewInt(1),
		allowed,
	)
	if _, err := (&novaMailboxManager{}).RunStateful(evm, owner, input, false); err != nil {
		t.Fatalf("configureMailbox: %v", err)
	}

	obj := MbGetMailbox(sdb, id)
	if obj == nil {
		t.Fatal("mailbox not found after configure")
	}
	cfg, err := types.DecodeMailboxConfig(obj.StateData)
	if err != nil {
		t.Fatalf("DecodeMailboxConfig: %v", err)
	}
	if cfg.ACLMode != types.MailboxACLModeWhitelist {
		t.Fatalf("ACLMode = %d, want whitelist", cfg.ACLMode)
	}
	if len(cfg.ACL) != 1 || cfg.ACL[0] != allowed {
		t.Fatalf("ACL = %#v, want [%s]", cfg.ACL, allowed.Hex())
	}
}

func TestMailboxSendCapacityCheckDoesNotOverflow(t *testing.T) {
	evm, sdb := newTestEVM(t)
	owner := common.HexToAddress("0x3333333333333333333333333333333333333333")
	sender := common.HexToAddress("0x4444444444444444444444444444444444444444")
	sdb.CreateAccount(owner)
	sdb.SetNonce(owner, 1)
	sdb.CreateAccount(sender)
	sdb.SetNonce(sender, 1)
	sdb.AddBalance(sender, uint256.NewInt(1_000_000))

	id := createTestMailbox(t, evm, owner, 10)
	mbWriteUint64(sdb, mbKeyCount(id), math.MaxUint64)
	mbWriteUint64(sdb, mbKeyPending(id), 1)

	input := []byte{0x01}
	input = append(input, id.Bytes()...)
	input = append(input, common.HexToHash("0x1234").Bytes()...)
	input = append(input, mailboxU64Word(0)...)
	_, err := (&novaMailboxOps{}).RunStateful(evm, sender, input, false)
	if err == nil || !strings.Contains(err.Error(), "mailbox full") {
		t.Fatalf("expected mailbox full rejection without uint64 wrap, got %v", err)
	}
}
