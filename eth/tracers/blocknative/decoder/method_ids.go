package decoder

var (
	// ERC20
	methodIDName     = methodIDToBytes(0x06fdde03) // name()
	methodIDSymbol   = methodIDToBytes(0x95d89b41) // symbol()
	methodIDDecimals = methodIDToBytes(0x313ce567) // decimals()

	methodIDTransfer     = methodIDToBytes(0xa9059cbb) // transfer(address,uint256)
	methodIDTransferFrom = methodIDToBytes(0x23b872dd) // transferFrom(address,address,uint256)
	methodIDTotalSupply  = methodIDToBytes(0x18160ddd) // totalSupply()
	methodIDBalanceOf    = methodIDToBytes(0x70a08231) // balanceOf(address)
	methodIDApprove      = methodIDToBytes(0x095ea7b3) // approve(address,uint256)
	methodIDAllowance    = methodIDToBytes(0xdd62ed3e) // allowance(address,address)

	// ERC721
	methodIDSafeTransferFrom  = methodIDToBytes(0x42842e0e) // safeTransferFrom(address,address,uint256)
	methodIDSafeTransferFrom2 = methodIDToBytes(0xb88d4fde) // safeTransferFrom(address,address,uint256,bytes)
	methodIDOwnerOf           = methodIDToBytes(0x6352211e) // ownerOf(uint256)
	methodIDSetApprovalForAll = methodIDToBytes(0xa22cb465) // setApprovalForAll(address,bool)
	methodIDGetApproved       = methodIDToBytes(0x081812fc) // getApproved(uint256)
	methodIDIsApprovedForAll  = methodIDToBytes(0xe985e9c5) // isApprovedForAll(address,address)

	// ERC1155
	methodIDBalanceOf2            = methodIDToBytes(0x00fdd58e) // balanceOf(address,uint256)
	methodIDBalanceOfBatch        = methodIDToBytes(0x4e1273f4) // balanceOfBatch(address[],uint256[])
	methodIDSafeTransferFrom3     = methodIDToBytes(0xf242432a) // safeTransferFrom(address,address,uint256,uint256,bytes)
	methodIDSafeBatchTransferFrom = methodIDToBytes(0x2eb2c2d6) // safeBatchTransferFrom(address,address,uint256[],uint256[],bytes)

	// Interfaces by list of methodIDs
	methodIDsERC20All      = [][]byte{methodIDName, methodIDSymbol, methodIDDecimals, methodIDTransfer, methodIDTransferFrom, methodIDTotalSupply, methodIDBalanceOf, methodIDApprove, methodIDAllowance}
	methodIDsERC721All     = [][]byte{methodIDBalanceOf, methodIDOwnerOf, methodIDSafeTransferFrom, methodIDSafeTransferFrom2, methodIDTransferFrom, methodIDApprove, methodIDSetApprovalForAll, methodIDGetApproved, methodIDIsApprovedForAll}
	methodIDsERC1155All    = [][]byte{methodIDSafeTransferFrom3, methodIDSafeBatchTransferFrom, methodIDBalanceOf2, methodIDBalanceOfBatch, methodIDSetApprovalForAll, methodIDIsApprovedForAll}
	methodIDsBasicMetadata = [][]byte{methodIDName, methodIDSymbol}
)

// methodIDToBytes converts a uint32 methodID to slice of up 4 bytes.
// Leading zero bytes are omitted because contracts can/do safely omit them.
func methodIDToBytes(i uint32) []byte {
	return []byte{
		byte(i >> 24),
		byte(i >> 16),
		byte(i >> 8),
		byte(i),
	}
}
