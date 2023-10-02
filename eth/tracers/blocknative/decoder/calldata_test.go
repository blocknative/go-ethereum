package decoder

import (
	"encoding/hex"
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
)

type decodeCallDataTest struct {
	name string
	args decodeCallDataTestArgs
	want *CallData
}

type decodeCallDataTestArgs struct {
	sender   common.Address
	input    string
	contract *Contract
}

func TestDecodeCalldata(t *testing.T) {
	tests := getTestCases()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executeTests(t, tt)
		})
	}
}

func BenchmarkDecodeCalldata(B *testing.B) {
	tests := getTestCases()
	for i := 0; i < B.N; i++ {
		executeTests(B, tests[i%len(tests)])
	}
}
func BenchmarkDecodeCalldataWithStandardVectors(b *testing.B) {
	testVectors, err := loadTestVectors()
	require.NoError(b, err)

	calls := []*testCall{}
	var addAllCalls func(calls []*testVector)
	addAllCalls = func(tests []*testVector) {
		for _, test := range tests {
			input, err := hex.DecodeString(test.Input[2:])
			require.NoError(b, err)
			value, _ := new(big.Int).SetString(test.Value, 10)
			contract := &Contract{Type: contractTypesByName[test.Type]}
			calls = append(calls, &testCall{
				contract: contract,
				from:     common.HexToAddress(test.From),
				to:       common.HexToAddress(test.To),
				value:    NewAmount(value),
				input:    input,
			})
			addAllCalls(test.Calls)
		}
	}
	for _, v := range testVectors {
		addAllCalls(v.Calls)
	}

	// Run the benchmark, decoding each call in turn.
	var call testCall
	for i := 0; i < b.N; i++ {
		call = *calls[i%len(calls)]
		_, _ = decodeCallData(call.from, call.contract, call.input)
	}
}

type testVector struct {
	Type  string        `json:"type"`
	From  string        `json:"from"`
	To    string        `json:"to,omitempty"`
	Value string        `json:"value,omitempty"`
	Input string        `json:"input"`
	Calls []*testVector `json:"calls,omitempty"`
}

type testCall struct {
	contract *Contract
	from     common.Address
	to       common.Address
	value    *Amount
	input    []byte
}

func loadTestVectors() ([]*testVector, error) {
	testVectorDirs := []string{
		"../../internal/tracetest/testdata/blocknative",
		"../../internal/tracetest/testdata/blocknative/with_decoding",
	}

	var testVectors []*testVector
	for _, dirPath := range testVectorDirs {
		files, err := os.ReadDir(dirPath)
		if err != nil {
			return nil, err
		}

		for _, file := range files {
			test := new(struct {
				Result *testVector `json:"result"`
			})
			if blob, err := os.ReadFile(filepath.Join(dirPath, file.Name())); err != nil {
				return nil, err
			} else if err := json.Unmarshal(blob, test); err != nil {
				return nil, err
			}

			testVectors = append(testVectors, test.Result.Calls...)
		}
	}
	return testVectors, nil
}

func getTestCases() []decodeCallDataTest {
	return []decodeCallDataTest{
		{
			"erc20 transferFrom(address,address,uint256)",
			decodeCallDataTestArgs{
				common.Address{},
				"23b872dd0000000000000000000000005470c5a6fce7447afd2c9be3a0f25e362c093661000000000000000000000000479ee0363a7ac2ef34cba7ee82d2c2e0652d466900000000000000000000000000000000000000000000000000000000000018c5",
				&Contract{interfaces: []Interface{interfaceTypeERC20}},
			},
			&CallData{
				MethodID:  methodIDTransferFrom,
				Signature: methodSignatures[methodIDTransferFrom.String()],
				Args:      []string{"0x5470c5a6Fce7447aFd2C9BE3A0F25e362C093661", "0x479ee0363a7Ac2ef34cba7ee82D2C2E0652D4669", "6341"},
				Transfers: []*Transfer{{
					From:    common.HexToAddress("0x5470c5a6Fce7447aFd2C9BE3A0F25e362C093661"),
					To:      common.HexToAddress("0x479ee0363a7Ac2ef34cba7ee82D2C2E0652D4669"),
					Value:   NewAmount(big.NewInt(6341)),
					TokenID: nil,
				}},
			},
		},
		{
			"erc721 transferFrom(address,address,uint256)",
			decodeCallDataTestArgs{
				common.Address{},
				"23b872dd0000000000000000000000005470c5a6fce7447afd2c9be3a0f25e362c093661000000000000000000000000479ee0363a7ac2ef34cba7ee82d2c2e0652d466900000000000000000000000000000000000000000000000000000000000018c5",
				&Contract{interfaces: []Interface{interfaceTypeERC721}},
			},
			&CallData{
				MethodID:  methodIDTransferFrom,
				Signature: methodSignatures[methodIDTransferFrom.String()],
				Args:      []string{"0x5470c5a6Fce7447aFd2C9BE3A0F25e362C093661", "0x479ee0363a7Ac2ef34cba7ee82D2C2E0652D4669", "6341"},
				Transfers: []*Transfer{{
					From:    common.HexToAddress("0x5470c5a6Fce7447aFd2C9BE3A0F25e362C093661"),
					To:      common.HexToAddress("0x479ee0363a7Ac2ef34cba7ee82D2C2E0652D4669"),
					Value:   NewAmount(common.Big1),
					TokenID: big.NewInt(6341),
				}},
			},
		},
		{
			"erc20+erc721 transferFrom(address,address,uint256)",
			decodeCallDataTestArgs{
				common.Address{},
				"23b872dd0000000000000000000000005470c5a6fce7447afd2c9be3a0f25e362c093661000000000000000000000000479ee0363a7ac2ef34cba7ee82d2c2e0652d466900000000000000000000000000000000000000000000000000000000000018c5",
				&Contract{interfaces: []Interface{interfaceTypeERC20, interfaceTypeERC721}},
			},
			&CallData{
				MethodID:  methodIDTransferFrom,
				Signature: methodSignatures[methodIDTransferFrom.String()],
				Args:      []string{"0x5470c5a6Fce7447aFd2C9BE3A0F25e362C093661", "0x479ee0363a7Ac2ef34cba7ee82D2C2E0652D4669", "6341"},
				Transfers: []*Transfer{{
					From:    common.HexToAddress("0x5470c5a6fce7447afd2c9be3a0f25e362c093661"),
					To:      common.HexToAddress("0x479ee0363a7Ac2ef34cba7ee82D2C2E0652D4669"),
					Value:   NewAmount(big.NewInt(6341)),
					TokenID: nil,
				}},
			},
		},
		{
			"erc1155 safeTransferFrom(address,address,uint256,uint256,bytes)",
			decodeCallDataTestArgs{
				common.Address{},
				"f242432a000000000000000000000000cb89354a1c6e7abd1972a68466db238e48a3b0c800000000000000000000000020964f741d2dffd2ccec658ca086e21af1d7df8e000000000000000000000000000000000000000000000000000000000000001d000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000a00000000000000000000000000000000000000000000000000000000000000000360c6ebe",
				&Contract{interfaces: []Interface{interfaceTypeERC20, interfaceTypeERC721}},
			},
			&CallData{
				MethodID:  methodIDSafeTransferFrom3,
				Signature: methodSignatures[methodIDSafeTransferFrom3.String()],
				Args:      []string{"0xcb89354a1c6e7ABd1972a68466Db238e48a3B0C8", "0x20964f741d2dfFD2cCec658CA086e21aF1D7dF8E", "29", "1"},
				Transfers: []*Transfer{{
					From:    common.HexToAddress("0xcb89354a1c6e7ABd1972a68466Db238e48a3B0C8"),
					To:      common.HexToAddress("0x20964f741d2dffd2ccec658ca086e21af1d7df8e"),
					Value:   NewAmount(common.Big1),
					TokenID: big.NewInt(29),
				}},
			},
		},
		{
			"erc1155 safeBatchTransferFrom(address,address,uint256[],uint256[],bytes)",
			decodeCallDataTestArgs{
				common.Address{},
				"2eb2c2d6000000000000000000000000381e840f4ebe33d0153e9a312105554594a98c42000000000000000000000000a2b876dbb382d40cecee2acc670f55ad95c4767e00000000000000000000000000000000000000000000000000000000000000a000000000000000000000000000000000000000000000000000000000000000e000000000000000000000000000000000000000000000000000000000000001200000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000006ed00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000000",
				&Contract{interfaces: []Interface{interfaceTypeERC20, interfaceTypeERC721}},
			},
			&CallData{
				MethodID:  methodIDSafeBatchTransferFrom,
				Signature: methodSignatures[methodIDSafeBatchTransferFrom.String()],
				Transfers: []*Transfer{{
					From:    common.HexToAddress("0x381E840F4eBe33d0153e9A312105554594A98C42"),
					To:      common.HexToAddress("0xA2b876dbb382d40cECeE2ACC670f55AD95c4767e"),
					TokenID: parseBigInt("603320636550823895720563178976525038911488"),
					Value:   NewAmount(common.Big1),
				}},
			},
		},
	}
}

func executeTests(t testing.TB, tt decodeCallDataTest) {
	input, err := hex.DecodeString(tt.args.input)
	require.NoError(t, err)

	got, err := decodeCallData(tt.args.sender, tt.args.contract, input)
	require.NoError(t, err)

	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(tt.want)
	require.Equal(t, string(wantJSON), string(gotJSON))
}

func parseBigInt(s string) *big.Int {
	b, _ := new(big.Int).SetString(s, 10)
	return b
}
