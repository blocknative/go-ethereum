package decoder

import (
	"fmt"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/lru"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/log"
)

const (
	cacheSize = 10_0000
)

var (
	abiStringType       abi.Type
	abiSingleStringArgs abi.Arguments

	ethAddress = common.Address{}
	EthAssetID = AssetID{ethAddress, nil}
	ethAsset   = &Asset{
		Address: ethAddress,
		Type:    AssetTypeNative,
		TokenMetadata: TokenMetadata{
			Name:     "Ether",
			Symbol:   "ETH",
			Decimals: 18,
		},
	}
)

func init() {
	var err error
	abiStringType, err = abi.NewType("string", "", nil)
	if err != nil {
		log.Error("failed to create abi string type", "err", err)
	}

	abiSingleStringArgs = abi.Arguments{abi.Argument{Type: abiStringType, Name: "name"}}
}

type AssetID struct {
	Address common.Address
	TokenID *big.Int
}

type AssetDecoder struct {
	cacheMu sync.RWMutex
	cache   lru.BasicLRU[AssetID, *Asset]
}

func NewAssetDecoder() *AssetDecoder {
	return &AssetDecoder{
		cache: lru.NewBasicLRU[AssetID, *Asset](cacheSize),
	}
}

func (d *AssetDecoder) Decode(evm *vm.EVM, assetID AssetID) (*Asset, error) {
	if assetID.Address == ethAddress {
		return ethAsset, nil
	}

	// Check the cache for an existing entry.
	d.cacheMu.RLock()
	asset, ok := d.cache.Get(assetID)
	d.cacheMu.RUnlock()
	if ok {
		return asset, nil
	}

	// Cache miss; decode from EVM and add to the cache.
	asset, err := d.decode(evm, assetID)
	if err != nil {
		return nil, err
	}

	d.cacheMu.Lock()
	d.cache.Add(assetID, asset)
	d.cacheMu.Unlock()

	return asset, nil
}

func (d *AssetDecoder) decode(evm *vm.EVM, assetID AssetID) (*Asset, error) {
	bytecode := Bytecode(evm.StateDB.GetCode(assetID.Address))

	asset := &Asset{
		Address: assetID.Address,
		Type:    AssetTypeForInterfaces(bytecode.DecodeInterfaces()),
		TokenID: assetID.TokenID,
	}

	var err error

	switch asset.Type {
	case AssetTypeERC20:
		asset.TokenMetadata, err = d.decodeERC20Metadata(evm, assetID.Address)
		if err != nil {
			return asset, err
		}
	case AssetTypeERC721:
		fallthrough
	case AssetTypeERC1155:
		if bytecode.IsBasicMetadata() {
			asset.TokenMetadata, err = d.decodeBasicMetadata(evm, assetID.Address)
			if err != nil {
				return asset, err
			}
		}
	}

	return asset, nil
}

// decodeERC20Metadata decodes the metadata for an ERC20 token from the EVM.
func (d *AssetDecoder) decodeERC20Metadata(evm *vm.EVM, contract common.Address) (TokenMetadata, error) {
	metadata, err := d.decodeBasicMetadata(evm, contract)
	if err != nil {
		return TokenMetadata{}, err
	}

	decimals, err := d.decodeMetadataUint8(evm, contract, methodIDDecimals)
	if err != nil {
		return TokenMetadata{}, err
	}

	metadata.Decimals = decimals
	return metadata, nil
}

func (d *AssetDecoder) decodeBasicMetadata(evm *vm.EVM, contract common.Address) (TokenMetadata, error) {
	name, err := d.decodeMetadataString(evm, contract, methodIDName)
	if err != nil {
		return TokenMetadata{}, err
	}

	symbol, err := d.decodeMetadataString(evm, contract, methodIDSymbol)
	if err != nil {
		return TokenMetadata{}, err
	}

	return TokenMetadata{
		Name:   name,
		Symbol: symbol,
	}, nil
}

func (d *AssetDecoder) decodeMetadataString(evm *vm.EVM, contract common.Address, method []byte) (string, error) {
	// Load bytes from the EVM.
	stringBytes, err := callEVMMethod(evm, contract, method)
	if err != nil {
		return "", err
	}

	// Parse into a string.
	stringInterface, err := abiSingleStringArgs.Unpack(stringBytes)
	if err != nil {
		return "", err
	}
	if len(stringInterface) < 1 {
		return "", fmt.Errorf("unexpected decoded size")
	}
	name, ok := stringInterface[0].(string)
	if !ok {
		return "", fmt.Errorf("unexpected type for decoded string")
	}

	return name, nil
}

func (d *AssetDecoder) decodeMetadataUint8(evm *vm.EVM, contract common.Address, method []byte) (uint8, error) {
	// Load bytes from the EVM.
	uint8Bytes, err := callEVMMethod(evm, contract, method)
	if err != nil {
		return 0, err
	}

	// Parse into a uint8.
	if len(uint8Bytes) < 1 {
		return 0, fmt.Errorf("unexpected decoded size")
	}
	return uint8Bytes[len(uint8Bytes)-1], nil
}

func callEVMMethod(evm *vm.EVM, contract common.Address, method []byte) ([]byte, error) {
	ret, _, err := evm.Call(vm.AccountRef(contract), contract, method, math.MaxUint64, common.Big0)
	if err != nil {
		return nil, err
	}
	return ret, nil
}
