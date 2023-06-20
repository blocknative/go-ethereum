package finch

import (
	"github.com/lthibault/log"
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
)

var (
	getReservesCallData = []byte{0x09, 0x02, 0xf1, 0xac, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	lower112BitsMask    = new(big.Int).SetBytes([]byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255})
)

func (b *Bot) loadReservesFromChain() error {
	statedb, err := b.blockChain.State()
	if err != nil {
		return err
	}

	var (
		from         = vm.AccountRef(common.Address{})
		header       = b.blockChain.CurrentHeader()
		blockContext = core.NewEVMBlockContext(header, b.blockChain, nil)
		vmenv        = vm.NewEVM(blockContext, vm.TxContext{}, statedb, b.chainCfg, b.vmCfg)
		gas          = uint64(math.MaxUint64)
	)

	for poolAddr, pool := range b.ammGraph.nodes {
		ret, _, err := vmenv.Call(from, poolAddr, getReservesCallData, gas, common.Big0)
		if err != nil {
			return err
		}
		if len(ret) > 32 {
			return ErrInvalidGetReservesResult
		}

		pool.loaded = true
		pool.reserves0, pool.reserves1 = parseReservesFromStorageValue(ret)
		log.Debug("finch: loaded reserves", "pool", poolAddr, "reserves0", pool.reserves0, "reserves1", pool.reserves1)
	}

	return nil
}

func parseReservesFromStorageValue(ret []byte) (*big.Int, *big.Int) {
	if len(ret) <= 4 {
		return common.Big0, common.Big0
	}

	value := new(big.Int).SetBytes(ret[:len(ret)-4])
	r1 := new(big.Int).And(value, lower112BitsMask)
	value.Rsh(value, 112)
	r0 := new(big.Int).And(value, lower112BitsMask)
	return r0, r1
}
