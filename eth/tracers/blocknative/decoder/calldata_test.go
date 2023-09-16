package decoder

import (
	"encoding/hex"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"math/big"
	"reflect"
	"testing"
)

func TestDecodeCalldata(t *testing.T) {
	type args struct {
		sender   common.Address
		input    string
		contract *Contract
	}
	tests := []struct {
		name string
		args args
		want *DecodedCall
	}{
		{
			"erc20 transferFrom",
			args{
				common.Address{},
				"23b872dd0000000000000000000000005470c5a6fce7447afd2c9be3a0f25e362c093661000000000000000000000000479ee0363a7ac2ef34cba7ee82d2c2e0652d466900000000000000000000000000000000000000000000000000000000000018c5",
				&Contract{Interfaces: []interfaceType{interfaceTypeERC20}},
			},
			&DecodedCall{
				CallType: AssetTransfer,
				From:     common.HexToAddress("0x5470c5a6Fce7447aFd2C9BE3A0F25e362C093661"),
				To:       common.HexToAddress("0x479ee0363a7Ac2ef34cba7ee82D2C2E0652D4669"),
				Value:    big.NewInt(6341),
				TokenID:  nil,
			},
		},
		{
			"erc721 transferFrom",
			args{
				common.Address{},
				"23b872dd0000000000000000000000005470c5a6fce7447afd2c9be3a0f25e362c093661000000000000000000000000479ee0363a7ac2ef34cba7ee82d2c2e0652d466900000000000000000000000000000000000000000000000000000000000018c5",
				&Contract{Interfaces: []interfaceType{interfaceTypeERC721}},
			},
			&DecodedCall{
				CallType: AssetTransfer,
				From:     common.HexToAddress("0x5470c5a6Fce7447aFd2C9BE3A0F25e362C093661"),
				To:       common.HexToAddress("0x479ee0363a7Ac2ef34cba7ee82D2C2E0652D4669"),
				Value:    big.NewInt(1),
				TokenID:  big.NewInt(6341),
			},
		},
		{
			"erc20 erc721 transferFrom",
			args{
				common.Address{},
				"23b872dd0000000000000000000000005470c5a6fce7447afd2c9be3a0f25e362c093661000000000000000000000000479ee0363a7ac2ef34cba7ee82d2c2e0652d466900000000000000000000000000000000000000000000000000000000000018c5",
				&Contract{Interfaces: []interfaceType{interfaceTypeERC20, interfaceTypeERC721}},
			},
			&DecodedCall{
				CallType: AssetTransfer,
				From:     common.HexToAddress("0x5470c5a6Fce7447aFd2C9BE3A0F25e362C093661"),
				To:       common.HexToAddress("0x479ee0363a7Ac2ef34cba7ee82D2C2E0652D4669"),
				Value:    big.NewInt(6341),
				TokenID:  nil,
			},
		},
		{
			"erc1155 safeTransferFrom",
			args{
				common.Address{},
				"f242432a000000000000000000000000cb89354a1c6e7abd1972a68466db238e48a3b0c800000000000000000000000020964f741d2dffd2ccec658ca086e21af1d7df8e000000000000000000000000000000000000000000000000000000000000001d000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000a00000000000000000000000000000000000000000000000000000000000000000360c6ebe",
				&Contract{Interfaces: []interfaceType{interfaceTypeERC20, interfaceTypeERC721}},
			},
			&DecodedCall{
				CallType: AssetTransfer,
				From:     common.HexToAddress("0xcb89354a1c6e7ABd1972a68466Db238e48a3B0C8"),
				To:       common.HexToAddress("0x20964f741d2dffd2ccec658ca086e21af1d7df8e"),
				Value:    big.NewInt(1),
				TokenID:  big.NewInt(29),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, err := hex.DecodeString(tt.args.input)
			require.NoError(t, err)
			if got := DecodeCalldata(tt.args.sender, input, tt.args.contract); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DecodeCalldata() = %v, want %v", got, tt.want)
			}
		})
	}
}
