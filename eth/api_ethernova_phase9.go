package eth

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params/ethernova"
)

// ChatMailboxResult is a Phase 9 convenience view over the Phase 4 mailbox
// owner index. The chat profile itself is intentionally anchored as a
// ContentRef, not embedded into MailboxConfig, so existing mailbox RLP remains
// backward compatible.
type ChatMailboxResult struct {
	Owner              common.Address   `json:"owner"`
	MailboxCount       int              `json:"mailboxCount"`
	Mailboxes          []*MailboxResult `json:"mailboxes"`
	ProfileContentType string           `json:"profileContentType"`
	ProfileLookup      string           `json:"profileLookup"`
}

// ChatConfig returns the NIP-0003-over-NIP-0004 proving-ground conventions.
// Wire names: ethernova_chatConfig and nova_chatConfig.
func (api *EthernovaAPI) ChatConfig() map[string]interface{} {
	head := api.e.blockchain.CurrentBlock().Number.Uint64()
	return map[string]interface{}{
		"phase":        9,
		"active":       true,
		"currentBlock": head,
		"description":  "NIP-0003 chat rebase onto NIP-0004 primitives: mailbox discovery, ContentRef payload anchors, Session channels, and Domain 1 group fanout.",
		"primitives": map[string]interface{}{
			"mailboxManager":   "0x2C",
			"mailboxOps":       "0x35",
			"contentRegistry":  "0x2B",
			"sessionArbiter":   "0x2D",
			"deferredQueue":    "0x2A",
			"protocolRegistry": "0x29",
		},
		"forkBlocks": map[string]uint64{
			"mailbox":    ethernova.MailboxForkBlock,
			"contentRef": ethernova.ContentRefForkBlock,
			"session":    ethernova.SessionForkBlock,
		},
		"chatProfile": map[string]interface{}{
			"contentType": "application/ethernova.chat-profile+json",
			"fields": []string{
				"version",
				"owner",
				"mailboxId",
				"x25519PublicKey",
				"createdAtBlock",
				"profileNonce",
			},
			"anchor": "Create a ContentRef whose contentHash is sha256(canonical chat profile JSON), then advertise the ContentRef ID off-chain or in explorer metadata.",
		},
		"directMessages": map[string]interface{}{
			"sessionType":     "Chat",
			"sessionTypeCode": 1,
			"path":            "Open a Phase 7 chat session, exchange encrypted payloads P2P, periodically checkpoint stateHash via novaSessionArbiter, and optionally send mailbox notification hashes via novaMailboxOps.",
		},
		"groupChat": map[string]interface{}{
			"domain": "Domain 1 / Nova",
			"path":   "Domain 1 ChatRoom contracts fan out mailbox notifications through Deferred Processing while encrypted message bodies remain off-chain and hash-anchored by ContentRef.",
		},
		"compatibility": []string{
			"MailboxConfig RLP is unchanged.",
			"Existing mailbox objects remain valid.",
			"Phase 9 is a proving-ground client/tooling layer on top of Phase 3/4/7 primitives.",
		},
	}
}

// GetChatMailbox returns mailboxes owned by an account and the Phase 9 profile
// convention used by the SDK/harness. Wire names: ethernova_getChatMailbox and
// nova_getChatMailbox.
func (api *EthernovaAPI) GetChatMailbox(ownerHex string, offset, limit uint64) (*ChatMailboxResult, error) {
	statedb, err := api.getStateDB()
	if err != nil {
		return nil, err
	}
	if limit == 0 || limit > 25 {
		limit = 25
	}
	owner := common.HexToAddress(ownerHex)
	ids := vm.MbListByOwner(statedb, owner, offset, limit)
	mailboxes := make([]*MailboxResult, 0, len(ids))
	for _, id := range ids {
		obj := vm.MbGetMailbox(statedb, id)
		if obj == nil {
			continue
		}
		r, err := mailboxToResult(statedb, obj)
		if err != nil {
			continue
		}
		mailboxes = append(mailboxes, r)
	}
	return &ChatMailboxResult{
		Owner:              owner,
		MailboxCount:       len(mailboxes),
		Mailboxes:          mailboxes,
		ProfileContentType: "application/ethernova.chat-profile+json",
		ProfileLookup:      "Use nova_listContentRefs(owner, offset, limit) and filter ContentRefs whose contentType is application/ethernova.chat-profile+json.",
	}, nil
}
