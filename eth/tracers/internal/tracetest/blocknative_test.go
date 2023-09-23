package tracetest

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/eth/tracers"
	"github.com/ethereum/go-ethereum/eth/tracers/blocknative"
	"github.com/ethereum/go-ethereum/eth/tracers/blocknative/decoder"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/tests"
)

var (
	testFlagLogLevl = flag.String("loglevel", "info", "Log level to use")
	testFlagFile    = flag.String("file", "", "Name of the file to run the test for")
)

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

type blocknativeTracerTest struct {
	Genesis      *core.Genesis      `json:"genesis"`
	Context      *callContext       `json:"context"`
	Input        string             `json:"input"`
	TracerConfig json.RawMessage    `json:"tracerConfig"`
	Result       *blocknative.Trace `json:"result"`

	name         string
	evm          *vm.EVM
	tx           *types.Transaction
	msg          *core.Message
	tracer       tracers.Tracer
	baseFee      *big.Int
	blockContext vm.BlockContext
	signer       types.Signer
	origin       common.Address
	txContext    vm.TxContext
}

func TestBlocknativeTracer(t *testing.T) {
	setLogging(t)

	testsCases, err := loadTestTxs("blocknative")
	if err != nil {
		t.Fatal(err)
	}
	decodingTestsCases, err := loadTestTxs("blocknative/with_decoding")
	if err != nil {
		t.Fatal(err)
	}
	testsCases = append(testsCases, decodingTestsCases...)

	for _, test := range testsCases {
		if *testFlagFile != "" {
			a := strings.ToLower(*testFlagFile)
			b := strings.ToLower(test.name)
			if a != b {
				continue
			}
		}
		t.Run(test.name, func(t *testing.T) {
			executeTestCase(test, t, true)
		})
	}
}

func BenchmarkBlocknativeTracerWithoutDecoding(b *testing.B) {
	benchmarkBlocknativeTracer(b, false, "blocknative", "blocknative/with_decoding")
}
func BenchmarkBlocknativeTracerWithDecoding(b *testing.B) {
	benchmarkBlocknativeTracer(b, true, "blocknative", "blocknative/with_decoding")
}

func benchmarkBlocknativeTracer(b *testing.B, decode bool, dirPaths ...string) {
	testCases := []*blocknativeTracerTest{}

	for _, dirPath := range dirPaths {
		files, err := os.ReadDir(filepath.Join("testdata", dirPath))
		if err != nil {
			b.Fatal(err)
		}

		for _, file := range files {
			if file.IsDir() {
				continue
			}
			var (
				test = new(blocknativeTracerTest)
				tx   = new(types.Transaction)
			)
			if blob, err := os.ReadFile(filepath.Join("testdata", dirPath, file.Name())); err != nil {
				b.Fatal(err)
			} else if err := json.Unmarshal(blob, test); err != nil {
				b.Fatal(err)
			}
			if err := tx.UnmarshalBinary(common.FromHex(test.Input)); err != nil {
				b.Fatal(err)
			}

			baseFee := big.NewInt(0x0)
			if test.Context.BaseFee != 0 {
				baseFee = new(big.Int).SetUint64(uint64(test.Context.BaseFee))
			}

			test.name = camel(strings.TrimSuffix(file.Name(), ".json"))
			test.tx = tx
			test.baseFee = baseFee

			test.signer = types.MakeSigner(test.Genesis.Config, new(big.Int).SetUint64(uint64(test.Context.Number)), uint64(test.Context.Time))
			test.blockContext = vm.BlockContext{
				CanTransfer: core.CanTransfer,
				Transfer:    core.Transfer,
				Coinbase:    test.Context.Miner,
				BlockNumber: new(big.Int).SetUint64(uint64(test.Context.Number)),
				Time:        uint64(test.Context.Time),
				Difficulty:  (*big.Int)(test.Context.Difficulty),
				GasLimit:    uint64(test.Context.GasLimit),
				BaseFee:     test.baseFee,
				Random:      test.Context.Random,
			}

			test.origin, _ = test.signer.Sender(tx)
			test.txContext = vm.TxContext{
				Origin:   test.origin,
				GasPrice: tx.GasPrice(),
			}

			testCases = append(testCases, test)
		}
	}

	for i := 0; i < b.N; i++ {
		test := testCases[i%len(testCases)]
		tx := test.tx

		_, _, statedb := tests.MakePreState(rawdb.NewMemoryDatabase(), test.Genesis.Alloc, false, rawdb.HashScheme)
		opts := blocknative.TracerOpts{Decode: decode}
		tracer, err := blocknative.NewTracerWithOpts(opts)
		if err != nil {
			b.Fatal(err)
		}

		evm := vm.NewEVM(test.blockContext, test.txContext, statedb, test.Genesis.Config, vm.Config{Tracer: tracer})
		msg, err := core.TransactionToMessage(tx, test.signer, test.blockContext.BaseFee)
		if err != nil {
			b.Fatal(err)
		}

		test.evm = evm
		test.msg = msg
		test.tracer = tracer

		executeTestCase(test, b, false)

	}
}

func setLogging(t testing.TB) {
	logHandler := log.StreamHandler(os.Stdout, log.TerminalFormat(true))
	level := log.LvlDebug
	if *testFlagLogLevl != "" {
		var err error
		level, err = log.LvlFromString(*testFlagLogLevl)
		if err != nil {
			t.Fatal(err)
		}
	}
	log.Root().SetHandler(log.LvlFilterHandler(level, logHandler))
}

func loadTestTxs(dirPath string) ([]*blocknativeTracerTest, error) {
	files, err := os.ReadDir(filepath.Join("testdata", dirPath))
	if err != nil {
		return nil, err
	}

	testCases := make([]*blocknativeTracerTest, 0, len(files))
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		var (
			test = new(blocknativeTracerTest)
			tx   = new(types.Transaction)
		)
		if blob, err := os.ReadFile(filepath.Join("testdata", dirPath, file.Name())); err != nil {
			return nil, err
		} else if err := json.Unmarshal(blob, test); err != nil {
			return nil, err
		}
		if err := tx.UnmarshalBinary(common.FromHex(test.Input)); err != nil {
			return nil, err
		}

		baseFee := big.NewInt(0xFF0000)
		if test.Context.BaseFee != 0 {
			baseFee = new(big.Int).SetUint64(uint64(test.Context.BaseFee))
		}

		// Configure a blockchain with the given prestate
		var (
			signer    = types.MakeSigner(test.Genesis.Config, new(big.Int).SetUint64(uint64(test.Context.Number)), uint64(test.Context.Time))
			origin, _ = signer.Sender(tx)
			txContext = vm.TxContext{
				Origin:   origin,
				GasPrice: tx.GasPrice(),
			}
			context = vm.BlockContext{
				CanTransfer: core.CanTransfer,
				Transfer:    core.Transfer,
				Coinbase:    test.Context.Miner,
				BlockNumber: new(big.Int).SetUint64(uint64(test.Context.Number)),
				Time:        uint64(test.Context.Time),
				Difficulty:  (*big.Int)(test.Context.Difficulty),
				GasLimit:    uint64(test.Context.GasLimit),
				BaseFee:     baseFee,
				Random:      test.Context.Random,
			}
			_, _, statedb = tests.MakePreState(rawdb.NewMemoryDatabase(), test.Genesis.Alloc, false, rawdb.HashScheme)
		)
		tracer, err := blocknative.NewTracer(test.TracerConfig)
		if err != nil {
			return nil, err
		}
		evm := vm.NewEVM(context, txContext, statedb, test.Genesis.Config, vm.Config{Tracer: tracer})
		msg, err := core.TransactionToMessage(tx, signer, context.BaseFee)
		if err != nil {
			return nil, err
		}

		name := strings.TrimPrefix(dirPath, "blocknative")
		name = strings.TrimPrefix(name, "/")
		if name != "" {
			name = name + "/"
		}
		name = name + camel(strings.TrimSuffix(file.Name(), ".json"))

		test.name = name
		test.evm = evm
		test.tx = tx
		test.msg = msg
		test.tracer = tracer
		testCases = append(testCases, test)
	}

	return testCases, nil
}

func executeTestCase(test *blocknativeTracerTest, t testing.TB, checkResult bool) {
	st := core.NewStateTransition(test.evm, test.msg, new(core.GasPool).AddGas(test.tx.Gas()))
	if _, err := st.TransitionDb(); err != nil {
		t.Fatalf("failed to execute transaction: %v", err)
	}

	res, err := test.tracer.GetResult()
	if err != nil {
		t.Fatalf("failed to retrieve trace result: %v", err)
	}
	ret := new(blocknative.Trace)
	if err := json.Unmarshal(res, ret); err != nil {
		t.Fatalf("failed to unmarshal trace result: %v", err)
	}

	if checkResult && !tracesEqual(ret, test.Result) {
		// Below are prints to show differences if we fail, can always just check against the specific test json files too!
		fmt.Println("Trace return: ")
		x, _ := json.Marshal(ret)
		// //x, _ := json.MarshalIndent(ret, "", "	")
		// y, _ := json.Marshal(test.Result)
		fmt.Println(string(x))
		fmt.Println("test.Result")
		// fmt.Println(string(y))
		t.Fatal("traces mismatch")
		// t.Fatalf("trace mismatch: \nhave %+v\nwant %+v", ret, test.Result)
	}
}

type NBCByAddress decoder.NetBalanceChanges

func (a NBCByAddress) Len() int           { return len(a) }
func (a NBCByAddress) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a NBCByAddress) Less(i, j int) bool { return a[i].Address.String() < a[j].Address.String() }

type BalanceChangesByAssetAddress []decoder.BalanceChange

func (a BalanceChangesByAssetAddress) Len() int      { return len(a) }
func (a BalanceChangesByAssetAddress) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a BalanceChangesByAssetAddress) Less(i, j int) bool {
	return a[i].Asset.Address.String() < a[j].Asset.Address.String()
}

func tracesEqual(x, y *blocknative.Trace) bool {
	// Clear out non-deterministic time
	x.Time = 0
	y.Time = 0

	// Sort the balance changes because we don't care about the order of the
	// breakdown.
	if len(x.BalanceChanges) != len(y.BalanceChanges) {
		return false
	}

	sort.Sort(NBCByAddress(x.BalanceChanges))
	sort.Sort(NBCByAddress(y.BalanceChanges))
	for i := range x.BalanceChanges {
		sort.Sort(BalanceChangesByAssetAddress(x.BalanceChanges[i].BalanceChanges))
		sort.Sort(BalanceChangesByAssetAddress(y.BalanceChanges[i].BalanceChanges))
	}

	xTrace := new(blocknative.Trace)
	yTrace := new(blocknative.Trace)
	if xj, err := json.Marshal(x); err == nil {
		if json.Unmarshal(xj, xTrace) != nil {
			return false
		}
	} else {
		return false
	}
	if yj, err := json.Marshal(y); err == nil {
		if json.Unmarshal(yj, yTrace) != nil {
			return false
		}
	} else {
		return false
	}

	return reflect.DeepEqual(xTrace, yTrace)
}
