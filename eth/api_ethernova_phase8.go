package eth

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params/ethernova"
)

// DomainResult is the explorer/tooling view for NIP-0004 Phase 6 bytecode
// domain metadata. Domain 0 is backward-compatible bytecode with no prefix;
// Domain 1/2 are EF01/EF02-prefixed runtime code.
type DomainResult struct {
	Address                common.Address `json:"address"`
	IsContract             bool           `json:"isContract"`
	CodeSize               int            `json:"codeSize"`
	RuntimeCodeSize        int            `json:"runtimeCodeSize"`
	CodeHash               common.Hash    `json:"codeHash"`
	Domain                 uint8          `json:"domain"`
	DomainName             string         `json:"domainName"`
	Prefix                 string         `json:"prefix"`
	PrefixBytes            int            `json:"prefixBytes"`
	CanCallNovaPrecompiles bool           `json:"canCallNovaPrecompiles"`
}

// GetDomain returns the declared execution domain for a contract or EOA.
// Wire names: ethernova_getDomain and nova_getDomain.
func (api *EthernovaAPI) GetDomain(addressHex string) (*DomainResult, error) {
	statedb, err := api.getStateDB()
	if err != nil {
		return nil, err
	}
	addr := common.HexToAddress(addressHex)
	code := statedb.GetCode(addr)
	domain, prefixLen := vm.InspectExecutionDomain(code)
	prefix := "none"
	if prefixLen > 0 {
		prefix = "0x" + common.Bytes2Hex(code[:prefixLen])
	}
	isContract := len(code) > 0
	return &DomainResult{
		Address:                addr,
		IsContract:             isContract,
		CodeSize:               len(code),
		RuntimeCodeSize:        len(code) - prefixLen,
		CodeHash:               statedb.GetCodeHash(addr),
		Domain:                 uint8(domain),
		DomainName:             vm.ExecutionDomainName(domain),
		Prefix:                 prefix,
		PrefixBytes:            prefixLen,
		CanCallNovaPrecompiles: !isContract || domain >= vm.DomainNova,
	}, nil
}

// CapabilityResult exposes the default effective Phase 6 capability set for
// an address at the current head. It is intentionally read-only and mirrors
// runtime rules: EOAs can call Nova precompiles directly, while Domain 0
// contracts cannot.
type CapabilityResult struct {
	Address                  common.Address     `json:"address"`
	IsContract               bool               `json:"isContract"`
	Domain                   uint8              `json:"domain"`
	DomainName               string             `json:"domainName"`
	CapabilityMask           string             `json:"capabilityMask"`
	Capabilities             []string           `json:"capabilities"`
	CapabilityDetails        []CapabilityDetail `json:"capabilityDetails"`
	PrecompileRequirements   []PrecompileGate   `json:"precompileRequirements"`
	LegacyPrecompilesUngated []string           `json:"legacyPrecompilesUngated"`
	Notes                    []string           `json:"notes"`
}

type CapabilityDetail struct {
	Name    string `json:"name"`
	Bit     uint64 `json:"bit"`
	Hex     string `json:"hex"`
	Enabled bool   `json:"enabled"`
}

type PrecompileGate struct {
	Address    string `json:"address"`
	Name       string `json:"name"`
	Capability string `json:"capability"`
	Mask       string `json:"mask"`
}

var phase8CapabilityCatalog = []vm.CapabilityMask{
	vm.CapabilityProtocolObjects,
	vm.CapabilityDeferredQueue,
	vm.CapabilityContentRegistry,
	vm.CapabilityMailboxManager,
	vm.CapabilityStateWitness,
	vm.CapabilityMailboxOps,
	vm.CapabilitySessionArbiter,
}

// GetCapabilities returns the Phase 6 capability mask that applies to an
// address before call-chain narrowing. Wire names: ethernova_getCapabilities
// and nova_getCapabilities.
func (api *EthernovaAPI) GetCapabilities(addressHex string) (*CapabilityResult, error) {
	statedb, err := api.getStateDB()
	if err != nil {
		return nil, err
	}
	addr := common.HexToAddress(addressHex)
	code := statedb.GetCode(addr)
	domain, _ := vm.InspectExecutionDomain(code)
	isContract := len(code) > 0
	mask := vm.DefaultCapabilitiesForDomain(domain)
	if !isContract && domain == vm.DomainLegacy {
		mask = vm.CapabilityNova
	}
	return &CapabilityResult{
		Address:                addr,
		IsContract:             isContract,
		Domain:                 uint8(domain),
		DomainName:             vm.ExecutionDomainName(domain),
		CapabilityMask:         capabilityMaskHex(mask),
		Capabilities:           vm.CapabilityNames(mask),
		CapabilityDetails:      capabilityDetails(mask),
		PrecompileRequirements: precompileGates(),
		LegacyPrecompilesUngated: []string{
			"0x20 novaBatchHash",
			"0x21 novaBatchVerify",
			"0x22 novaAccountManager",
			"0x23 novaFrameApprove",
			"0x24 novaFrameIntrospect",
			"0x25 novaTokenManager",
			"0x26 novaShieldedPool",
			"0x27 novaContractUpgrade",
			"0x28 novaOracle",
		},
		Notes: []string{
			"Direct EOA calls keep full Nova capability for developer tooling.",
			"Contract calls can only narrow capabilities down the call stack.",
			"Domain 0 contracts cannot call Domain 1/2 Nova precompiles.",
		},
	}, nil
}

func capabilityDetails(mask vm.CapabilityMask) []CapabilityDetail {
	out := make([]CapabilityDetail, 0, len(phase8CapabilityCatalog))
	for _, cap := range phase8CapabilityCatalog {
		out = append(out, CapabilityDetail{
			Name:    vm.CapabilityName(cap),
			Bit:     uint64(cap),
			Hex:     capabilityMaskHex(cap),
			Enabled: mask&cap != 0,
		})
	}
	return out
}

func precompileGates() []PrecompileGate {
	precompiles := []struct {
		addr string
		name string
	}{
		{"0x29", "novaProtocolObjectRegistry"},
		{"0x2A", "novaDeferredQueue"},
		{"0x2B", "novaContentRegistry"},
		{"0x2C", "novaMailboxManager"},
		{"0x2D", "novaSessionArbiter"},
		{"0x2F", "novaStateWitness"},
		{"0x35", "novaMailboxOps"},
	}
	out := make([]PrecompileGate, 0, len(precompiles))
	for _, p := range precompiles {
		addr := common.HexToAddress(p.addr)
		cap := vm.RequiredCapabilityForPrecompile(addr)
		out = append(out, PrecompileGate{
			Address:    p.addr,
			Name:       p.name,
			Capability: vm.CapabilityName(cap),
			Mask:       capabilityMaskHex(cap),
		})
	}
	return out
}

func capabilityMaskHex(mask vm.CapabilityMask) string {
	return fmt.Sprintf("0x%x", uint64(mask))
}

// SessionResult is the explorer/tooling view for a Phase 7 Session Protocol
// Object and its decoded arbiter state.
type SessionResult struct {
	ID                 common.Hash    `json:"id"`
	Exists             bool           `json:"exists"`
	Owner              common.Address `json:"owner"`
	ExpiryBlock        uint64         `json:"expiryBlock"`
	LastTouchedBlock   uint64         `json:"lastTouchedBlock"`
	RentBalance        string         `json:"rentBalance"`
	Initiator          common.Address `json:"initiator"`
	Counterparty       common.Address `json:"counterparty"`
	InitiatorSigner    common.Address `json:"initiatorSigner"`
	CounterpartySigner common.Address `json:"counterpartySigner"`
	SessionType        uint8          `json:"sessionType"`
	SessionTypeName    string         `json:"sessionTypeName"`
	Status             uint8          `json:"status"`
	StatusName         string         `json:"statusName"`
	StateHash          common.Hash    `json:"stateHash"`
	SequenceNumber     uint64         `json:"sequenceNumber"`
	TimeoutBlock       uint64         `json:"timeoutBlock"`
	DisputeDeadline    uint64         `json:"disputeDeadline"`
	DisputeRules       common.Hash    `json:"disputeRules"`
	OpenedBlock        uint64         `json:"openedBlock"`
	ClosedBlock        uint64         `json:"closedBlock"`
}

// GetSession returns a decoded Phase 7 Session by Protocol Object ID. Wire
// names: ethernova_getSession and nova_getSession.
func (api *EthernovaAPI) GetSession(idHex string) (*SessionResult, error) {
	statedb, err := api.getStateDB()
	if err != nil {
		return nil, err
	}
	id := common.HexToHash(idHex)
	obj := vm.SessGetSession(statedb, id)
	if obj == nil {
		return &SessionResult{ID: id, Exists: false}, nil
	}
	st := vm.SessGetSessionState(statedb, id)
	if st == nil {
		return &SessionResult{ID: id, Exists: false}, nil
	}
	rent := "0"
	if obj.RentBalance != nil {
		rent = obj.RentBalance.String()
	}
	return &SessionResult{
		ID:                 id,
		Exists:             true,
		Owner:              obj.Owner,
		ExpiryBlock:        obj.ExpiryBlock,
		LastTouchedBlock:   obj.LastTouchedBlock,
		RentBalance:        rent,
		Initiator:          st.Initiator,
		Counterparty:       st.Counterparty,
		InitiatorSigner:    st.InitiatorSigner,
		CounterpartySigner: st.CounterpartySigner,
		SessionType:        st.SessionType,
		SessionTypeName:    sessionTypeName(st.SessionType),
		Status:             st.Status,
		StatusName:         sessionStatusName(st.Status),
		StateHash:          st.StateHash,
		SequenceNumber:     st.SequenceNumber,
		TimeoutBlock:       st.TimeoutBlock,
		DisputeDeadline:    st.DisputeDeadline,
		DisputeRules:       st.DisputeRules,
		OpenedBlock:        st.OpenedBlock,
		ClosedBlock:        st.ClosedBlock,
	}, nil
}

// SessionConfig returns Phase 7 Session/Channel constants for SDKs and
// explorers. Wire names: ethernova_sessionConfig and nova_sessionConfig.
func (api *EthernovaAPI) SessionConfig() map[string]interface{} {
	head := api.e.blockchain.CurrentBlock().Number.Uint64()
	return map[string]interface{}{
		"forkBlock":              ethernova.SessionForkBlock,
		"active":                 head >= ethernova.SessionForkBlock,
		"currentBlock":           head,
		"precompile":             "0x2D",
		"arbiterAddress":         vm.SessionArbiterAddr.Hex(),
		"protocolObjectType":     types.ProtoTypeSession,
		"protocolObjectTypeName": types.ProtocolObjectTypeName(types.ProtoTypeSession),
		"minTimeoutBlocks":       ethernova.SessionMinTimeoutBlocks,
		"maxTimeoutBlocks":       ethernova.SessionMaxTimeoutBlocks,
		"disputeGraceBlocks":     ethernova.SessionDisputeGraceBlocks,
		"maxTimeoutsPerBlock":    ethernova.MaxSessionTimeoutsPerBlock,
		"maxStateBytes":          ethernova.MaxSessionStateBytes,
		"maxSignatures":          ethernova.MaxSessionSignatures,
		"supportedSessionTypes": []map[string]interface{}{
			{"tag": types.SessionTypeGeneric, "name": sessionTypeName(types.SessionTypeGeneric)},
			{"tag": types.SessionTypeChat, "name": sessionTypeName(types.SessionTypeChat)},
			{"tag": types.SessionTypeGame, "name": sessionTypeName(types.SessionTypeGame)},
		},
		"supportedStatuses": []map[string]interface{}{
			{"tag": types.SessionStatusOpen, "name": sessionStatusName(types.SessionStatusOpen)},
			{"tag": types.SessionStatusDisputed, "name": sessionStatusName(types.SessionStatusDisputed)},
			{"tag": types.SessionStatusClosed, "name": sessionStatusName(types.SessionStatusClosed)},
			{"tag": types.SessionStatusExpired, "name": sessionStatusName(types.SessionStatusExpired)},
		},
		"description": "NIP-0004 Phase 7: Session/Channel primitive — bilateral off-chain state with on-chain checkpoint, dispute, close, and timeout.",
	}
}

// DeveloperTooling returns the Phase 8 client/tooling surface. It gives
// explorers and SDKs one cheap endpoint to discover the canonical namespace.
func (api *EthernovaAPI) DeveloperTooling() map[string]interface{} {
	return map[string]interface{}{
		"phase":              10,
		"canonicalNamespace": "nova",
		"legacyNamespace":    "ethernova",
		"sdkPath":            "devnet/nova-sdk",
		"hardhatPluginPath":  "devnet/nova-hardhat-plugin",
		"rpcMethods": []string{
			"nova_getProtocolObject",
			"nova_getProtocolObjectTier",
			"nova_getMailbox",
			"nova_getMessages",
			"nova_getContentRef",
			"nova_getSession",
			"nova_getStateTier",
			"nova_getStateWitness",
			"nova_getPendingEffects",
			"nova_getCapabilities",
			"nova_getDomain",
			"nova_chatConfig",
			"nova_getChatMailbox",
			"nova_resourceConfig",
			"nova_resourcePrices",
			"nova_estimateResourceLimits",
			"nova_getResourceVector",
		},
	}
}

func sessionTypeName(sessionType uint8) string {
	switch sessionType {
	case types.SessionTypeGeneric:
		return "Generic"
	case types.SessionTypeChat:
		return "Chat"
	case types.SessionTypeGame:
		return "Game"
	default:
		return "Unknown"
	}
}

func sessionStatusName(status uint8) string {
	switch status {
	case types.SessionStatusOpen:
		return "Open"
	case types.SessionStatusDisputed:
		return "Disputed"
	case types.SessionStatusClosed:
		return "Closed"
	case types.SessionStatusExpired:
		return "Expired"
	default:
		return "Unknown"
	}
}
