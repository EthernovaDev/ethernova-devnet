package ethernova

import (
	_ "embed"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params/types/genesisT"
)

const ExpectedGenesisHashHex = "0xb963e8e5a0b51ea6117906fb2a20a643256920b1d6b4be8200469c7f3a6a46b9"

var ExpectedGenesisHash = common.HexToHash(ExpectedGenesisHashHex)

//go:embed genesis-121526-devnet.json
var genesisJSON []byte

// EmbeddedGenesisJSON returns the embedded Ethernova devnet genesis JSON.
func EmbeddedGenesisJSON() []byte {
	return genesisJSON
}

// EmbeddedGenesisSHA256Hex returns the SHA256 hash of the embedded genesis JSON.
func EmbeddedGenesisSHA256Hex() string {
	sum := sha256.Sum256(genesisJSON)
	return hex.EncodeToString(sum[:])
}

// MustGenesis returns the embedded Ethernova devnet genesis or panics if invalid.
func MustGenesis() *genesisT.Genesis {
	genesis := new(genesisT.Genesis)
	if err := genesis.UnmarshalJSON(genesisJSON); err != nil {
		panic(fmt.Errorf("invalid embedded ethernova genesis: %w", err))
	}
	return genesis
}
