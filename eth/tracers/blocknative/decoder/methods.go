package decoder

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
)

const (
	methodIDLen        = 4
	methodIDEncodedLen = methodIDLen * 2
)

var (
	ErrMethodIDTooLong = fmt.Errorf("methodID is too long")

	// ERC20
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

	// Metadata
	methodIDName     = methodIDToBytes(0x06fdde03) // name()
	methodIDSymbol   = methodIDToBytes(0x95d89b41) // symbol()
	methodIDDecimals = methodIDToBytes(0x313ce567) // decimals()
	methodIDTokenURI = methodIDToBytes(0xc87b56dd) // tokenURI(uint256)
	methodIDURI      = methodIDToBytes(0x0e89341c) // uri(uint256)

	// Interfaces by list of methodIDs
	interfaceMethodsERC20   = []MethodID{methodIDName, methodIDSymbol, methodIDDecimals, methodIDTransfer, methodIDTransferFrom, methodIDTotalSupply, methodIDBalanceOf, methodIDApprove, methodIDAllowance}
	interfaceMethodsERC721  = []MethodID{methodIDBalanceOf, methodIDOwnerOf, methodIDSafeTransferFrom, methodIDSafeTransferFrom2, methodIDTransferFrom, methodIDApprove, methodIDSetApprovalForAll, methodIDGetApproved, methodIDIsApprovedForAll}
	interfaceMethodsERC1155 = []MethodID{methodIDSafeTransferFrom3, methodIDSafeBatchTransferFrom, methodIDBalanceOf2, methodIDBalanceOfBatch, methodIDSetApprovalForAll, methodIDIsApprovedForAll}
)

type MethodID []byte

func (m MethodID) Is(other MethodID) bool {
	return bytes.Compare(m, other) == 0
}

func (m MethodID) String() string {
	return "0x" + hex.EncodeToString(m)
}

func (m MethodID) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.String())
}

// UnmarshalJSON implements the json.Unmarshaler interface.
// The bytes come as ASCII-encoded hex, quoted and with a 0x prefix.
func (m *MethodID) UnmarshalJSON(b []byte) error {
	// Remove quotes and 0x prefix, and ensure the remainder isn't too long.
	if len(b) > 2 {
		if b[0] == '"' && b[len(b)-1] == '"' {
			b = b[1 : len(b)-1]
		}
	}
	if len(b) > 2 {
		if b[0] == '0' && (b[1] == 'x' || b[1] == 'X') {
			b = b[2:]
		}
	}

	if len(b) > methodIDEncodedLen {
		fmt.Println("sdfasdfasdf:", len(b), string(b), b)
		return ErrMethodIDTooLong
	}

	// Decode hex into m.
	methodIDBytes := make([]byte, 4)
	if _, err := hex.Decode(methodIDBytes, b); err != nil {
		panic(err)
		return err
	}

	// Method IDs with leading zero bytes are sometimes expressed externally
	// with fewer than 4 bytes. We pad them here to ensure they are always the
	// right length internally.
	common.LeftPadBytes(methodIDBytes, 0)

	*m = methodIDBytes
	return nil
}

// methodIDToBytes converts a uint32 methodID to slice of up 4 bytes.
// Leading zero bytes are omitted because contracts can/do safely omit them.
func methodIDToBytes(i uint32) MethodID {
	return MethodID{
		byte(i >> 24),
		byte(i >> 16),
		byte(i >> 8),
		byte(i),
	}
}
