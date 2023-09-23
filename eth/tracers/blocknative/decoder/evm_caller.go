package decoder

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

//
// General callers
//

// callAndDecodeString calls a method and decodes the result as a string.
func callAndDecodeString(evmCall evmCallFn, addr common.Address, method []byte) (string, error) {
	// Load bytes from the EVM.
	stringBytes, err := evmCall(addr, method)
	if err != nil {
		return "", err
	}

	// Parse into a string.
	stringInterface, err := abiArgs.singleString.Unpack(stringBytes)
	if err != nil {
		return "", err
	}
	if len(stringInterface) < len(abiArgs.singleString) {
		return "", fmt.Errorf("unexpected decoded size")
	}
	str, ok := stringInterface[0].(string)
	if !ok {
		return "", fmt.Errorf("unexpected type for decoded string")
	}

	return str, nil
}

// callAndDecodeUint8 calls a method and decodes the result as a uint8.
func callAndDecodeUint8(evmCall evmCallFn, addr common.Address, method []byte) (uint8, error) {
	// Load bytes from the EVM.
	uint8Bytes, err := evmCall(addr, method)
	if err != nil {
		return 0, err
	}

	// Parse into a uint8.
	if len(uint8Bytes) < 1 {
		return 0, fmt.Errorf("unexpected decoded size")
	}
	return uint8Bytes[len(uint8Bytes)-1], nil
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
	tokenIDBytes := tokenID.Bytes()
	if len(tokenIDBytes) > 32 {
		return "", fmt.Errorf("tokenID is too large")
	}
	common.LeftPadBytes(tokenIDBytes, 32)
	input := append(methodIDURI, tokenIDBytes...)
	return callAndDecodeString(evmCall, addr, input)
}
