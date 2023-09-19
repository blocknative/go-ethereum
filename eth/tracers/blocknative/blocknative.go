package blocknative

import (
	"encoding/json"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/eth/tracers/blocknative/decoder"
)

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
	Time           string                    `json:"time,omitempty"`
	BalanceChanges decoder.NetBalanceChanges `json:"balanceChanges"`
}

// BlockContext contains information about the block we simulate transactions in.
type BlockContext struct {
	Number    uint64 `json:"number"`
	StateRoot string `json:"stateRoot,omitempty"`
	BaseFee   uint64 `json:"baseFee"`
	Time      uint64 `json:"time"`
	Coinbase  string `json:"coinbase"`
	GasLimit  uint64 `json:"gasLimit"`
	Random    string `json:"random,omitempty"`
}

type CallFrame struct {
	Type        string             `json:"type"`
	From        string             `json:"from"`
	To          string             `json:"to,omitempty"`
	Value       uint64             `json:"value,omitempty"`
	Gas         uint64             `json:"gas"`
	GasUsed     uint64             `json:"gasUsed"`
	Input       string             `json:"input"`
	Output      string             `json:"output,omitempty"`
	Error       string             `json:"error,omitempty"`
	ErrorReason string             `json:"errorReason,omitempty"`
	Calls       []CallFrame        `json:"calls,omitempty"`
	Decoded     *decoder.CallFrame `json:"decoded,omitempty"`
}

// CallLog represents a single log entry from the receipt of a transaction.
type CallLog struct {
	// Address is the address of the contract that emitted the log.
	Address common.Address `json:"address"`

	// Data is the encoded memory provided with the log.
	Data string `json:"data"`

	// Topics is a slice of up to 4 32byte words provided with the log.
	Topics []common.Hash `json:"topics"`
}
