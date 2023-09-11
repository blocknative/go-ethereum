package decoder

import (
	"fmt"

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

	EthAddress = common.Address{}
	ethAsset   = &Asset{
		Address: EthAddress,
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

type MetadataDecoder struct {
	cache lru.BasicLRU[common.Address, *Asset]
}

func NewMetadataDecoder() *MetadataDecoder {
	return &MetadataDecoder{
		cache: lru.NewBasicLRU[common.Address, *Asset](cacheSize),
	}
}

func (d *MetadataDecoder) Read(evm *vm.EVM, contract common.Address) (*Asset, error) {
	if contract == EthAddress {
		return ethAsset, nil
	}

	// Check the cache for an existing entry.
	if asset, ok := d.cache.Get(contract); ok {
		return asset, nil
	}

	// Cache miss; read from EVM and add to the cache.
	asset, err := d.readFromEVM(evm, contract)
	if err != nil {
		return nil, err
	}
	d.cache.Add(contract, asset)

	return asset, nil
}

func (d *MetadataDecoder) readFromEVM(evm *vm.EVM, contract common.Address) (*Asset, error) {
	bytecode := Bytecode(evm.StateDB.GetCode(contract))

	asset := &Asset{
		Address: contract,
		Type:    AssetTypeForInterfaces(bytecode.DecodeInterfaces()),
	}

	var err error

	switch asset.Type {
	case AssetTypeERC20:
		asset.TokenMetadata, err = d.readERC20Metadata(evm, contract)
		if err != nil {
			return asset, err
		}
	case AssetTypeERC721:
		fallthrough
	case AssetTypeERC1155:
		if bytecode.IsBasicMetadata() {
			asset.TokenMetadata, err = d.readBasicMetadata(evm, contract)
			if err != nil {
				return asset, err
			}
		}
	}

	return asset, nil
}

// readERC20Metadata reads the metadata for an ERC20 token from the EVM.
func (d *MetadataDecoder) readERC20Metadata(evm *vm.EVM, contract common.Address) (TokenMetadata, error) {
	metadata, err := d.readBasicMetadata(evm, contract)
	if err != nil {
		return TokenMetadata{}, err
	}

	decimals, err := d.readMetadataUint8(evm, contract, methodIDDecimals)
	if err != nil {
		return TokenMetadata{}, err
	}

	metadata.Decimals = decimals
	return metadata, nil
}

func (d *MetadataDecoder) readBasicMetadata(evm *vm.EVM, contract common.Address) (TokenMetadata, error) {
	name, err := d.readMetadataString(evm, contract, methodIDName)
	if err != nil {
		return TokenMetadata{}, err
	}

	symbol, err := d.readMetadataString(evm, contract, methodIDSymbol)
	if err != nil {
		return TokenMetadata{}, err
	}

	return TokenMetadata{
		Name:   name,
		Symbol: symbol,
	}, nil
}

func (d *MetadataDecoder) readMetadataString(evm *vm.EVM, contract common.Address, method []byte) (string, error) {
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

func (d *MetadataDecoder) readMetadataUint8(evm *vm.EVM, contract common.Address, method []byte) (uint8, error) {
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
