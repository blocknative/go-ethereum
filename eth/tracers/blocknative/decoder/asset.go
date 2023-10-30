package decoder

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

var (
	ethAddress = common.Address{}
	EthAssetID = AssetID{ethAddress, nil}
	EthAsset   = &Asset{
		AssetID: EthAssetID,
		AssetMetadata: &AssetMetadata{
			Type:     AssetTypeNative,
			Name:     "Ether",
			Symbol:   "ETH",
			Decimals: 18,
		},
	}
)

// evmCallFn executes the given method on the code for the given address.
// The result is returned as raw, ABI-encoded bytes.
type evmCallFn func(addr common.Address, method []byte) ([]byte, error)

// DecodeAsset decodes the metadata for an asset by calling various methods on
// the underlying contract inside the EVM and decoding the results.
func DecodeAsset(evmCall evmCallFn, contract *Contract, assetID AssetID) (*AssetMetadata, error) {
	var metadata AssetMetadata
	switch {
	case contract.IsERC20():
		metadata = decodeERC20Metadata(evmCall, assetID.Address)
	case contract.IsERC721():
		metadata = decodeERC721Metadata(evmCall, assetID.Address, assetID.TokenID)
	case contract.IsERC1155():
		metadata = decodeERC1155Metadata(evmCall, assetID.Address, assetID.TokenID)
	}
	return &metadata, nil
}

// decodeERC20Metadata decodes the metadata for an ERC20 token from the EVM.
func decodeERC20Metadata(evmCall evmCallFn, addr common.Address) AssetMetadata {
	var err error
	metadata := AssetMetadata{Type: AssetTypeERC20}

	if metadata.Name, err = decodeMetadataName(evmCall, addr); err != nil {
		log.Trace("failed to decode ERC20 name", "err", err)
	}
	if metadata.Symbol, err = decodeMetadataSymbol(evmCall, addr); err != nil {
		log.Trace("failed to decode ERC20 symbol", "err", err)
	}
	if metadata.Decimals, err = decodeMetadataDecimals(evmCall, addr); err != nil {
		log.Trace("failed to decode ERC20 decimals", "err", err)
	}

	return metadata
}

// decodeERC721Metadata decodes the metadata for an ERC721 token from the EVM.
func decodeERC721Metadata(evmCall evmCallFn, addr common.Address, tokenID *big.Int) AssetMetadata {
	var err error
	metadata := AssetMetadata{Type: AssetTypeERC721}

	if metadata.Name, err = decodeMetadataName(evmCall, addr); err != nil {
		log.Trace("failed to decode ERC721 name", "err", err)
	}
	if metadata.Symbol, err = decodeMetadataSymbol(evmCall, addr); err != nil {
		log.Trace("failed to decode ERC721 symbol", "err", err)
	}
	if metadata.URI, err = decodeMetadataTokenURI(evmCall, addr, tokenID); err != nil {
		log.Trace("failed to decode ERC721 tokenURI", "err", err)
	}

	return metadata
}

// decodeERC1155Metadata decodes the metadata for an ERC1155 token from the EVM.
func decodeERC1155Metadata(evmCall evmCallFn, addr common.Address, tokenID *big.Int) AssetMetadata {
	var err error
	metadata := AssetMetadata{Type: AssetTypeERC1155}

	if metadata.URI, err = decodeMetadataURI(evmCall, addr, tokenID); err != nil {
		log.Trace("failed to decode ERC1155 URI", "err", err)
	}

	return metadata
}

// decodeMetadataName decodes the name of an asset from the EVM.
func decodeMetadataName(evmCall evmCallFn, addr common.Address) (string, error) {
	return callAndDecodeString(evmCall, addr, methodIDName)
}

// decodeMetadataSymbol decodes the symbol of an asset from the EVM.
func decodeMetadataSymbol(evmCall evmCallFn, addr common.Address) (string, error) {
	return callAndDecodeString(evmCall, addr, methodIDSymbol)
}

// decodeMetadataDecimals decodes the decimals of an asset from the EVM.
func decodeMetadataDecimals(evmCall evmCallFn, addr common.Address) (uint8, error) {
	return callAndDecodeUint8(evmCall, addr, methodIDDecimals)
}

// decodeMetadataTokenURI decodes the tokenURI of an asset from the EVM.
func decodeMetadataTokenURI(evmCall evmCallFn, addr common.Address, tokenID *big.Int) (string, error) {
	tokenIDBytes := tokenID.Bytes()
	if len(tokenIDBytes) > 32 {
		return "", fmt.Errorf("tokenID is too large")
	}
	common.LeftPadBytes(tokenIDBytes, 32)
	input := append(methodIDTokenURI, tokenIDBytes...)
	return callAndDecodeString(evmCall, addr, input)
}

// decodeMetadataURI decodes the URI of an asset from the EVM.
func decodeMetadataURI(evmCall evmCallFn, addr common.Address, tokenID *big.Int) (string, error) {
	tokenIDBytes := tokenID.Bytes()
	if len(tokenIDBytes) > 32 {
		return "", fmt.Errorf("tokenID is too large")
	}
	common.LeftPadBytes(tokenIDBytes, 32)
	input := append(methodIDURI, tokenIDBytes...)
	return callAndDecodeString(evmCall, addr, input)
}

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

// decodeArgsSafeBatchTransferFrom calls a method and decodes the result as a uint256[].
func decodeArgsSafeBatchTransferFrom(bytes []byte) ([]*Transfer, error) {
	args, err := abiArgs.batchTransfer.UnpackValues(bytes)
	if err != nil {
		return nil, err
	}
	if len(args) < len(abiArgs.batchTransfer) {
		return nil, fmt.Errorf("unexpected decoded size")
	}

	from, ok := args[0].(common.Address)
	if !ok {
		return nil, fmt.Errorf("unexpected type for decoded address")
	}
	to, ok := args[1].(common.Address)
	if !ok {
		return nil, fmt.Errorf("unexpected type for decoded address")
	}
	tokenIDs, ok := args[2].([]*big.Int)
	if !ok {
		return nil, fmt.Errorf("unexpected type for decoded uint265[]")
	}
	values, ok := args[3].([]*big.Int)
	if !ok {
		return nil, fmt.Errorf("unexpected type for decoded uint265[]")
	}

	if len(tokenIDs) != len(values) {
		return nil, fmt.Errorf("expected matching array lengths")
	}

	transfers := make([]*Transfer, len(tokenIDs))
	for i := 0; i < len(tokenIDs); i++ {
		transfers[i] = &Transfer{
			From:    from,
			To:      to,
			Value:   NewAmount(values[i]),
			TokenID: tokenIDs[i],
		}
	}

	return transfers, nil
}
