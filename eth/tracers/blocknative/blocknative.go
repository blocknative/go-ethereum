package blocknative

import (
	"encoding/json"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
)

// todo alex: experiment with removing these in favour of common geth usage, and tracer interface which copies this from the tracers lib
var Tracers = map[string]func(cfg json.RawMessage) (Tracer, error){
	"txnOpCodeTracer": func(cfg json.RawMessage) (Tracer, error) {
		return NewTxnOpCodeTracer(cfg)
	},
}

type Tracer interface {
	vm.EVMLogger
	GetResult() (json.RawMessage, error)
	Stop(err error)
}

// TracerOpts configure the tracer to save or ignore various aspects of a transaction execution.
type TracerOpts struct {
	Logs          bool `json:"logs"`
	NetBalChanges bool `json:"netBalChanges"`
}

// Trace contains all the accumulated details of a transaction execution.
type Trace struct {
	CallFrame
	BlockContext  BlockContext  `json:"blockContext"`
	Logs          []CallLog     `json:"logs,omitempty"`
	Time          string        `json:"time,omitempty"`
	NetBalChanges NetBalChanges `json:"netBalChanges,omitempty"`
	startTime     time.Time
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

// NetBalChanges represents the difference of value (ETH, erc20, erc721) after the transaction for all addresses
type NetBalChanges struct {
	InitialGas uint64         `json:"-"` // Bought gas, used to find initial bal for the from address as the buy happens before trace starts
	Pre        state          `json:"-"` //`json:"pre"`
	Post       state          `json:"-"` //`json:"post"`
	Balances   balances       `json:"balances,omitempty"`
	Tokens     []Tokenchanges `json:"tokenchanges,omitempty"`
}

type difference = map[common.Address]*valueChanges

type valueChanges struct {
	Eth      *big.Float      `json:"eth,omitempty"` // TODO ALEX: this term ETH may need to be generalised
	EthInWei *big.Int        `json:"ethinwei,omitempty"`
	Erc20    placeholderType `json:"erc20,omitempty"`
	Erc721   placeholderType `json:"erc721,omitempty"`
}

// TODO ALEX: expand on these types (types of tokens to track, erc20, erc721, etc...)
type placeholderType interface{}

type state = map[common.Address]*account

type account struct {
	Balance *big.Int                    `json:"balance,omitempty"`
	Code    []byte                      `json:"code,omitempty"`
	Storage map[common.Hash]common.Hash `json:"storage,omitempty"`
}

type balances = map[common.Address]*valueChange

type valueChange struct {
	Eth      *big.Float `json:"eth,omitempty"`
	EthInWei *big.Int   `json:"ethinwei,omitempty"`
}

type Tokenchanges struct {
	From     common.Address `json:"from,omitempty"`
	To       common.Address `json:"to,omitempty"`
	Asset    *big.Int       `json:"asset,omitempty"`
	Contract common.Address `json:"contractAddress,omitempty"`
}

// This event signiture hash is constant for "Transfer(address,address,uint256)"
// Which is used both by erc20 and erc721
// erc20: from, to, value; erc721: from, to, tokenId
const transferEventHex = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
