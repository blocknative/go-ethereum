package blocknative

import (
	"bytes"
	"github.com/ethereum/go-ethereum/log"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// balanceTracker represents the difference of value (ETH, erc20, erc721) after the transaction for all addresses.
type balanceTracker struct {
	stateDB        stateDB
	assetGetter    assetGetter
	balanceChanges balanceChangeByOwnerByAsset
}

// newBalanceChangeTracker creates a new balanceTracker.
func newBalanceChangeTracker(stateDB stateDB, assetGetter assetGetter) *balanceTracker {
	return &balanceTracker{
		stateDB:        stateDB,
		assetGetter:    assetGetter,
		balanceChanges: make(balanceChangeByOwnerByAsset, 4),
	}
}

// captureStart begins the balance change tracking process.
func (bt *balanceTracker) captureStart(from common.Address, to common.Address, value *big.Int, input []byte, _ common.Address, _ uint64, _ *big.Int) {
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
	// Add the native transfer
	bt.balanceChanges.addAssetTransfer(bt, sender, contract, ethAddress, value)

	// Check if the input is capable of being a function call selector.
	// If not then we're done. If so then check if it's a transfer call.
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

	// Add the token transfer
	bt.balanceChanges.addAssetTransfer(bt, from, to, contract, amount)
}

// formatNetBalanceChanges aggregates the balanceChanges into a NetBalanceChanges.
func (bt *balanceTracker) formatNetBalanceChanges() NetBalanceChanges {
	// Turn the balanceChanges map into a list of [{addr, [{token, change}]}]
	netBalanceChanges := make(NetBalanceChanges, 0, len(bt.balanceChanges))
	for addr, changes := range bt.balanceChanges {
		abc := AccountBalanceChanges{Address: addr}
		for _, change := range changes {
			abc.BalanceChanges = append(abc.BalanceChanges, change)
		}
		netBalanceChanges = append(netBalanceChanges, abc)
	}
	return netBalanceChanges
}

// stateDB is an interface that provides access to account balances.
type stateDB interface {
	GetBalance(common.Address) *big.Int
}

// assetGetter is a function that returns the metadata for a given asset address.
type assetGetter func(common.Address) (*Asset, error)

// balanceChangeByAsset is a map of asset address to BalanceChange.
type balanceChangeByAsset map[common.Address]BalanceChange

// balanceChangeByOwnerByAsset is a map of owner address to balanceChangeByAsset.
type balanceChangeByOwnerByAsset map[common.Address]balanceChangeByAsset

// addAssetChange adds a single-sided asset change to the balanceChangeByOwnerByAsset map.
func (changeMap balanceChangeByOwnerByAsset) addAssetChange(bt *balanceTracker, owner common.Address, counterparty common.Address, assetAddr common.Address, delta *big.Int) {
	// If this is the first time we've seen this owner then create a new
	// balanceByAsset map.
	if _, ok := changeMap[owner]; !ok {
		changeMap[owner] = make(balanceChangeByAsset)
	}

	// If this is the first time we've seen this asset for this owner then
	// create a new BalanceChange, including loading the asset metadata.
	if _, ok := changeMap[owner][assetAddr]; !ok {
		asset, err := bt.assetGetter(assetAddr)
		if err != nil {
			log.Trace("failed to read token metadata", "err", err)
		}
		changeMap[owner][assetAddr] = BalanceChange{
			Delta: Amount{big.NewInt(0)},
			Asset: asset,
		}
	}

	// Update the delta and add a breakdown event.
	ownerBalanceChange := changeMap[owner][assetAddr]
	ownerBalanceChange.Delta.Int.Add(ownerBalanceChange.Delta.Int, delta)
	ownerBalanceChange.Breakdown = append(ownerBalanceChange.Breakdown, AssetTransferEvent{
		Counterparty: counterparty,
		Amount:       Amount{delta},
	})
	changeMap[owner][assetAddr] = ownerBalanceChange

}

// addAssetTransfer adds a double-sided asset transfer to the balanceChangeByOwnerByAsset map.
func (changeMap balanceChangeByOwnerByAsset) addAssetTransfer(bt *balanceTracker, from common.Address, to common.Address, asset common.Address, value *big.Int) {
	if value == nil || value.Sign() == 0 {
		return
	}

	changeMap.addAssetChange(bt, to, from, asset, value)
	negValue := new(big.Int).Neg(value)
	changeMap.addAssetChange(bt, from, to, asset, negValue)
}
