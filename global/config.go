package global

import (
	"crypto/ecdsa"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
)

type Vesuvius struct {
	Address common.Address
	Pk      string
	RawPk   *ecdsa.PrivateKey
	// passing eth client in this way is sort of a post bad structural decision type of hack
	EthClient *ethclient.Client
}

// account settings

var accountAddress = "0x2611CE081FA996e3E61cA6a7d5269629A224B72f"
var accountPk = "7f873146a491048110c8d2d561e572746729745264c081493656169c82893822"
var Vesuvius_x0 Vesuvius

// network settings
var CHAIN_ID = big.NewInt(5) // 5 = goerli

// addresses of interest
var USDC_ADDRESS = common.HexToAddress("0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c")
var cETH_ADDRESS = common.HexToAddress("0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c")
var ETHFLI2X_ADDRESS = common.HexToAddress("0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c")

const GETH_IPC = "/Users/dmarz/Library/Ethereum/goerli/geth.ipc"

// initialize
func InitConfig(ethClient *ethclient.Client) {
	client, err := ethclient.Dial(GETH_IPC)
	log.Info("vesuvius_x0", "InitConfig", "starting")
	Vesuvius_x0.EthClient = client
	Vesuvius_x0.Address = common.HexToAddress(accountAddress)
	Vesuvius_x0.Pk = accountPk
	rawPk, err := crypto.HexToECDSA(accountPk)
	if err != nil {
		log.Info("vesuvius_x0", "HexToECDSA", "error")
	}
	Vesuvius_x0.RawPk = rawPk
	log.Info("vesuvius_x0", "InitConfig", "finished")
}
