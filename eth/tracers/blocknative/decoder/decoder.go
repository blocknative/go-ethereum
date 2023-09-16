package decoder

import (
	"errors"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"math/big"
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

func (d *Decoder) DecodeCallFrame(sender common.Address, receiver common.Address, value *big.Int, input []byte) (*CallFrame, error) {
	contract, err := d.DecodeContract(receiver)
	if err != nil {
		return nil, err
	}

	callData, err := decodeCallData(sender, contract, input)
	if err != nil {
		return nil, err
	}

	// Add decoded Assets to any transfers.
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
	}

	cf := &CallFrame{Contract: contract, CallData: callData}

	d.balances.captureCall(sender, receiver, NewAmount(value), cf)

	return cf, nil
}

// DecodeContract decodes the contract at the given address.
func (d *Decoder) DecodeContract(addr common.Address) (*Contract, error) {
	// Check the cache for an existing entry.
	d.caches.contractsMu.RLock()
	contract, ok := d.caches.contracts.Get(addr)
	d.caches.contractsMu.RUnlock()
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
		d.caches.contractsMu.Lock()
		d.caches.contracts.Add(addr, nil)
		d.caches.contractsMu.Unlock()
		return nil, ErrAccountNotAContract
	}

	// We have an unknown contract; decode itm, add it to the cache, and return it.
	contract, err := decodeContract(bytecode)
	if err != nil {
		return nil, err
	}
	d.caches.contractsMu.Lock()
	d.caches.contracts.Add(addr, contract)
	d.caches.contractsMu.Unlock()
	return contract, nil
}

func (d *Decoder) GetBalanceChanges() NetBalanceChanges {
	return d.balances.formatNetBalanceChanges()
}

// CaptureGas is a hack to expose the captureGas method to the tracer.
// Ideally this would happen internally but requires the tracer to cal
// DecodeCallFrame in CaptureEnd/CaptureExit instead of Start/Enter.
func (d *Decoder) CaptureGas(origin common.Address, coinbase common.Address, gasUsed uint64, gasFee *big.Int, gasBaseFee *big.Int) {
	d.balances.captureGas(origin, coinbase, gasUsed, gasFee, gasBaseFee)
}

func (d *Decoder) decodeAsset(contract *Contract, assetID AssetID) (*AssetMetadata, error) {
	// Check for native ETH and skip the cache check entire.
	if assetID.Address == ethAddress {
		return EthAsset.AssetMetadata, nil
	}

	// Check the cache for an existing entry.
	d.caches.assetsMu.RLock()
	asset, ok := d.caches.assets.Get(assetID)
	d.caches.assetsMu.RUnlock()
	if ok {
		return asset, nil
	}

	// Cache miss; decode and add to the cache.

	asset, err := DecodeAsset(d.evm.CallCode, contract, assetID)
	if err != nil {
		return nil, err
	}

	d.caches.assetsMu.Lock()
	d.caches.assets.Add(assetID, asset)
	d.caches.assetsMu.Unlock()

	return asset, nil
}

type evm interface {
	GetCode(common.Address) []byte
	CallCode(common.Address, []byte) ([]byte, error)
}

type EVMCallFn func(addr common.Address, method []byte) ([]byte, error)
type GetCodeFn func(addr common.Address) []byte
