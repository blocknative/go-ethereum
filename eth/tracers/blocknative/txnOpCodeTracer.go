package blocknative

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/eth/tracers/blocknative/decoder"
	"github.com/ethereum/go-ethereum/log"
)

var (
	decoderCache = decoder.NewCaches()
)

// txnOpCodeTracer is a go implementation of the Tracer interface which
// only returns a restricted trace of a transaction consisting of transaction
// op codes and relevant gas data.
// This is intended for Blocknative usage.
type txnOpCodeTracer struct {
	opts    TracerOpts
	env     *vm.EVM
	decoder *decoder.Decoder

	trace     Trace
	startTime time.Time
	callStack []CallFrame
	interrupt uint32
	reason    error
}

// NewTxnOpCodeTracer returns a new txnOpCodeTracer tracer with the given
// options applied.
func NewTxnOpCodeTracer(cfg json.RawMessage) (Tracer, error) {
	var opts TracerOpts

	if cfg != nil {
		if err := json.Unmarshal(cfg, &opts); err != nil {
			return nil, err
		}
	}

	return NewTxnOpCodeTracerWithOpts(opts)
}

func NewTxnOpCodeTracerWithOpts(opts TracerOpts) (Tracer, error) {
	opts.Decode = opts.Decode || opts.BalanceChanges

	var t = txnOpCodeTracer{
		opts:      opts,
		callStack: make([]CallFrame, 1),
	}

	if !t.opts.DisableBlockContext {
		t.trace.BlockContext = &BlockContext{}
	}

	return &t, nil

}

// GetTrace returns the resulting Trace object.
func (t *txnOpCodeTracer) GetTrace() (*Trace, error) {
	if t.opts.Decode {
		t.trace.BalanceChanges = t.decoder.GetBalanceChanges()
	}

	t.trace.CallFrame = t.callStack[0]
	return &t.trace, nil
}

// GetResult returns an empty json object.
func (t *txnOpCodeTracer) GetResult() (json.RawMessage, error) {
	trace, err := t.GetTrace()
	if err != nil {
		return nil, err
	}

	res, err := json.Marshal(trace)
	if err != nil {
		return nil, err
	}
	return res, t.reason
}

// CaptureStart implements the EVMLogger interface to initialize the tracing operation.
func (t *txnOpCodeTracer) CaptureStart(env *vm.EVM, from common.Address, to common.Address, create bool, input []byte, gas uint64, value *big.Int) {
	t.startTime = time.Now()
	t.env = env

	if t.opts.Decode {
		t.decoder = decoder.New(decoderCache, decoderEVM{env})
	}

	if !t.opts.DisableBlockContext {
		// Blocks only contain `Random` post-merge, but we still have pre-merge tests.
		random := ""
		if env.Context.Random != nil {
			random = bytesToHex(env.Context.Random.Bytes())
		}

		t.trace.BlockContext.Number = env.Context.BlockNumber.Uint64()
		t.trace.BlockContext.BaseFee = env.Context.BaseFee.Uint64()
		t.trace.BlockContext.Time = env.Context.Time
		t.trace.BlockContext.Coinbase = addrToHex(env.Context.Coinbase)
		t.trace.BlockContext.GasLimit = env.Context.GasLimit
		t.trace.BlockContext.Random = random
	}

	// Create a call-frame for the top level call.
	t.callStack[0] = CallFrame{
		Type:  "CALL",
		From:  addrToHex(from),
		To:    addrToHex(to),
		Input: bytesToHex(input),
		Gas:   gas,
		Value: value.Uint64(),
	}
	if create {
		t.callStack[0].Type = "CREATE"
	}

	// Try adding decode information but don't fail if we can't.
	if t.opts.Decode {
		if decoded, err := t.decoder.DecodeCallFrame(from, to, value, input); err == nil {
			t.callStack[0].Decoded = decoded
		}
	}
}

// CaptureEnd is called after the call finishes to finalize the tracing.
func (t *txnOpCodeTracer) CaptureEnd(output []byte, gasUsed uint64, err error) {
	finalizeCallFrame(&t.callStack[0], output, gasUsed, err)

	// If the user wants the logs, grab them from the state
	if t.opts.Logs {
		for _, stateLog := range t.env.StateDB.Logs() {
			t.trace.Logs = append(t.trace.Logs, CallLog{
				Address: stateLog.Address,
				Data:    bytesToHex(stateLog.Data),
				Topics:  stateLog.Topics,
			})
		}
	}

	// Add gas payments to balance changes
	if t.opts.Decode {
		t.decoder.CaptureGas(t.env.TxContext.Origin, t.env.Context.Coinbase, gasUsed, t.env.TxContext.GasPrice, t.env.Context.BaseFee)
	}

	// Add total time duration for this trace request
	elapsedTime := time.Now().Sub(t.startTime)
	t.trace.Time = fmt.Sprintf("%v", elapsedTime)
}

// CaptureState implements the EVMLogger interface to trace a single step of VM execution.
func (t *txnOpCodeTracer) CaptureState(_ uint64, _ vm.OpCode, _, _ uint64, _ *vm.ScopeContext, _ []byte, depth int, _ error) {
	defer func() {
		if r := recover(); r != nil {
			t.callStack[depth].Error = "internal failure"
			log.Warn("Panic during trace. Recovered.", "err", r)
		}
	}()
}

// CaptureFault implements the EVMLogger interface to trace an execution fault.
func (t *txnOpCodeTracer) CaptureFault(_ uint64, _ vm.OpCode, _, _ uint64, _ *vm.ScopeContext, _ int, _ error) {
}

// CaptureEnter is called when EVM enters a new scope (via call, create or selfdestruct).
func (t *txnOpCodeTracer) CaptureEnter(typ vm.OpCode, from common.Address, to common.Address, input []byte, gas uint64, value *big.Int) {
	// Skip if tracing was interrupted
	if atomic.LoadUint32(&t.interrupt) > 0 {
		t.env.Cancel()
		return
	}

	// Create CallFrame, decode it, and all it to the end of the callstack.
	call := CallFrame{
		Type:  typ.String(),
		From:  addrToHex(from),
		To:    addrToHex(to),
		Input: bytesToHex(input),
		Gas:   gas,
		Value: value.Uint64(),
	}
	if t.opts.Decode {
		if decoded, err := t.decoder.DecodeCallFrame(from, to, value, input); err == nil {
			call.Decoded = decoded
		}
	}

	t.callStack = append(t.callStack, call)
}

// CaptureExit is called when EVM exits a scope, even if the scope didn't execute any code.
func (t *txnOpCodeTracer) CaptureExit(output []byte, gasUsed uint64, err error) {
	// Skip if we have no call frames.
	size := len(t.callStack)
	if size == 0 {
		return
	}

	// We have a call frame, so finalize it.
	finalizeCallFrame(&t.callStack[size-1], output, gasUsed, err)

	// If we have a parent call frame nest this one into it.
	if size <= 1 {
		return
	}
	end := size - 1
	call := t.callStack[end]
	t.callStack = t.callStack[:end]
	end -= 1
	t.callStack[end].Calls = append(t.callStack[end].Calls, call)
}

// CaptureTxStart fulfils the standard Tracer interface, but we don't use it.
func (t *txnOpCodeTracer) CaptureTxStart(_ uint64) {}

// CaptureTxEnd fulfils the standard Tracer interface, but we don't use it.
func (t *txnOpCodeTracer) CaptureTxEnd(_ uint64) {}

// Stop terminates execution of the tracer at the first opportune moment.
func (t *txnOpCodeTracer) Stop(err error) {
	t.reason = err
	atomic.StoreUint32(&t.interrupt, 1)
}

// SetStateRoot implements core.stateRootSetter and stores the given root in the trace's BlockContext.
func (t *txnOpCodeTracer) SetStateRoot(root common.Hash) {
	if !t.opts.DisableBlockContext {
		t.trace.BlockContext.StateRoot = bytesToHex(root.Bytes())
	}
}

func finalizeCallFrame(call *CallFrame, output []byte, gasUsed uint64, err error) {
	call.GasUsed = gasUsed

	// If there was an error then try decoding it and stop.
	if err != nil {
		call.Error = err.Error()
		if err.Error() == "execution reverted" && len(output) > 0 {
			call.Output = bytesToHex(output)
			revertReason, _ := abi.UnpackRevert(output)
			call.ErrorReason = revertReason
		}

		if call.Type == "CREATE" || call.Type == "CREATE2" {
			call.To = ""
		}
		return
	}

	// The call was successful so decode the output.
	call.Output = bytesToHex(output)
}

type decoderEVM struct {
	*vm.EVM
}

func (d decoderEVM) GetCode(addr common.Address) []byte {
	return d.StateDB.GetCode(addr)
}

func (d decoderEVM) CallCode(addr common.Address, method []byte) ([]byte, error) {
	code := d.StateDB.GetCode(addr)
	contract := vm.NewContract(vm.AccountRef(common.Address{}), vm.AccountRef(addr), common.Big0, math.MaxUint64)
	contract.SetCallCode(&addr, d.StateDB.GetCodeHash(addr), code)
	ret, err := d.Interpreter().Run(contract, method, false)
	if err != nil {
		return nil, err
	}
	return ret, nil
}
