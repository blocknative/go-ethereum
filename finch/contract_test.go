package finch

import (
	"encoding/hex"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
	"testing"
)

var (
	goldFinchAddress = common.HexToAddress("0x435f81d5b37c27ff3F543145F994CfFD72eE1B8B")

	weth = common.HexToAddress("0xB4FBF271143F4FBf7B91A5ded31805e42b2208d6")
	uni  = common.HexToAddress("0x1f9840a85d5aF5bf1D1762F925BDADdC4201F984")
	usdc = common.HexToAddress("0x1F2cd0D7E5a7d8fE41f886063E9F11A05dE217Fa")
	usdt = common.HexToAddress("0xC2C527C0CACF457746Bd31B2a698Fe89de2b6d49")

	wethUniUni    = common.HexToAddress("0x28cee28a7C4b4022AC92685C07d2f33Ab1A0e122")
	wethUniSushi  = common.HexToAddress("0x6D2fAf643Fe564e0204f35e38d1a1b08D9620d14")
	usdcWethUniV2 = common.HexToAddress("0x513777Ce6d97a6E9373bAE5a2b69f84Ff10B9b93")
	usdtWethUniV3 = common.HexToAddress("0x04783562Cea329a593f4c8AF534ffF025715Ad31")
	usdcUsdtUniV2 = common.HexToAddress("0x650690daAAa5dab3Fa720038C1fD51553D446e7C")
	wethUsdcSushi = common.HexToAddress("0xf510EF2320Aebc94DA59A45CC9C5fBB6006EcB27")
	usdcWethUniV3 = common.HexToAddress("0xdaBdF64236fBd8723EF03fAe8eDC347BBbe17A6c")

	wethUniUniReserves0   = big.NewInt(10000)
	wethUniUniReserves1   = big.NewInt(10)
	wethUniSushiReserves0 = big.NewInt(99999)
	wethUniSushiReserves1 = big.NewInt(8888888)
)

func TestContractCallGeneration(t *testing.T) {
	//	# #v2v2
	//	# sequence = [
	//	#     {
	//	#         "pair": wethUniUni,
	//	#         "tokenIn": weth,
	//	#         "tokenOut": uni,
	//	#         "DEX": "v2"
	//	#     },
	//	#     {
	//	#         "pair": wethUniSushi,
	//	#         "tokenIn": uni,
	//	#         "tokenOut": weth,
	//	#         "DEX": "v2"
	//	#     },
	//	#     {
	//	#         "pair": goldFinchAddress,
	//	#         "tokenIn": "",
	//	#         "tokenOut": "",
	//	#         "DEX": "v2"
	//	#     }
	//	# ]
	tests := []struct {
		seq      []ammTradeStep
		callData string
	}{
		{
			callData: "bffb36940000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000008a0200000000000000000000000000000000000000000000000000000000000098968028cee28a7c4b4022ac92685c07d2f33ab1a0e1220000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000270f6d2faf643fe564e0204f35e38d1a1b08d9620d1400000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000008a0201000000000000000000000000000000000000000000000000000000000000270f6d2faf643fe564e0204f35e38d1a1b08d9620d1400000000000000000000000000000000000000000000000000000000000c4bb20000000000000000000000000000000000000000000000000000000000000000435f81d5b37c27ff3f543145f994cffd72ee1b8b00000000000000000000000000000000000000000000",
			seq: []ammTradeStep{
				{
					reserves0: wethUniUniReserves0,
					reserves1: wethUniUniReserves1,
					pair:      wethUniUni,

					tokenIn:  weth,
					tokenOut: uni,
					dex:      dexUniswapV2,
				},
				{
					reserves0: wethUniSushiReserves0,
					reserves1: wethUniSushiReserves1,
					pair:      wethUniSushi,

					tokenIn:  uni,
					tokenOut: weth,
					dex:      dexUniswapV2,
				},
				{
					reserves0: common.Big0,
					reserves1: common.Big0,
					pair:      goldFinchAddress,

					tokenIn:  common.Address{},
					tokenOut: common.Address{},
					dex:      dexUniswapV2,
				},
			},
		},
	}

	for _, test := range tests {
		swaps := getPayload(big.NewInt(10000000), test.seq)

		iSwaps := make([]interface{}, len(swaps))
		for i, swap := range swaps {
			iSwaps[i] = swap
		}

		callValueBytes, err := contractABI.Pack("arbitrage", swaps)
		if err != nil {
			t.Fatal(err)
		}

		if test.callData != hex.EncodeToString(callValueBytes) {
			t.Fatal("call data mismatch")
		}

		fmt.Println(hex.EncodeToString(callValueBytes))
	}
}
