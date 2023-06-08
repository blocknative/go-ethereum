package finch

import (
	"encoding/json"
	"io"
	"math"
	"math/big"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/lthibault/log"
)

const (
	dexUniswapV2 dex = 2 + iota
	dexUniswapV3
)

var (
	bigFloatZero = big.NewFloat(0)

	initialTokenIn = common.HexToAddress("C02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2")
)

type dex uint8

func (d dex) ID() byte {
	return byte(d)
}

type ammGraph struct {
	nodes map[common.Address]*ammGraphNode
}

type ammGraphNode struct {
	reserves0 *big.Int
	reserves1 *big.Int
	cycles    [][]common.Address
	token0    common.Address
	token1    common.Address
	dex       dex
}

type ammTradeResult struct {
	amountIn    *big.Int
	profit      *big.Int
	data        string
	pairAddress common.Address
	cycle       []common.Address
	sequence    []ammTradeStep
}

type ammTradeStep struct {
	fee       *big.Int
	reserves0 *big.Int
	reserves1 *big.Int

	pair     common.Address
	tokenIn  common.Address
	tokenOut common.Address

	dex dex
	//dir bool
}

type jsonAmmGraph map[string]struct {
	PairInfo *struct {
		Token0    string `json:"token0"`
		Token1    string `json:"token1"`
		Reserves0 string `json:"reserves0"`
		Reserves1 string `json:"reserves1"`
		Dex       uint8  `json:"dex"`
	} `json:"pairInfo"`
	Cycles [][]string `json:"cycles"`
}

type reserveUpdates map[string]*big.Int

type ammReserveUpdates map[common.Address]uniswapV2PoolSyncEvent

type uniswapV2PoolSyncEvent struct {
	address   common.Address
	reserves0 *big.Int
	reserves1 *big.Int
}

func loadAMMGraph(source io.Reader) (ammGraph, error) {
	var ptt jsonAmmGraph
	if err := json.NewDecoder(source).Decode(&ptt); err != nil {
		return ammGraph{}, err
	}

	g := ammGraph{nodes: make(map[common.Address]*ammGraphNode, len(ptt))}
	for addrStr, pair := range ptt {
		addr := common.HexToAddress(addrStr)

		r0, err := parseStringReserves(pair.PairInfo.Reserves0)
		if err != nil {
			return ammGraph{}, err
		}
		r1, err := parseStringReserves(pair.PairInfo.Reserves1)
		if err != nil {
			return ammGraph{}, err
		}

		cycles := make([][]common.Address, 0, len(pair.Cycles))
		for _, cycle := range pair.Cycles {
			newCycle := make([]common.Address, 0, len(cycle))
			for _, addr := range cycle {
				newCycle = append(newCycle, common.HexToAddress(addr))
			}
			cycles = append(cycles, newCycle)
		}

		g.nodes[addr] = &ammGraphNode{
			cycles:    cycles,
			token0:    common.HexToAddress(pair.PairInfo.Token0),
			token1:    common.HexToAddress(pair.PairInfo.Token1),
			reserves0: r0,
			reserves1: r1,
			dex:       dex(pair.PairInfo.Dex),
		}
	}
	return g, nil
}

func parseStringReserves(s string) (*big.Int, error) {
	if s == "" {
		return big.NewInt(0), nil
	} else {
		r, ok := new(big.Int).SetString(s[2:], 16)
		if ok {
			return r, nil
		}
		return nil, ErrInvalidReservesValue

	}

}

func (g ammGraph) getCycleReserves(poolCycle []common.Address, updates ammReserveUpdates) (reserveUpdates, []ammTradeStep, error) {
	var (
		tokenIn  = initialTokenIn
		tokenOut = initialTokenIn
		rs       = make(reserveUpdates)
		seqArray = make([]ammTradeStep, 0, len(poolCycle))
	)

	i := 1
	for _, address := range poolCycle {
		pair, ok := g.nodes[address]
		if !ok {
			return nil, nil, ErrPairNotFound
		}

		// Decide which token is in and which is out.
		tokenIn = pair.token0
		tokenOut = pair.token1
		reservesIn := pair.reserves0
		reservesOut := pair.reserves1
		if pair.token0 != tokenOut {
			tokenIn, tokenOut = tokenOut, tokenIn
			reservesIn, reservesOut = reservesOut, reservesIn
		}

		// Get reserves, using updated reserves if available.
		r0 := reservesIn
		r1 := reservesOut
		if updates, ok := updates[address]; ok {
			r0 = updates.reserves0
			r1 = updates.reserves1
		}
		// Store the reserves for this pair.
		key0 := strconv.Itoa(i) + "_" + strconv.Itoa(i+1)
		key1 := strconv.Itoa(i+1) + "_" + strconv.Itoa(i)
		rs[key0] = r0
		rs[key1] = r1

		seqArray = append(seqArray, ammTradeStep{
			pair:      address,
			reserves0: r0,
			reserves1: r1,
			tokenIn:   tokenIn,
			tokenOut:  tokenOut,
		})

		i++
	}

	return rs, seqArray, nil
}

func (g ammGraph) getOptimalCycle(cycles [][]common.Address, pairAddress common.Address, reserveUpdates ammReserveUpdates) (*ammTradeResult, error) {
	tr := &ammTradeResult{
		pairAddress: pairAddress,
		profit:      big.NewInt(0),
		amountIn:    big.NewInt(0),
	}

	// For each cycle calculate the current reserveUpdates after updates and the
	// profit we'd get by trading through it.
	for _, cycle := range cycles {
		reserves, seqArray, err := g.getCycleReserves(cycle, reserveUpdates)
		if err != nil {
			return nil, err
		}

		numPairs := len(cycle)
		amountIn := big.NewFloat(0)
		one := big.NewFloat(1)
		two := big.NewFloat(1)
		denominator := big.NewFloat(0)

		r1Num := new(big.Float)
		r2Num := new(big.Float)
		r1r2Num := new(big.Float)
		r1Den := new(big.Float)
		r2Den := new(big.Float)

		for i := 1; i < numPairs+1; i++ {
			r1Num.SetInt(reserves[strconv.Itoa(i)+"_"+strconv.Itoa(i+1)])
			r2Num.SetInt(reserves[strconv.Itoa(i+1)+"_"+strconv.Itoa(i)])

			r1r2Num.Mul(r1Num, r2Num)
			one.Mul(one, r1r2Num)
			two.Mul(two, r1Num)

			three := big.NewFloat(1)
			four := big.NewFloat(1)

			for j := 1; j < i; j++ {
				r1Den.SetInt(reserves[strconv.Itoa(j+1)+"_"+strconv.Itoa(j)])
				three.Mul(three, r1Den)
			}

			for j := i + 1; j < numPairs+1; j++ {
				r2Den.SetInt(reserves[strconv.Itoa(j)+"_"+strconv.Itoa(j+1)])
				four.Mul(four, r2Den)
			}

			insideFee := big.NewFloat(math.Pow(UniswapV2InsideFeeRate, float64(i)))
			three.Mul(three, four)
			three.Mul(three, insideFee)
			denominator.Add(denominator, three)
		}

		outsideFee := big.NewFloat(math.Pow(UniswapV2OutsideFeeRate, float64(numPairs)))
		one.Mul(one, outsideFee)
		one.Sqrt(one)
		numerator := new(big.Float)
		numerator.Sub(one, two)
		amountIn.Quo(numerator, denominator)

		// If amountIn is <= 0 we can't trade through this cycle.
		if amountIn.Cmp(bigFloatZero) != 1 {
			log.Debug("finch: cycle amountIn too small", "amount", amountIn, "cycle", cycle)
			continue
		}

		// If profit is <= profit we shouldn't trade through this cycle.
		// TODO: If profit is exactly equal to profit we should check which
		// one has the smallest cycle to minimize gas costs.
		amountInInt64, _ := amountIn.Int64()
		amountInBigInt := big.NewInt(amountInInt64)
		profit := getAMMTradeProfit(amountInBigInt, seqArray)
		if profit.Cmp(tr.profit) < 1 {
			continue
		}

		tr.profit = profit
		tr.cycle = cycle
		tr.amountIn = amountInBigInt
		tr.sequence = seqArray
	}

	return tr, nil
}

func getAMMTradeProfit(amountIn *big.Int, sequences []ammTradeStep) *big.Int {
	start := amountIn

	for _, step := range sequences {
		amountOut := getAmountOutUniswapV2(amountIn, step)
		amountIn = amountOut
	}

	return amountIn.Sub(amountIn, start)
}
