package finch

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/misc"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/lthibault/log"
)

func parseUniswapV2SyncEvent(log *types.Log) (uniswapV2PoolSyncEvent, error) {
	if len(log.Topics) != 1 {
		return uniswapV2PoolSyncEvent{}, ErrNotAUniswapV2PoolSync
	}

	if log.Topics[0] != common.HexToHash(UniswapV2LogTopicSync) {
		return uniswapV2PoolSyncEvent{}, ErrNotAUniswapV2PoolSync
	}

	if len(log.Data) != 64 {
		return uniswapV2PoolSyncEvent{}, ErrInvalidUniswapV2PoolSync
	}

	pr := uniswapV2PoolSyncEvent{
		address:   log.Address,
		reserves0: new(big.Int),
		reserves1: new(big.Int),
	}
	pr.reserves0.SetBytes(log.Data[:32])
	pr.reserves1.SetBytes(log.Data[32:])
	return pr, nil
}

func prepareHeader(chain *core.BlockChain, chainConfig *params.ChainConfig, engine consensus.Engine) (*types.Header, error) {
	parent := chain.CurrentBlock()
	header := &types.Header{
		ParentHash: parent.Hash(),
		Time:       parent.Time + 1,
		GasLimit:   parent.GasLimit,
		Coinbase:   parent.Coinbase,
		MixDigest:  parent.MixDigest,
		Number:     new(big.Int).Add(parent.Number, common.Big1),
		BaseFee:    misc.CalcBaseFee(chainConfig, parent),
	}

	// Run the consensus preparation with the default or customized consensus engine.
	if err := engine.Prepare(chain, header); err != nil {
		log.Error("Failed to prepare header for sealing", "err", err)
		return nil, err
	}

	return header, nil
}
