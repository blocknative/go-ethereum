package filters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sync"
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
	"github.com/ethereum/go-ethereum/eth/tracers/blocknative/cache"
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
				)

				statedb, err := api.sys.chain.State()
				if err != nil {
					log.Error("failed to get state", "err", err)
					return
				}

				for _, tx := range txs {
					msg, _ = core.TransactionToMessage(tx, signer, header.BaseFee)
					if err != nil {
						log.Error("failed to create tx message", "err", err, "tx", tx.Hash())
						continue
					}

					traceCtx.TxHash = tx.Hash()
					trace, err := traceTx(msg, traceCtx, blockCtx, chainConfig, statedb, tracerOpts)
					if err != nil {
						log.Error("failed to trace tx", "err", err, "tx", tx.Hash())
						continue
					}

					gasPrice := hexutil.Big(*tx.GasPrice())
					rpcTx := newRPCPendingTransaction(tx)
					rpcTx.BlockHash = &blockHash
					rpcTx.BlockNumber = &blockNumber
					rpcTx.TransactionIndex = &txIndex
					rpcTx.Trace = trace
					rpcTx.GasPrice = &gasPrice
					tracedTxs = append(tracedTxs, rpcTx)
				}

				if len(tracedTxs) == 0 {
					continue
				}

				notifier.Notify(rpcSub.ID, tracedTxs)
			case <-rpcSub.Err():
				return
			case <-notifier.Closed():
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

			for _, hash := range hashes {
				block, err := api.sys.backend.BlockByHash(ctx, hash)
				if err != nil {
					log.Error("failed to get block", "err", err, "hash", hash)
					continue
				}

				marshalBlock, err := RPCMarshalBlock(block, true, true, api.sys.backend.ChainConfig())
				if err != nil {
					continue
				}

				trace, err := traceBlock(block, chainConfig, api.sys.chain, tracerOpts)
				if err != nil {
					log.Error("failed to trace block", "err", err, "block", block.Number())
					continue
				}
				marshalBlock["trace"] = trace

				marshalReceipts := make(map[common.Hash]map[string]interface{})
				receipts, err := api.sys.backend.GetReceipts(ctx, hash)
				if err != nil {
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
			}
		}
	}()

	return rpcSub, nil
}

var (
	txTraceLocksMu      sync.RWMutex
	txTraceLocks        = make(map[common.Hash]chan struct{})
	txTraceLocksTimeout = 1 * time.Second
)

// traceTx traces a transaction with the given contexts.
func traceTx(message *core.Message, txCtx *tracers.Context, vmctx vm.BlockContext, chainConfig *params.ChainConfig, statedb *state.StateDB, tracerOpts blocknative.TracerOpts) (*blocknative.Trace, error) {
	// Check cached trace or an in-progress trace before executing.
	trace, ok := cache.GetTrace(txCtx.TxHash, txCtx.BlockHash)
	if ok {
		return trace, nil
	}

	// Check if there is an in-progress trace for this transaction.
	// If there is then wait for it to finish and return it.
	// If there is not then lock the trace and create a new one.
	// Don't wait longer than a second for the cache to finish.
	txTraceLocksMu.Lock()
	if unlockCh, ok := txTraceLocks[txCtx.TxHash]; ok {
		timeOut := time.NewTimer(txTraceLocksTimeout)
		select {
		case <-unlockCh:
			if trace, ok := cache.GetTrace(txCtx.TxHash, txCtx.BlockHash); ok {
				txTraceLocksMu.Unlock()
				return trace, nil
			}
		case <-timeOut.C:
		}
	}
	unlockCh := make(chan struct{})
	txTraceLocks[txCtx.TxHash] = unlockCh
	txTraceLocksMu.Unlock()

	// No trace in cache or in-progress so create a new one.
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
	trace, err = tracer.GetTrace()
	if err != nil {
		return nil, err
	}

	// Cache the result and unlock/delete the trace lock.
	cache.PutTrace(txCtx.TxHash, txCtx.BlockHash, trace)
	close(unlockCh)
	txTraceLocksMu.Lock()
	delete(txTraceLocks, txCtx.TxHash)
	txTraceLocksMu.Unlock()

	return trace, err
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
