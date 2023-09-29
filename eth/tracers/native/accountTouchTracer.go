// Copyright 2022 flashbots
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package native

import (
	"encoding/json"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/eth/tracers"
)

func init() {
	tracers.DefaultDirectory.Register("accountTouchTracer", newAccountTouchTracer, false)
}

type accountTouchTracer struct {
	touched map[common.Address]struct{}
}

// newAccountTouchTracer creates new AccountTouchTracer
// that collect all addresses touched in the given tx
// including tx sender and tx.to from the top level call
func newAccountTouchTracer(ctx *tracers.Context, _ json.RawMessage) (tracers.Tracer, error) {
	t := &accountTouchTracer{
		touched: make(map[common.Address]struct{}),
	}
	return t, nil
}

func (t *accountTouchTracer) TouchedAddressesSet() map[common.Address]struct{} {
	return t.touched
}

func (t *accountTouchTracer) TouchedAddresses() []common.Address {
	result := make([]common.Address, 0, len(t.touched))

	for address := range t.touched {
		result = append(result, address)
	}
	return result
}

func (t *accountTouchTracer) CaptureTxStart(uint64) {}

func (t *accountTouchTracer) CaptureTxEnd(uint64) {}

func (t *accountTouchTracer) CaptureStart(_ *vm.EVM, from common.Address, to common.Address, _ bool, _ []byte, _ uint64, _ *big.Int) {
	t.touched[from] = struct{}{}
	t.touched[to] = struct{}{}
}

func (t *accountTouchTracer) CaptureEnd([]byte, uint64, error) {}

func (t *accountTouchTracer) CaptureEnter(_ vm.OpCode, _ common.Address, to common.Address, _ []byte, _ uint64, _ *big.Int) {
	t.touched[to] = struct{}{}
}

func (t *accountTouchTracer) CaptureExit([]byte, uint64, error) {}

func (t *accountTouchTracer) CaptureState(_ uint64, op vm.OpCode, _, _ uint64, scope *vm.ScopeContext, _ []byte, _ int, _ error) {
	stack := scope.Stack
	stackData := stack.Data()
	stackLen := len(stackData)
	if (op == vm.EXTCODECOPY || op == vm.EXTCODEHASH || op == vm.EXTCODESIZE || op == vm.BALANCE || op == vm.SELFDESTRUCT) && stackLen >= 1 {
		addr := common.Address(stackData[stackLen-1].Bytes20())
		t.touched[addr] = struct{}{}
	}
}

func (t *accountTouchTracer) CaptureFault(uint64, vm.OpCode, uint64, uint64, *vm.ScopeContext, int, error) {
}

// GetResult returns an empty json object.
func (t *accountTouchTracer) GetResult() (json.RawMessage, error) {
	res, err := json.Marshal(t.TouchedAddresses())
	if err != nil {
		return nil, err
	}
	return json.RawMessage(res), nil
}

// Stop terminates execution of the tracer at the first opportune moment.
func (t *accountTouchTracer) Stop(err error) {
}
