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
	BalanceChanges: true,
	Logs:           true,
}

var defaultBlockTraceOpts = blocknative.TracerOpts{
	BalanceChanges:      true,
	DisableBlockContext: true,
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
		pendingTxs := make(chan []*types.Transaction, 128)
		pendingTxSub := api.events.SubscribePendingTxs(pendingTxs)
		defer pendingTxSub.Unsubscribe()

		futureTxs := make(chan []*types.Transaction, 128)
		futureTxSub := api.events.SubscribeFutureTxs(futureTxs)
		defer futureTxSub.Unsubscribe()

		tracerOpts, err := getTracerOpts(tracerOptsJSON, defaultTxTraceOpts)
		if err != nil {
			log.Error("pending_txs_stream: failed to parse tracer options", "err", err)
			return
		}

		metricsPendingTxsNew.Inc(1)
		defer metricsPendingTxsEnd.Inc(1)

		// Recover from any panics. Should be the last deferred call so it runs
		// first.
		defer func() {
			if r := recover(); r != nil {
				log.Error("pending_txs_stream panic:", r)
			}
		}()

		var (
			txs          []*types.Transaction
			txsAreFuture bool
		)
		for {
			select {
			case txs = <-pendingTxs:
				txsAreFuture = false
			case txs = <-futureTxs:
				txsAreFuture = true
			case <-rpcSub.Err():
				return
			case <-notifier.Closed():
				return
			}

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
				signer = types.MakeSigner(chainConfig, header.Number, header.Time)

				blockCtx = core.NewEVMBlockContext(header, api.sys.chain, nil)
				traceCtx = &tracers.Context{
					BlockHash:   header.Hash(),
					BlockNumber: header.Number,
				}

				msg         *core.Message
				tracedTxs   = make([]*RPCTransaction, 0, len(txs))
				blockNumber = hexutil.Big(*header.Number)
				blockHash   = header.Hash()
				txIndex     = hexutil.Uint64(0)

				err     error
				statedb *state.StateDB
			)

			// If we fail to get the statedb we continue. We'll guard usage of
			// it against nils later.
			statedb, err = api.sys.chain.State()
			if err != nil {
				log.Error("pending_txs_stream: failed to get state", "err", err)
			}

			metricsPendingTxsReceived.Inc(int64(len(txs)))
			for _, tx := range txs {
				// First add the tx to the list to return
				gasPrice := hexutil.Big(*tx.GasPrice())
				rpcTx := newRPCPendingTransaction(tx)
				rpcTx.BlockHash = &blockHash
				rpcTx.BlockNumber = &blockNumber
				rpcTx.TransactionIndex = &txIndex
				rpcTx.Future = txsAreFuture
				rpcTx.GasPrice = &gasPrice
				tracedTxs = append(tracedTxs, rpcTx)

				// If we failed to get a statedb earlier then skip tracing.g
				if statedb == nil {
					continue
				}

				msg, _ = core.TransactionToMessage(tx, signer, header.BaseFee)
				if err != nil {
					log.Error("pending_txs_stream: failed to create tx message", "err", err, "tx", tx.Hash())
					continue
				}

				if msg.GasFeeCap.Cmp(header.BaseFee) < 0 {
					log.Trace("pending_txs_stream: tx gas fee too low", "tx", tx.Hash(), "gasFeeCap", msg.GasFeeCap, "baseFee", header.BaseFee)
					metricsPendingTxsGasTooLow.Inc(1)
					continue
				}

				traceCtx.TxHash = tx.Hash()
				startTime := time.Now()
				trace, err := traceTx(msg, traceCtx, blockCtx, chainConfig, statedb, tracerOpts)
				if err != nil {
					log.Error("pending_txs_stream: failed to trace tx", "err", err, "tx", tx.Hash())
					metricsPendingTxsTraceFailed.Inc(1)
					continue
				}
				metricsTracePendingTxTimer.Update(time.Since(startTime).Milliseconds())
				metricsPendingTxsTraceSuccess.Inc(1)

				// Add the trace if we were able to generate it
				tracedTxs[len(tracedTxs)-1].Trace = trace
			}

			if len(tracedTxs) == 0 {
				continue
			}

			if err := notifier.Notify(rpcSub.ID, tracedTxs); err != nil {
				log.Error("pending_txs_stream: failed to notify", "err", err)
				return
			}
			metricsPendingTxsSent.Inc(int64(len(tracedTxs)))
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
			log.Error("block_stream: failed to parse tracer options", "err", err)
			return
		}

		metricsBlocksNew.Inc(1)
		defer metricsBlocksEnd.Inc(1)

		// Recover from any panics. Should be the last deferred call so it runs
		// first.
		defer func() {
			if r := recover(); r != nil {
				log.Error("block_stream panic:", r)
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
			case <-headersSub.Err():
				return
			case <-reorgSub.Err():
				return
			case <-notifier.Closed():
				return
			}

			metricsBlocksReceived.Inc(int64(len(hashes)))
			for _, hash := range hashes {
				block, err := api.sys.backend.BlockByHash(ctx, hash)
				if err != nil {
					log.Error("block_stream: failed to get block by hash", "err", err, "hash", hash)
					continue
				}
				log.Info("block_stream: received block", "hash", hash, "number", block.Number())

				marshalBlock, err := RPCMarshalBlock(block, true, true, api.sys.backend.ChainConfig())
				if err != nil {
					log.Error("block_stream: failed to marshal block", "err", err, "block", block.Number())
					continue
				}

				trace, err := traceBlock(block, chainConfig, api.sys.chain, tracerOpts)
				if err != nil {
					metricsBlocksTraceFailed.Inc(1)
					log.Error("block_stream: failed to trace block", "err", err, "block", block.Number())
					continue
				}
				metricsBlocksTraceSuccess.Inc(1)
				marshalBlock["trace"] = trace

				marshalReceipts := make(map[common.Hash]map[string]interface{})
				receipts, err := api.sys.backend.GetReceipts(ctx, hash)
				if err != nil {
					log.Error("block_stream: failed to get receipts", "err", err, "block", block.Number())
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

				if err := notifier.Notify(rpcSub.ID, marshalBlock); err != nil {
					log.Error("block_stream: failed to notify", "err", err)
					return
				}
				log.Info("block_stream: sent block", "hash", hash, "number", block.Number(), "sub_id", rpcSub.ID)
				metricsBlocksSent.Inc(1)
			}
		}
	}()

	return rpcSub, nil
}

// traceTx traces a transaction with the given contexts.
func traceTx(message *core.Message, txCtx *tracers.Context, vmctx vm.BlockContext, chainConfig *params.ChainConfig, statedb *state.StateDB, tracerOpts blocknative.TracerOpts) (*blocknative.Trace, error) {
	tracer, err := blocknative.NewTracerWithOpts(tracerOpts)
	if err != nil {
		return nil, err
	}

	txContext := core.NewEVMTxContext(message)
	vmenv := vm.NewEVM(vmctx, txContext, statedb, chainConfig, vm.Config{Tracer: tracer})
	statedb.SetTxContext(txCtx.TxHash, txCtx.TxIndex)

	if _, err = core.ApplyMessage(vmenv, message, new(core.GasPool).AddGas(message.GasLimit)); err != nil {
		return nil, fmt.Errorf("tracing failed: %w", err)
	}

	return tracer.GetTrace()
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
	)

	startTime := time.Now()
	for i, tx := range txs {
		msg, err := core.TransactionToMessage(tx, signer, block.BaseFee())
		if err != nil {
			return nil, err
		}
		txCtx := &tracers.Context{
			BlockHash:   blockHash,
			BlockNumber: block.Number(),
			TxIndex:     i,
			TxHash:      tx.Hash(),
		}
		results[i], err = traceTx(msg, txCtx, blockCtx, chainConfig, statedb, tracerOpts)
		if err != nil {
			return nil, err
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
