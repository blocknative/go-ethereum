package tracetest

import (
	"encoding/json"
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

type txnOpCodeTracerTest struct {
	Genesis      *core.Genesis      `json:"genesis"`
	Context      *callContext       `json:"context"`
	Input        string             `json:"input"`
	TracerConfig json.RawMessage    `json:"tracerConfig"`
	Result       *blocknative.Trace `json:"result"`
}

func TestTxnOpCodeTracer(t *testing.T) {
	log.Root().SetHandler(log.StreamHandler(os.Stdout, log.TerminalFormat(true)))

	//testTxnOpCodeTracer("txnOpCodeTracer", "txnOpCode_tracer", t)
	testTxnOpCodeTracer("txnOpCodeTracer", "txnOpCode_tracer_with_netbalchanges", t)
}

func testTxnOpCodeTracer(tracerName string, dirPath string, t *testing.T) {
	files, err := os.ReadDir(filepath.Join("testdata", dirPath))
	if err != nil {
		t.Fatalf("failed to retrieve tracer tgest suite: %v", err)
	}
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
		file := file
		t.Run(camel(strings.TrimSuffix(file.Name(), ".json")), func(t *testing.T) {
			//t.Parallel()

			var (
				test = new(txnOpCodeTracerTest)
				tx   = new(types.Transaction)
			)
			// Call tracer test found, read if from disk
			if blob, err := os.ReadFile(filepath.Join("testdata", dirPath, file.Name())); err != nil {
				t.Fatalf("failed to read testcase: %v", err)
			} else if err := json.Unmarshal(blob, test); err != nil {
				t.Fatalf("failed to parse testcase: %v", err)
			}
			// Here we use unmarshalBinary as it can account for EIP2718 typed transactions in the tests
			if err := tx.UnmarshalBinary(common.FromHex(test.Input)); err != nil {
				t.Fatalf("failed to parse testcase input: %v", err)
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
			tracer, err := tracers.DefaultDirectory.New(tracerName, new(tracers.Context), test.TracerConfig)
			if err != nil {
				t.Fatalf("failed to create call tracer: %v", err)
			}
			evm := vm.NewEVM(context, txContext, statedb, test.Genesis.Config, vm.Config{Tracer: tracer})
			msg, err := core.TransactionToMessage(tx, signer, context.BaseFee)
			if err != nil {
				t.Fatalf("failed to prepare transaction for tracing: %v", err)
			}
			st := core.NewStateTransition(evm, msg, new(core.GasPool).AddGas(tx.Gas()))
			if _, err = st.TransitionDb(); err != nil {
				t.Fatalf("failed to execute transaction: %v", err)
			}

			res, err := tracer.GetResult()
			if err != nil {
				t.Fatalf("failed to retrieve trace result: %v", err)
			}
			ret := new(blocknative.Trace)
			if err := json.Unmarshal(res, ret); err != nil {
				t.Fatalf("failed to unmarshal trace result: %v", err)
			}

			if !tracesEqual(ret, test.Result) {
				// Below are prints to show differences if we fail, can always just check against the specific test json files too!
				//fmt.Println("Trace return: ")
				//x, _ := json.Marshal(ret)
				////x, _ := json.MarshalIndent(ret, "", "	")
				//y, _ := json.Marshal(test.Result)
				//fmt.Println(string(x))
				//fmt.Println("test.Result")
				//fmt.Println(string(y))
				t.Fatal("traces mismatch")
				//t.Fatalf("trace mismatch: \nhave %+v\nwant %+v", ret, test.Result)
			}
		})
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
	x.Time = ""
	y.Time = ""

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
