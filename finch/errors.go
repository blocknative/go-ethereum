package finch

import "errors"

var (
	ErrNotAUniswapV2PoolSync       = errors.New("Not a uniswap v2 pool sync")
	ErrInvalidUniswapV2PoolSync    = errors.New("Invalid uniswap v2 pool sync")
	ErrNoUniswapV2PoolSyncsInTrade = errors.New("No uniswap v2 pool syncs in trade")
	ErrPairNotFound                = errors.New("pair not found")

	ErrInvalidReservesValue = errors.New("invalid reserves value")
	ErrInvalidTxBuilder     = errors.New("invalid tx builder")
)
