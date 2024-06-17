package filters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/consensus/misc/eip1559"
	"github.com/ethereum/go-ethereum/consensus/misc/eip4844"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/eth/tracers"
	"github.com/ethereum/go-ethereum/eth/tracers/blocknative"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
)

var defaultTxTraceOpts = blocknative.TracerOpts{
	Logs: true,
}

var defaultBlockTraceOpts = blocknative.TracerOpts{
	DisableBlockContext: true,
	Logs:                true,
}

// TraceNewPendingTransactions creates a subscription that is triggered each time a
// transaction enters the transaction pool. The tx is traced and sent to the client.
func (api *FilterAPI) NewPendingTransactionsWithTrace(ctx context.Context, tracerOptsJSON *[]byte) (*rpc.Subscription, error) {
	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
	}

	rpcSub := notifier.CreateSubscription()

	go func() {
		chainConfig := api.sys.backend.ChainConfig()
		txs := make(chan []*types.Transaction, 128)
		pendingTxSub := api.events.SubscribePendingTxs(txs)
		defer pendingTxSub.Unsubscribe()

		tracerOpts, err := getTracerOpts(tracerOptsJSON, defaultTxTraceOpts)
		if err != nil {
			log.Error("failed to parse tracer options", "err", err)
			return
		}

		var (
			txIndex = hexutil.Uint64(0)
			msg     *core.Message
		)

		metricsPendingTxsNew.Inc(1)
		defer metricsPendingTxsEnd.Inc(1)

		// Recover from any panics. Should be the last deferred call so it runs
		// first.
		defer func() {
			if r := recover(); r != nil {
				log.Error("pending_txs_stream panic:", "err", r)
			}
		}()

		for {
			select {
			case txs := <-txs:
				var (
					currentHeader = api.sys.backend.CurrentHeader()
					header        = &types.Header{
						ParentHash: currentHeader.Hash(),
						Coinbase:   currentHeader.Coinbase,
						Difficulty: currentHeader.Difficulty,
						GasLimit:   currentHeader.GasLimit,
						Time:       currentHeader.Time + 12,
						BaseFee:    eip1559.CalcBaseFee(chainConfig, currentHeader),
						Number:     new(big.Int).Add(currentHeader.Number, common.Big1),
					}
					signer   = types.MakeSigner(chainConfig, header.Number, header.Time)
					traceCtx = &tracers.Context{
						BlockHash:   header.Hash(),
						BlockNumber: header.Number,
					}
					tracedTxs = make([]*RPCTransaction, 0, len(txs))
				)

				if currentHeader.BlobGasUsed != nil && currentHeader.ExcessBlobGas != nil {
					ex := eip4844.CalcExcessBlobGas(*currentHeader.ExcessBlobGas, *currentHeader.BlobGasUsed)
					header.ExcessBlobGas = &ex
				}

				metricsPendingTxsReceived.Inc(int64(len(txs)))

				for _, tx := range txs {
					rpcTx := newRPCPendingTransaction(tx)
					if rpcTx == nil {
						tracedTxs = append(tracedTxs, nil)
						continue
					}
					blockHash := header.Hash()
					rpcTx.BlockHash = &blockHash
					blockNumber := hexutil.Big(*header.Number)
					rpcTx.BlockNumber = &blockNumber
					rpcTx.TransactionIndex = &txIndex
					gasPrice := hexutil.Big(*tx.GasPrice())
					rpcTx.GasPrice = &gasPrice
					tracedTxs = append(tracedTxs, rpcTx)
				}

				sDB, err := api.sys.chain.State()
				if err != nil {
					log.Error("failed to get state", "err", err)
					notifier.Notify(rpcSub.ID, tracedTxs)
					return
				}
				blockCtx := core.NewEVMBlockContext(header, api.sys.chain, nil)

				for i, tx := range tracedTxs {
					if tx == nil {
						continue
					}

					msg, err = core.TransactionToMessage(txs[i], signer, header.BaseFee)
					if err != nil {
						log.Error("failed to create tx message", "err", err, "tx", tx.Hash)
						continue
					}
					msg.SkipAccountChecks = true
					msg.BlobGasFeeCap = common.Big0 // skip the check of ErrBlobFeeCapTooLow
					msg.GasFeeCap = common.Big0     // skip the check of ErrFeeCapTooLow
					msg.GasTipCap = common.Big0     // skip the check of ErrFeeCapTooLow

					startTime := time.Now()

					traceCtx.TxHash = tx.Hash
					tx.Trace, err = traceTx(txs[i], msg, traceCtx, blockCtx, chainConfig, sDB.Copy(), tracerOpts)
					if err != nil {
						log.Info("failed to trace tx", "err", err, "tx", tx.Hash)
						metricsPendingTxsTraceFailed.Inc(1)
						continue
					}

					metricsTracePendingTxTimer.Update(time.Since(startTime).Milliseconds())
					metricsPendingTxsTraceSuccess.Inc(1)

				}

				notifier.Notify(rpcSub.ID, tracedTxs)
				metricsPendingTxsSent.Inc(int64(len(tracedTxs)))

			case <-rpcSub.Err():
				return
			}
		}
	}()

	return rpcSub, nil
}

// TraceNewFullBlocks creates a subscription that is triggered each time a
// block is added to the chain. The block is traced and sent to the client.
func (api *FilterAPI) NewFullBlocksWithTrace(ctx context.Context, tracerOptsJSON *[]byte) (*rpc.Subscription, error) {
	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
	}

	rpcSub := notifier.CreateSubscription()

	go func() {
		headers := make(chan *types.Header, 1024)
		headersSub := api.events.SubscribeNewHeads(headers)
		reorgs := make(chan *core.Reorg, 1024)
		reorgSub := core.SubscribeReorgs(reorgs)
		defer headersSub.Unsubscribe()
		defer reorgSub.Unsubscribe()
		chainConfig := api.sys.backend.ChainConfig()

		tracerOpts, err := getTracerOpts(tracerOptsJSON, defaultBlockTraceOpts)
		if err != nil {
			log.Error("failed to parse tracer options", "err", err)
			return
		}

		metricsBlocksNew.Inc(1)
		defer metricsBlocksEnd.Inc(1)

		// Recover from any panics. Should be the last deferred call so it runs
		// first.
		defer func() {
			if r := recover(); r != nil {
				log.Error("block_stream panic:", "err", r)
			}
		}()

		var hashes []common.Hash
		for {
			select {
			case r := <-reorgs:
				// Reverse the added blocks in the reorgs, excluding the latest block
				// as it will be emitted on the newHeads channels.
				hashes = make([]common.Hash, 0, len(r.Added)-1)
				for i := len(r.Added) - 1; i > 0; i-- {
					hashes = append(hashes, r.Added[i])
				}
			case h := <-headers:
				hashes = []common.Hash{h.Hash()}
			case err := <-headersSub.Err():
				log.Error("HeaderSub error", "error", err)
				return
			case err := <-reorgSub.Err():
				log.Error("ReorgSub error", "error", err)
				return
			}

			metricsBlocksReceived.Inc(int64(len(hashes)))
			for _, hash := range hashes {
				block, err := api.sys.backend.BlockByHash(ctx, hash)
				if err != nil {
					log.Error("failed to get block", "err", err, "hash", hash)
					continue
				}

				marshalBlock, err := RPCMarshalBlock(block, true, true, api.sys.backend.ChainConfig())
				if err != nil {
					log.Error("failed to marshal block", "err", err, "block", block.Number())
					continue
				}

				trace, err := traceBlock(block, chainConfig, api.sys.chain, tracerOpts)
				if err != nil {
					log.Info("failure in block trace", "err", err, "hash", hash, "block", block.Number())
					metricsBlocksTraceFailed.Inc(1)
				}
				metricsBlocksTraceSuccess.Inc(1)

				marshalBlock["trace"] = trace
				marshalReceipts := make(map[common.Hash]map[string]interface{})
				receipts, err := api.sys.backend.GetReceipts(ctx, hash)
				if err != nil {
					log.Error("failed to get receipts for block", "err", err, "hash ", hash, "block", block.Number())
					continue
				}
				for index, receipt := range receipts {
					fields := map[string]interface{}{
						"transactionIndex":  hexutil.Uint64(index),
						"gasUsed":           hexutil.Uint64(receipt.GasUsed),
						"cumulativeGasUsed": hexutil.Uint64(receipt.CumulativeGasUsed),
						"contractAddress":   nil,
						"logs":              receipt.Logs,
						"logsBloom":         receipt.Bloom,
						"status":            hexutil.Uint64(receipt.Status),
					}
					if receipt.Logs == nil {
						fields["logs"] = [][]*types.Log{}
					}
					// If the ContractAddress is 20 0x0 bytes, assume it is not a contract creation
					if receipt.ContractAddress != (common.Address{}) {
						fields["contractAddress"] = receipt.ContractAddress
					}
					if reason, ok := core.GetRevertReason(receipt.TxHash, hash); ok {
						fields["revertReason"] = reason
					}
					marshalReceipts[receipt.TxHash] = fields
				}
				marshalBlock["receipts"] = marshalReceipts

				notifier.Notify(rpcSub.ID, marshalBlock)
				metricsBlocksSent.Inc(1)
			}
		}
	}()

	return rpcSub, nil
}

// traceTx traces a transaction with the given contexts.
func traceTx(tx *types.Transaction, message *core.Message, txCtx *tracers.Context, vmctx vm.BlockContext, chainConfig *params.ChainConfig, statedb *state.StateDB, tracerOpts blocknative.TracerOpts) (*blocknative.Trace, error) {
	tracer, err := blocknative.NewTracer(tracerOpts)
	if err != nil {
		return nil, err
	}

	hooks := tracer.Hooks()

	vmenv := vm.NewEVM(vmctx, core.NewEVMTxContext(message), statedb, chainConfig, vm.Config{Tracer: hooks, NoBaseFee: true})
	statedb.SetTxContext(txCtx.TxHash, txCtx.TxIndex)

	hooks.BlockNativeInitHook(vmenv)
	hooks.OnTxStart(vmenv.GetVMContext(), tx, message.From)
	result, err := core.ApplyMessage(vmenv, message, new(core.GasPool).AddGas(message.GasLimit))
	if err != nil {
		return nil, fmt.Errorf("tracing failed: %w", err)
	}
	hooks.OnTxEnd(&types.Receipt{GasUsed: result.UsedGas}, err)

	trace, err := tracer.GetTrace()
	if err != nil {
		return nil, err
	}

	return trace, err
}

// traceBlockTx traces a transaction with the given contexts.
func traceBlockTx(tx *types.Transaction, message *core.Message, txCtx *tracers.Context, vmctx vm.BlockContext, chainConfig *params.ChainConfig, statedb *state.StateDB, tracerOpts blocknative.TracerOpts) (*core.ExecutionResult, *blocknative.Trace, error) {
	tracerOpts.DisableBlockContext = false
	tracerOpts.PerHashLogs = true
	tracer, err := blocknative.NewTracer(tracerOpts)
	if err != nil {
		return nil, nil, err
	}

	hooks := tracer.Hooks()

	vmenv := vm.NewEVM(vmctx, core.NewEVMTxContext(message), statedb, chainConfig, vm.Config{Tracer: hooks, NoBaseFee: false})
	statedb.SetTxContext(txCtx.TxHash, txCtx.TxIndex)

	hooks.BlockNativeInitHook(vmenv)
	hooks.OnTxStart(vmenv.GetVMContext(), tx, message.From)
	result, err := core.ApplyMessage(vmenv, message, new(core.GasPool).AddGas(message.GasLimit))
	if err != nil {
		return result, nil, fmt.Errorf("tracing failed: %w", err)
	}
	hooks.OnTxEnd(&types.Receipt{GasUsed: result.UsedGas}, err)

	trace, err := tracer.GetTrace()
	if err != nil {
		return nil, nil, err
	}

	return result, trace, err
}

// traceBlock traces all transactions in a block.
func traceBlock(block *types.Block, chainConfig *params.ChainConfig, chain *core.BlockChain, tracerOpts blocknative.TracerOpts) ([]*blocknative.Trace, error) {
	parent := chain.GetBlockByHash(block.ParentHash())
	if parent == nil {
		return nil, errors.New("parent block not found")
	}

	statedb, err := chain.StateAt(parent.Root())
	if err != nil {
		return nil, err
	}

	var (
		txs       = block.Transactions()
		blockHash = block.Hash()
		is158     = chainConfig.IsEIP158(block.Number())
		blockCtx  = core.NewEVMBlockContext(block.Header(), chain, nil)
		signer    = types.MakeSigner(chainConfig, block.Number(), block.Time())
		results   = make([]*blocknative.Trace, len(txs))
		results2  = make([]*core.ExecutionResult, len(txs))
	)

	startTime := time.Now()
	for i, tx := range txs {
		msg, err := core.TransactionToMessage(tx, signer, blockCtx.BaseFee)
		if err != nil {
			log.Error("failed to trace block in transaction to message", "err", err, "tx", tx.Hash())
			return nil, err
		}
		txCtx := &tracers.Context{
			BlockHash:   blockHash,
			BlockNumber: block.Number(),
			TxIndex:     i,
			TxHash:      tx.Hash(),
		}
		results2[i], results[i], err = traceBlockTx(tx, msg, txCtx, blockCtx, chainConfig, statedb, tracerOpts)
		if results2[i] != nil {
			results2[i].Hash = tx.Hash()
		}
		if err != nil {
			cconf, _ := json.Marshal(chainConfig)
			topts, _ := json.Marshal(tracerOpts)
			exec, _ := json.Marshal(results2)
			log.Error("failed to trace block in tx config",
				"err", err,
				"blockHash", block.Hash(),
				"tx", tx.Hash(),
				"conf", string(cconf),
				"tracerOpts", string(topts))
			log.Error("failed to trace block in transaction 1a", "err", err, "blockHash", block.Hash(), "tx", tx.Hash(), "exec", string(exec))
			return results, err
		}
		statedb.Finalise(is158)
	}
	metricsTraceBlockTimer.Update(time.Since(startTime).Milliseconds())

	return results, nil
}

// getTracerOpts parses the tracer options from the given JSON and applies them
// on top of the default options.
func getTracerOpts(optsJSON *[]byte, defaults blocknative.TracerOpts) (blocknative.TracerOpts, error) {
	opts := defaults
	if optsJSON != nil {
		if err := json.Unmarshal(*optsJSON, &opts); err != nil {
			return blocknative.TracerOpts{}, err
		}
	}
	return opts, nil
}
