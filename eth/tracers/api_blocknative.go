package tracers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/misc/eip1559"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
)

// BNTraceBundleArgs represents the arguments for a multi-sim
type BNMultiSimArgs struct {
	Txs        []ethapi.TransactionArgs `json:"txs"`
	Coinbase   *string                  `json:"coinbase"`
	Timestamp  *uint64                  `json:"timestamp"`
	Timeout    *int64                   `json:"timeout"`
	GasLimit   *uint64                  `json:"gasLimit"`
	Difficulty *big.Int                 `json:"difficulty"`
	BaseFee    *big.Int                 `json:"baseFee"`
}

// func (api *API) TraceCall(ctx context.Context, args ethapi.TransactionArgs, blockNrOrHash rpc.BlockNumberOrHash, config *TraceCallConfig) (interface{}, error) {
// func (s *BundleAPI) BNMultiSim(ctx context.Context, args BNMultiSimArgs) (map[string]interface{}, error) {
func (api *API) BNMultiSim(ctx context.Context, args BNMultiSimArgs) (map[string]interface{}, error) {
	if len(args.Txs) == 0 {
		return nil, errors.New("bundle missing unsigned txs")
	}

	defer func(start time.Time) { log.Debug("Executing EVM call finished", "runtime", time.Since(start)) }(time.Now())

	// Start a timeout trigger for the execution
	timeoutMilliSeconds := int64(5000)
	if args.Timeout != nil {
		timeoutMilliSeconds = *args.Timeout
	}
	timeout := time.Millisecond * time.Duration(timeoutMilliSeconds)

	// Get state at current block number
	// If we want the "pending" blocknumber we would call rpc.BlockNumber(-2)
	blockNumberRPC := rpc.BlockNumber(-1)
	//state, parent, err := api.backend.StateAndHeaderByNumber(ctx, blockNumberRPC)
	//state, parent, err := api.backend.StateAtBlock

	// We can no longer use StateAndHeaderByNumber and must instead of api.backend.StateAtBlock
	// We'll have to load the *Block object by number and then get the state from that

	// Get the parent block
	parent, err := api.backend.BlockByNumber(ctx, blockNumberRPC)
	if err != nil {
		return nil, err
	}

	// Get the state from the block and exit if we cannot find the requested state to use
	state, release, err := api.backend.StateAtBlock(ctx, parent, defaultTraceReexec, nil, true, false)
	if err != nil {
		return nil, err
	}
	defer release()
	if state == nil {
		return nil, err
	}

	// Configure defaults for processing a state
	timestamp := parent.Time() + 1
	if args.Timestamp != nil {
		timestamp = *args.Timestamp
	}
	coinbase := parent.Coinbase()
	if args.Coinbase != nil {
		coinbase = common.HexToAddress(*args.Coinbase)
	}
	difficulty := parent.Difficulty()
	if args.Difficulty != nil {
		difficulty = args.Difficulty
	}
	gasLimit := parent.GasLimit()
	if args.GasLimit != nil {
		gasLimit = *args.GasLimit
	}
	var baseFee *big.Int

	baseFee = eip1559.CalcBaseFee(api.backend.ChainConfig(), parent.Header())

	// Create a block header for our wanted block
	header := &types.Header{
		ParentHash: parent.Hash(),
		Number:     big.NewInt(parent.Number().Int64()),
		GasLimit:   gasLimit,
		Time:       timestamp,
		Difficulty: difficulty,
		Coinbase:   coinbase,
		BaseFee:    baseFee,
	}

	// Setup context so it may be cancelled the call has completed
	// or, in case of unmetered gas, setup a context with a timeout.
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}
	// Make sure the context is cancelled when the call has completed
	// this makes sure resources are cleaned up.
	defer cancel()

	// Create a vmconfig with no options, we add the trcer later
	vmconfig := vm.Config{}

	// Setup the gas pool (also for unmetered requests)
	var globalGasCap uint64 = 50000000
	gp := new(core.GasPool).AddGas(globalGasCap)

	// Create a results structure for returning array of trace results
	results := []map[string]interface{}{}

	// Init gasFees variabel
	var totalGasUsed uint64
	gasFees := new(big.Int)

	// Now we iterate through argument transactions and apply each message to state and collect trace results as they happen
	for i, tx := range args.Txs {
		// Since its a txCall we'll just prepare the
		// state with a random hash
		var randomHash common.Hash
		rand.Read(randomHash[:])
		// New random hash since its a call
		state.SetTxContext(randomHash, i)

		// Convert the transaction into a "Message type" which the core evm can understand
		// TODO: figure out what base fee to use here. Probably need to calc next baseFee
		// from the two args
		//msg := tx.ToMessage(globalGasCap, header.BaseFee)
		msg := tx.ToMessage(header.BaseFee, true, true)

		api.chainContext(ctx)
		// Apply transaction to state and collect traceResult
		receipt, result, traceResult, err := ApplyUnsignedTransactionWithResult(api.backend.ChainConfig(), api.chainContext(ctx), &coinbase, gp, state, header, msg, &header.GasUsed, vmconfig)
		if err != nil {
			return nil, fmt.Errorf("err: %w; txhash %s", err, tx.From)
		}

		from := common.Address{}
		if tx.From != nil {
			from = *tx.From
		}

		jsonResult := map[string]interface{}{
			"gasUsed":     receipt.GasUsed,
			"fromAddress": from,
			"toAddress":   tx.To,
			"traceResult": traceResult,
		}
		totalGasUsed += receipt.GasUsed
		if result.Err != nil {
			jsonResult["error"] = result.Err.Error()
			revert := result.Revert()
			if len(revert) > 0 {
				jsonResult["revert"] = string(revert)
			}
		} else {
			dst := make([]byte, hex.EncodedLen(len(result.Return())))
			hex.Encode(dst, result.Return())
			jsonResult["value"] = "0x" + string(dst)
		}

		results = append(results, jsonResult)
	}

	ret := map[string]interface{}{}
	ret["results"] = results
	ret["gasFees"] = gasFees.String()
	ret["gasUsed"] = totalGasUsed
	ret["blockNumber"] = parent.Number().Int64()

	ret["args"] = header
	return ret, nil
}
