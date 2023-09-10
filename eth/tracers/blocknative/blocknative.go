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
	Logs           bool `json:"logs"`
	BalanceChanges bool `json:"balanceChanges"`
}

// Trace contains all the accumulated details of a transaction execution.
type Trace struct {
	CallFrame
	BlockContext      BlockContext      `json:"blockContext"`
	Logs              []CallLog         `json:"logs,omitempty"`
	Time              string            `json:"time,omitempty"`
	NetBalanceChanges NetBalanceChanges `json:"netBalanceChanges"`
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

type NetBalanceChanges []AddressBalanceChanges

type AddressBalanceChanges struct {
	Address        common.Address       `json:"address"`
	BalanceChanges []AssetBalanceChange `json:"balanceChanges"`
}

type AssetBalanceChange struct {
	Delta     Amount          `json:"delta"`
	Asset     *Asset          `json:"asset"`
	Breakdown []assetTransfer `json:"breakdown"`
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

type assetType int

func (t assetType) String() string {
	switch t {
	case assetTypeEther:
		return "eoa"
	case assetTypeERC20:
		return "erc20"
	case assetTypeERC721:
		return "erc721"
	default:
		return ""
	}
}

func (t assetType) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.String())
}

const (
	assetTypeEther = iota
	assetTypeERC20
	assetTypeERC721
)

type accountSnapshotsMap = map[common.Address]*accountSnapshot

type accountSnapshot struct {
	balance *big.Int
}

type amountsMap = map[common.Address]Amount

type assetTransfer struct {
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
