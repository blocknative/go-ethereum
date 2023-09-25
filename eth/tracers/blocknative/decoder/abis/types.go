package abis

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

const MD5HashSize = 16

type MD5Hash [MD5HashSize]byte
type ABI struct {
	HashMD5      MD5Hash
	ContractType string
	ABI          *abi.ABI
}
type Contract struct {
	HashMD5      MD5Hash
	Address      common.Address
	ContractName string
}

type Method struct {
	Selector  string
	Signature string
	ABI       *abi.ABI
}
