package decoder

import (
	"encoding/json"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
	"strings"
)

type Asset struct {
	Address common.Address `json:"address,omitempty"`
	Type    AssetType      `json:"type"`
	TokenID *big.Int       `json:"TokenID,omitempty"`
	TokenMetadata
}

type TokenMetadata struct {
	Name     string `json:"name"`
	Symbol   string `json:"symbol"`
	Decimals uint8  `json:"decimals,omitempty"`
}

const (
	AssetTypeUnknown AssetType = iota
	AssetTypeNative
	AssetTypeERC20
	AssetTypeERC721
	AssetTypeERC1155
)

type AssetType int

func (t AssetType) String() string {
	switch t {
	case AssetTypeNative:
		return "eth"
	case AssetTypeERC20:
		return "erc20"
	case AssetTypeERC721:
		return "erc721"
	case AssetTypeUnknown:
		return "unknown"
	default:
		return ""
	}
}

func (t AssetType) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.String())
}

func (t *AssetType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch strings.ToLower(s) {
	case "eth":
		*t = AssetTypeNative
	case "erc20":
		*t = AssetTypeERC20
	case "erc721":
		*t = AssetTypeERC721
	default:
		*t = AssetTypeUnknown
	}
	return nil
}

// AssetTypeForInterfaces returns the asset type for the given interfaces.
func AssetTypeForInterfaces(interfaces []InterfaceType) AssetType {
	for _, i := range interfaces {
		switch i {
		case interfaceTypeERC1155:
			return AssetTypeERC1155
		case interfaceTypeERC20:
			return AssetTypeERC20
		case interfaceTypeERC721:
			return AssetTypeERC721
		}
	}
	return AssetTypeUnknown
}
