package blocknative

var (
	// ERC20 methods
	methodIDERC20Transfer     = uint32ToBytes(0xa9059cbb) // transfer(address,uint256)
	methodIDERC20TransferFrom = uint32ToBytes(0x23b872dd) // transferFrom(address,address,uint256)

	// ERC721 methods
	methodIDERC721TransferFrom         = uint32ToBytes(0x23b872dd) // transferFrom(address,address,uint256)
	methodIDERC721SafeTransfer         = uint32ToBytes(0x42842e0e) // safeTransferFrom(address,address,uint256)
	methodIDERC721SafeTransferWithData = uint32ToBytes(0xa9059cbb) // safeTransferFrom(address,address,uint256,bytes)

	// Metadata getter methods
	methodIDMetadataName     = uint32ToBytes(0x06fdde03) // name()
	methodIDMetadataSymbol   = uint32ToBytes(0x95d89b41) // symbol()
	methodIDMetadataDecimals = uint32ToBytes(0x313ce567) // decimals()

)

// uint32ToBytes converts a uint32 to a 4 byte slice.
func uint32ToBytes(i uint32) []byte {
	return []byte{
		byte(i >> 24),
		byte(i >> 16),
		byte(i >> 8),
		byte(i),
	}
}
