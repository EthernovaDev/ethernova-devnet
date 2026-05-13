package eth

// api_ethernova_phase8_fixes.go
//
// Phase 8 audit follow-ups (BUG-1, BUG-2). Additive only: adds two RPC
// methods that were called out as missing in the Phase 8 audit, without
// touching consensus or storage layout.
//
//   * ListProtocolObjects(typeTag, owner, offset, limit)
//       Wire names: nova_listProtocolObjects, ethernova_listProtocolObjects.
//       Provides the (type, owner) filter the audit listed under BUG-1.
//       Owner is required because this build does not maintain a global
//       type index in state - type-only enumeration would require iterating
//       every owner address. The method scans the owner index, filters by
//       TypeTag at the API layer, and paginates the filtered result.
//
//   * GetDeferredStats(blockNumber)
//       Wire names: nova_getDeferredStats, ethernova_getDeferredStats.
//       Resolves BUG-2 by satisfying the spec method name. Only "latest"
//       (or an explicit current-head value, or empty) is fully supported;
//       any other block number returns a clear error because historical
//       per-block stats are not indexed in this build (the existing
//       comment on DeferredProcessingStats notes that historical stats
//       "can be reconstructed from logs"). Callers wanting truly historical
//       data should subscribe to the relevant deferred-queue logs.

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params/ethernova"
)

// listProtocolObjectsOwnerScanCap caps the owner-index scan window in
// ListProtocolObjects so a misuse against a hyper-active owner does not block
// an RPC worker. Real workloads should be far below this cap; if an owner
// genuinely has more than this many objects, a finer index is required.
const listProtocolObjectsOwnerScanCap uint64 = 10000

// listProtocolObjectsMaxLimit is the per-request page cap. Mirrors the cap
// applied by GetProtocolObjectsByOwner.
const listProtocolObjectsMaxLimit uint64 = 100

// ListProtocolObjects returns Protocol Object IDs that match the given
// (typeTag, owner) filter. typeTag must be one of the registered protocol
// object type tags. owner must be a non-zero address.
//
// The Phase 8 audit (BUG-1) tracks this as the proper backing for the spec
// method nova_listProtocolObjects(type, owner, offset, limit). It is
// implemented by scanning the owner index and filtering at the API layer:
// no state migration and no new storage keys.
func (api *EthernovaAPI) ListProtocolObjects(typeTag uint8, ownerHex string, offset, limit uint64) (interface{}, error) {
	statedb, err := api.getStateDB()
	if err != nil {
		return nil, err
	}
	if limit == 0 || limit > listProtocolObjectsMaxLimit {
		limit = listProtocolObjectsMaxLimit
	}
	owner := common.HexToAddress(ownerHex)
	if owner == (common.Address{}) {
		return nil, errors.New("listProtocolObjects: owner is required (no global type index in this build; pass a non-zero owner address)")
	}
	if typeTag == 0 || typeTag > types.MaxProtocolObjectTypes {
		return nil, fmt.Errorf("listProtocolObjects: invalid typeTag 0x%02x (valid range: 1..%d)", typeTag, types.MaxProtocolObjectTypes)
	}

	// Scan a generous window of the owner's objects. We deliberately fetch
	// more than the requested page because we still need to filter by type
	// before applying offset/limit.
	allIDs := vm.PoGetObjectsByOwner(statedb, owner, 0, listProtocolObjectsOwnerScanCap)

	matching := make([]common.Hash, 0, limit)
	for _, id := range allIDs {
		obj := vm.PoGetObject(statedb, id)
		if obj == nil {
			continue
		}
		if obj.TypeTag == typeTag {
			matching = append(matching, id)
		}
	}

	// Pagination on the filtered list.
	total := uint64(len(matching))
	if offset >= total {
		matching = nil
	} else {
		end := offset + limit
		if end > total {
			end = total
		}
		matching = matching[offset:end]
	}

	results := make([]string, len(matching))
	for i, id := range matching {
		results[i] = id.Hex()
	}
	return map[string]interface{}{
		"typeTag":       typeTag,
		"typeName":      protocolObjectTypeName(typeTag),
		"owner":         owner.Hex(),
		"count":         len(results),
		"offset":        offset,
		"limit":         limit,
		"ids":           results,
		"scannedSlots":  len(allIDs),
		"matchedBefore": int(total), // total matches before offset/limit
		"scanCap":       listProtocolObjectsOwnerScanCap,
	}, nil
}

// protocolObjectTypeName mirrors the small lookup used elsewhere in
// api_ethernova.go (ProtocolObjectConfig supportedTypes). Kept local to this
// file so we don't introduce a new exported helper.
func protocolObjectTypeName(tag uint8) string {
	switch tag {
	case types.ProtoTypeMailbox:
		return "Mailbox"
	case types.ProtoTypeSession:
		return "Session"
	case types.ProtoTypeContentReference:
		return "ContentReference"
	case types.ProtoTypeIdentity:
		return "Identity"
	case types.ProtoTypeSubscription:
		return "Subscription"
	case types.ProtoTypeGameRoom:
		return "GameRoom"
	default:
		return "Unknown"
	}
}

// GetDeferredStats is the spec-aligned name for DeferredProcessingStats. It
// accepts an optional block-number argument so tooling that follows the
// Phase 8 spec (nova_getDeferredStats(blockNumber)) does not see -32601.
//
// Supported blockNumber values:
//
//   - ""           - current head (latest)
//   - "latest"     - current head
//   - "pending"    - current head (no separate pending state for the queue)
//   - "0x...", "<decimal>", or "earliest" if the value equals the current
//     head, returns current head stats. Any other value returns an error.
//
// Historical stats are NOT reconstructed - this matches the long-standing
// comment on DeferredProcessingStats. Callers that need historical data
// should iterate the deferred-queue logs.
func (api *EthernovaAPI) GetDeferredStats(blockNumber string) (map[string]interface{}, error) {
	head, err := api.resolveDeferredStatsBlock(blockNumber)
	if err != nil {
		return nil, err
	}
	statedb, err := api.getStateDB()
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"requestedBlock":      blockNumber,
		"resolvedBlock":       head,
		"isHead":              true,
		"queueHead":           vm.DqGetHead(statedb),
		"queueTail":           vm.DqGetTail(statedb),
		"pendingCount":        vm.DqGetPendingCount(statedb),
		"totalProcessed":      vm.DqGetTotalProcessed(statedb),
		"enqueuesAtThisBlock": vm.DqGetEnqueueCountAtBlock(statedb, head),
		"queueAddress":        vm.DeferredQueueAddr.Hex(),
		"forkBlock":           ethernova.DeferredExecForkBlock,
		"forkActive":          head >= ethernova.DeferredExecForkBlock,
		"maxEnqueuePerBlock":  ethernova.MaxPendingEffectsPerBlock,
		"maxDrainPerBlock":    ethernova.MaxDeferredProcessingPerBlock,
		"maxPayloadBytes":     ethernova.MaxDeferredEffectPayloadBytes,
		"note":                "historical per-block stats are not indexed in this build; only current head is supported. Use deferred-queue logs for historical reconstruction.",
	}, nil
}

// resolveDeferredStatsBlock parses the blockNumber argument and either
// returns the current head (when the user wants latest / pending / current)
// or an error (when historical lookup is requested). It is intentionally
// strict: anything that looks like a non-current historical request fails
// with a clear message instead of silently returning head-state data.
func (api *EthernovaAPI) resolveDeferredStatsBlock(blockNumber string) (uint64, error) {
	head := api.e.blockchain.CurrentBlock().Number.Uint64()
	bn := strings.TrimSpace(strings.ToLower(blockNumber))
	if bn == "" || bn == "latest" || bn == "pending" || bn == "finalized" || bn == "safe" {
		return head, nil
	}
	// Numeric forms: hex (0x...) or decimal.
	var asked uint64
	if strings.HasPrefix(bn, "0x") {
		v, err := strconv.ParseUint(strings.TrimPrefix(bn, "0x"), 16, 64)
		if err != nil {
			return 0, fmt.Errorf("getDeferredStats: invalid hex blockNumber %q: %w", blockNumber, err)
		}
		asked = v
	} else if bn == "earliest" {
		asked = 0
	} else {
		v, err := strconv.ParseUint(bn, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("getDeferredStats: unrecognized blockNumber %q (use \"latest\", a hex number, or a decimal)", blockNumber)
		}
		asked = v
	}
	if asked == head {
		return head, nil
	}
	return 0, fmt.Errorf("getDeferredStats: historical lookup at block %d not supported (head=%d); use deferred-queue logs for historical data", asked, head)
}
