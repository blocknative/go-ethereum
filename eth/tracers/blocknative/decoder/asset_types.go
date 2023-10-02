package decoder

import (
	"encoding/json"
	"strings"
)

const (
	AssetTypeUnknown AssetType = iota
	AssetTypeNative
	AssetTypeERC20
	AssetTypeERC721
	AssetTypeERC1155
)

type AssetType uint8

func (t AssetType) String() string {
	switch t {
	case AssetTypeNative:
		return "eth"
	case AssetTypeERC20:
		return "erc20"
	case AssetTypeERC721:
		return "erc721"
	case AssetTypeERC1155:
		return "erc1155"
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
	case "erc1155":
		*t = AssetTypeERC1155
	default:
		*t = AssetTypeUnknown
	}
	return nil
}

// AssetTypeForInterfaces returns the asset type for the given interfaces.
// A contract can implement multiple interfaces, so we check them in the order
// of precedence and take the first.
func AssetTypeForInterfaces(interfaces []Interface) AssetType {
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
