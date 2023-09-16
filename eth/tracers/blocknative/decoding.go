package blocknative

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"math"
)

type decoderEVM struct {
	*vm.EVM
}

func (d decoderEVM) GetCode(addr common.Address) []byte {
	return d.StateDB.GetCode(addr)
}

func (d decoderEVM) CallCode(addr common.Address, method []byte) ([]byte, error) {
	code := d.StateDB.GetCode(addr)
	contract := vm.NewContract(vm.AccountRef(common.Address{}), vm.AccountRef(addr), common.Big0, math.MaxUint64)
	contract.SetCallCode(&addr, d.StateDB.GetCodeHash(addr), code)
	ret, err := d.Interpreter().Run(contract, method, false)
	if err != nil {
		return nil, err
	}
	return ret, nil
}
