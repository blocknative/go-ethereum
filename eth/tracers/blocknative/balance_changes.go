package blocknative

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/eth/tracers/blocknative/decoder"
	"github.com/ethereum/go-ethereum/log"
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

// captureCall decodes potential balance change data out of calldata.
func (bt *balanceTracker) captureCall(sender common.Address, contract common.Address, value *big.Int, input []byte) {
	// Add the native transfer.
	bt.balanceChanges.addAssetTransfer(bt, sender, contract, decoder.EthAssetID, value)

	// Decode the call data and if it's a transfer then add it.
	decodedCall := decoder.DecodeCalldata(sender, input)
	if decodedCall == nil || decodedCall.CallType != decoder.AssetTransfer {
		return
	}
	asset := decoder.AssetID{Address: contract, TokenID: decodedCall.TokenID}
	bt.balanceChanges.addAssetTransfer(bt, decodedCall.From, decodedCall.To, asset, decodedCall.Value)
}

// captureGas adds the gas payments to the balance changes.
func (bt *balanceTracker) captureGas(origin common.Address, coinbase common.Address, gasUsed uint64, gasFee *big.Int, gasBaseFee *big.Int) {
	gasUsedBig := new(big.Int).SetUint64(gasUsed)

	// Calculate the gas cost for the base fee.
	gasCostBase := new(big.Int).Mul(gasBaseFee, gasUsedBig)
	gasCostBase.Neg(gasCostBase)

	// The tip is the difference between the gas fee and the base fee.
	gasCostTip := new(big.Int).Sub(gasFee, gasBaseFee)
	gasCostTip.Mul(gasCostTip, gasUsedBig)

	// Add the gas tip as a two-way transfer between origin and coinbase.
	bt.balanceChanges.accountAssetChange(bt, origin, common.Address{}, decoder.EthAssetID, gasCostBase)

	// Add the base fee as a one-way transfer from origin to empty address.
	bt.balanceChanges.addAssetTransfer(bt, origin, coinbase, decoder.EthAssetID, gasCostTip)
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
type assetGetter func(decoder.AssetID) (*decoder.Asset, error)

// balanceChangeByAsset is a map of asset address to BalanceChange.
type balanceChangeByAsset map[decoder.AssetID]BalanceChange

// balanceChangeByOwnerByAsset is a map of owner address to balanceChangeByAsset.
type balanceChangeByOwnerByAsset map[common.Address]balanceChangeByAsset

// accountAssetChange adds a single side change of an asset transfer to the balanceChangeByOwnerByAsset map.
func (changeMap balanceChangeByOwnerByAsset) accountAssetChange(bt *balanceTracker, owner common.Address, counterparty common.Address, assetID decoder.AssetID, delta *big.Int) {
	// If this is the first time we've seen this owner then create a new
	// balanceByAsset map.
	if _, ok := changeMap[owner]; !ok {
		changeMap[owner] = make(balanceChangeByAsset)
	}

	// If this is the first time we've seen this asset for this owner then
	// create a new BalanceChange, including loading the asset metadata.
	if _, ok := changeMap[owner][assetID]; !ok {
		asset, err := bt.assetGetter(assetID)
		if err != nil {
			log.Trace("failed to read token metadata", "err", err)
		}
		changeMap[owner][assetID] = BalanceChange{
			Delta: Amount{big.NewInt(0)},
			Asset: asset,
		}
	}

	// Update the delta and add a breakdown event.
	ownerBalanceChange := changeMap[owner][assetID]
	ownerBalanceChange.Delta.Int.Add(ownerBalanceChange.Delta.Int, delta)
	ownerBalanceChange.Breakdown = append(ownerBalanceChange.Breakdown, AssetTransferEvent{
		Counterparty: counterparty,
		Amount:       Amount{delta},
	})
	changeMap[owner][assetID] = ownerBalanceChange

}

// addAssetTransfer adds a double-sided asset transfer to the balanceChangeByOwnerByAsset map.
func (changeMap balanceChangeByOwnerByAsset) addAssetTransfer(bt *balanceTracker, from common.Address, to common.Address, assetID decoder.AssetID, value *big.Int) {
	if value == nil || value.Sign() == 0 {
		return
	}

	changeMap.accountAssetChange(bt, to, from, assetID, value)
	negValue := new(big.Int).Neg(value)
	changeMap.accountAssetChange(bt, from, to, assetID, negValue)
}
