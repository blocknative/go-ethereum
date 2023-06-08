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
	DefaultProfitTarget = big.NewInt(2000000000000000)
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
)

var AMMRouters = []string{
	UniswapUniversalRouter,
	UniswapAutoRouter,
	UniswapV2Router,
}
