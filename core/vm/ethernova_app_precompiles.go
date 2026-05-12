// Ethernova: Application-Layer Precompiles (NIP-0004 Phase 11)
//
// Phase 11 adds higher-level application primitives on top of the lower
// Protocol Object, Mailbox, ContentRef, Session, Domain, and Resource layers.
// The original draft reused 0x2B/0x2C for app helpers, but those slots are
// already live in this codebase as ContentRegistry and MailboxManager. The
// devnet-safe map is therefore:
//   0x30 novaAsyncCallback
//   0x31 novaIdentityAttestation
//   0x32 novaSocialGraph
//   0x33 novaContentManifest
//   0x34 novaGameState
//   0x36 novaComputeBounty
// 0x35 intentionally remains novaMailboxOps.

package vm

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params/ethernova"
)

const (
	appPrecompileGasRead   uint64 = 2000
	appPrecompileGasVerify uint64 = 4000
	appPrecompileGasWrite  uint64 = 18000
)

var AppPrecompileRegistryAddr = common.HexToAddress("0x000000000000000000000000000000000000FF11")

type novaAsyncCallback struct{}
type novaIdentityAttestation struct{}
type novaSocialGraph struct{}
type novaContentManifest struct{}
type novaGameState struct{}
type novaComputeBounty struct{}

var _ StatefulPrecompiledContract = (*novaAsyncCallback)(nil)
var _ StatefulPrecompiledContract = (*novaIdentityAttestation)(nil)
var _ StatefulPrecompiledContract = (*novaSocialGraph)(nil)
var _ StatefulPrecompiledContract = (*novaContentManifest)(nil)
var _ StatefulPrecompiledContract = (*novaGameState)(nil)
var _ StatefulPrecompiledContract = (*novaComputeBounty)(nil)

func (c *novaAsyncCallback) RequiredGas(input []byte) uint64       { return appRequiredGas(input) }
func (c *novaIdentityAttestation) RequiredGas(input []byte) uint64 { return appRequiredGas(input) }
func (c *novaSocialGraph) RequiredGas(input []byte) uint64         { return appRequiredGas(input) }
func (c *novaContentManifest) RequiredGas(input []byte) uint64     { return appRequiredGas(input) }
func (c *novaGameState) RequiredGas(input []byte) uint64           { return appRequiredGas(input) }
func (c *novaComputeBounty) RequiredGas(input []byte) uint64       { return appRequiredGas(input) }

func (c *novaAsyncCallback) Run(input []byte) ([]byte, error) {
	return nil, errors.New("novaAsyncCallback: requires stateful execution")
}
func (c *novaIdentityAttestation) Run(input []byte) ([]byte, error) {
	return nil, errors.New("novaIdentityAttestation: requires stateful execution")
}
func (c *novaSocialGraph) Run(input []byte) ([]byte, error) {
	return nil, errors.New("novaSocialGraph: requires stateful execution")
}
func (c *novaContentManifest) Run(input []byte) ([]byte, error) {
	return nil, errors.New("novaContentManifest: requires stateful execution")
}
func (c *novaGameState) Run(input []byte) ([]byte, error) {
	return nil, errors.New("novaGameState: requires stateful execution")
}
func (c *novaComputeBounty) Run(input []byte) ([]byte, error) {
	return nil, errors.New("novaComputeBounty: requires stateful execution")
}

func (c *novaAsyncCallback) RunStateful(evm *EVM, caller common.Address, input []byte, readOnly bool) ([]byte, error) {
	if err := appPrecompileActive(evm, input); err != nil {
		return nil, err
	}
	switch input[0] {
	case 0x01:
		if readOnly {
			return nil, ErrWriteProtection
		}
		return appRegisterAsyncCallback(evm, caller, input[1:])
	case 0x02:
		return appGetAsyncCallback(evm, input[1:])
	case 0x03:
		if readOnly {
			return nil, ErrWriteProtection
		}
		return appMarkAsyncCallbackFired(evm, caller, input[1:])
	case 0x04:
		return appAsyncCallbackReady(evm, input[1:])
	default:
		return nil, errors.New("novaAsyncCallback: unknown selector")
	}
}

func (c *novaIdentityAttestation) RunStateful(evm *EVM, caller common.Address, input []byte, readOnly bool) ([]byte, error) {
	if err := appPrecompileActive(evm, input); err != nil {
		return nil, err
	}
	switch input[0] {
	case 0x01:
		if readOnly {
			return nil, ErrWriteProtection
		}
		return appCreateIdentityAttestation(evm, caller, input[1:])
	case 0x02:
		return appVerifyIdentityAttestation(evm, input[1:])
	case 0x03:
		if readOnly {
			return nil, ErrWriteProtection
		}
		return appRevokeIdentityAttestation(evm, caller, input[1:])
	case 0x04:
		return appGetIdentityAttestation(evm, input[1:])
	default:
		return nil, errors.New("novaIdentityAttestation: unknown selector")
	}
}

func (c *novaSocialGraph) RunStateful(evm *EVM, caller common.Address, input []byte, readOnly bool) ([]byte, error) {
	if err := appPrecompileActive(evm, input); err != nil {
		return nil, err
	}
	switch input[0] {
	case 0x01:
		if readOnly {
			return nil, ErrWriteProtection
		}
		return appFollow(evm, caller, input[1:])
	case 0x02:
		if readOnly {
			return nil, ErrWriteProtection
		}
		return appUnfollow(evm, caller, input[1:])
	case 0x03:
		return appIsFollowing(evm, input[1:])
	case 0x04:
		return appTrustScore(evm, input[1:])
	default:
		return nil, errors.New("novaSocialGraph: unknown selector")
	}
}

func (c *novaContentManifest) RunStateful(evm *EVM, caller common.Address, input []byte, readOnly bool) ([]byte, error) {
	if err := appPrecompileActive(evm, input); err != nil {
		return nil, err
	}
	switch input[0] {
	case 0x01:
		if readOnly {
			return nil, ErrWriteProtection
		}
		return appCreateContentManifest(evm, caller, input[1:])
	case 0x02:
		return appVerifyContentManifest(evm, input[1:])
	case 0x03:
		return appGetContentManifest(evm, input[1:])
	default:
		return nil, errors.New("novaContentManifest: unknown selector")
	}
}

func (c *novaGameState) RunStateful(evm *EVM, caller common.Address, input []byte, readOnly bool) ([]byte, error) {
	if err := appPrecompileActive(evm, input); err != nil {
		return nil, err
	}
	switch input[0] {
	case 0x01:
		if readOnly {
			return nil, ErrWriteProtection
		}
		return appCommitGameState(evm, caller, input[1:])
	case 0x02:
		if readOnly {
			return nil, ErrWriteProtection
		}
		return appRevealGameState(evm, caller, input[1:])
	case 0x03:
		return appGetGameState(evm, input[1:])
	default:
		return nil, errors.New("novaGameState: unknown selector")
	}
}

func (c *novaComputeBounty) RunStateful(evm *EVM, caller common.Address, input []byte, readOnly bool) ([]byte, error) {
	if err := appPrecompileActive(evm, input); err != nil {
		return nil, err
	}
	switch input[0] {
	case 0x01:
		if readOnly {
			return nil, ErrWriteProtection
		}
		return appCreateComputeBounty(evm, caller, input[1:])
	case 0x02:
		if readOnly {
			return nil, ErrWriteProtection
		}
		return appSubmitComputeBounty(evm, caller, input[1:])
	case 0x03:
		return appVerifyComputeSubmission(evm, input[1:])
	case 0x04:
		return appGetComputeBounty(evm, input[1:])
	default:
		return nil, errors.New("novaComputeBounty: unknown selector")
	}
}

func appRequiredGas(input []byte) uint64 {
	if len(input) == 0 {
		return 0
	}
	switch input[0] {
	case 0x01:
		return appPrecompileGasWrite
	case 0x02, 0x03, 0x04:
		return appPrecompileGasVerify
	default:
		return appPrecompileGasRead
	}
}

func appPrecompileActive(evm *EVM, input []byte) error {
	if len(input) == 0 {
		return errors.New("application precompile: empty input")
	}
	if evm.Context.BlockNumber.Uint64() < ethernova.ApplicationPrecompileForkBlock {
		return errors.New("application precompile: not yet active")
	}
	return nil
}

func appEnsureRegistryExists(sdb StateDB) {
	if !sdb.Exist(AppPrecompileRegistryAddr) {
		sdb.CreateAccount(AppPrecompileRegistryAddr)
	}
	if sdb.GetNonce(AppPrecompileRegistryAddr) == 0 {
		sdb.SetNonce(AppPrecompileRegistryAddr, 1)
	}
}

func appKey(parts ...[]byte) common.Hash {
	return crypto.Keccak256Hash(parts...)
}

func appKindKey(kind string, id common.Hash, field string) common.Hash {
	return appKey([]byte("app"), []byte(kind), id.Bytes(), []byte(field))
}

func appIndexKey(kind string, parts ...[]byte) common.Hash {
	all := [][]byte{[]byte("app-index"), []byte(kind)}
	all = append(all, parts...)
	return appKey(all...)
}

func appReadUint64(sdb StateDB, key common.Hash) uint64 {
	return new(big.Int).SetBytes(sdb.GetState(AppPrecompileRegistryAddr, key).Bytes()).Uint64()
}

func appWriteUint64(sdb StateDB, key common.Hash, v uint64) {
	sdb.SetState(AppPrecompileRegistryAddr, key, common.BigToHash(new(big.Int).SetUint64(v)))
}

func appWriteHash(sdb StateDB, key common.Hash, v common.Hash) {
	sdb.SetState(AppPrecompileRegistryAddr, key, v)
}

func appReadHash(sdb StateDB, key common.Hash) common.Hash {
	return sdb.GetState(AppPrecompileRegistryAddr, key)
}

func appWriteAddress(sdb StateDB, key common.Hash, addr common.Address) {
	sdb.SetState(AppPrecompileRegistryAddr, key, common.BytesToHash(addr.Bytes()))
}

func appReadAddress(sdb StateDB, key common.Hash) common.Address {
	return common.BytesToAddress(sdb.GetState(AppPrecompileRegistryAddr, key).Bytes())
}

func appWriteBool(sdb StateDB, key common.Hash, enabled bool) {
	if enabled {
		sdb.SetState(AppPrecompileRegistryAddr, key, common.BytesToHash([]byte{0x01}))
		return
	}
	sdb.SetState(AppPrecompileRegistryAddr, key, common.Hash{})
}

func appReadBool(sdb StateDB, key common.Hash) bool {
	return sdb.GetState(AppPrecompileRegistryAddr, key) != (common.Hash{})
}

func appNextID(sdb StateDB, kind string, caller common.Address, blockNum uint64, seed ...[]byte) common.Hash {
	appEnsureRegistryExists(sdb)
	nonceKey := appIndexKey(kind, []byte("nonce"))
	nonce := appReadUint64(sdb, nonceKey)
	var blockBuf, nonceBuf [8]byte
	binary.BigEndian.PutUint64(blockBuf[:], blockNum)
	binary.BigEndian.PutUint64(nonceBuf[:], nonce)
	parts := [][]byte{[]byte("app-id"), []byte(kind), caller.Bytes(), blockBuf[:], nonceBuf[:]}
	parts = append(parts, seed...)
	id := appKey(parts...)
	appWriteUint64(sdb, nonceKey, nonce+1)
	return id
}

func appRequireLen(input []byte, need int, label string) error {
	if len(input) < need {
		return fmt.Errorf("%s: input too short (need %d, got %d)", label, need, len(input))
	}
	return nil
}

func appWord(input []byte, idx int) []byte      { return input[idx*32 : (idx+1)*32] }
func appHash(input []byte, idx int) common.Hash { return common.BytesToHash(appWord(input, idx)) }
func appAddress(input []byte, idx int) common.Address {
	return common.BytesToAddress(appWord(input, idx))
}

func appUint64(input []byte, idx int, label string) (uint64, error) {
	word := new(big.Int).SetBytes(appWord(input, idx))
	if word.BitLen() > 64 {
		return 0, fmt.Errorf("%s exceeds uint64", label)
	}
	return word.Uint64(), nil
}

func appWordUint64(v uint64) common.Hash {
	return common.BigToHash(new(big.Int).SetUint64(v))
}

func appWordBool(v bool) common.Hash {
	if v {
		return common.BytesToHash([]byte{0x01})
	}
	return common.Hash{}
}

func appReturn(words ...common.Hash) []byte {
	out := make([]byte, 0, len(words)*32)
	for _, w := range words {
		out = append(out, w.Bytes()...)
	}
	return out
}

func appReturnBool(v bool) []byte { return appReturn(appWordBool(v)) }

func appExists(sdb StateDB, kind string, id common.Hash) bool {
	return appReadBool(sdb, appKindKey(kind, id, "exists"))
}

func appSetExists(sdb StateDB, kind string, id common.Hash, exists bool) {
	appWriteBool(sdb, appKindKey(kind, id, "exists"), exists)
}

// 0x30 novaAsyncCallback -------------------------------------------------

func appRegisterAsyncCallback(evm *EVM, caller common.Address, input []byte) ([]byte, error) {
	if err := appRequireLen(input, 96, "registerAsyncCallback"); err != nil {
		return nil, err
	}
	target, err := appUint64(input, 2, "targetBlock")
	if err != nil {
		return nil, err
	}
	sdb := evm.StateDB
	id := appNextID(sdb, "async", caller, evm.Context.BlockNumber.Uint64(), input[:96])
	appSetExists(sdb, "async", id, true)
	appWriteHash(sdb, appKindKey("async", id, "condition"), appHash(input, 0))
	appWriteHash(sdb, appKindKey("async", id, "callback"), appHash(input, 1))
	appWriteUint64(sdb, appKindKey("async", id, "target"), target)
	appWriteAddress(sdb, appKindKey("async", id, "owner"), caller)
	appWriteBool(sdb, appKindKey("async", id, "fired"), false)
	return id.Bytes(), nil
}

func appGetAsyncCallback(evm *EVM, input []byte) ([]byte, error) {
	if err := appRequireLen(input, 32, "getAsyncCallback"); err != nil {
		return nil, err
	}
	id := appHash(input, 0)
	sdb := evm.StateDB
	if !appExists(sdb, "async", id) {
		return nil, errors.New("getAsyncCallback: callback not found")
	}
	return appReturn(
		appReadHash(sdb, appKindKey("async", id, "condition")),
		appReadHash(sdb, appKindKey("async", id, "callback")),
		appWordUint64(appReadUint64(sdb, appKindKey("async", id, "target"))),
		common.BytesToHash(appReadAddress(sdb, appKindKey("async", id, "owner")).Bytes()),
		appWordBool(appReadBool(sdb, appKindKey("async", id, "fired"))),
	), nil
}

func appMarkAsyncCallbackFired(evm *EVM, caller common.Address, input []byte) ([]byte, error) {
	if err := appRequireLen(input, 32, "markAsyncCallbackFired"); err != nil {
		return nil, err
	}
	id := appHash(input, 0)
	sdb := evm.StateDB
	if !appExists(sdb, "async", id) {
		return nil, errors.New("markAsyncCallbackFired: callback not found")
	}
	if appReadAddress(sdb, appKindKey("async", id, "owner")) != caller {
		return nil, errors.New("markAsyncCallbackFired: caller is not owner")
	}
	appWriteBool(sdb, appKindKey("async", id, "fired"), true)
	return appReturnBool(true), nil
}

func appAsyncCallbackReady(evm *EVM, input []byte) ([]byte, error) {
	if err := appRequireLen(input, 32, "asyncCallbackReady"); err != nil {
		return nil, err
	}
	id := appHash(input, 0)
	sdb := evm.StateDB
	if !appExists(sdb, "async", id) {
		return appReturnBool(false), nil
	}
	target := appReadUint64(sdb, appKindKey("async", id, "target"))
	fired := appReadBool(sdb, appKindKey("async", id, "fired"))
	return appReturnBool(!fired && evm.Context.BlockNumber.Uint64() >= target), nil
}

// 0x31 novaIdentityAttestation -------------------------------------------

func appCreateIdentityAttestation(evm *EVM, caller common.Address, input []byte) ([]byte, error) {
	if err := appRequireLen(input, 96, "attestIdentity"); err != nil {
		return nil, err
	}
	subject := appAddress(input, 0)
	if subject == (common.Address{}) {
		return nil, errors.New("attestIdentity: zero subject")
	}
	expiry, err := appUint64(input, 2, "expiryBlock")
	if err != nil {
		return nil, err
	}
	sdb := evm.StateDB
	id := appNextID(sdb, "identity", caller, evm.Context.BlockNumber.Uint64(), input[:96])
	appSetExists(sdb, "identity", id, true)
	appWriteAddress(sdb, appKindKey("identity", id, "subject"), subject)
	appWriteHash(sdb, appKindKey("identity", id, "claim"), appHash(input, 1))
	appWriteAddress(sdb, appKindKey("identity", id, "issuer"), caller)
	appWriteUint64(sdb, appKindKey("identity", id, "expiry"), expiry)
	appWriteBool(sdb, appKindKey("identity", id, "revoked"), false)
	return id.Bytes(), nil
}

func appIdentityValid(evm *EVM, id common.Hash) bool {
	sdb := evm.StateDB
	if !appExists(sdb, "identity", id) || appReadBool(sdb, appKindKey("identity", id, "revoked")) {
		return false
	}
	expiry := appReadUint64(sdb, appKindKey("identity", id, "expiry"))
	return expiry == 0 || evm.Context.BlockNumber.Uint64() <= expiry
}

func appVerifyIdentityAttestation(evm *EVM, input []byte) ([]byte, error) {
	if err := appRequireLen(input, 32, "verifyIdentity"); err != nil {
		return nil, err
	}
	return appReturnBool(appIdentityValid(evm, appHash(input, 0))), nil
}

func appRevokeIdentityAttestation(evm *EVM, caller common.Address, input []byte) ([]byte, error) {
	if err := appRequireLen(input, 32, "revokeIdentity"); err != nil {
		return nil, err
	}
	id := appHash(input, 0)
	sdb := evm.StateDB
	if !appExists(sdb, "identity", id) {
		return nil, errors.New("revokeIdentity: attestation not found")
	}
	if appReadAddress(sdb, appKindKey("identity", id, "issuer")) != caller {
		return nil, errors.New("revokeIdentity: caller is not issuer")
	}
	appWriteBool(sdb, appKindKey("identity", id, "revoked"), true)
	return appReturnBool(true), nil
}

func appGetIdentityAttestation(evm *EVM, input []byte) ([]byte, error) {
	if err := appRequireLen(input, 32, "getIdentity"); err != nil {
		return nil, err
	}
	id := appHash(input, 0)
	sdb := evm.StateDB
	if !appExists(sdb, "identity", id) {
		return nil, errors.New("getIdentity: attestation not found")
	}
	return appReturn(
		common.BytesToHash(appReadAddress(sdb, appKindKey("identity", id, "subject")).Bytes()),
		appReadHash(sdb, appKindKey("identity", id, "claim")),
		common.BytesToHash(appReadAddress(sdb, appKindKey("identity", id, "issuer")).Bytes()),
		appWordUint64(appReadUint64(sdb, appKindKey("identity", id, "expiry"))),
		appWordBool(appReadBool(sdb, appKindKey("identity", id, "revoked"))),
		appWordBool(appIdentityValid(evm, id)),
	), nil
}

// 0x32 novaSocialGraph ----------------------------------------------------

func appSocialEdgeKey(follower, target common.Address) common.Hash {
	return appIndexKey("social-edge", follower.Bytes(), target.Bytes())
}

func appSocialEdgeID(follower, target common.Address) common.Hash {
	return appKey([]byte("social-edge-id"), follower.Bytes(), target.Bytes())
}

func appFollow(evm *EVM, caller common.Address, input []byte) ([]byte, error) {
	if err := appRequireLen(input, 32, "follow"); err != nil {
		return nil, err
	}
	target := appAddress(input, 0)
	if target == (common.Address{}) || target == caller {
		return nil, errors.New("follow: invalid target")
	}
	appEnsureRegistryExists(evm.StateDB)
	appWriteBool(evm.StateDB, appSocialEdgeKey(caller, target), true)
	return appSocialEdgeID(caller, target).Bytes(), nil
}

func appUnfollow(evm *EVM, caller common.Address, input []byte) ([]byte, error) {
	if err := appRequireLen(input, 32, "unfollow"); err != nil {
		return nil, err
	}
	target := appAddress(input, 0)
	appEnsureRegistryExists(evm.StateDB)
	appWriteBool(evm.StateDB, appSocialEdgeKey(caller, target), false)
	return appReturnBool(true), nil
}

func appIsFollowing(evm *EVM, input []byte) ([]byte, error) {
	if err := appRequireLen(input, 64, "isFollowing"); err != nil {
		return nil, err
	}
	return appReturnBool(appReadBool(evm.StateDB, appSocialEdgeKey(appAddress(input, 0), appAddress(input, 1)))), nil
}

func appTrustScore(evm *EVM, input []byte) ([]byte, error) {
	if err := appRequireLen(input, 64, "trustScore"); err != nil {
		return nil, err
	}
	a, b := appAddress(input, 0), appAddress(input, 1)
	ab := appReadBool(evm.StateDB, appSocialEdgeKey(a, b))
	ba := appReadBool(evm.StateDB, appSocialEdgeKey(b, a))
	score := uint64(0)
	if ab && ba {
		score = 100
	} else if ab {
		score = 50
	}
	return appReturn(appWordUint64(score)), nil
}

// 0x33 novaContentManifest -----------------------------------------------

func appCreateContentManifest(evm *EVM, caller common.Address, input []byte) ([]byte, error) {
	if err := appRequireLen(input, 128, "createContentManifest"); err != nil {
		return nil, err
	}
	size, err := appUint64(input, 3, "size")
	if err != nil {
		return nil, err
	}
	sdb := evm.StateDB
	id := appNextID(sdb, "manifest", caller, evm.Context.BlockNumber.Uint64(), input[:128])
	appSetExists(sdb, "manifest", id, true)
	appWriteHash(sdb, appKindKey("manifest", id, "root"), appHash(input, 0))
	appWriteHash(sdb, appKindKey("manifest", id, "contentRef"), appHash(input, 1))
	appWriteHash(sdb, appKindKey("manifest", id, "mime"), appHash(input, 2))
	appWriteUint64(sdb, appKindKey("manifest", id, "size"), size)
	appWriteAddress(sdb, appKindKey("manifest", id, "owner"), caller)
	return id.Bytes(), nil
}

func appVerifyContentManifest(evm *EVM, input []byte) ([]byte, error) {
	if err := appRequireLen(input, 64, "verifyContentManifest"); err != nil {
		return nil, err
	}
	id := appHash(input, 0)
	ok := appExists(evm.StateDB, "manifest", id) && appReadHash(evm.StateDB, appKindKey("manifest", id, "root")) == appHash(input, 1)
	return appReturnBool(ok), nil
}

func appGetContentManifest(evm *EVM, input []byte) ([]byte, error) {
	if err := appRequireLen(input, 32, "getContentManifest"); err != nil {
		return nil, err
	}
	id := appHash(input, 0)
	sdb := evm.StateDB
	if !appExists(sdb, "manifest", id) {
		return nil, errors.New("getContentManifest: manifest not found")
	}
	return appReturn(
		appReadHash(sdb, appKindKey("manifest", id, "root")),
		appReadHash(sdb, appKindKey("manifest", id, "contentRef")),
		appReadHash(sdb, appKindKey("manifest", id, "mime")),
		appWordUint64(appReadUint64(sdb, appKindKey("manifest", id, "size"))),
		common.BytesToHash(appReadAddress(sdb, appKindKey("manifest", id, "owner")).Bytes()),
	), nil
}

// 0x34 novaGameState ------------------------------------------------------

func appCommitGameState(evm *EVM, caller common.Address, input []byte) ([]byte, error) {
	if err := appRequireLen(input, 96, "commitGameState"); err != nil {
		return nil, err
	}
	gameID, stateHash := appHash(input, 0), appHash(input, 1)
	turn, err := appUint64(input, 2, "turn")
	if err != nil {
		return nil, err
	}
	sdb := evm.StateDB
	currentTurn := appReadUint64(sdb, appKindKey("game", gameID, "turn"))
	if appReadBool(sdb, appKindKey("game", gameID, "exists")) && turn <= currentTurn {
		return nil, errors.New("commitGameState: turn must increase")
	}
	appEnsureRegistryExists(sdb)
	appWriteBool(sdb, appKindKey("game", gameID, "exists"), true)
	appWriteHash(sdb, appKindKey("game", gameID, "state"), stateHash)
	appWriteUint64(sdb, appKindKey("game", gameID, "turn"), turn)
	appWriteAddress(sdb, appKindKey("game", gameID, "player"), caller)
	commitmentID := appKey([]byte("game-commit"), gameID.Bytes(), stateHash.Bytes(), appWordUint64(turn).Bytes(), caller.Bytes())
	appWriteHash(sdb, appKindKey("game", gameID, "commitment"), commitmentID)
	return commitmentID.Bytes(), nil
}

func appRevealGameState(evm *EVM, caller common.Address, input []byte) ([]byte, error) {
	if err := appRequireLen(input, 128, "revealGameState"); err != nil {
		return nil, err
	}
	gameID, stateHash := appHash(input, 0), appHash(input, 1)
	turn, err := appUint64(input, 2, "turn")
	if err != nil {
		return nil, err
	}
	salt := appHash(input, 3)
	sdb := evm.StateDB
	if !appReadBool(sdb, appKindKey("game", gameID, "exists")) {
		return nil, errors.New("revealGameState: game not found")
	}
	if appReadHash(sdb, appKindKey("game", gameID, "state")) != stateHash || appReadUint64(sdb, appKindKey("game", gameID, "turn")) != turn {
		return nil, errors.New("revealGameState: state mismatch")
	}
	if appReadAddress(sdb, appKindKey("game", gameID, "player")) != caller {
		return nil, errors.New("revealGameState: caller is not current player")
	}
	revealHash := appKey([]byte("game-reveal"), gameID.Bytes(), stateHash.Bytes(), appWordUint64(turn).Bytes(), salt.Bytes())
	appWriteHash(sdb, appKindKey("game", gameID, "reveal"), revealHash)
	return revealHash.Bytes(), nil
}

func appGetGameState(evm *EVM, input []byte) ([]byte, error) {
	if err := appRequireLen(input, 32, "getGameState"); err != nil {
		return nil, err
	}
	gameID := appHash(input, 0)
	sdb := evm.StateDB
	if !appReadBool(sdb, appKindKey("game", gameID, "exists")) {
		return nil, errors.New("getGameState: game not found")
	}
	return appReturn(
		appReadHash(sdb, appKindKey("game", gameID, "state")),
		appWordUint64(appReadUint64(sdb, appKindKey("game", gameID, "turn"))),
		common.BytesToHash(appReadAddress(sdb, appKindKey("game", gameID, "player")).Bytes()),
		appReadHash(sdb, appKindKey("game", gameID, "reveal")),
	), nil
}

// 0x36 novaComputeBounty --------------------------------------------------

func appCreateComputeBounty(evm *EVM, caller common.Address, input []byte) ([]byte, error) {
	if err := appRequireLen(input, 96, "createComputeBounty"); err != nil {
		return nil, err
	}
	reward, err := appUint64(input, 1, "reward")
	if err != nil {
		return nil, err
	}
	expiry, err := appUint64(input, 2, "expiryBlock")
	if err != nil {
		return nil, err
	}
	sdb := evm.StateDB
	id := appNextID(sdb, "bounty", caller, evm.Context.BlockNumber.Uint64(), input[:96])
	appSetExists(sdb, "bounty", id, true)
	appWriteHash(sdb, appKindKey("bounty", id, "spec"), appHash(input, 0))
	appWriteUint64(sdb, appKindKey("bounty", id, "reward"), reward)
	appWriteUint64(sdb, appKindKey("bounty", id, "expiry"), expiry)
	appWriteAddress(sdb, appKindKey("bounty", id, "owner"), caller)
	appWriteBool(sdb, appKindKey("bounty", id, "open"), true)
	return id.Bytes(), nil
}

func appSubmitComputeBounty(evm *EVM, caller common.Address, input []byte) ([]byte, error) {
	if err := appRequireLen(input, 96, "submitComputeBounty"); err != nil {
		return nil, err
	}
	bountyID := appHash(input, 0)
	sdb := evm.StateDB
	if !appExists(sdb, "bounty", bountyID) || !appReadBool(sdb, appKindKey("bounty", bountyID, "open")) {
		return nil, errors.New("submitComputeBounty: bounty not open")
	}
	expiry := appReadUint64(sdb, appKindKey("bounty", bountyID, "expiry"))
	if expiry != 0 && evm.Context.BlockNumber.Uint64() > expiry {
		return nil, errors.New("submitComputeBounty: bounty expired")
	}
	submissionID := appNextID(sdb, "bounty-submission", caller, evm.Context.BlockNumber.Uint64(), input[:96])
	appSetExists(sdb, "bounty-submission", submissionID, true)
	appWriteHash(sdb, appKindKey("bounty-submission", submissionID, "bounty"), bountyID)
	appWriteHash(sdb, appKindKey("bounty-submission", submissionID, "result"), appHash(input, 1))
	appWriteHash(sdb, appKindKey("bounty-submission", submissionID, "proof"), appHash(input, 2))
	appWriteAddress(sdb, appKindKey("bounty-submission", submissionID, "submitter"), caller)
	return submissionID.Bytes(), nil
}

func appVerifyComputeSubmission(evm *EVM, input []byte) ([]byte, error) {
	if err := appRequireLen(input, 64, "verifyComputeSubmission"); err != nil {
		return nil, err
	}
	submissionID := appHash(input, 0)
	ok := appExists(evm.StateDB, "bounty-submission", submissionID) && appReadHash(evm.StateDB, appKindKey("bounty-submission", submissionID, "result")) == appHash(input, 1)
	return appReturnBool(ok), nil
}

func appGetComputeBounty(evm *EVM, input []byte) ([]byte, error) {
	if err := appRequireLen(input, 32, "getComputeBounty"); err != nil {
		return nil, err
	}
	id := appHash(input, 0)
	sdb := evm.StateDB
	if !appExists(sdb, "bounty", id) {
		return nil, errors.New("getComputeBounty: bounty not found")
	}
	return appReturn(
		appReadHash(sdb, appKindKey("bounty", id, "spec")),
		appWordUint64(appReadUint64(sdb, appKindKey("bounty", id, "reward"))),
		appWordUint64(appReadUint64(sdb, appKindKey("bounty", id, "expiry"))),
		common.BytesToHash(appReadAddress(sdb, appKindKey("bounty", id, "owner")).Bytes()),
		appWordBool(appReadBool(sdb, appKindKey("bounty", id, "open"))),
	), nil
}
