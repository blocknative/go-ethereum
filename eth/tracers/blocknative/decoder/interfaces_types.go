package decoder

import (
	"encoding/json"
	"strings"
)

const (
	interfaceTypeUnknown InterfaceType = iota
	interfaceTypeERC20
	interfaceTypeERC721
	interfaceTypeERC1155
)

type InterfaceType int

func (t InterfaceType) String() string {
	switch t {
	case interfaceTypeERC20:
		return "erc20"
	case interfaceTypeERC721:
		return "erc721"
	case interfaceTypeERC1155:
		return "erc1155"
	default:
		return ""
	}
}

func (t InterfaceType) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.String())
}

func (t *InterfaceType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch strings.ToLower(s) {
	case "erc20":
		*t = interfaceTypeERC20
	case "erc721":
		*t = interfaceTypeERC721
	case "erc1155":
		*t = interfaceTypeERC1155
	default:
		*t = interfaceTypeUnknown
	}
	return nil
}
