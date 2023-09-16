package decoder

import (
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/lru"
)

const (
	cacheSizeContracts = 10_0000
	cacheSizeAssets    = 10_0000
)

type Caches struct {
	contractsMu sync.RWMutex
	contracts   lru.BasicLRU[common.Address, *Contract]
	assetsMu    sync.RWMutex
	assets      lru.BasicLRU[AssetID, *AssetMetadata]
}

func NewCaches() *Caches {
	return &Caches{
		contracts: lru.NewBasicLRU[common.Address, *Contract](cacheSizeContracts),
		assets:    lru.NewBasicLRU[AssetID, *AssetMetadata](cacheSizeAssets),
	}
}
