package decoder

import (
	"bytes"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
)

const (
	CallTypeUnknown CallType = iota
	AssetTransfer
)

type CallType int

type DecodedCall struct {
	CallType CallType
	From     common.Address
	To       common.Address
	Value    *big.Int
}

func DecodeCalldata(sender common.Address, input []byte) *DecodedCall {
	// Check if the input is capable of being a function call selector.
	// If not then we're done. If so then check if it's a transfer call.
	if len(input) < 4 {
		return nil
	}

	var (
		idx      = 4
		methodID = input[:idx]
		from     common.Address
		to       common.Address
		amount   = new(big.Int)
	)

	// scanWord gets the next 32 bytes and advances the index
	scanWord := func() []byte {
		word := input[idx : idx+32]
		idx += 32
		return word
	}

	switch {

	// Transfer event; payload is [to, amount]
	case bytes.Compare(methodID, methodIDTransfer) == 0:
		if len(input) < 68 {
			return nil
		}

		from = sender
		to = common.BytesToAddress(scanWord())
		amount.SetBytes(scanWord())

	//(Safe)TransferFrom event; payload is [from, to, amount]
	case bytes.Compare(methodID, methodIDTransferFrom) == 0:
		fallthrough
	case bytes.Compare(methodID, methodIDTransferFrom) == 0:
		fallthrough
	case bytes.Compare(methodID, methodIDSafeTransferFrom) == 0:
		fallthrough
	case bytes.Compare(methodID, methodIDSafeTransferFrom2) == 0:
		if len(input) < 100 {
			return nil
		}

		from = common.BytesToAddress(scanWord())
		to = common.BytesToAddress(scanWord())
		amount.SetBytes(scanWord())

	// Not a matching event; ignore
	default:
		return nil
	}

	return &DecodedCall{
		CallType: AssetTransfer,
		From:     from,
		To:       to,
		Value:    amount,
	}
}
