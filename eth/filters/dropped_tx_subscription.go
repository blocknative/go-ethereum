package filters

import (
	"context"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/tracers/blocknative"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
	lru "github.com/hashicorp/golang-lru"
	"sync"
	"time"
)

type dropNotification struct {
	// TxHash common.Hash `json:"txhash"`
	Tx          *RPCTransaction `json:"tx"`
	Reason      string                 `json:"reason"`
	Replacement *RPCTransaction `json:"replacedby,omitempty"`
	Peer interface{}                   `json:"peer,omitempty"`
	Time        int64                  `json:"ts"`
}

type rejectNotification struct {
	Tx     *RPCTransaction `json:"tx"`
	Reason string                 `json:"reason"`
	Peer   interface{} `json:"peer,omitempty"`
	Time   int64                  `json:"ts"`
}


// Note: Copied from internal/ethapi to avoid import loops
// RPCTransaction represents a transaction that will serialize to the RPC representation of a transaction
type RPCTransaction struct {
	BlockHash           *common.Hash      `json:"blockHash"`
	BlockNumber         *hexutil.Big      `json:"blockNumber"`
	From                common.Address    `json:"from"`
	Gas                 hexutil.Uint64    `json:"gas"`
	GasPrice            *hexutil.Big      `json:"gasPrice"`
	GasFeeCap           *hexutil.Big      `json:"maxFeePerGas,omitempty"`
	GasTipCap           *hexutil.Big      `json:"maxPriorityFeePerGas,omitempty"`
	MaxFeePerBlobGas    *hexutil.Big      `json:"maxFeePerBlobGas,omitempty"`
	Hash                common.Hash       `json:"hash"`
	Input               hexutil.Bytes     `json:"input"`
	Nonce               hexutil.Uint64    `json:"nonce"`
	To                  *common.Address   `json:"to"`
	TransactionIndex    *hexutil.Uint64   `json:"transactionIndex"`
	Value               *hexutil.Big      `json:"value"`
	Type                hexutil.Uint64    `json:"type"`
	Accesses            *types.AccessList `json:"accessList,omitempty"`
	ChainID             *hexutil.Big      `json:"chainId,omitempty"`
	BlobVersionedHashes []common.Hash     `json:"blobVersionedHashes,omitempty"`
	V                   *hexutil.Big      `json:"v"`
	R                   *hexutil.Big      `json:"r"`
	S                   *hexutil.Big      `json:"s"`
	YParity             *hexutil.Uint64   `json:"yParity,omitempty"`

	Trace  *blocknative.Trace `json:"trace,omitempty"`
	Future bool               `json:"future"`
}

// newRPCTransaction returns a transaction that will serialize to the RPC
// representation, with the given location metadata set (if available).
func newRPCPendingTransaction(tx *types.Transaction) *RPCTransaction {
	if tx == nil {
		return nil
	}
	var signer types.Signer
	if tx.Protected() {
		signer = types.LatestSignerForChainID(tx.ChainId())
	} else {
		signer = types.HomesteadSigner{}
	}
	from, _ := types.Sender(signer, tx)
	v, r, s := tx.RawSignatureValues()
	result := &RPCTransaction{
		Type:     hexutil.Uint64(tx.Type()),
		From:     from,
		Gas:      hexutil.Uint64(tx.Gas()),
		GasPrice: (*hexutil.Big)(tx.GasPrice()),
		Hash:     tx.Hash(),
		Input:    hexutil.Bytes(tx.Data()),
		Nonce:    hexutil.Uint64(tx.Nonce()),
		To:       tx.To(),
		Value:    (*hexutil.Big)(tx.Value()),
		V:        (*hexutil.Big)(v),
		R:        (*hexutil.Big)(r),
		S:        (*hexutil.Big)(s),
	}
	switch tx.Type() {
	case types.AccessListTxType:
		al := tx.AccessList()
		result.Accesses = &al
		result.ChainID = (*hexutil.Big)(tx.ChainId())
	case types.DynamicFeeTxType:
		al := tx.AccessList()
		result.Accesses = &al
		result.ChainID = (*hexutil.Big)(tx.ChainId())
		result.GasFeeCap = (*hexutil.Big)(tx.GasFeeCap())
		result.GasTipCap = (*hexutil.Big)(tx.GasTipCap())
		// if the transaction has been mined, compute the effective gas price
		result.GasPrice = nil
	case types.BlobTxType:
		al := tx.AccessList()
		yparity := hexutil.Uint64(v.Sign())
		result.Accesses = &al
		result.ChainID = (*hexutil.Big)(tx.ChainId())
		result.YParity = &yparity
		result.GasFeeCap = (*hexutil.Big)(tx.GasFeeCap())
		result.GasTipCap = (*hexutil.Big)(tx.GasTipCap())
		result.GasPrice = nil
		result.MaxFeePerBlobGas = (*hexutil.Big)(tx.BlobGasFeeCap())
		result.BlobVersionedHashes = tx.BlobHashes()
	}
	return result
}

func replacementHashString(h common.Hash) string {
	if h == (common.Hash{}) {
		return ""
	}
	return h.String()
}

// DroppedTransactions send a notification each time a transaction is dropped from the mempool
func (api *FilterAPI) DroppedTransactions(ctx context.Context) (*rpc.Subscription, error) {
	if txPeerMap == nil { txPeerMap, _ = lru.New(100000) }
	if peerIDMap == nil { peerIDMap = &sync.Map{} }
	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
	}

	rpcSub := notifier.CreateSubscription()

	go func() {
		dropped := make(chan core.DropTxsEvent)
		droppedSub := api.sys.backend.SubscribeDropTxsEvent(dropped)

		metricsDroppedTxsNew.Inc(1)
		defer metricsDroppedTxsEnd.Inc(1)

		for {
			select {
			case d := <-dropped:
				metricsDroppedTxsReceived.Inc(int64(len(d.Txs)))
				for _, tx := range d.Txs {
					notification := &dropNotification{
						Tx: newRPCPendingTransaction(tx),
						Reason: d.Reason,
						Replacement: newRPCPendingTransaction(d.Replacement),
						Time: time.Now().UnixNano(),
					}
					if d.Replacement != nil {
						peerid, _ := txPeerMap.Get(tx.Hash())
						notification.Peer, _ = peerIDMap.Load(peerid)
					}
					metricsDroppedTxsSent.Inc(1)
					if err := notifier.Notify(rpcSub.ID, notification); err != nil {
						log.Error("dropped_txs_stream: failed to notify", "err", err)
						return
					}
				}
			case <-rpcSub.Err():
				droppedSub.Unsubscribe()
				return
			case <-notifier.Closed():
				droppedSub.Unsubscribe()
				return
			}
		}
	}()

	return rpcSub, nil
}

func (api *FilterAPI) dropLoop() {
	dropped := make(chan core.DropTxsEvent)
	droppedSub := api.sys.backend.SubscribeDropTxsEvent(dropped)
	defer droppedSub.Unsubscribe()
	for d := range dropped {
		for _, tx := range d.Txs {
			h := tx.Hash()
			if tsMap != nil { tsMap.Remove(h) }
			if txPeerMap != nil { txPeerMap.Remove(h) }
		}
	}
}

// RejectedTransactions send a notification each time a transaction is rejected from entering the mempool
func (api *FilterAPI) RejectedTransactions(ctx context.Context) (*rpc.Subscription, error) {
	if txPeerMap == nil { txPeerMap, _ = lru.New(100000) }
	if peerIDMap == nil { peerIDMap = &sync.Map{} }
	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
	}

	rpcSub := notifier.CreateSubscription()

	go func() {
		rejected := make(chan core.RejectedTxEvent)
		rejectedSub := api.sys.backend.SubscribeRejectedTxEvent(rejected)

		for {
			select {
			case d := <-rejected:
				reason := ""
				if d.Reason != nil {
					reason = d.Reason.Error()
				}
				peerid, _ := txPeerMap.Get(d.Tx.Hash())
				peer, _ := peerIDMap.Load(peerid)
				notifier.Notify(rpcSub.ID, &rejectNotification{
					Tx: newRPCPendingTransaction(d.Tx),
					Reason: reason,
					Peer: peer,
					Time: time.Now().UnixNano(),
				})
			case <-rpcSub.Err():
				rejectedSub.Unsubscribe()
				return
			case <-notifier.Closed():
				rejectedSub.Unsubscribe()
				return
			}
		}
	}()

	return rpcSub, nil
}
