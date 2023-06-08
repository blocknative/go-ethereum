package finch

import "math/big"

// Market Parameters
const (
	UniswapV2InsideFeeRate  = 0.997
	UniswapV2OutsideFeeRate = 0.997
)

// Default configs
const (
	DefaultBotGasPrice = 50_000_000_000
)

var (
	DefaultProfitTarget = big.NewInt(1)
)

// Log selectors
const (
	UniswapV2LogTopicSync = "0x1c411e9a96e071241c2f21f7726b17ae89e3cab4c78be50e062b03a9fffbbad1"
)

// Addresses
const (
	UniswapUniversalRouter = "0xEf1c6E67703c7BD7107eed8303Fbe6EC2554BF6B"
	UniswapAutoRouter      = "0xe592427a0aece92de3edee1f18e0157c05861564"
	UniswapV2Router        = "0x7a250d5630b4cf539739df2c5dacb4c659f2488d"

	CoinbaseWalletProxy = "0xe66B31678d6C16E9ebf358268a790B763C133750"
	UnknownRouter1      = "0x3fC91A3afd70395Cd496C647d5a6CC9D4B2b7FAD"
)

var AMMRouters = []string{
	UniswapUniversalRouter,
	UniswapAutoRouter,
	UniswapV2Router,
	CoinbaseWalletProxy,
	UnknownRouter1,
}
