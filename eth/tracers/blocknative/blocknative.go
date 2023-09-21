package blocknative

import (
	"encoding/json"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/eth/tracers/blocknative/decoder"
)

// Tracer is the interface for the Blocknative tracer.
// It implements the standard EVMLogger tracer interface, but also exposes the
// resulting Trace object directly.
type Tracer interface {
	vm.EVMLogger
	GetTrace() (*Trace, error)
	GetResult() (json.RawMessage, error)
	Stop(err error)
}

// TracerOpts configure the tracer to save or ignore various aspects of a transaction execution.
type TracerOpts struct {
	Logs           bool `json:"logs"`
	Decode         bool `json:"decode"`
	BalanceChanges bool `json:"balanceChanges"`

	// DisableBlockContext disables the block context in the trace.
	// The negative is used so that the empty value maintains legacy behavior.
	DisableBlockContext bool `json:"disableBlockContext"`
}

// Trace contains all the accumulated details of a transaction execution.
type Trace struct {
	CallFrame
	BlockContext   *BlockContext             `json:"blockContext,omitempty"`
	Logs           []CallLog                 `json:"logs,omitempty"`
	Time           int64                     `json:"time,omitempty"`
	BalanceChanges decoder.NetBalanceChanges `json:"balanceChanges"`
}

// BlockContext contains information about the block we simulate transactions in.
type BlockContext struct {
	Number    uint64         `json:"number"`
	BaseFee   uint64         `json:"baseFee"`
	GasLimit  uint64         `json:"gasLimit"`
	Time      uint64         `json:"time"`
	Coinbase  common.Address `json:"coinbase"`
	StateRoot common.Hash    `json:"stateRoot"`
	Random    common.Hash    `json:"random,omitempty"`
}

type CallFrame struct {
	Type    string             `json:"type"`
	From    common.Address     `json:"from"`
	To      common.Address     `json:"to,omitempty"`
	Value   hexutil.Big        `json:"value,omitempty"`
	Gas     hexutil.Uint64     `json:"gas"`
	GasUsed hexutil.Uint64     `json:"gasUsed"`
	Input   hexutil.Bytes      `json:"input"`
	Output  hexutil.Bytes      `json:"output,omitempty"`
	Calls   []CallFrame        `json:"calls,omitempty"`
	Decoded *decoder.CallFrame `json:"decoded,omitempty"`

	Error       string `json:"error,omitempty"`
	ErrorReason string `json:"errorReason,omitempty"`
}

// CallLog represents a single log entry from the receipt of a transaction.
type CallLog struct {
	// Address is the address of the contract that emitted the log.
	Address common.Address `json:"address"`

	// Data is the encoded memory provided with the log.
	Data hexutil.Bytes `json:"data"`

	// Topics is a slice of up to 4 32byte words provided with the log.
	Topics []common.Hash `json:"topics"`
}
