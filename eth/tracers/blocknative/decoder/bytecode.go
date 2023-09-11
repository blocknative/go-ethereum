package decoder

import "bytes"

type Bytecode []byte

// ContainsMethods returns true if all given methodIDs are found in the code.
func (b Bytecode) ContainsMethods(methodIDs ...[]byte) bool {
	for _, methodID := range methodIDs {
		firstNonZeroIndex := 0
		for firstNonZeroIndex < len(methodID) && methodID[firstNonZeroIndex] == 0 {
			firstNonZeroIndex++
		}

		if !bytes.Contains(b, methodID[firstNonZeroIndex:]) {
			return false
		}
	}
	return true
}

// IsMetadataBasic returns true iff the bytecode implements basic metadata.
func (b Bytecode) IsBasicMetadata() bool {
	return b.ContainsMethods(methodIDsBasicMetadata...)
}

// IsERC20 returns true iff the bytecode implements ERC20.
func (b Bytecode) IsERC20() bool {
	return b.ContainsMethods(methodIDsERC20All...)
}

// IsERC721 returns true iff the bytecode implements ERC721.
func (b Bytecode) IsERC721() bool {
	return b.ContainsMethods(methodIDsERC721All...)
}

// IsERC1155 returns true iff the bytecode implements ERC1155.
func (b Bytecode) IsERC1155() bool {
	return b.ContainsMethods(methodIDsERC1155All...)
}

// DecodeInterface determines the first interface implemented by a contract.
func (b Bytecode) DecodeInterface() InterfaceType {
	interfaces := b.DecodeInterfaces()
	if len(interfaces) == 0 {
		return interfaceTypeUnknown
	}
	return interfaces[0]
}

// DecodeInterfaces determines all interfaces implemented by a contract.
func (b Bytecode) DecodeInterfaces() []InterfaceType {
	var interfaces []InterfaceType

	if b.IsERC20() {
		interfaces = append(interfaces, interfaceTypeERC20)
	}
	if b.IsERC721() {
		interfaces = append(interfaces, interfaceTypeERC721)
	}
	if b.IsERC1155() {
		interfaces = append(interfaces, interfaceTypeERC1155)
	}
	return interfaces
}
