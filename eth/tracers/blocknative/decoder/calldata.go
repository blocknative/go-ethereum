package decoder

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"math/big"
)

func decodeCallData(sender common.Address, contract *Contract, input []byte) (*CallData, error) {
	// Check if the input is capable of being a function call.
	// If not then we're done. If so then check if it's a transfer.
	inputLen := len(input)
	switch {
	case inputLen < 4:
		return nil, ErrCallDataTooShort
	}

	methodBytes := make([]byte, 4)
	copy(methodBytes, input[:4])

	var (
		idx    = 4
		method = MethodID(methodBytes)
		args   = make([]string, 0, 4)

		from    common.Address
		to      common.Address
		amount  = new(Amount)
		tokenID *big.Int

		transfers []*Transfer
	)

	// scanWord gets the next 32 bytes and advances the index
	scanWord := func() []byte {
		word := input[idx : idx+32]
		idx += 32
		return word
	}

	switch {

	// transfer(address,uint256).
	case method.Is(methodIDTransfer):
		if len(input) < 68 {
			return nil, ErrCallDataTooShort
		}
		from = sender
		to = common.BytesToAddress(scanWord())
		amount.SetBytes(scanWord())

		args = append(args, to.String(), amount.String())
		transfers = append(transfers, &Transfer{
			From:  from,
			To:    to,
			Value: amount,
		})

	// transferFrom(address,address,uint256).
	case method.Is(methodIDTransferFrom):
		fallthrough
	// safeTransferFrom(address,address,uint256).
	case method.Is(methodIDSafeTransferFrom):
		fallthrough
	// safeTransferFrom(address,address,uint256,bytes).
	case method.Is(methodIDSafeTransferFrom2):
		if len(input) < 100 {
			return nil, ErrCallDataTooShort
		}

		from = common.BytesToAddress(scanWord())
		to = common.BytesToAddress(scanWord())
		amount.SetBytes(scanWord())

		args = append(args, from.String(), to.String(), amount.String())

		// If the contract is an ERC-721, but not an ERC-20, then move the
		// scanned amount to the tokenID and set the amount to 1.
		if contract.IsERC721() && !contract.IsERC20() {
			tokenID = (*big.Int)(amount)
			amount = NewAmount(common.Big1)
		}

		transfers = append(transfers, &Transfer{
			From:    from,
			To:      to,
			Value:   amount,
			TokenID: tokenID,
		})

	// safeTransferFrom(address,address,uint256,uint256,bytes)
	case method.Is(methodIDSafeTransferFrom3):
		if len(input) < 100 {
			return nil, ErrCallDataTooShort
		}
		from = common.BytesToAddress(scanWord())
		to = common.BytesToAddress(scanWord())
		tokenID = new(big.Int).SetBytes(scanWord())
		amount.SetBytes(scanWord())

		args = append(args, from.String(), to.String(), tokenID.String(), amount.String())

		transfers = append(transfers, &Transfer{
			From:    from,
			To:      to,
			Value:   amount,
			TokenID: tokenID,
		})

	// safeBatchTransferFrom(address,address,uint256[],uint256[],bytes)
	case method.Is(methodIDSafeBatchTransferFrom):
		var err error
		transfers, err = decodeArgsSafeBatchTransferFrom(input[idx:])
		if err != nil {
			return nil, err
		}
	default:
		// We don't have a known method so we can't parse the args.
		// Just add them as hex-encoded words.
		for idx < len(input) {
			args = append(args, hexutil.Encode(scanWord()))
		}
	}

	return &CallData{
		MethodID:  method,
		Args:      args,
		Transfers: transfers,
	}, nil
}
