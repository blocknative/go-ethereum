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
	methodIDTransfer     = uint32ToFourByteID(0xa9059cbb)
	methodIDTransferFrom = uint32ToFourByteID(0x23b872dd)
	methodIDTotalSupply  = uint32ToFourByteID(0x18160ddd)
	methodIDBalanceOf    = uint32ToFourByteID(0x70a08231)
	methodIDApprove      = uint32ToFourByteID(0x095ea7b3)
	methodIDAllowance    = uint32ToFourByteID(0xdd62ed3e)

	// ERC721
	methodIDSafeTransferFrom  = uint32ToFourByteID(0x42842e0e)
	methodIDSafeTransferFrom2 = uint32ToFourByteID(0xb88d4fde)
	methodIDOwnerOf           = uint32ToFourByteID(0x6352211e)
	methodIDSetApprovalForAll = uint32ToFourByteID(0xa22cb465)
	methodIDGetApproved       = uint32ToFourByteID(0x081812fc)
	methodIDIsApprovedForAll  = uint32ToFourByteID(0xe985e9c5)

	// ERC1155
	methodIDBalanceOf2            = uint32ToFourByteID(0x00fdd58e)
	methodIDBalanceOfBatch        = uint32ToFourByteID(0x4e1273f4)
	methodIDSafeTransferFrom3     = uint32ToFourByteID(0xf242432a)
	methodIDSafeBatchTransferFrom = uint32ToFourByteID(0x2eb2c2d6)

	// Metadata
	methodIDName     = uint32ToFourByteID(0x06fdde03)
	methodIDSymbol   = uint32ToFourByteID(0x95d89b41)
	methodIDDecimals = uint32ToFourByteID(0x313ce567)
	methodIDTokenURI = uint32ToFourByteID(0xc87b56dd)
	methodIDURI      = uint32ToFourByteID(0x0e89341c)

	// Interfaces by list of methodIDs
	interfaceMethodsERC20   = []FourByteID{methodIDName, methodIDSymbol, methodIDDecimals, methodIDTransfer, methodIDTransferFrom, methodIDTotalSupply, methodIDBalanceOf, methodIDApprove, methodIDAllowance}
	interfaceMethodsERC721  = []FourByteID{methodIDBalanceOf, methodIDOwnerOf, methodIDSafeTransferFrom, methodIDSafeTransferFrom2, methodIDTransferFrom, methodIDApprove, methodIDSetApprovalForAll, methodIDGetApproved, methodIDIsApprovedForAll}
	interfaceMethodsERC1155 = []FourByteID{methodIDSafeTransferFrom3, methodIDSafeBatchTransferFrom, methodIDBalanceOf2, methodIDBalanceOfBatch, methodIDSetApprovalForAll, methodIDIsApprovedForAll}
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

type FourByteID []byte

func (id FourByteID) Is(other FourByteID) bool {
	return bytes.Compare(id, other) == 0
}

func (id FourByteID) String() string {
	return "0x" + hex.EncodeToString(id)
}

func (id FourByteID) Signature() string {
	return methodSignatures[id.String()]
}

func (id FourByteID) MarshalJSON() ([]byte, error) {
	return json.Marshal(id.String())
}

// UnmarshalJSON implements the json.Unmarshaler interface.
// The bytes come as ASCII-encoded hex, quoted and with a 0x prefix.
func (id *FourByteID) UnmarshalJSON(b []byte) error {
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
		return err
	}

	// Method IDs with leading zero bytes are sometimes expressed externally
	// with fewer than 4 bytes. We pad them here to ensure they are always the
	// right length internally.
	common.LeftPadBytes(methodIDBytes, 0)

	*id = methodIDBytes
	return nil
}

// uint32ToFourByteID converts a uint32 to slice of 4 bytes.
func uint32ToFourByteID(i uint32) FourByteID {
	return FourByteID{
		byte(i >> 24),
		byte(i >> 16),
		byte(i >> 8),
		byte(i),
	}
}
