package decoder

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/lru"
)

const (
	cacheSizeContracts = 10_0000
	cacheSizeAssets    = 10_0000
)

type Caches struct {
	contracts *lru.Cache[common.Address, *Contract]
	assets    *lru.Cache[AssetID, *AssetMetadata]
}

func NewCaches() *Caches {
	return &Caches{
		contracts: lru.NewCache[common.Address, *Contract](cacheSizeContracts),
		assets:    lru.NewCache[AssetID, *AssetMetadata](cacheSizeAssets),
	}
}
