package cache

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/lru"
	"github.com/ethereum/go-ethereum/eth/tracers/blocknative"
)

var (
	traceCacheSize = 10_000
	traceCache     = lru.NewCache[[64]byte, *blocknative.Trace](traceCacheSize)
)

func PutTrace(blockHash, txHash common.Hash, trace *blocknative.Trace) bool {
	return traceCache.Add(traceCacheKey(blockHash, txHash), trace)
}

func GetTrace(blockHash, txHash common.Hash) (*blocknative.Trace, bool) {
	return traceCache.Get(traceCacheKey(blockHash, txHash))
}

func traceCacheKey(blockHash, txHash common.Hash) [64]byte {
	key := [64]byte{}
	copy(key[:32], blockHash[:])
	copy(key[32:], txHash[:])
	return key
}
