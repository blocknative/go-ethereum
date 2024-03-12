package blocknative

import (
	"encoding/json"
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
	// decoderCache is a global cache for the decoder, shared across traces.
	decoderCache = decoder.NewCaches()
)

// tracer is Blocknative's transaction tracer. It decodes messages into
// call-frames and decodes them into higher-level abstractions. It returns all
// the information required to reconstruct a transaction's execution while
// decoding inputs, outputs, logs, and environmental effects.
type tracer struct {
	opts    TracerOpts
	evm     *vm.EVM
	decoder *decoder.Decoder

	trace     Trace
	startTime time.Time
	callStack []CallFrame

	interrupt       *atomic.Bool
	interruptReason error
}

// NewTracer returns a new tracer with the given json decoded as TracerOpts.
// This allows us to easily construct a tracer from the standard tracer API
// which receives options as json.
func NewTracer(cfg json.RawMessage) (Tracer, error) {
	var opts TracerOpts
	if cfg != nil {
		if err := json.Unmarshal(cfg, &opts); err != nil {
			return nil, err
		}
	}
	return NewTracerWithOpts(opts)
}

// NewTracerWithOpts is the primary constructor for the tracer.
func NewTracerWithOpts(opts TracerOpts) (Tracer, error) {
	opts.Decode = opts.Decode || opts.BalanceChanges

	var t = tracer{
		opts:      opts,
		callStack: make([]CallFrame, 1, 4),
		interrupt: new(atomic.Bool),
	}

	if !opts.DisableBlockContext {
		t.trace.BlockContext = &BlockContext{}
	}

	return &t, nil

}

// SetStateRoot implements core.stateRootSetter and stores the given root in the
// trace's BlockContext. It's called between the constructor and the first
// call-frame.
func (t *tracer) SetStateRoot(root common.Hash) {
	if t.trace.BlockContext != nil {
		t.trace.BlockContext.StateRoot = root.Bytes()
	}
}

// CaptureStart is called before the top-level call starts.
// This is also where we get the EVM instance, so we initialize the things that
// need it here instead of the constructor.
func (t *tracer) CaptureStart(evm *vm.EVM, from common.Address, to common.Address, create bool, input []byte, gas uint64, value *big.Int) {
	t.startTime = time.Now()
	t.evm = evm
	/*
		if t.opts.Decode {
			t.decoder = decoder.New(decoderCache, decoderEVM{evm})
		}

		if !t.opts.DisableBlockContext {
			t.trace.BlockContext.Number = evm.Context.BlockNumber.Uint64()
			t.trace.BlockContext.BaseFee = evm.Context.BaseFee.Uint64()
			t.trace.BlockContext.Time = evm.Context.Time
			t.trace.BlockContext.Coinbase = evm.Context.Coinbase
			t.trace.BlockContext.GasLimit = evm.Context.GasLimit
			if evm.Context.Random != nil {
				copy(t.trace.BlockContext.Random[:], evm.Context.Random[:])
			}
		}
	*/
	// Create a call-frame for the top-level call.
	var bigValue Big
	if value != nil {
		bigValue = Big(*value)
	}
	t.callStack[0] = CallFrame{
		Type:  "CALL",
		From:  from,
		To:    to,
		Input: cloneBytes(input),
		Gas:   Uint64(gas),
		Value: bigValue,
	}
	if create {
		t.callStack[0].Type = "CREATE"
	}

	// Try adding decode information, but don't fail if we can't.
	if t.opts.Decode {
		if decoded, err := t.decoder.DecodeCallFrameStart(from, to, value, input); err == nil {
			t.callStack[0].Decoded = decoded
		}
	}
}

// CaptureEnd is called after the top-level call finishes to finalize tracing.
func (t *tracer) CaptureEnd(output []byte, gasUsed uint64, err error) {
	if err := t.finalizeCallFrame(&t.callStack[0], output, gasUsed, err); err != nil {
		log.Error("failed to finalize call frame", "err", err)
	}

	// If the user wants the logs, grab them from the state
	if t.opts.Logs {
		for _, stateLog := range t.evm.StateDB.Logs() {
			t.trace.Logs = append(t.trace.Logs, CallLog{
				Address: stateLog.Address,
				Data:    stateLog.Data,
				Topics:  stateLog.Topics,
			})
		}
	}

	// Add gas payments to balance changes
	if t.opts.Decode {
		t.decoder.CaptureGas(t.evm.TxContext.Origin, t.evm.Context.Coinbase, gasUsed, t.evm.TxContext.GasPrice, t.evm.Context.BaseFee)
	}

	// Add total time duration for this trace request
	t.trace.Time = time.Now().Sub(t.startTime).Nanoseconds()
}

// CaptureEnter is called before any new sub-call starts.
// (via call, create or selfdestruct).
func (t *tracer) CaptureEnter(typ vm.OpCode, from common.Address, to common.Address, input []byte, gas uint64, value *big.Int) {
	if t.interrupt.Load() {
		return
	}

	// Create CallFrame, decode it, and all it to the end of the callstack.
	var bigValue Big
	if value != nil {
		bigValue = Big(*value)
	}
	call := CallFrame{
		Type:  typ.String(),
		From:  from,
		To:    to,
		Input: cloneBytes(input),
		Gas:   Uint64(gas),
		Value: bigValue,
	}
	if t.opts.Decode {
		if decoded, err := t.decoder.DecodeCallFrameStart(from, to, value, input); err == nil {
			call.Decoded = decoded
		}
	}

	t.callStack = append(t.callStack, call)
}

// CaptureExit is called after any sub call ends.
func (t *tracer) CaptureExit(output []byte, gasUsed uint64, err error) {
	// Skip if we have no call-frames.
	size := len(t.callStack)
	if size == 0 {
		return
	}

	// We have a call-frame, so finalize it.
	call := t.callStack[size-1]
	if err := t.finalizeCallFrame(&call, output, gasUsed, err); err != nil {
		log.Error("failed to finalize call frame", "err", err)
		return
	}

	// We have a parent call-frame, so nest this one under it.
	if size <= 1 {
		return
	}
	// Pop our call of the stack.
	end := size - 1
	t.callStack = t.callStack[:end]
	// Append this call to the parent's calls.
	end -= 1
	t.callStack[end].Calls = append(t.callStack[end].Calls, call)
}

// Stop terminates execution of the tracer at the first opportune moment.
func (t *tracer) Stop(err error) {
	t.interrupt.Store(true)
	t.interruptReason = err
	t.evm.Cancel()
}

// GetTrace returns a Trace from the current state.
func (t *tracer) GetTrace() (*Trace, error) {
	if t.interrupt.Load() {
		return nil, t.interruptReason
	}

	t.trace.CallFrame = t.callStack[0]

	if t.opts.Decode {
		t.trace.BalanceChanges = t.decoder.GetBalanceChanges()
	}

	return &t.trace, nil
}

// GetResult returns a JSON encoded Trace from the current state.
func (t *tracer) GetResult() (json.RawMessage, error) {
	trace, err := t.GetTrace()
	if err != nil {
		return nil, err
	}

	return json.Marshal(trace)
}

func (t *tracer) finalizeCallFrame(call *CallFrame, output []byte, gasUsed uint64, err error) error {
	call.GasUsed = Uint64(gasUsed)

	// Finalize the decoding.
	if t.opts.Decode && call.Decoded != nil {
		if err := t.decoder.DecodeCallFrameEnd(call.Decoded); err != nil {
			return err
		}
	}

	// If there was an error then try decoding it and stop.
	if err != nil {
		call.Error = err.Error()
		if err.Error() == "execution reverted" && len(output) > 0 {
			call.Output = output
			revertReason, _ := abi.UnpackRevert(output)
			call.ErrorReason = revertReason
		}

		if call.Type == "CREATE" || call.Type == "CREATE2" {
			call.To = common.Address{}
		}
		return nil
	}

	call.Output = output
	return nil
}

func cloneBytes(src []byte) []byte {
	dst := make([]byte, len(src))
	copy(dst, src)
	return dst
}

// EmptyCache is for testing purposes. It clears the global cache so tests don't
// interfere with each other.
func EmptyCache() {
	decoderCache = decoder.NewCaches()
}

//
// Unused interface methods.
//

// CaptureState implements the tracer interface, but is unused.
//func (t *tracer) CaptureState(_ uint64, _ vm.OpCode, _, _ uint64, _ *vm.ScopeContext, _ []byte, _ int, _ error) {
//}

func (t *tracer) CaptureState(pc uint64, op vm.OpCode, gas, cost uint64, scope *vm.ScopeContext, rData []byte, depth int, err error) {
	// log.Debug("cs", "name", op.String(), "gas", gas, "cost", cost)
}

// CaptureFault implements the tracer interface, but is unused.
func (t *tracer) CaptureFault(_ uint64, _ vm.OpCode, _, _ uint64, _ *vm.ScopeContext, _ int, _ error) {
}

// CaptureTxStart implements the tracer interface, but is unused.
func (t *tracer) CaptureTxStart(_ uint64) {}

// CaptureTxEnd implements the tracer interface, but is unused.
func (t *tracer) CaptureTxEnd(_ uint64) {}
