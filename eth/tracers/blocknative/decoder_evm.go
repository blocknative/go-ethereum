package blocknative

import (
	"math"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
)

// decoderEVM contains the functionality required by the decoder from the EVM.
type decoderEVM struct {
	*vm.EVM
}

// GetCode returns the bytecode of the account with the given address, or nil
// if the account is not a contract.
func (d decoderEVM) GetCode(addr common.Address) []byte {
	return d.StateDB.GetCode(addr)
}

// CallCode executes the given method on the code at the given address.
var callCodeCallerAddr = vm.AccountRef(common.HexToAddress("0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF"))

func (d decoderEVM) CallCode(addr common.Address, method []byte) ([]byte, error) {
	code := d.StateDB.GetCode(addr)
	contract := vm.NewContract(callCodeCallerAddr, vm.AccountRef(addr), common.Big0, math.MaxUint64)
	contract.SetCallCode(&addr, d.StateDB.GetCodeHash(addr), code)

	// Stash the tracer and disable tracing for the call, then replace it.
	// Otherwise the call will be traced as part of the current trace.
	t := d.EVM.Config.Tracer
	d.EVM.Config.Tracer = nil
	ret, err := d.Interpreter().Run(contract, method, false)
	d.EVM.Config.Tracer = t

	return ret, err
}
