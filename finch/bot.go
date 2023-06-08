package finch

import (
	"bytes"
	"crypto/ecdsa"
	"errors"
	"io"
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/txpool"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"
	"github.com/lthibault/log"
)

type Config struct {
	TxBuilder      *TxBuilder
	MetricsEnabled bool
}

type TxBuilder struct {
	ContractAddr *common.Address
	GasLimit     uint64
	GasFeeCap    *big.Int
	NextNonce    uint64
	PrivateKey   *ecdsa.PrivateKey
	Signer       types.Signer
}

// isValid checks the TxBuilder to ensure all required fields are set.
func (t TxBuilder) isValid() bool {
	return t.ContractAddr != nil &&
		t.GasLimit != 0 &&
		t.GasFeeCap != nil &&
		t.PrivateKey != nil &&
		t.Signer != nil
}

type backend interface {
	BlockChain() *core.BlockChain
	TxPool() *txpool.TxPool
	ChainDb() ethdb.Database
}

type Bot struct {
	backend  backend
	ammGraph ammGraph

	// Configs
	txBuilder TxBuilder

	// Cached chain configs
	blockChain *core.BlockChain
	chainCfg   *params.ChainConfig
	vmCfg      vm.Config

	// Cached chain state
	lastBlockNumber uint64
}

func NewBot(config Config, backend backend) (*Bot, error) {
	return NewBotFromJSON(config, backend, bytes.NewReader(DefaultPairsToTokensJSON))
}

func NewBotFromJSON(config Config, backend backend, pairsToTokensJSON io.Reader) (*Bot, error) {
	b := &Bot{
		backend:    backend,
		blockChain: backend.BlockChain(),
		chainCfg:   backend.BlockChain().Config(),
		vmCfg:      *backend.BlockChain().GetVMConfig(),
	}

	if config.TxBuilder != nil {
		b.txBuilder = *config.TxBuilder
		if !b.txBuilder.isValid() {
			return nil, ErrInvalidTxBuilder
		}
	}

	// Load the amm graph.
	var err error
	b.ammGraph, err = loadAMMGraph(pairsToTokensJSON)
	if err != nil {
		return nil, err
	}

	// Start watching for new blocks and txs
	go b.subscriptionEventLoop()

	return b, nil
}

//
// Chain watching routines
//

// subscriptionEventLoop watches for new blocks and txs and dispatches them to
// the appropriate handler.
func (b *Bot) subscriptionEventLoop() {
	txEvents := make(chan core.NewTxsEvent)
	txPoolSub := b.backend.TxPool().SubscribeNewTxsEvent(txEvents)

	chainEvents := make(chan core.ChainEvent)
	chainEventSub := b.blockChain.SubscribeChainEvent(chainEvents)

	defer func() {
		txPoolSub.Unsubscribe()
		close(txEvents)

		chainEventSub.Unsubscribe()
		close(chainEvents)
	}()

	for {
		select {
		// Handle incoming events.
		case event := <-chainEvents:
			b.handleIncomingBlock(event)
		case event := <-txEvents:
			for _, tx := range event.Txs {
				to := tx.To()
				if to == nil {
					continue
				}

				// Check if this is to a watched AMM and if so handle it as a trade.
				toStr := to.String()
				for _, addr := range AMMRouters {
					if toStr != addr {
						continue
					}

					if err := b.handleIncomingTradeTx(tx); err != nil {
						log.Error("finch: error handling trade tx ", tx.Hash().String(), err)
					}
				}
			}
		}
	}
}

// handleIncomingBlock handles a new block event. It checks if the block is
// extending the chain and if so checks for any AMM reserve changes.
func (b *Bot) handleIncomingBlock(event core.ChainEvent) {
	// Ensure the new block is extending the chain
	newBlockNumber := event.Block.NumberU64()
	if newBlockNumber <= b.lastBlockNumber {
		return
	}
	b.lastBlockNumber = newBlockNumber

	// Check each log to find ones for our watched addresses and update the reserveUpdates
	reservesUpdated := 0
	reservesCount := len(b.ammGraph.nodes)
	for _, eventLog := range event.Logs {
		entry, ok := b.ammGraph.nodes[eventLog.Address]
		if !ok {
			continue
		}

		syncEvent, err := parseUniswapV2SyncEvent(eventLog)
		if err != nil {
			continue
		}

		reservesUpdated += 1
		entry.reserves0 = syncEvent.reserves0
		entry.reserves1 = syncEvent.reserves1

		if reservesUpdated >= reservesCount {
			break
		}
	}

	receivedAMMPoolUpdateCounter.Add(float64(reservesUpdated))
}

// handleIncomingTradeTx handles a newly-seen AMM router txs and checks them
// for profitable trade opportunities.
func (b *Bot) handleIncomingTradeTx(tx *types.Transaction) error {
	receivedTradeTxCounter.Inc()

	log.Info("finch: received trade tx", "tx", tx.Hash().String())

	// Execute this transaction and get the receipt.
	var (
		usedGas uint64
		gasPool = new(core.GasPool).AddGas(math.MaxUint64)
	)

	header, err := prepareHeader(b.blockChain, b.chainCfg, b.blockChain.Engine())
	if err != nil {
		return err
	}

	statedb, err := b.blockChain.State()
	if err != nil {
		return err
	}
	receipt, _, err := core.ApplyTransactionWithResult(b.chainCfg, b.blockChain, nil, gasPool, statedb, header, tx, &usedGas, b.vmCfg)

	// Ignore common errors.
	if errors.Is(err, core.ErrNonceTooHigh) || errors.Is(err, core.ErrNonceTooLow) || errors.Is(err, core.ErrTipAboveFeeCap) || errors.Is(err, core.ErrFeeCapTooLow) {
		return nil
	}

	if err != nil {
		log.Error("finch: error executing trade tx", "error", err, "hash", tx.Hash(), "parentBlock", b.blockChain.CurrentHeader().Hash())
		return err
	}

	log.Info("finch: executed trade tx", "hash", tx.Hash(), "logs", len(receipt.Logs))

	simulatedTradeTxCounter.Inc()

	// Iterate logs backwards to find the last sync event for each pool.
	reserveUpdates := make(ammReserveUpdates, len(receipt.Logs))
	for i := len(receipt.Logs) - 1; i >= 0; i-- {
		eventLog := receipt.Logs[i]

		// Only process each pool once (the last log for each pool).
		if _, ok := reserveUpdates[eventLog.Address]; ok {
			continue
		}

		// Check if this is a sync event.
		pr, err := parseUniswapV2SyncEvent(eventLog)
		if err != nil {
			continue
		}

		// Add the sync event to the map of updates
		reserveUpdates[eventLog.Address] = pr
	}
	log.Debug("finch: trade reserve updates", "updateCount", len(reserveUpdates))

	// Check if this tx is an opportunity.
	b.checkTxForOpportunity(tx, reserveUpdates)

	return nil
}

// handleIncomingBotTx handles a newly-seen bot tx and updates our local nonce.
func (b *Bot) handleIncomingBotTx(tx *types.Transaction) {
	receivedBotTxCounter.Inc()

	if tx.Nonce() < b.txBuilder.NextNonce {
		return
	}

	b.txBuilder.NextNonce = tx.Nonce() + 1
}

func (b *Bot) checkTxForOpportunity(targetTx *types.Transaction, reserveUpdates ammReserveUpdates) {
	for address := range reserveUpdates {
		// Ensure this sync event is for a pool we are watching.
		pair, ok := b.ammGraph.nodes[address]
		if !ok {
			log.Debug("finch: skipping unknown pool", "address", address)
			continue
		}

		// Get the optimal cycle and see if it's profitable.
		tr, err := b.ammGraph.getOptimalCycle(pair.cycles, address, reserveUpdates)
		if err != nil {
			log.Error("finch: error getting optimal cycle", "err", err, "address", address)
			continue
		}

		// Observe the potential profit even if it's not enough to execute on.
		arbProfitFoundCounter.Add(float64(tr.profit.Int64() / params.Ether))

		if tr.profit.Cmp(DefaultProfitTarget) != 1 {
			continue
		}

		// If we found a profitable opportunity then execute it.
		err = b.executeTrade(targetTx, tr)
		if err != nil {
			log.Error("finch: error executing trade", "err", err)
			continue
		}
	}

	ammPoolReserveChangesCheckedCounter.Add(float64(len(reserveUpdates)))
}

func (b *Bot) executeTrade(targetTx *types.Transaction, tr *ammTradeResult) error {
	arbOpportunityFoundCounter.Inc()

	// Stop now if we don't have a txBuilder configured
	if b.txBuilder.ContractAddr == nil {
		return nil
	}

	arbTx, err := b.buildArbTx(tr)
	if err != nil {
		return err
	}

	serializedArbTx, err := arbTx.MarshalBinary()
	if err != nil {
		return err
	}

	serializedTargetTx, err := targetTx.MarshalBinary()
	if err != nil {
		return err
	}

	bundleTxs := []string{
		common.Bytes2Hex(serializedTargetTx),
		common.Bytes2Hex(serializedArbTx),
	}

	log.Debug("finch: profitable arb found", "target", targetTx.Hash(), "profit", tr.profit, "bundleTxs", bundleTxs)
	return nil
}

// buildArbTx builds a new transaction to execute the arb.
func (b *Bot) buildArbTx(tr *ammTradeResult) (*types.Transaction, error) {
	contractSwaps := getPayload(tr.amountIn, tr.sequence)
	callData, err := contractABI.Pack(contractArbMethod, contractSwaps)
	if err != nil {
		return nil, err
	}

	rawTx := &types.DynamicFeeTx{
		ChainID:   b.chainCfg.ChainID,
		Nonce:     b.txBuilder.NextNonce,
		GasTipCap: common.Big0,
		GasFeeCap: b.txBuilder.GasFeeCap,
		Gas:       b.txBuilder.GasLimit,
		To:        b.txBuilder.ContractAddr,
		Value:     common.Big0,
		Data:      callData,
	}

	signedTx, err := types.SignNewTx(b.txBuilder.PrivateKey, b.txBuilder.Signer, rawTx)
	if err != nil {
		return nil, err
	}

	return signedTx, nil
}
