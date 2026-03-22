package ethernova

import (
	_ "embed"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params/types/genesisT"
)

const ExpectedGenesisHashHex = "0xc3812eb81498965a3f9ff3e73d2f423934e6d440578d4f4fbb6623cc61c453d9"

var ExpectedGenesisHash = common.HexToHash(ExpectedGenesisHashHex)

//go:embed genesis-121525-alloc.json
var genesisJSON []byte

// EmbeddedGenesisJSON returns the embedded Ethernova mainnet genesis JSON.
func EmbeddedGenesisJSON() []byte {
	return genesisJSON
}

// EmbeddedGenesisSHA256Hex returns the SHA256 hash of the embedded genesis JSON.
func EmbeddedGenesisSHA256Hex() string {
	sum := sha256.Sum256(genesisJSON)
	return hex.EncodeToString(sum[:])
}

// MustGenesis returns the embedded Ethernova genesis or panics if invalid.
func MustGenesis() *genesisT.Genesis {
	genesis := new(genesisT.Genesis)
	if err := genesis.UnmarshalJSON(genesisJSON); err != nil {
		panic(fmt.Errorf("invalid embedded ethernova genesis: %w", err))
	}
	return genesis
}
