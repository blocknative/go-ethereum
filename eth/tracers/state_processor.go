package tracers

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/eth/tracers/blocknative"
	"github.com/ethereum/go-ethereum/params"
)

func applyTransactionWithResult(msg *core.Message, config *params.ChainConfig, bc core.ChainContext, author *common.Address, gp *core.GasPool, statedb *state.StateDB, header *types.Header, msgTx *core.Message, usedGas *uint64, evm *vm.EVM, bnTracer *blocknative.Tracer) (*types.Receipt, *core.ExecutionResult, interface{}, error) {
	// Create a new context to be used in the EVM environment.
	txContext := core.NewEVMTxContext(msg)
	evm.Reset(txContext, statedb)

	// Apply the transaction to the current state (included in the env).
	result, err := core.ApplyMessage(evm, msg, gp)
	if err != nil {
		return nil, nil, nil, err
	}

	traceResult, err := bnTracer.GetResult()
	// Update the state with pending changes.
	var root []byte
	if config.IsByzantium(header.Number) {
		// statedb.GetRefund()

	} else {
		root = statedb.IntermediateRoot(config.IsEIP158(header.Number)).Bytes()
	}
	*usedGas += result.UsedGas

	// Create a new receipt for the transaction, storing the intermediate root and gas used
	// by the tx.
	receipt := &types.Receipt{Type: 0, PostState: root, CumulativeGasUsed: *usedGas}
	if result.Failed() {
		receipt.Status = types.ReceiptStatusFailed
	} else {
		receipt.Status = types.ReceiptStatusSuccessful
	}
	// receipt.TxHash = tx.Hash()
	receipt.GasUsed = result.UsedGas

	// Set the receipt logs and create the bloom filter.
	receipt.BlockHash = header.Hash()
	receipt.BlockNumber = header.Number
	receipt.TransactionIndex = uint(statedb.TxIndex())
	return receipt, result, traceResult, err
}

func ApplyUnsignedTransactionWithResult(config *params.ChainConfig, bc core.ChainContext, author *common.Address, gp *core.GasPool, statedb *state.StateDB, header *types.Header, msg *core.Message, usedGas *uint64, cfg vm.Config) (*types.Receipt, *core.ExecutionResult, interface{}, error) {
	// Create a blocknative tracer to get execution traces.
	bnTracer, err := blocknative.NewBlocknativeTracerWithOpts(blocknative.TracerOpts{
		Decode: true,
	})
	if err != nil {
		return nil, nil, nil, err
	}

	// Create a new context to be used in the EVM environment
	blockContext := core.NewEVMBlockContext(header, bc, author)
	txContext := vm.TxContext{
		Origin:     msg.From,
		GasPrice:   msg.GasPrice,
		BlobHashes: msg.BlobHashes,
		BlobFeeCap: msg.BlobGasFeeCap,
	}
	if txContext.GasPrice == nil {
		txContext.GasPrice = common.Big0
	}
	vmenv := vm.NewEVM(blockContext, txContext, statedb, config, vm.Config{Tracer: bnTracer.Hooks(), NoBaseFee: true})
	return applyTransactionWithResult(msg, config, bc, author, gp, statedb, header, msg, usedGas, vmenv, bnTracer)
}
