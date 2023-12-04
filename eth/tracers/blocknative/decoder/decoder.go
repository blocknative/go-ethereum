package decoder

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

var (
	ErrAccountNotAContract  = errors.New("account is not a contract")
	ErrCallDataTooShort     = errors.New("call data too short")
	ErrCallDataNotFullWords = errors.New("call data not full words")
)

type Decoder struct {
	caches   *Caches
	evm      evm
	balances *balances
}

func New(caches *Caches, evm evm) *Decoder {
	return &Decoder{
		caches:   caches,
		evm:      evm,
		balances: newBalances(),
	}
}

// DecodeCallFrame decodes the given call frame into its method and arguments.
// If the call frame is determined to represent one or more Asset transfers we
// add those too.
func (d *Decoder) DecodeCallFrame(sender common.Address, receiver common.Address, value *big.Int, input []byte) (*CallFrame, error) {
	// Always account for any eth transferred.
	d.balances.captureNativeTransfer(sender, receiver, value)

	// Decode the contract. As long as we can decode a contract we'll return
	// a CallFrame object.
	contract, err := d.DecodeContract(receiver)
	if err != nil {
		return nil, err
	}
	cf := &CallFrame{Contract: contract}

	// Try to decode the call data. If we fail we'll still return the CallFrame
	// we have.
	callData, err := decodeCallData(sender, contract, input)
	if err != nil {
		return cf, err
	}
	cf.CallData = callData

	// Decode and capture any calldata transfers.
	for _, transfer := range callData.Transfers {
		// Always add the assetID.
		assetID := AssetID{Address: receiver, TokenID: transfer.TokenID}
		transfer.Asset = &Asset{AssetID: assetID}

		// Add metadata if we can decode it but don't fail if we can't.
		assetMetadata, err := d.decodeAsset(contract, assetID)
		if err != nil {
			log.Trace("failed to decode asset", "err", err)
			continue
		}
		transfer.Asset.AssetMetadata = assetMetadata

		// Account for the balance changes.
		d.balances.balanceChanges.addAssetTransfer(transfer.Asset, transfer.From, transfer.To, transfer.Value)
	}

	return cf, nil
}

func (d *Decoder) DecodeCallFrameEnd(cf *CallFrame) error {
	if cf == nil || cf.Contract == nil || cf.CallData == nil {
		return nil
	}

	// Check updated balances for taxable transfers and look for active taxes.
	// If we find them, add them as new transfers.
	transfers := cf.CallData.Transfers
	for _, transfer := range transfers {
		// First check if we set a balanceBeforeTo. If we didn't then we can't
		// utilize this inference.
		if transfer.balanceBeforeTo == nil {
			continue
		}

		// Get the balance after the transfer. At this point balanceOf worked
		// once so we don't expect it to fail. If it does then we report it as
		// an error so we can inspect later, and then we continue to the next
		// transfer.
		var (
			err            error
			balanceAfterTo *big.Int
		)
		switch cf.Contract.Type {
		case ContractTypeERC20:
			if balanceAfterTo, err = evmCallMethodBalanceOf(d.evm.CallCode, cf.Contract.address, transfer.To); err != nil {
				log.Error("failed to get balance after for receiver", "err", err)
				continue
			}
		case ContractTypeERC1155:
			if balanceAfterTo, err = evmCallMethodBalanceOf2(d.evm.CallCode, cf.Contract.address, transfer.To, transfer.TokenID); err != nil {
				log.Error("failed to get balance after for receiver", "err", err)
				continue
			}
		default:
			continue
		}

		// If the increase in balance of the transfer recipient is less than we
		// decoded it to be, then we know that the transfer was taxed.
		//
		// We don't know where the tax went but it will often go to the contract
		// itself so we check the contract balance to see if that's the case.
		decodedValue := transfer.Value.ToInt()
		deltaTo := balanceAfterTo.Sub(balanceAfterTo, transfer.balanceBeforeTo)
		if deltaTo.Cmp(decodedValue) < 0 {
			// First, update the transfer value to the actual delta.
			transfer.Value = NewAmount(deltaTo)

			// Now let's see if the contract was the tax recipient.
			balanceAfterContract, err := evmCallMethodBalanceOf(d.evm.CallCode, cf.Contract.address, cf.Contract.address)
			if err != nil {
				log.Error("failed to get balance after for contract", "err", err)
				continue
			}
			deltaContract := balanceAfterContract.Sub(balanceAfterContract, transfer.balanceBeforeContract)
			if deltaContract.Sign() == 1 {
				// The contract balance increased so we know at least some of
				// the tax went there. Add a tax transfer.
				cf.Transfers = append(cf.Transfers, &Transfer{
					Asset: transfer.Asset,
					From:  transfer.From,

					To:    cf.Contract.address,
					Value: NewAmount(deltaContract),
				})
			}

			// Check to see if there was a tax amount not accounted for by
			// subtracting the known deltas from the decoded value.
			unaccountedTax := new(big.Int).Add(deltaTo, deltaContract)
			unaccountedTax.Sub(decodedValue, unaccountedTax)
			if unaccountedTax.Sign() == 1 {
				cf.Transfers = append(cf.Transfers, &Transfer{
					Asset: transfer.Asset,
					From:  transfer.From,

					To:    common.Address{},
					Value: NewAmount(unaccountedTax),
				})
			}
		}
	}

	d.balances.captureCallFrameEnd(cf)

	return nil
}

// DecodeContract decodes the contract at the given address.
func (d *Decoder) DecodeContract(addr common.Address) (*Contract, error) {
	// Check the cache for an existing entry.
	contract, ok := d.caches.contracts.Get(addr)
	if ok {
		if contract == nil {
			return nil, ErrAccountNotAContract
		}
		return contract, nil
	}

	// Cache miss; First check if the account has code. If it doesn't we cache
	// a nil to avoid hitting the stateDB repeatedly for accounts that don't
	// have code.
	bytecode := ByteCode(d.evm.GetCode(addr))
	if len(bytecode) == 0 {
		d.caches.contracts.Add(addr, nil)
		return nil, ErrAccountNotAContract
	}

	// We have an unknown contract; decode itm, add it to the cache, and return it.
	contract = &Contract{address: addr}
	decodeContract(contract, bytecode)
	d.caches.contracts.Add(addr, contract)
	return contract, nil
}

// GetBalanceChanges returns the net balance changes for the currently decoded
// call-frames.
func (d *Decoder) GetBalanceChanges() NetBalanceChanges {
	return d.balances.formatNetBalanceChanges()
}

// CaptureGas is a hack to expose the captureGas method to the tracer.
// Ideally this would happen internally but requires the tracer to cal
// DecodeCallFrame in CaptureEnd/CaptureExit instead of Start/Enter.
func (d *Decoder) CaptureGas(origin common.Address, coinbase common.Address, gasUsed uint64, gasFee *big.Int, gasBaseFee *big.Int) {
	d.balances.captureGas(origin, coinbase, gasUsed, gasFee, gasBaseFee)
}

// decodeAsset finds the metadata for the given assetID. It heuristically uses
// information from the decoded contract when possible.
// Results are cached.
func (d *Decoder) decodeAsset(contract *Contract, assetID AssetID) (*AssetMetadata, error) {
	// Check for native ETH and skip the cache check entire.
	if assetID.Address == ethAddress {
		return EthAsset.AssetMetadata, nil
	}

	// Check the cache for an existing entry.
	asset, ok := d.caches.assets.Get(assetID)
	if ok {
		return asset, nil
	}

	// Cache miss; decode and add to the cache.
	asset, err := DecodeAsset(d.evm.CallCode, contract, assetID)
	if err != nil {
		return nil, err
	}
	d.caches.assets.Add(assetID, asset)
	return asset, nil
}

// evm is the functionality we need from the EVM to decode.
type evm interface {
	GetCode(common.Address) []byte
	CallCode(common.Address, []byte) ([]byte, error)
}
