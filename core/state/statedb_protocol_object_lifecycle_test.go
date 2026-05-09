package state

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestProtocolObjectTouchesAreJournaled(t *testing.T) {
	state, err := New(types.EmptyRootHash, NewDatabase(rawdb.NewMemoryDatabase()), nil)
	if err != nil {
		t.Fatalf("New StateDB: %v", err)
	}
	id := common.HexToHash("0x1234")

	snap := state.Snapshot()
	state.RecordProtocolObjectTouch(id)
	if got := state.LifecycleTouchedObjects(); len(got) != 1 || got[0] != id {
		t.Fatalf("touch not recorded: %#v", got)
	}
	state.RevertToSnapshot(snap)
	if got := state.LifecycleTouchedObjects(); len(got) != 0 {
		t.Fatalf("reverted touch leaked into block set: %#v", got)
	}
}

func TestProtocolObjectTouchesSurviveFinaliseAndClearOnCommit(t *testing.T) {
	state, err := New(types.EmptyRootHash, NewDatabase(rawdb.NewMemoryDatabase()), nil)
	if err != nil {
		t.Fatalf("New StateDB: %v", err)
	}
	id := common.HexToHash("0xabcd")

	state.RecordProtocolObjectTouch(id)
	state.Finalise(true)
	if got := state.LifecycleTouchedObjects(); len(got) != 1 || got[0] != id {
		t.Fatalf("touch should survive tx Finalise until block hook runs: %#v", got)
	}
	if _, err := state.Commit(1, true); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if got := state.LifecycleTouchedObjects(); len(got) != 0 {
		t.Fatalf("touch set should clear after block commit: %#v", got)
	}
}
