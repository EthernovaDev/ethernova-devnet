// Ethernova: Global DB Registry
// Provides precompiles access to the chain database for persistent storage.
// Set once during node startup, used by all stateful precompiles.

package vm

import (
	"github.com/ethereum/go-ethereum/ethdb"
)

// GlobalChainDB is the chain database, set during node initialization.
// Used by precompiles that need persistent storage (tokens, privacy, oracle).
var GlobalChainDB ethdb.Database
