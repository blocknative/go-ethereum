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
	methodIDTransfer     = methodIDToBytes(0xa9059cbb)
	methodIDTransferFrom = methodIDToBytes(0x23b872dd)
	methodIDTotalSupply  = methodIDToBytes(0x18160ddd)
	methodIDBalanceOf    = methodIDToBytes(0x70a08231)
	methodIDApprove      = methodIDToBytes(0x095ea7b3)
	methodIDAllowance    = methodIDToBytes(0xdd62ed3e)

	// ERC721
	methodIDSafeTransferFrom  = methodIDToBytes(0x42842e0e)
	methodIDSafeTransferFrom2 = methodIDToBytes(0xb88d4fde)
	methodIDOwnerOf           = methodIDToBytes(0x6352211e)
	methodIDSetApprovalForAll = methodIDToBytes(0xa22cb465)
	methodIDGetApproved       = methodIDToBytes(0x081812fc)
	methodIDIsApprovedForAll  = methodIDToBytes(0xe985e9c5)

	// ERC1155
	methodIDBalanceOf2            = methodIDToBytes(0x00fdd58e)
	methodIDBalanceOfBatch        = methodIDToBytes(0x4e1273f4)
	methodIDSafeTransferFrom3     = methodIDToBytes(0xf242432a)
	methodIDSafeBatchTransferFrom = methodIDToBytes(0x2eb2c2d6)

	// Metadata
	methodIDName     = methodIDToBytes(0x06fdde03)
	methodIDSymbol   = methodIDToBytes(0x95d89b41)
	methodIDDecimals = methodIDToBytes(0x313ce567)
	methodIDTokenURI = methodIDToBytes(0xc87b56dd)
	methodIDURI      = methodIDToBytes(0x0e89341c)

	// Interfaces by list of methodIDs
	interfaceMethodsERC20   = []MethodID{methodIDName, methodIDSymbol, methodIDDecimals, methodIDTransfer, methodIDTransferFrom, methodIDTotalSupply, methodIDBalanceOf, methodIDApprove, methodIDAllowance}
	interfaceMethodsERC721  = []MethodID{methodIDBalanceOf, methodIDOwnerOf, methodIDSafeTransferFrom, methodIDSafeTransferFrom2, methodIDTransferFrom, methodIDApprove, methodIDSetApprovalForAll, methodIDGetApproved, methodIDIsApprovedForAll}
	interfaceMethodsERC1155 = []MethodID{methodIDSafeTransferFrom3, methodIDSafeBatchTransferFrom, methodIDBalanceOf2, methodIDBalanceOfBatch, methodIDSetApprovalForAll, methodIDIsApprovedForAll}
)

// methodSignatures is a map of methodID strings to their signatures.
var methodSignatures = map[string]string{
	methodIDTransfer.String():     "transfer(address,uint256)",
	methodIDTransferFrom.String(): "transferFrom(address,address,uint256)",
	methodIDTotalSupply.String():  "totalSupply()",
	methodIDBalanceOf.String():    "balanceOf(address)",
	methodIDApprove.String():      "approve(address,uint256)",
	methodIDAllowance.String():    "allowance(address,address)",

	methodIDSafeTransferFrom.String():  "safeTransferFrom(address,address,uint256)",
	methodIDSafeTransferFrom2.String(): "safeTransferFrom(address,address,uint256,bytes)",
	methodIDOwnerOf.String():           "ownerOf(uint256)",
	methodIDSetApprovalForAll.String(): "setApprovalForAll(address,bool)",
	methodIDGetApproved.String():       "getApproved(uint256)",
	methodIDIsApprovedForAll.String():  "isApprovedForAll(address,address)",

	methodIDBalanceOf2.String():            "balanceOf(address,uint256)",
	methodIDBalanceOfBatch.String():        "balanceOfBatch(address[],uint256[])",
	methodIDSafeTransferFrom3.String():     "safeTransferFrom(address,address,uint256,uint256,bytes)",
	methodIDSafeBatchTransferFrom.String(): "safeBatchTransferFrom(address,address,uint256[],uint256[],bytes)",

	methodIDName.String():     "name()",
	methodIDSymbol.String():   "symbol()",
	methodIDDecimals.String(): "decimals()",
	methodIDTokenURI.String(): "tokenURI(uint256)",
	methodIDURI.String():      "uri(uint256)",
}

type MethodID []byte

func (m MethodID) Is(other MethodID) bool {
	return bytes.Equal(m, other)
}

func (m MethodID) String() string {
	return "0x" + hex.EncodeToString(m)
}

func (m MethodID) Signature() string {
	return methodSignatures[m.String()]
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
		return ErrMethodIDTooLong
	}

	// Decode hex into m.
	methodIDBytes := make([]byte, 4)
	if _, err := hex.Decode(methodIDBytes, b); err != nil {
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
