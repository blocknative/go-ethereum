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
	bot := newTestBotWithLoadedPools(t)
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

func TestFinchNoLoadedPools(t *testing.T) {
	bot := newTestBot(t)
	pair := bot.ammGraph.nodes[testPair]
	tr, err := bot.ammGraph.getOptimalCycle(pair.cycles, testPair, ammReserveUpdates{})
	if err != nil {
		t.Fatal(err)
	}
	if tr.profit.Cmp(common.Big0) != 0 {
		t.Fatal("Expected profit to be 0")
	}
}

func TestFinchParseReservesFromStorageValue(t *testing.T) {
	type testCase struct {
		storageValue []byte
		r0           *big.Int
		r1           *big.Int
	}

	tests := []testCase{
		// Both reserves are not set
		{[]byte{}, common.Big0, common.Big0},
		{[]byte{0}, common.Big0, common.Big0},
		{[]byte{0, 0}, common.Big0, common.Big0},
		{[]byte{0, 0, 0}, common.Big0, common.Big0},
		{[]byte{0, 0, 0, 0}, common.Big0, common.Big0},

		// r0 is not set, r1 is
		{[]byte{1, 0, 0, 0, 0}, common.Big0, common.Big1},
		{[]byte{2, 0, 0, 0, 0}, common.Big0, common.Big2},
		{[]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0}, common.Big0, common.Big1},

		// r0 is set, r1 is not
		{[]byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, common.Big1, common.Big0},
		{[]byte{2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, common.Big2, common.Big0},

		// Both reserves are set
		{[]byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0}, common.Big1, common.Big1},
		{[]byte{2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0}, common.Big2, common.Big1},
	}

	for _, test := range tests {
		r0, r1 := parseReservesFromStorageValue(test.storageValue)
		if r0.Cmp(test.r0) != 0 {
			t.Fatalf("Expected r0 to be %v, got %v", test.r0, r0)
		}
		if r1.Cmp(test.r1) != 0 {
			t.Fatalf("Expected r1 to be %v, got %v", test.r1, r1)
		}
	}
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
	txPool := txpool.NewTxPool(txPoolConfig, chainConfig, chain, core.Banlist{})

	return &testBackend{
		chain:  chain,
		txPool: txPool,
		db:     db,
	}, nil
}

func newTestBot(t *testing.T) *Bot {
	backend, err := newTestBackend()
	if err != nil {
		t.Fatal(err)
	}

	bot, err := NewBot(Config{}, backend)
	if err != nil {
		t.Fatal(err)
	}
	return bot
}

func newTestBotWithLoadedPools(t *testing.T) *Bot {
	bot := newTestBot(t)

	for _, pair := range bot.ammGraph.nodes {
		pair.loaded = true
	}

	return bot
}
