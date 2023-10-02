package decoder

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/log"
)

var (
	abiTypes = struct {
		_string       abi.Type
		_address      abi.Type
		_uint256      abi.Type
		_uint256Array abi.Type
		_bytes        abi.Type
	}{}

	abiArgs = struct {
		singleString  abi.Arguments
		batchTransfer abi.Arguments
	}{}
)

func init() {
	if err := initABITypes(); err != nil {
		log.Error("failed to initialize abi types", "err", err)
	}
	initABIArgs()
}

// initABITypes initializes all the abiTypes.
func initABITypes() error {
	var err error
	if abiTypes._string, err = abi.NewType("string", "", nil); err != nil {
		return err
	}
	if abiTypes._address, err = abi.NewType("address", "", nil); err != nil {
		return err
	}
	if abiTypes._uint256, err = abi.NewType("uint256", "", nil); err != nil {
		return err
	}
	if abiTypes._uint256Array, err = abi.NewType("uint256[]", "", nil); err != nil {
		return err
	}
	if abiTypes._bytes, err = abi.NewType("bytes", "", nil); err != nil {
		return err
	}
	return nil
}

// initABIArgs initializes all the abiArgs.
func initABIArgs() {
	abiArgs.singleString = abi.Arguments{abi.Argument{Type: abiTypes._string, Name: "name"}}
	abiArgs.batchTransfer = abi.Arguments{
		abi.Argument{Type: abiTypes._address, Name: "from"},
		abi.Argument{Type: abiTypes._address, Name: "to"},
		abi.Argument{Type: abiTypes._uint256Array, Name: "tokenIDs"},
		abi.Argument{Type: abiTypes._uint256Array, Name: "values"},
		abi.Argument{Type: abiTypes._bytes, Name: "data"},
	}
}
