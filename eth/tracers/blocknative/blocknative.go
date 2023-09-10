package blocknative

import (
	"encoding/json"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"math/big"
)

type Tracer interface {
	vm.EVMLogger
	GetTrace() (*Trace, error)
	GetResult() (json.RawMessage, error)
	Stop(err error)
}

// TracerOpts configure the tracer to save or ignore various aspects of a
// transaction execution.
type TracerOpts struct {
	Logs              bool `json:"logs"`
	NetBalanceChanges bool `json:"netBalanceChanges"`
}

// Trace contains all the accumulated details of a transaction execution.
type Trace struct {
	CallFrame
	BlockContext      BlockContext            `json:"blockContext"`
	Logs              []CallLog               `json:"logs,omitempty"`
	Time              string                  `json:"time,omitempty"`
	NetBalanceChanges []AddressBalanceChanges `json:"netBalanceChanges,omitempty"`
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
	InitialGas     uint64                  `json:"-"` // Bought gas, used to find initial bal for the from address as the buy happens before trace starts
	Pre            state                   `json:"-"` //`json:"pre"`
	Post           state                   `json:"-"` //`json:"post"`
	BalanceChanges []AddressBalanceChanges `json:"balanceChanges,omitempty"`
	Balances       balances                `json:"-"`
	Tokens         []Tokenchanges          `json:"-"`
}

type AddressBalanceChanges struct {
	Address        common.Address  `json:"address"`
	BalanceChanges []BalanceChange `json:"balanceChanges"`
}

type BalanceChange struct {
	Delta     Amount         `json:"delta"`
	Asset     *Asset         `json:"asset"`
	Breakdown []Tokenchanges `json:"breakdown"`
}

type Asset struct {
	Address common.Address `json:"contractAddress"`
	Type    accountType    `json:"type,omitempty"`
	TokenMetadata
}

type TokenMetadata struct {
	Name     string `json:"name"`
	Symbol   string `json:"symbol"`
	Decimals uint8  `json:"decimals,omitempty"`
}

type accountType int

func (t accountType) String() string {
	switch t {
	case accountTypeEOA:
		return "eoa"
	case accountTypeERC20:
		return "erc20"
	case accountTypeERC721:
		return "erc721"
	default:
		return ""
	}
}

func (t accountType) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.String())
}

const (
	accountTypeUnknown accountType = iota
	accountTypeEOA
	accountTypeERC20
	accountTypeERC721
)

type state = map[common.Address]*account

type account struct {
	Balance *big.Int `json:"balance,omitempty"`
}

type balances = map[common.Address]*valueChange

type valueChange struct {
	Eth      *big.Float `json:"eth,omitempty"`
	EthInWei Amount     `json:"ethinwei,omitempty"`
}

type Tokenchanges struct {
	From     common.Address `json:"counterparty,omitempty"`
	To       common.Address `json:"-"`
	Amount   Amount         `json:"amount,omitempty"`
	Contract common.Address `json:"-"`
	Asset    *Asset         `json:"-"`
}

type Amount struct{ *big.Int }

func (b Amount) MarshalJSON() ([]byte, error) {
	return json.Marshal(b.Int.String())
}

const (
	// eventSigTransfer is the signature for "Transfer(address,address,uint256)"
	// Which is used both by erc20 and erc721
	// erc20: from, to, value; erc721: from, to, tokenId
	eventSigTransfer = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
)
