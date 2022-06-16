package native

import (
	"encoding/json"
	"fmt"
	"math/big"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/eth/tracers"
	"github.com/ethereum/go-ethereum/log"
)

func init() {
	register("txnOpCodeTracer", newtxnOpCodeTracer)
}

type callFrameBN struct {
	Type    string        `json:"type"`
	From    string        `json:"from"`
	To      string        `json:"to,omitempty"`
	Value   string        `json:"value,omitempty"`
	Gas     string        `json:"gas"`
	GasUsed string        `json:"gasUsed"`
	Input   string        `json:"input"`
	Output  string        `json:"output,omitempty"`
	Error   string        `json:"error,omitempty"`
	Calls   []callFrameBN `json:"calls,omitempty"`

	// Added fields from 'callFrame' in 'call.go'
	gasIn   uint64
	gasCost uint64
	Time    string `json:"time,omitempty"`

	// TODO ALEX: Investigate precompiles usage, could reduce latency as they are "those are just fancy opcodes" - 4byte.go
	// activePrecompiles []common.Address // Updated on CaptureStart based on given rules

}

// txnOpCodeTracer is a go implementation of the Tracer interface which
// only returns a smilled trace of a transaction consisting of transaction
// op codes and relevant gas data.
// This is intended for Blocknative usage.
type txnOpCodeTracer struct {
	env       *vm.EVM       // TODO ALEX: is this used when going step by step?
	callStack []callFrameBN // TODO ALEX: this is being imported from call.go, which should be fine!
	interrupt uint32        // Atomic flag to signal execution interruption
	reason    error         // Textual reason for the interruption
}

// newtxnOpCodeTracer returns a new txnOpCodeTracer tracer.
func newtxnOpCodeTracer(ctx *tracers.Context) tracers.Tracer {
	// First callframe contains tx context info
	// and is populated on start and end.
	return &txnOpCodeTracer{callStack: make([]callFrameBN, 1)}
}

// CaptureStart implements the EVMLogger interface to initialize the tracing operation.
func (t *txnOpCodeTracer) CaptureStart(env *vm.EVM, from common.Address, to common.Address, create bool, input []byte, gas uint64, value *big.Int) {
	t.env = env
	// This is the initial call
	t.callStack[0] = callFrameBN{
		Type:  "CALL",
		From:  addrToHex(from),
		To:    addrToHex(to),
		Input: bytesToHex(input),
		Gas:   uintToHex(gas),
		Value: bigToHex(value),
	}
	if create {
		fmt.Println("DEBUG | found CREATE on CaptureStart")
		t.callStack[0].Type = "CREATE"
	}

	// TODO ALEX: Look at `Compute intrinsic gas` line in Js tracer, understand how gas changed and if current tracing deals with it!
}

// CaptureEnd is called after the call finishes to finalize the tracing.
func (t *txnOpCodeTracer) CaptureEnd(output []byte, gasUsed uint64, time time.Duration, err error) {
	// Collect final gasUsed
	// TODO ALEX: Add metrics to bn-server for all tracers behind config flag for datat team to analyze this!
	t.callStack[0].GasUsed = uintToHex(gasUsed)

	// Add total time duration for this trace request
	// TODO ALEX: Double check this time here!
	t.callStack[0].Time = fmt.Sprintf("%v", time)

	// This is the final output of a call
	// TODO QUESTION: Is this the outmost layer call final, or is this needed for nested calls?
	// TODO ENHANCE: This isn't required for Blocknative purposes, could slim this down by cutting
	if err != nil {
		t.callStack[0].Error = err.Error()
		if err.Error() == "execution reverted" && len(output) > 0 {
			t.callStack[0].Output = bytesToHex(output)
		}
	} else {
		t.callStack[0].Output = bytesToHex(output)
	}
}

// CaptureState implements the EVMLogger interface to trace a single step of VM execution.
// TODO: NOT IMPLEMENTED
func (t *txnOpCodeTracer) CaptureState(pc uint64, op vm.OpCode, gas, cost uint64, scope *vm.ScopeContext, rData []byte, depth int, err error) {
	// TODO ALEX: this is where we log specific op codes!

	defer func() {
		if r := recover(); r != nil {
			// TODO ALEX: Ensure this `depth` is indeed correct! Not sure as of how to do this yet!
			// It used to be tracer.i() here, which is `len(tracer.callStack) - 1` so be careful future me!
			t.callStack[depth].Error = "internal failure"
			log.Warn("Panic during trace. Recovered.", "err", r)
		}
	}()

	// TODO ALEX: Here I must do a switch statement on each op code we are interested in
	// Op codes we like at BN, on different lines indicating different handling (via AusIV tracer)
	// CREATE, CREATE2
	// SELFDESTRUCT
	// CALL, CALLCODE, DELEGATECALL, STATICCALL
	// REVERT

	// TODO ALEX: look up other op codes we may want to use, there might be some in the CaptureState for prestate.go?
}

// CaptureFault implements the EVMLogger interface to trace an execution fault.
// TODO: NOT IMPLEMENTED
func (t *txnOpCodeTracer) CaptureFault(pc uint64, op vm.OpCode, gas, cost uint64, _ *vm.ScopeContext, depth int, err error) {
}

// CaptureEnter is called when EVM enters a new scope (via call, create or selfdestruct).
// TODO: NOT IMPLEMENTED
func (t *txnOpCodeTracer) CaptureEnter(typ vm.OpCode, from common.Address, to common.Address, input []byte, gas uint64, value *big.Int) {
	// Skip if tracing was interrupted
	if atomic.LoadUint32(&t.interrupt) > 0 {
		t.env.Cancel()
		return
	}

	// TODO ALEX: Ensure this is not doubling up on information from a possible other capture function
	// Where is this exactly being called?
	call := callFrameBN{
		Type:  typ.String(),
		From:  addrToHex(from),
		To:    addrToHex(to),
		Input: bytesToHex(input),
		Gas:   uintToHex(gas),
		Value: bigToHex(value),
	}
	t.callStack = append(t.callStack, call)
}

// CaptureExit is called when EVM exits a scope, even if the scope didn't
// execute any code.
// TODO: NOT IMPLEMENTED
func (t *txnOpCodeTracer) CaptureExit(output []byte, gasUsed uint64, err error) {
	// TODO ALEX: Here in call.go, we pop the call from the stack
	// Does this delete a history of calls? That's what we want though?

	// TODO ALEX: double check all the logic here!!! None has been proof read!
	size := len(t.callStack)
	if size <= 1 {
		return
	}
	// pop call
	call := t.callStack[size-1]
	t.callStack = t.callStack[:size-1]
	size -= 1

	call.GasUsed = uintToHex(gasUsed)
	if err == nil {
		call.Output = bytesToHex(output)
	} else {
		call.Error = err.Error()
		if call.Type == "CREATE" || call.Type == "CREATE2" {
			call.To = ""
		}
	}
	t.callStack[size-1].Calls = append(t.callStack[size-1].Calls, call)
}

// TODO: NOT IMPLEMENTED
func (*txnOpCodeTracer) CaptureTxStart(gasLimit uint64) {
	// TODO ALEX: Does the gasLimit need to be attached here, I think it is already there?
}

// TODO: NOT IMPLEMENTED
// TODO ALEX: Might not need this, check where in geth we may ever need to call this!
func (*txnOpCodeTracer) CaptureTxEnd(restGas uint64) {}

// GetResult returns an empty json object.
func (t *txnOpCodeTracer) GetResult() (json.RawMessage, error) {
	// TODO ALEX: Ensure this result is correct, place a bunch of fmt.Println("DEBUG | ")
	res, err := json.Marshal(t.callStack[0])
	if err != nil {
		return nil, err
	}
	return json.RawMessage(res), t.reason
}

// Stop terminates execution of the tracer at the first opportune moment.
func (t *txnOpCodeTracer) Stop(err error) {
	t.reason = err
	atomic.StoreUint32(&t.interrupt, 1)
}
