package decoder

import (
	"github.com/ethereum/go-ethereum/common"
	"math/big"
	"reflect"
	"testing"
)

func TestDecodeCalldata(t *testing.T) {
	type args struct {
		sender common.Address
		input  []byte
	}
	tests := []struct {
		name string
		args args
		want *DecodedCall
	}{
		{
			"erc721",
			args{
				common.Address{},
				[]byte{35, 184, 114, 221, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 84, 112, 197, 166, 252, 231, 68, 122, 253, 44, 155, 227, 160, 242, 94, 54, 44, 9, 54, 97, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 71, 158, 224, 54, 58, 122, 194, 239, 52, 203, 167, 238, 130, 210, 194, 224, 101, 45, 70, 105, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 24, 197},
			},
			&DecodedCall{
				CallType:  AssetTransfer,
				AssetType: AssetTypeERC721,
				From:      common.HexToAddress("0x5470c5a6Fce7447aFd2C9BE3A0F25e362C093661"),
				To:        common.HexToAddress("0x479ee0363a7Ac2ef34cba7ee82D2C2E0652D4669"),
				Value:     big.NewInt(1),
				TokenID:   nil,
			}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DecodeCalldata(tt.args.sender, tt.args.input); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DecodeCalldata() = %v, want %v", got, tt.want)
			}
		})
	}
}
