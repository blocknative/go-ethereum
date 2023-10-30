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

	if metadata.Name, err = evmCallMethodName(evmCall, addr); err != nil {
		log.Trace("failed to decode ERC20 name", "err", err)
	}
	if metadata.Symbol, err = evmCallMethodSymbol(evmCall, addr); err != nil {
		log.Trace("failed to decode ERC20 symbol", "err", err)
	}
	if metadata.Decimals, err = evmCallMethodDecimals(evmCall, addr); err != nil {
		log.Trace("failed to decode ERC20 decimals", "err", err)
	}

	return metadata
}

// decodeERC721Metadata decodes the metadata for an ERC721 token from the EVM.
func decodeERC721Metadata(evmCall evmCallFn, addr common.Address, tokenID *big.Int) AssetMetadata {
	var err error
	metadata := AssetMetadata{Type: AssetTypeERC721}

	if metadata.Name, err = evmCallMethodName(evmCall, addr); err != nil {
		log.Trace("failed to decode ERC721 name", "err", err)
	}
	if metadata.Symbol, err = evmCallMethodSymbol(evmCall, addr); err != nil {
		log.Trace("failed to decode ERC721 symbol", "err", err)
	}

	if tokenID != nil {
		if metadata.URI, err = evmCallMethodTokenURI(evmCall, addr, tokenID); err != nil {
			log.Trace("failed to decode ERC721 tokenURI", "err", err)
		}
	}

	return metadata
}

// decodeERC1155Metadata decodes the metadata for an ERC1155 token from the EVM.
func decodeERC1155Metadata(evmCall evmCallFn, addr common.Address, tokenID *big.Int) AssetMetadata {
	var err error
	metadata := AssetMetadata{Type: AssetTypeERC1155}

	if tokenID != nil {
		if metadata.URI, err = evmCallMethodURI(evmCall, addr, tokenID); err != nil {
			log.Trace("failed to decode ERC1155 URI", "err", err)
		}
	}

	return metadata
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
