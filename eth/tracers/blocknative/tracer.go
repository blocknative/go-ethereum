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

	thash   common.Hash // transaction has
	txIndex int         // transaction index

	trace     Trace
	startTime time.Time
	callStack []CallFrame

	interrupt       *atomic.Bool
	interruptReason error
}

// NewTracerWithOpts is the primary constructor for the tracer.
func NewBlocknativeTracerWithOpts(opts TracerOpts) (*Tracer, error) {
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

func (t *Tracer) SetTxContext(thash common.Hash, ti int) {
	t.thash = thash
	t.txIndex = ti
}

// SetStateRoot implements core.stateRootSetter and stores the given root in the
// trace's BlockContext. It's called between the constructor and the first
// call-frame.
func (t *Tracer) SetStateRoot(root common.Hash) {
	if t.trace.BlockContext != nil {
		t.trace.BlockContext.StateRoot = root.Bytes()
	}
}

func (t *Tracer) Hooks() *tracing.Hooks {
	return &tracing.Hooks{
		OnTxStart: t.onTxStart,
		OnTxEnd:   t.onTxEnd,
		OnEnter:   t.onEnter,
		OnExit:    t.onExit,
		OnLog:     t.onLog,
	}
}

// Stop terminates execution of the Tracer at the first opportune moment.
func (t *Tracer) Stop(err error) {
	t.interrupt.Store(true)
	t.interruptReason = err
	t.evm.Cancel()
}

// GetTrace returns a Trace from the current state.
func (t *Tracer) GetTrace() (*Trace, error) {
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
func (t *Tracer) GetResult() (json.RawMessage, error) {
	trace, err := t.GetTrace()
	if err != nil {
		return nil, err
	}

	return json.Marshal(trace)
}

func (t *Tracer) onTxStart(env *tracing.VMContext, tx *types.Transaction, from common.Address) {
}

func (t *Tracer) onTxEnd(receipt *types.Receipt, err error) {
}

func (t *Tracer) onLog(log *types.Log) {
	// Only logs need to be captured via opcode processing
	if !t.opts.Logs {
		return
	}

	// Skip if tracing was interrupted
	if t.interrupt.Load() {
		return
	}

	// TODO: Make this work to replace our log gathering
	//l := callLog{
	//	Address:  log.Address,
	//	Topics:   log.Topics,
	//	Data:     log.Data,
	//	Position: hexutil.Uint(len(t.callstack[len(t.callstack)-1].Calls)),
	//}
	//t.callstack[len(t.callstack)-1].Logs = append(t.callstack[len(t.callstack)-1].Logs, l)
}

// onEnter is called when EVM enters a new scope (via call, create or selfdestruct).
func (t *Tracer) onEnter(depth int, typ byte, from common.Address, to common.Address, input []byte, gas uint64, value *big.Int) {
	if t.interrupt.Load() {
		return
	}

	if depth == 0 {
		t.captureStart(t.evm, from, to, vm.OpCode(typ) == vm.CREATE, input, gas, value)
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

	t.captureExit(output, gasUsed, err)
}

// captureStart is called before the top-level call starts.
// This is also where we get the EVM instance, so we initialize the things that
// need it here instead of the constructor.
func (t *Tracer) captureStart(evm *vm.EVM, from common.Address, to common.Address, create bool, input []byte, gas uint64, value *big.Int) {
	t.startTime = time.Now()
	t.evm = evm

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
	if err := t.finalizeCallFrame(&t.callStack[0], output, gasUsed, err); err != nil {
		log.Error("failed to finalize call frame", "err", err)
	}

	// Add gas payments to balance changes iff the tx succeeded.
	if err == nil && t.opts.Decode {
		t.decoder.CaptureGas(t.evm.TxContext.Origin, t.evm.Context.Coinbase, gasUsed, t.evm.TxContext.GasPrice, t.evm.Context.BaseFee)
	}

	// If the user wants the logs, grab them from the state
	if t.opts.Logs {
		if t.opts.PerHashLogs {
			for _, stateLog := range t.evm.StateDB.GetLogs(t.thash, 0, common.Hash{}) {
				t.trace.Logs = append(t.trace.Logs, CallLog{
					Address: stateLog.Address,
					Data:    stateLog.Data,
					Topics:  stateLog.Topics,
				})
			}
		} else {
			for _, stateLog := range t.evm.StateDB.Logs() {
				t.trace.Logs = append(t.trace.Logs, CallLog{
					Address: stateLog.Address,
					Data:    stateLog.Data,
					Topics:  stateLog.Topics,
				})
			}
		}
	}

	// Add total time duration for this trace request
	t.trace.Time = time.Since(t.startTime).Nanoseconds()
}

// captureExit is called after any sub call ends.
func (t *Tracer) captureExit(output []byte, gasUsed uint64, err error) {
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

func (t *Tracer) finalizeCallFrame(call *CallFrame, output []byte, gasUsed uint64, err error) error {
	call.GasUsed = Uint64(gasUsed)

	// Finalize the decoding.
	if t.opts.Decode && call.Decoded != nil {
		if err := t.decoder.DecodeCallFrameEnd(call.Decoded); err != nil {
			return err
		}
	}

	// If there was an error then try decoding it and Stop.
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
