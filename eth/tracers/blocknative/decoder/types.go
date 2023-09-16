package decoder

import (
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
)

type CallFrame struct {
	*Contract
	*CallData
}

type Contract struct {
	Type       ContractType `json:"type,omitempty"`
	interfaces []Interface
}

type CallData struct {
	MethodID  MethodID    `json:"methodID"`
	Args      []string    `json:"args,omitempty"`
	Transfers []*Transfer `json:"-"`
}

type Transfer struct {
	Asset   *Asset         `json:"asset,omitempty"`
	From    common.Address `json:"from"`
	To      common.Address `json:"to"`
	Value   *Amount        `json:"value"`
	TokenID *big.Int       `json:"tokenID,omitempty"`
}

type Asset struct {
	AssetID
	*AssetMetadata
}

type AssetID struct {
	Address common.Address `json:"address"`
	TokenID *big.Int       `json:"tokenID,omitempty"`
}

type AssetMetadata struct {
	Type     AssetType `json:"type,omitempty"`
	Name     string    `json:"name,omitempty"`
	Symbol   string    `json:"symbol,omitempty"`
	Decimals uint8     `json:"decimals,omitempty"`
	URI      string    `json:"uri,omitempty"`
}

type ContractType uint8

func (ct ContractType) String() string {
	return contractTypes[ct]
}

func (ct ContractType) MarshalJSON() ([]byte, error) {
	return json.Marshal(ct.String())
}

func (ct *ContractType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*ct = contractTypesByName[s]
	return nil
}

const (
	ContractTypeUnknown ContractType = iota
	ContractTypeERC20
	ContractTypeERC721
	ContractTypeERC1155
	ContractTypeOPStack
)

var (
	contractTypes = map[ContractType]string{
		ContractTypeUnknown: "",
		ContractTypeERC20:   "erc20",
		ContractTypeERC721:  "erc721",
		ContractTypeERC1155: "erc1155",
		ContractTypeOPStack: "op_stack",
	}
	contractTypesByName = reverseMap(contractTypes)
)

func reverseMap[K comparable, V comparable](in map[K]V) map[V]K {
	out := make(map[V]K, len(in))
	for k, v := range in {
		out[v] = k
	}
	return out
}

type Amount big.Int

func NewAmount(i *big.Int) *Amount {
	return (*Amount)(i)
}

func (a *Amount) SetBytes(b []byte) {
	*a = *(*Amount)(big.NewInt(0).SetBytes(b))
}

func (a *Amount) Add(x *Amount, y *Amount) {
	a.ToInt().Add(x.ToInt(), y.ToInt())
}

func (a *Amount) AddInt(x *Amount, y *big.Int) {
	a.ToInt().Add(x.ToInt(), y)
}

func (a *Amount) Neg() *Amount {
	neg := new(big.Int).Neg(a.ToInt())
	return NewAmount(neg)
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
