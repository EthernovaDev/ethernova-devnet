//go:build !cgo
// +build !cgo

package lyra2

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
)

var errLyra2Unavailable = errors.New("lyra2 requires cgo (rebuild with CGO_ENABLED=1)")

type Lyra2 struct{}

type Config struct{}

func New(config *Config, notify []string, noverify bool) *Lyra2 {
	log.Warn("Lyra2 disabled (built without cgo); consensus engine unavailable")
	return &Lyra2{}
}

func NewTester(notify []string, noverify bool) *Lyra2 {
	log.Warn("Lyra2 disabled (built without cgo); consensus engine unavailable")
	return &Lyra2{}
}

func (lyra2 *Lyra2) Author(header *types.Header) (common.Address, error) {
	return common.Address{}, errLyra2Unavailable
}

func (lyra2 *Lyra2) VerifyHeader(chain consensus.ChainHeaderReader, header *types.Header, seal bool) error {
	return errLyra2Unavailable
}

func (lyra2 *Lyra2) VerifyHeaders(chain consensus.ChainHeaderReader, headers []*types.Header, seals []bool) (chan<- struct{}, <-chan error) {
	quit := make(chan struct{})
	errc := make(chan error, len(headers))
	for range headers {
		errc <- errLyra2Unavailable
	}
	close(errc)
	return quit, errc
}

func (lyra2 *Lyra2) VerifyUncles(chain consensus.ChainReader, block *types.Block) error {
	return errLyra2Unavailable
}

func (lyra2 *Lyra2) Prepare(chain consensus.ChainHeaderReader, header *types.Header) error {
	return errLyra2Unavailable
}

func (lyra2 *Lyra2) Finalize(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB, txs []*types.Transaction, uncles []*types.Header, withdrawals []*types.Withdrawal) {
	panic(errLyra2Unavailable)
}

func (lyra2 *Lyra2) FinalizeAndAssemble(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB, txs []*types.Transaction, uncles []*types.Header, receipts []*types.Receipt, withdrawals []*types.Withdrawal) (*types.Block, error) {
	return nil, errLyra2Unavailable
}

func (lyra2 *Lyra2) Seal(chain consensus.ChainHeaderReader, block *types.Block, results chan<- *types.Block, stop <-chan struct{}) error {
	return errLyra2Unavailable
}

func (lyra2 *Lyra2) SealHash(header *types.Header) common.Hash {
	panic(errLyra2Unavailable)
}

func (lyra2 *Lyra2) CalcDifficulty(chain consensus.ChainHeaderReader, time uint64, parent *types.Header) *big.Int {
	panic(errLyra2Unavailable)
}

func (lyra2 *Lyra2) APIs(chain consensus.ChainHeaderReader) []rpc.API {
	return nil
}

func (lyra2 *Lyra2) Close() error {
	return nil
}

func (lyra2 *Lyra2) Hashrate() float64 {
	return 0
}
