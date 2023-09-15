package blocknative

import (
	"encoding/json"
	"fmt"
	"math/big"

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
	BalanceChanges bool `json:"balanceChanges"`

	// DisableBlockContext disables the block context in the trace.
	// The negative is used so that the empty value maintains legacy behavior.
	DisableBlockContext bool `json:"disableBlockContext"`
}

// Trace contains all the accumulated details of a transaction execution.
type Trace struct {
	CallFrame
	BlockContext   *BlockContext     `json:"blockContext,omitempty"`
	Logs           []CallLog         `json:"logs,omitempty"`
	Time           string            `json:"time,omitempty"`
	BalanceChanges NetBalanceChanges `json:"balanceChanges"`
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
	Type        string      `json:"type"`
	From        string      `json:"from"`
	To          string      `json:"to,omitempty"`
	Value       string      `json:"value,omitempty"`
	Gas         string      `json:"gas"`
	GasUsed     string      `json:"gasUsed"`
	Input       string      `json:"input"`
	Output      string      `json:"output,omitempty"`
	Error       string      `json:"error,omitempty"`
	ErrorReason string      `json:"errorReason,omitempty"`
	Calls       []CallFrame `json:"calls,omitempty"`
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

// NetBalanceChanges is a list of account balance changes.
type NetBalanceChanges []AccountBalanceChanges

// AccountBalanceChanges is a list of balance changes for a single account.
type AccountBalanceChanges struct {
	Address        common.Address  `json:"address"`
	BalanceChanges []BalanceChange `json:"balanceChanges"`
}

// BalanceChange is a change in an account's balance for a single asset.
type BalanceChange struct {
	Delta     *Amount              `json:"delta"`
	Asset     *decoder.Asset       `json:"asset"`
	Breakdown []AssetTransferEvent `json:"breakdown"`
}

// AssetTransferEvent is a single transfer of an asset.
type AssetTransferEvent struct {
	Counterparty common.Address `json:"counterparty"`
	Amount       *Amount        `json:"amount"`
}

type Amount big.Int

func NewAmount(i *big.Int) *Amount {
	return (*Amount)(i)
}

func (a *Amount) Add(x *Amount, y *big.Int) {
	a.ToInt().Add(x.ToInt(), y)
}

func (a *Amount) ToInt() *big.Int {
	return (*big.Int)(a)
}

func (a *Amount) String() string {
	return a.ToInt().String()
}

func (a *Amount) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.ToInt().String())
}

func (a *Amount) UnmarshalJSON(data []byte) error {
	if data == nil || len(data) == 0 {
		*a = *NewAmount(big.NewInt(0))
		return nil
	}

	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	aInt, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return fmt.Errorf("failed to convert string to Amount")
	}
	*a = *(*Amount)(aInt)
	return nil
}
