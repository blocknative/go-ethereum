package blocknative

import (
	"bytes"
	"fmt"
	"github.com/ethereum/go-ethereum/log"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

const CHANGE_MAP_PRE_ALLOCATION_SIZE = 4

// balanceTracker represents the difference of value (ETH, erc20, erc721) after the transaction for all addresses.
type balanceTracker struct {
	stateDB     balanceDB
	assetGetter assetGetter

	//pre                  accountSnapshotsMap
	//nativeBalanceChanges amountsMap
	assetTransfers []assetTransfer
}

// newBalanceChangeTracker creates a new balanceTracker.
func newBalanceChangeTracker(stateDB balanceDB, assetGetter assetGetter) *balanceTracker {
	return &balanceTracker{
		stateDB:     stateDB,
		assetGetter: assetGetter,

		//pre:                  make(accountSnapshotsMap, 4),
		//nativeBalanceChanges: make(amountsMap, CHANGE_MAP_PRE_ALLOCATION_SIZE),
		assetTransfers: []assetTransfer{},
	}
}

// captureStart begins the balance change tracking process.
func (bt *balanceTracker) captureStart(from common.Address, to common.Address, value *big.Int, input []byte, _ common.Address, _ uint64, _ *big.Int) {
	//bt.lookupAccount(from)
	//bt.lookupAccount(to)
	//bt.lookupAccount(coinbase)
	bt.captureCall(from, to, value, input)
	//// Update the to address
	//// The recipient balance includes the value transferred.
	//bt.pre[to].balance.Sub(bt.pre[to].balance, value)
	//
	//// Update the from address to include the gas cost and value transferred.
	//// TODO: Move this to the end because we can't know the entire gas consumed yet
	//consumedGas := new(big.Int).SetUint64(gas)
	//gasCost := consumedGas.Mul(consumedGas, gasPrice)
	//bt.pre[from].balance.Add(bt.pre[from].balance, gasCost)
	//bt.pre[from].balance.Add(bt.pre[from].balance, value)
}

// captureCall decodes potential balance change data out of calldata.
func (bt *balanceTracker) captureCall(sender common.Address, contract common.Address, value *big.Int, input []byte) {
	if value == nil {
		fmt.Println("value is nil 57")
	}
	// Handle native transfers
	bt.assetTransfers = append(bt.assetTransfers, assetTransfer{
		From:     sender,
		To:       contract,
		Contract: common.Address{},
		Asset:    ethAsset,
		Amount:   Amount{value},
	})

	// Check that the input is capable of being a function call selector.
	// If not then we're done.
	if len(input) < 4 {
		return
	}

	var (
		idx      = 4
		methodID = input[:idx]
		from     common.Address
		to       common.Address
		amount   = new(big.Int)
	)

	// scanWord gets the next 32 bytes and advances the index
	scanWord := func() []byte {
		word := input[idx : idx+32]
		idx += 32
		return word
	}

	switch {

	// Transfer event; payload is [to, amount]
	case bytes.Compare(methodID, methodIDERC20Transfer) == 0:
		if len(input) < 68 {
			return
		}

		from = sender
		to = common.BytesToAddress(scanWord())
		amount.SetBytes(scanWord())

	// (Safe)TransferFrom event; payload is [from, to, amount]
	case bytes.Compare(methodID, methodIDERC20TransferFrom) == 0:
		fallthrough
	case bytes.Compare(methodID, methodIDERC721TransferFrom) == 0:
		fallthrough
	case bytes.Compare(methodID, methodIDERC721SafeTransfer) == 0:
		fallthrough
	case bytes.Compare(methodID, methodIDERC721SafeTransferWithData) == 0:
		if len(input) < 100 {
			return
		}

		from = common.BytesToAddress(scanWord())
		to = common.BytesToAddress(scanWord())
		amount.SetBytes(scanWord())

	// Not a matching event; ignore
	default:
		return
	}

	// Attempt to load metadata, but don't fail if we don't.
	asset, err := bt.assetGetter(contract)
	if err != nil {
		log.Trace("failed to read token metadata", "err", err)
	}
	if value == nil {
		fmt.Println("amount is nil 128")
	}
	// Append a new token transfer object
	bt.assetTransfers = append(bt.assetTransfers, assetTransfer{
		From:     from,
		To:       to,
		Contract: contract,
		Asset:    asset,
		Amount:   Amount{amount},
	})
}

// lookupAccount fetches details of an account and adds it to the pre-state.
//func (bt *balanceTracker) lookupAccount(addr common.Address) {
//	if _, ok := bt.pre[addr]; ok {
//		return
//	}
//
//	bt.pre[addr] = &accountSnapshot{
//		bt.stateDB.GetBalance(addr),
//	}
//}

func (bt *balanceTracker) calculateNetBalanceChanges() NetBalanceChanges {
	// Iterate through touched accounts and calculate native balance changes.
	//for addr, preState := range bt.pre {
	//	// Calculate the balance difference. If it's zero just continue.
	//	balance := bt.stateDB.GetBalance(addr)
	//	delta := new(big.Int).Sub(preState.balance, balance)
	//	if delta.Sign() == 0 {
	//		continue
	//	}
	//
	//	// Add the diff to native balance change map.
	//	amount := new(big.Int).Sub(preState.balance, balance)
	//	bt.nativeBalanceChanges[addr] = Amount{amount}
	//}

	// Collate token changes
	// TODO(TS): Cleanup/use real algorithm
	// Map of Account address -> [Map of token address -> Aggregated change]
	accountTokenChanges := map[common.Address]map[common.Address]AssetBalanceChange{}
	for _, transfer := range bt.assetTransfers {
		if _, ok := accountTokenChanges[transfer.From]; !ok {
			accountTokenChanges[transfer.From] = map[common.Address]AssetBalanceChange{}
		}
		if _, ok := accountTokenChanges[transfer.To]; !ok {
			accountTokenChanges[transfer.To] = map[common.Address]AssetBalanceChange{}
		}

		if _, ok := accountTokenChanges[transfer.From][transfer.Contract]; !ok {
			asset, err := bt.assetGetter(transfer.Contract)
			if err != nil {
				log.Trace("failed to read token metadata", "err", err)
			}
			accountTokenChanges[transfer.From][transfer.Contract] = AssetBalanceChange{
				Delta: Amount{big.NewInt(0)},
				Asset: asset,
			}
		}
		if _, ok := accountTokenChanges[transfer.To][transfer.Contract]; !ok {
			asset, err := bt.assetGetter(transfer.Contract)
			if err != nil {
				log.Trace("failed to read token metadata", "err", err)
			}
			accountTokenChanges[transfer.To][transfer.Contract] = AssetBalanceChange{
				Delta: Amount{big.NewInt(0)},
				Asset: asset,
			}
		}

		fromChanges := accountTokenChanges[transfer.From][transfer.Contract]
		toChanges := accountTokenChanges[transfer.To][transfer.Contract]

		fmt.Println("accountTokenChanges[transfer.From][transfer.Contract].Delta.Int:", accountTokenChanges[transfer.From][transfer.Contract].Delta.Int)
		fmt.Println("transfer.Amount.Int:", transfer.Amount.Int)
		accountTokenChanges[transfer.From][transfer.Contract].Delta.Int.Sub(accountTokenChanges[transfer.From][transfer.Contract].Delta.Int, transfer.Amount.Int)
		accountTokenChanges[transfer.To][transfer.Contract].Delta.Int.Add(accountTokenChanges[transfer.To][transfer.Contract].Delta.Int, transfer.Amount.Int)
		fromChanges.Breakdown = append(fromChanges.Breakdown, transfer)
		toChanges.Breakdown = append(toChanges.Breakdown, transfer)

		accountTokenChanges[transfer.From][transfer.Contract] = fromChanges
		accountTokenChanges[transfer.To][transfer.Contract] = toChanges
	}

	// Turn the map into a list of [{addr, [{token, change}]}]
	netBalanceChanges := make(NetBalanceChanges, len(accountTokenChanges))
	for addr, changes := range accountTokenChanges {
		abc := AddressBalanceChanges{
			Address: addr,
		}
		for _, change := range changes {
			abc.BalanceChanges = append(abc.BalanceChanges, change)
		}
		netBalanceChanges = append(netBalanceChanges, abc)
	}

	return netBalanceChanges
}

// balanceDB is an interface that provides access to the stateDB.
type balanceDB interface {
	GetBalance(common.Address) *big.Int
}

// assetGetter is a function that returns the metadata for a given asset address.
type assetGetter func(common.Address) (*Asset, error)
