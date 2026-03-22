package ethernova

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

const (
	LegacyChainID              uint64 = 77777
	LegacyForkEnforcementBlock uint64 = 138396
)

const LegacyGenesisHashHex = "0xc67bd6160c1439360ab14abf7414e8f07186f3bed095121df3f3b66fdc6c2183"

var LegacyGenesisHash = common.HexToHash(LegacyGenesisHashHex)

type EnforcementDecision struct {
	Block   uint64
	Reason  string
	Warning string
	ChainID uint64
	Genesis common.Hash
}

func ForkEnforcementDecision(chainID *big.Int, genesis common.Hash) EnforcementDecision {
	var chainIDValue uint64
	if chainID != nil {
		chainIDValue = chainID.Uint64()
	}
	decision := EnforcementDecision{
		ChainID: chainIDValue,
		Genesis: genesis,
	}

	if chainID != nil && chainID.Cmp(NewChainIDBig) == 0 {
		decision.Block = EVMCompatibilityForkBlock
		decision.Reason = "chain=121525"
		if genesis != (common.Hash{}) && genesis != ExpectedGenesisHash {
			decision.Warning = fmt.Sprintf("chainId=121525 but genesis mismatch (got %s want %s); using enforcement %d",
				genesis.Hex(), ExpectedGenesisHash.Hex(), decision.Block)
			decision.Reason = "chain=121525 (genesis mismatch)"
		}
		return decision
	}

	if chainID != nil && chainIDValue == LegacyChainID {
		decision.Block = LegacyForkEnforcementBlock
		decision.Reason = "legacy chainId=77777"
		return decision
	}

	if genesis == LegacyGenesisHash {
		decision.Block = LegacyForkEnforcementBlock
		decision.Reason = "legacy genesis"
		return decision
	}

	decision.Block = 0
	decision.Reason = "unknown chain"
	decision.Warning = "unknown chain; enforcement disabled (no legacy fallback)"
	return decision
}

func FormatBlockWithCommas(n uint64) string {
	raw := fmt.Sprintf("%d", n)
	if n < 1000 {
		return raw
	}
	var parts []string
	for len(raw) > 3 {
		parts = append([]string{raw[len(raw)-3:]}, parts...)
		raw = raw[:len(raw)-3]
	}
	if raw != "" {
		parts = append([]string{raw}, parts...)
	}
	return strings.Join(parts, ",")
}
