package finch

import (
	"bytes"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
)

const (
	contractUniswapV2PayloadBytesLen = 138
	contractAmountPayloadBytesLen    = 32
	contractAddressPayloadBytesLen   = 20

	contractABIJSON = `[{"inputs":[{"internalType":"bytes[]","name":"data","type":"bytes[]"}],"name":"arbitrage","outputs":[],"stateMutability":"nonpayable","type":"function"}]`
)

var (
	isFirstToBytes = map[bool]byte{true: 0, false: 1}

	uniswapV2FeeNumerator   = big.NewInt(997)
	uniswapV2FeeDenominator = big.NewInt(1000)

	contractABI, _    = abi.JSON(bytes.NewReader([]byte(contractABIJSON)))
	contractArbMethod = "arbitrage"
)

func getPayload(amountIn *big.Int, sequence []ammTradeStep) [][]byte {
	var result [][]byte
	var swap []byte
	//var v2Data string
	//var amountToPay []*big.Int
	//var path []common.Address
	//var tokenPath []common.Address
	//var count int

	for i := 0; i < len(sequence)-1; i++ {
		//pairAddress := sequence[i].pair
		//tokenIn := sequence[i].tokenIn
		//tokenOut := sequence[i].tokenOut
		dex := sequence[i].dex
		nextAddress := sequence[i+1].pair
		nextDex := sequence[i+1].dex

		switch dex {
		case dexUniswapV2:
			amountOut, v2Data := getUniswapV2Payload(amountIn, sequence[i], nextAddress, len(result) == 0)
			amountIn = amountOut
			swap = v2Data
		case dexUniswapV3:
			//amountToPay = append([]*big.Int{amountIn}, amountToPay...)
			//
			//path = append([]common.Address{nextAddress, pairAddress}, path...)
			//tokenPath = append([]common.Address{tokenOut, tokenIn}, tokenPath...)
			//
			//fee := sequence[i].fee
			//amountOut := getAmountOutUniswapV3(amountIn, fee, tokenIn, tokenOut)
			//amountIn = amountOut
			//countStr := ""
			//if v2Data != "" {
			//	countStr = fmt.Sprintf("%02d", count)
			//}
			//swap = getUniswapV3Payload(amountToPay, path, tokenPath, countStr+v2Data[2:])
			//count++
		}

		if nextDex == dexUniswapV2 {
			result = append(result, swap)
			swap = nil
			//amountToPay = []*big.Int{}
			//path = []common.Address{}
			//tokenPath = []common.Address{}
			//v2Data = ""
			//count = 0
		}
	}

	return result
}

func getAmountOutUniswapV2(amountIn *big.Int, step ammTradeStep) *big.Int {
	reservesIn, reservesOut := step.reserves0, step.reserves1
	if step.tokenIn.String() >= step.tokenOut.String() {
		reservesIn, reservesOut = reservesOut, reservesIn
	}

	amountInWithFee := new(big.Int).Mul(amountIn, uniswapV2FeeNumerator)
	numerator := new(big.Int).Mul(amountInWithFee, reservesOut)
	denominator := new(big.Int).Add(new(big.Int).Mul(reservesIn, uniswapV2FeeDenominator), amountInWithFee)
	return new(big.Int).Div(numerator, denominator)
}

func getUniswapV2Payload(amountIn *big.Int, step ammTradeStep, nextAddress common.Address, isFirst bool) (*big.Int, []byte) {
	amountOut := getAmountOutUniswapV2(amountIn, step)
	amountOut0, amountOut1 := amountOut, big.NewInt(0)
	if step.tokenIn.String() >= step.tokenOut.String() {
		amountOut0, amountOut1 = amountOut1, amountOut
	}

	res := bytes.NewBuffer(make([]byte, 0, contractUniswapV2PayloadBytesLen))
	res.WriteByte(step.dex.ID())
	res.WriteByte(isFirstToBytes[isFirst])
	res.Write(amountToPayloadBytes(amountIn))
	res.Write(addressToPayloadBytes(step.pair))
	res.Write(amountToPayloadBytes(amountOut0))
	res.Write(amountToPayloadBytes(amountOut1))
	res.Write(addressToPayloadBytes(nextAddress))
	return amountOut, res.Bytes()
}

func getAmountOutUniswapV3(amountIn *big.Int, fee *big.Int, tokenIn common.Address, tokenOut common.Address) *big.Int {
	return big.NewInt(0)
}

func getUniswapV3Payload(amountToPay []*big.Int, path []common.Address, tokenPath []common.Address, data string) string {
	return ""
}

func amountToPayloadBytes(amount *big.Int) []byte {
	// Create a 32 byte payload
	var payloadBytes [contractAmountPayloadBytesLen]byte
	ammountBytes := amount.Bytes()

	// Copy the bytes into the payload right-aligned
	offset := contractAmountPayloadBytesLen - len(ammountBytes)
	if offset < 0 {
		offset = 0
	}
	copy(payloadBytes[offset:], ammountBytes)

	return payloadBytes[:]
}

func addressToPayloadBytes(addr common.Address) []byte {
	var payloadBytes [contractAddressPayloadBytesLen]byte
	addrBytes := addr.Bytes()

	offset := contractAddressPayloadBytesLen - len(addrBytes)
	if offset < 0 {
		offset = 0
	}
	copy(payloadBytes[offset:], addrBytes)
	return payloadBytes[:]
}
