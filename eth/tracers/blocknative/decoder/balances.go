package decoder

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// balanceTracker represents the difference of value (ETH, erc20, erc721) after the transaction for all addresses.
type balances struct {
	balanceChanges balanceChangeByOwnerByAsset
}

// newBalanceChangeTracker creates a new balanceTracker.
func newBalances() *balances {
	return &balances{
		balanceChanges: make(balanceChangeByOwnerByAsset, 4),
	}
}

// captureCallFrameStart decodes potential balance change data out of calldata.
func (bt *balances) captureCallFrameStart(sender common.Address, receiver common.Address, value *Amount, decoded *CallFrame) {
	// Add the native transfer.
	if value != nil && value.ToInt().Sign() > 0 {
		bt.balanceChanges.addAssetTransfer(EthAsset, sender, receiver, value)
	}
}

func (bt *balances) captureCallFrameEnd(decoded *CallFrame) {
	if decoded == nil {
		return
	}

	// Add any decoded transfers.
	for _, transfer := range decoded.Transfers {
		// Add the decoded transfer now.
		bt.balanceChanges.addAssetTransfer(transfer.Asset, transfer.From, transfer.To, transfer.Value)
	}
}

// captureGas adds the gas payments to the balance changes.
func (bt *balances) captureGas(origin common.Address, coinbase common.Address, gasUsed uint64, gasFee *big.Int, gasBaseFee *big.Int) {
	gasUsedBig := new(big.Int).SetUint64(gasUsed)

	// Calculate the gas cost for the base fee.
	gasCostBase := new(big.Int).Mul(gasBaseFee, gasUsedBig)
	gasCostBase.Neg(gasCostBase)

	// The tip is the difference between the gas fee and the base fee.
	gasCostTip := new(big.Int).Sub(gasFee, gasBaseFee)
	gasCostTip.Mul(gasCostTip, gasUsedBig)

	// Add the gas tip as a two-way transfer between origin and coinbase.
	bt.balanceChanges.accountAssetChange(origin, common.Address{}, EthAsset, NewAmount(gasCostBase))

	// Add the base fee as a one-way transfer from origin to empty address.
	bt.balanceChanges.addAssetTransfer(EthAsset, origin, coinbase, NewAmount(gasCostTip))
}

// formatNetBalanceChanges aggregates the balanceChanges into a NetBalanceChanges.
func (bt *balances) formatNetBalanceChanges() NetBalanceChanges {
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

// balanceChangeByAsset is a map of asset address to BalanceChange.
type balanceChangeByAsset map[AssetID]BalanceChange

// balanceChangeByOwnerByAsset is a map of owner address to balanceChangeByAsset.
type balanceChangeByOwnerByAsset map[common.Address]balanceChangeByAsset

// addAssetTransfer adds a double-sided asset transfer to the balanceChangeByOwnerByAsset map.
func (changeMap balanceChangeByOwnerByAsset) addAssetTransfer(asset *Asset, from, to common.Address, value *Amount) {
	changeMap.accountAssetChange(to, from, asset, value)
	changeMap.accountAssetChange(from, to, asset, value.Neg())
}

// accountAssetChange adds a single side change of an asset transfer to the balanceChangeByOwnerByAsset map.
func (changeMap balanceChangeByOwnerByAsset) accountAssetChange(owner common.Address, counterparty common.Address, asset *Asset, delta *Amount) {
	// If this is the first time we've seen this owner then create a new
	// balanceByAsset map.
	if _, ok := changeMap[owner]; !ok {
		changeMap[owner] = make(balanceChangeByAsset)
	}

	// If this is the first time we've seen this asset for this owner then
	// create a new BalanceChange, including loading the asset metadata.
	if _, ok := changeMap[owner][asset.AssetID]; !ok {
		changeMap[owner][asset.AssetID] = BalanceChange{
			Delta: NewAmount(big.NewInt(0)),
			Asset: asset,
		}
	}

	// Update the delta and add a breakdown event.
	ownerBalanceChange := changeMap[owner][asset.AssetID]
	ownerBalanceChange.Delta.Add(ownerBalanceChange.Delta, delta)
	ownerBalanceChange.Breakdown = append(ownerBalanceChange.Breakdown, AssetTransferEvent{
		Counterparty: counterparty,
		Amount:       delta,
	})
	changeMap[owner][asset.AssetID] = ownerBalanceChange
}

// NetBalanceChanges is a list of account balance changes.
type NetBalanceChanges []AccountBalanceChanges

// AccountBalanceChanges is a list of balance changes for a single account.
type AccountBalanceChanges struct {
	Address        common.Address  `json:"address"`
	BalanceChanges []BalanceChange `json:"balanceChanges"`
}

// BalanceChange is a change in an account's balance for a single asset.
type BalanceChange struct {
	Delta     *Amount              `json:"delta"`
	Asset     *Asset               `json:"asset"`
	Breakdown []AssetTransferEvent `json:"breakdown"`
}

// AssetTransferEvent is a single transfer of an asset.
type AssetTransferEvent struct {
	Counterparty common.Address `json:"counterparty"`
	Amount       *Amount        `json:"amount"`
}
