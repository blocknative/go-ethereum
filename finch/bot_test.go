package finch

import (
	"crypto/ecdsa"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/txpool"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"
	"math/big"
	"testing"
)

var (
	testPair = common.HexToAddress("B4e16d0168e52d35CaCD2c6185b44281Ec28C9Dc")
)

type testBackend struct {
	chain  *core.BlockChain
	txPool *txpool.TxPool
	db     ethdb.Database
}

func (b *testBackend) BlockChain() *core.BlockChain {
	return b.chain
}
func (b *testBackend) TxPool() *txpool.TxPool {
	return b.txPool
}
func (b *testBackend) ChainDb() ethdb.Database {
	return b.db
}

func TestFinch(t *testing.T) {
	b, err := newTestBackend()
	if err != nil {
		t.Fatal(err)
	}

	bot, err := NewBot(Config{TxBuilder: nil}, b)
	if err != nil {
		t.Fatal(err)
	}

	pair := bot.ammGraph.nodes[testPair]
	reserves := make(ammReserveUpdates)
	reserves[testPair] = uniswapV2PoolSyncEvent{
		reserves0: big.NewInt(10000),
		reserves1: big.NewInt(2000000),
	}

	tr, err := bot.ammGraph.getOptimalCycle(pair.cycles, testPair, reserves)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println("Profit: ", tr.profit.String())
	fmt.Println("Amount in: ", tr.amountIn.String())
	fmt.Println("Cycle: ", tr.cycle)
	fmt.Println("Pair: ", tr.pairAddress)
	fmt.Println("Sequence: ", tr.sequence)
}

func newTxBuilder() *TxBuilder {
	contractAddr := common.HexToAddress("0x6d3797426B1CCf0cF5028Cd7A27d6b100e5D378b")
	return &TxBuilder{
		ContractAddr: &contractAddr,
		GasLimit:     250000,
		GasFeeCap:    big.NewInt(1000000000),
		PrivateKey:   &ecdsa.PrivateKey{D: big.NewInt(1)},
		Signer:       types.NewEIP155Signer(big.NewInt(1)),
	}
}

func newTestBackend() (*testBackend, error) {
	db := rawdb.NewMemoryDatabase()

	chainConfig := params.AllEthashProtocolChanges
	engine := ethash.NewFaker()
	var genesis = core.Genesis{
		Config: chainConfig,
		Alloc:  core.GenesisAlloc{},
	}

	chain, err := core.NewBlockChain(db, &core.CacheConfig{TrieDirtyDisabled: true}, &genesis, nil, engine, vm.Config{}, nil, nil)
	if err != nil {
		return nil, err
	}
	txPoolConfig := txpool.DefaultConfig
	txPoolConfig.Journal = ""
	txPool := txpool.NewTxPool(txPoolConfig, chainConfig, chain)

	return &testBackend{
		chain:  chain,
		txPool: txPool,
		db:     db,
	}, nil
}
