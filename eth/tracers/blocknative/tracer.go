package blocknative

import (
	"encoding/json"
	"math/big"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/eth/tracers/blocknative/decoder"
	"github.com/ethereum/go-ethereum/log"
)

var (
	// decoderCache is a global cache for the decoder, shared across traces.
	decoderCache = decoder.NewCaches()
)

// Tracer is Blocknative's transaction Tracer. It decodes messages into
// call-frames and decodes them into higher-level abstractions. It returns all
// the information required to reconstruct a transaction's execution while
// decoding inputs, outputs, logs, and environmental effects.
type Tracer struct {
	opts    TracerOpts
	evm     *vm.EVM
	decoder *decoder.Decoder

	vmCtx  *tracing.VMContext
	origin common.Address
	tx     *types.Transaction

	trace     Trace
	startTime time.Time
	callStack []CallFrame

	interrupt       *atomic.Bool
	interruptReason error
}

// NewTracer is the primary constructor for the tracer.
func NewTracer(opts TracerOpts) (*Tracer, error) {
	opts.Decode = opts.Decode || opts.BalanceChanges

	var t = Tracer{
		opts:      opts,
		callStack: make([]CallFrame, 1, 4),
		interrupt: new(atomic.Bool),
	}

	if !opts.DisableBlockContext {
		t.trace.BlockContext = &BlockContext{}
	}

	return &t, nil
}

func NewTracerFromJSON(raw json.RawMessage) (*Tracer, error) {
	var opts TracerOpts
	if raw != nil {
		if err := json.Unmarshal(raw, &opts); err != nil {
			return nil, err
		}
	}

	t, err := NewTracer(opts)
	if err != nil {
		return nil, err
	}

	return t, nil
}

func (t *Tracer) Hooks() *tracing.Hooks {
	h := &tracing.Hooks{
		OnTxStart: t.onTxStart,
		OnTxEnd:   t.onTxEnd,
		OnEnter:   t.onEnter,
		OnExit:    t.onExit,
		OnLog:     t.onLog,

		BlockNativeInitHook: t.blockNativeInitHook,
	}

	return h
}

// Stop terminates execution of the Tracer at the first opportune moment.
func (t *Tracer) Stop(err error) {
	t.interrupt.Store(true)
	t.interruptReason = err
}

// GetTrace returns a Trace from the current state.
func (t *Tracer) GetTrace() (*Trace, error) {
	if t.interrupt.Load() {
		return nil, t.interruptReason
	}

	t.trace.CallFrame = t.callStack[0]

	if t.opts.BalanceChanges {
		t.trace.BalanceChanges = t.decoder.GetBalanceChanges()
	}

	return &t.trace, nil
}

// GetResult returns a JSON encoded Trace from the current state.
func (t *Tracer) GetResult() (json.RawMessage, error) {
	trace, err := t.GetTrace()
	if err != nil {
		return nil, err
	}

	return json.Marshal(trace)
}

func (t *Tracer) blockNativeInitHook(evmInt interface{}) {
	evm, ok := evmInt.(*vm.EVM)
	if !ok {
		log.Error("blocknative: invalid EVM instance passed to BlockNativeInitHook")
		return
	}

	t.evm = evm
}

func (t *Tracer) onTxStart(vmCtx *tracing.VMContext, tx *types.Transaction, from common.Address) {
	t.vmCtx = vmCtx
	t.origin = from
	t.tx = tx
}

func (t *Tracer) onTxEnd(receipt *types.Receipt, _ error) {
	// If the transaction reverts we don't get a receipt.
	if receipt != nil {
		t.trace.GasUsed = Uint64(receipt.GasUsed)
	}
}

// onLog is called when a log is emitted.
func (t *Tracer) onLog(log *types.Log) {
	if !t.opts.Logs || (t.opts.PerHashLogs && log.TxHash != t.tx.Hash()) {
		return
	}
	t.trace.Logs = append(t.trace.Logs, CallLog{
		Address: log.Address,
		Data:    log.Data,
		Topics:  log.Topics,
	})
}

// onEnter is called when EVM enters a new scope (via call, create or selfdestruct).
func (t *Tracer) onEnter(depth int, typ byte, from common.Address, to common.Address, input []byte, gas uint64, value *big.Int) {
	if t.interrupt.Load() {
		return
	}

	if depth == 0 {
		t.captureStart(from, to, vm.OpCode(typ) == vm.CREATE, input, gas, value)
		return
	}

	t.captureEnter(vm.OpCode(typ), from, to, input, gas, value)
}

// onExit is called when EVM exits a scope, even if the scope didn't
// execute any code.
func (t *Tracer) onExit(depth int, output []byte, gasUsed uint64, err error, reverted bool) {
	if depth == 0 {
		t.captureEnd(output, gasUsed, err, reverted)
		return
	}

	t.captureExit(output, gasUsed, err, reverted)
}

// captureStart is called before the top-level call starts.
// We use it to initialize values now that we've received the vmCtx and evm.
func (t *Tracer) captureStart(from common.Address, to common.Address, create bool, input []byte, gas uint64, value *big.Int) {
	t.startTime = time.Now()

	if t.opts.Decode {
		t.decoder = decoder.New(decoderCache, decoderEVM{t.evm})
	}

	if !t.opts.DisableBlockContext {
		t.trace.BlockContext = &BlockContext{}

		t.trace.BlockContext.Number = t.vmCtx.BlockNumber.Uint64()
		t.trace.BlockContext.Time = t.vmCtx.Time
		t.trace.BlockContext.Coinbase = t.vmCtx.Coinbase
		t.trace.BlockContext.StateRoot = t.vmCtx.StateDB.IntermediateRoot(false).Bytes()

		t.trace.BlockContext.GasLimit = t.evm.Context.GasLimit
		t.trace.BlockContext.BaseFee = t.evm.Context.BaseFee.Uint64()
		if t.evm.Context.Random != nil {
			copy(t.trace.BlockContext.Random[:], t.vmCtx.Random[:])
		}
	}

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

// captureEnter is called before any new sub-call starts.
// (via call, create or selfdestruct).
func (t *Tracer) captureEnter(typ vm.OpCode, from common.Address, to common.Address, input []byte, gas uint64, value *big.Int) {
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

// captureEnd is called after the top-level call finishes to finalize tracing.
func (t *Tracer) captureEnd(output []byte, gasUsed uint64, err error, reverted bool) {
	if err := t.finalizeCallFrame(&t.callStack[0], output, gasUsed, err, reverted); err != nil {
		log.Error("failed to finalize call frame", "err", err)
	}

	// Add gas payments to balance changes iff the tx succeeded.
	if err == nil && !reverted && t.opts.Decode {
		t.decoder.CaptureGas(t.origin, t.vmCtx.Coinbase, gasUsed, t.vmCtx.GasPrice, t.evm.Context.BaseFee)

	}

	// Add total time duration for this trace request
	t.trace.Time = time.Since(t.startTime).Nanoseconds()
}

// captureExit is called after any sub call ends.
func (t *Tracer) captureExit(output []byte, gasUsed uint64, err error, reverted bool) {
	// Skip if we have no call-frames.
	size := len(t.callStack)
	if size == 0 {
		return
	}

	// We have a call-frame, so finalize it.
	call := t.callStack[size-1]
	if err := t.finalizeCallFrame(&call, output, gasUsed, err, reverted); err != nil {
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

func (t *Tracer) finalizeCallFrame(call *CallFrame, output []byte, gasUsed uint64, err error, reverted bool) error {
	// If there was an error or revert then handle it right away and then stop.
	if err != nil {
		call.Error = err.Error()
	} else if reverted {
		call.Error = "execution reverted"
	}

	if err != nil || reverted {
		if len(output) > 0 {
			call.Output = output
			revertReason, _ := abi.UnpackRevert(output)
			call.ErrorReason = revertReason
		}

		if call.Type == "CREATE" || call.Type == "CREATE2" {
			call.To = common.Address{}
		}
		return nil
	}

	// Finalize the decoding.
	call.GasUsed = Uint64(gasUsed)
	if t.opts.Decode && call.Decoded != nil {
		if err := t.decoder.DecodeCallFrameEnd(call.Decoded); err != nil {
			return err
		}
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
