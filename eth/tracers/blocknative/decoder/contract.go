package decoder

import (
	"bytes"

	"golang.org/x/exp/slices"
)

// decodeContract decodes the contract bytecode and determines all interfaces
func decodeContract(c *Contract, bytecode ByteCode) {
	c.interfaces = bytecode.DecodeInterfaces()
	c.Type = contractTypeForInterfaces(c.interfaces)
}

// IsERC20 returns true iff the contract interfaces contains ERC-20.
func (c *Contract) IsERC20() bool {
	return slices.Contains(c.interfaces, interfaceTypeERC20)
}

// IsERC721 returns true iff the contract interfaces contains ERC-721.
func (c *Contract) IsERC721() bool {
	return slices.Contains(c.interfaces, interfaceTypeERC721)
}

// IsERC1155 returns true iff the contract interfaces contains ERC-1155.
func (c *Contract) IsERC1155() bool {
	return slices.Contains(c.interfaces, interfaceTypeERC1155)
}

type ByteCode []byte

func containsMethod(b ByteCode, id FourByteID) bool {
	if id[0] == 0 {
		id = id[1:]
	}
	if id[0] == 0 {
		id = id[1:]
	}

	if !bytes.Contains(b, id) {
		return false
	}

	return true
}

// containsAllMethods returns true iff all ids are found in the ByteCode.
func containsAllMethods(b ByteCode, ids ...FourByteID) bool {
	for _, id := range ids {
		if !containsMethod(b, id) {
			return false
		}
	}
	return true
}

// DecodeInterfaces determines all interfaces implemented by a contract.
func (b ByteCode) DecodeInterfaces() []Interface {
	var interfaces []Interface

	if containsAllMethods(b, interfaceMethodsERC20...) {
		interfaces = append(interfaces, interfaceTypeERC20)
	}
	if containsAllMethods(b, interfaceMethodsERC721...) {
		interfaces = append(interfaces, interfaceTypeERC721)
	}
	if containsAllMethods(b, interfaceMethodsERC1155...) {
		interfaces = append(interfaces, interfaceTypeERC1155)
	}
	return interfaces
}

func contractTypeForInterfaces(interfaces []Interface) ContractType {
	for _, i := range interfaces {

		switch i {
		case interfaceTypeERC1155:
			return ContractTypeERC1155
		case interfaceTypeERC721:
			return ContractTypeERC721
		case interfaceTypeERC20:
			return ContractTypeERC20
		}
	}
	return ContractTypeUnknown
}
