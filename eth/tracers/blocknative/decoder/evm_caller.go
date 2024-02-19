package decoder

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

var (
	ErrUnexpectedDecodedSize = fmt.Errorf("unexpected decoded size")
	ErrUnexpectedType        = fmt.Errorf("unexpected type")
)

//
// General callers
//

// callAndDecodeString calls a method and decodes the result as a string.
func callAndDecodeString(evmCall evmCallFn, addr common.Address, msg []byte) (string, error) {
	// Load bytes from the EVM.
	ret, err := evmCall(addr, msg)
	if err != nil {
		return "", err
	}

	// Parse into a string.
	stringIntf, err := abiArgs.singleString.Unpack(ret)
	if err != nil {
		return "", fmt.Errorf("decoding string: %w", err)
	}
	if len(stringIntf) < len(abiArgs.singleString) {
		return "", fmt.Errorf("decoding string: %w", ErrUnexpectedDecodedSize)
	}
	str, ok := stringIntf[0].(string)
	if !ok {
		return "", fmt.Errorf("decoding string: %w", ErrUnexpectedType)
	}

	return str, nil
}

// callAndDecodeUint8 calls a method and decodes the result as a uint8.
func callAndDecodeUint8(evmCall evmCallFn, addr common.Address, msg []byte) (uint8, error) {
	// Load bytes from the EVM.
	ret, err := evmCall(addr, msg)
	if err != nil {
		return 0, err
	}

	// Parse into a uint8.
	if len(ret) < 1 {
		return 0, fmt.Errorf("decoding uint8: %w", ErrUnexpectedDecodedSize)
	}
	return ret[len(ret)-1], nil
}

// callAndDecodeUint256 calls a method and decodes the result as a *big.Int.
func callAndDecodeUint256(evmCall evmCallFn, addr common.Address, msg []byte) (*big.Int, error) {
	ret, err := evmCall(addr, msg)
	if err != nil {
		return nil, err
	}
	if len(ret) > 32 {
		return nil, fmt.Errorf("decoding uint256: %w", ErrUnexpectedDecodedSize)
	}
	return new(big.Int).SetBytes(ret), nil
}

//
// Specific method callers
//

// evmCallMethodName decodes the name of an asset from the EVM.
func evmCallMethodName(evmCall evmCallFn, addr common.Address) (string, error) {
	return callAndDecodeString(evmCall, addr, methodIDName)
}

// evmCallMethodSymbol decodes the symbol of an asset from the EVM.
func evmCallMethodSymbol(evmCall evmCallFn, addr common.Address) (string, error) {
	return callAndDecodeString(evmCall, addr, methodIDSymbol)
}

// evmCallMethodDecimals decodes the decimals of an asset from the EVM.
func evmCallMethodDecimals(evmCall evmCallFn, addr common.Address) (uint8, error) {
	return callAndDecodeUint8(evmCall, addr, methodIDDecimals)
}

// evmCallMethodTokenURI decodes the tokenURI of an asset from the EVM.
func evmCallMethodTokenURI(evmCall evmCallFn, addr common.Address, tokenID *big.Int) (string, error) {
	if tokenID == nil {
		return "", fmt.Errorf("tokenID is nil")
	}

	tokenIDBytes := tokenID.Bytes()
	if len(tokenIDBytes) > 32 {
		return "", fmt.Errorf("tokenID is too large")
	}
	common.LeftPadBytes(tokenIDBytes, 32)
	input := append(methodIDTokenURI, tokenIDBytes...)
	return callAndDecodeString(evmCall, addr, input)
}

// evmCallMethodURI decodes the URI of an asset from the EVM.
func evmCallMethodURI(evmCall evmCallFn, addr common.Address, tokenID *big.Int) (string, error) {
	if tokenID == nil {
		return "", fmt.Errorf("tokenID is nil")
	}

	tokenIDBytes := common.LeftPadBytes(tokenID.Bytes(), 32)
	input := append(methodIDURI, tokenIDBytes...)
	return callAndDecodeString(evmCall, addr, input)
}

// evmCallMethodBalanceOf decodes the balance of an asset from the EVM.
func evmCallMethodBalanceOf(evmCall evmCallFn, addr common.Address, owner common.Address) (*big.Int, error) {
	ownerBytes := common.LeftPadBytes(owner.Bytes(), 32)
	input := append(methodIDBalanceOf, ownerBytes...)
	return callAndDecodeUint256(evmCall, addr, input)
}

// evmCallMethodBalanceOf2 decodes the balance of an asset from the EVM.
func evmCallMethodBalanceOf2(evmCall evmCallFn, addr common.Address, owner common.Address, tokenID *big.Int) (*big.Int, error) {
	tokenIDBytes := common.LeftPadBytes(tokenID.Bytes(), 32)
	ownerBytes := common.LeftPadBytes(owner.Bytes(), 32)
	input := append(methodIDBalanceOf2, ownerBytes...)
	input = append(input, tokenIDBytes...)
	return callAndDecodeUint256(evmCall, addr, input)
}
