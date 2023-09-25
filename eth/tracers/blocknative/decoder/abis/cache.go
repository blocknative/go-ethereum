package abis

import (
	"context"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/lru"
	"github.com/ethereum/go-ethereum/log"
)

var (
	cacheABIsSize      = 10000
	cacheContractsSize = 10000

	cacheWarmupTimeout = 10 * time.Second
	cacheABIs          = lru.NewCache[MD5Hash, *ABI](cacheABIsSize)
	cacheContracts     = lru.NewCache[common.Address, *Contract](cacheContractsSize)
)

func LoadAndCacheAll(dsn string, chainID *big.Int) error {
	store := &PostgresStore{}
	err := store.Open(dsn)
	if err != nil {
		return err
	}
	defer func(store *PostgresStore) {
		err := store.Close()
		if err != nil {
			log.Error("error closing store: " + err.Error())
		}
	}(store)

	return LoadAndCacheAllFromStore(store, chainID)
}

func GetABI(hashMD5 MD5Hash) (*abi.ABI, bool) {
	a, ok := cacheABIs.Get(hashMD5)
	if !ok {
		return nil, false
	}
	return a.ABI, true
}

func GetABIForContract(address common.Address) (*abi.ABI, bool) {
	c, ok := cacheContracts.Get(address)
	if !ok {
		return nil, false
	}
	return GetABI(c.HashMD5)
}

func LoadAndCacheAllFromStore(store Store, chainID *big.Int) error {
	ctx, cancel := context.WithTimeout(context.Background(), cacheWarmupTimeout)
	defer cancel()

	abis, err := store.GetABIs(ctx)
	if err != nil {
		return err
	}
	for _, abi := range abis {
		cacheABIs.Add(abi.HashMD5, abi)
	}
	log.Info("loaded %d abis", len(abis))

	contracts, err := store.GetContracts(ctx, chainID)
	if err != nil {
		return err
	}
	for _, contract := range contracts {
		cacheContracts.Add(contract.Address, contract)
	}
	log.Info("loaded %d contracts", len(contracts))

	return nil
}
