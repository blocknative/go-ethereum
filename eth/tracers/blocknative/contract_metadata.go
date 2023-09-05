package blocknative

import (
	"bytes"
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
	abiStringType   abi.Type
	abiMetadataArgs abi.Arguments
)

func init() {
	var err error
	abiStringType, err = abi.NewType("string", "", nil)
	if err != nil {
		log.Error("failed to create abi string type", "err", err)
	}

	abiMetadataArgs = abi.Arguments{abi.Argument{Type: abiStringType, Name: "name"}}
}

type tokenMetadataReader struct {
	cache lru.BasicLRU[common.Address, *Asset]
}

func newTokenMetadataReader() *tokenMetadataReader {
	return &tokenMetadataReader{
		cache: lru.NewBasicLRU[common.Address, *Asset](cacheSize),
	}
}
func (l *tokenMetadataReader) read(evm *vm.EVM, contract common.Address) (*Asset, error) {
	// Check the cache first.
	if asset, ok := l.cache.Get(contract); ok {
		return asset, nil
	}

	// Cache miss; read from EVM and add to the cache.
	asset, err := l.readFromEVM(evm, contract)
	if err != nil {
		return nil, err
	}
	l.cache.Add(contract, asset)

	return asset, nil
}

func (l *tokenMetadataReader) readFromEVM(evm *vm.EVM, contract common.Address) (*Asset, error) {
	asset := &Asset{
		Address: contract,
		Type:    findAccountType(evm.StateDB, contract),
	}

	var err error
	switch asset.Type {
	case accountTypeERC20:
		asset.TokenMetadata, err = l.readERC20Metadata(evm, contract)
		if err != nil {
			return nil, err
		}
	case accountTypeERC721:
		asset.TokenMetadata, err = l.readERC721Metadata(evm, contract)
		if err != nil {
			return nil, err
		}
	default:
		asset.TokenMetadata, err = l.readERC20Metadata(evm, contract)
		if err != nil {
			return nil, err
		}
	}

	return asset, nil
}

// readERC20Metadata reads the metadata for an ERC20 token from the EVM.
func (l *tokenMetadataReader) readERC20Metadata(evm *vm.EVM, contract common.Address) (TokenMetadata, error) {
	metadata, err := l.readCommonMetadata(evm, contract)
	if err != nil {
		return TokenMetadata{}, err
	}

	decimals, err := l.readMetadataUint8(evm, contract, methodIDMetadataDecimals)
	if err != nil {
		return TokenMetadata{}, err
	}

	metadata.Decimals = decimals
	return metadata, nil
}

func (l *tokenMetadataReader) readERC721Metadata(evm *vm.EVM, contract common.Address) (TokenMetadata, error) {
	return l.readCommonMetadata(evm, contract)
}

// readERC721Metadata reads the metadata for an ERC20 token from the EVM.
func (l *tokenMetadataReader) readCommonMetadata(evm *vm.EVM, contract common.Address) (TokenMetadata, error) {
	name, err := l.readMetadataString(evm, contract, methodIDMetadataName)
	if err != nil {
		return TokenMetadata{}, err
	}

	symbol, err := l.readMetadataString(evm, contract, methodIDMetadataSymbol)
	if err != nil {
		return TokenMetadata{}, err
	}

	return TokenMetadata{
		Name:   name,
		Symbol: symbol,
	}, nil
}

func (l *tokenMetadataReader) readMetadataString(evm *vm.EVM, contract common.Address, method []byte) (string, error) {
	// Load bytes from the EVM.
	stringBytes, err := callEVMMethod(evm, contract, method)
	if err != nil {
		return "", err
	}

	// Parse into a string.
	stringInterface, err := abiMetadataArgs.Unpack(stringBytes)
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

func (l *tokenMetadataReader) readMetadataUint8(evm *vm.EVM, contract common.Address, method []byte) (uint8, error) {
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

// findAccountType attempts to determine the type of contract by looking at
// the contract's bytecode.
func findAccountType(state vm.StateDB, account common.Address) accountType {
	bytecode := state.GetCode(account)

	switch {
	case bytecode == nil:
		return accountTypeEOA
	case codeContainsAllERC20Methods(bytecode):
		return accountTypeERC20
	case codeContainsAllERC721Methods(bytecode):
		return accountTypeERC721
	}

	return accountTypeUnknown
}

// bytesContainAll returns true if all given byte slices are found in the
// search slice.
func bytesContainAll(code []byte, methodIDs ...[]byte) bool {
	for _, methodID := range methodIDs {
		if !bytes.Contains(code, methodID) {
			return false
		}
	}
	return true
}

// codeContainsAllERC20Methods returns true if all ERC20 method IDs are found
// in the bytecode.
func codeContainsAllERC20Methods(code []byte) bool {
	return bytesContainAll(code, methodIDERC20Transfer, methodIDERC20TransferFrom)
}

// codeContainsAllERC721Methods returns true if all ERC721 method IDs are found
// in the bytecode.
func codeContainsAllERC721Methods(code []byte) bool {
	return bytesContainAll(code, methodIDERC721TransferFrom, methodIDERC721SafeTransfer, methodIDERC721SafeTransferWithData)
}
