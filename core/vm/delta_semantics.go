// Copyright 2014 The go-ethereum Authors
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

package vm

import (
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/research"
)

// DeltaSemantics contains recorded traces and substrate traces
type DeltaSemantics struct {
	MaxGas       uint64
	MaxInstCount int64

	// Information recorded from EVM
	RecordedTrace      *research.EVMTrace
	RecordedAllocCount int

	// SubstrateInterpreter and TACInterpreter
	SubstrateTrace     *research.EVMTrace
	AllocCount         int
	UsedEVM            bool
	SubstrateResult    string
	SubstrateErr       error
	SubstrateInstCount int64
}

func NewDeltaSemantics(evm *EVM, substate *research.Substate, trace *research.EVMTrace) *DeltaSemantics {
	maxInstCount := tacMaxInstCountValue
	if tacGasInstCountValue {
		txGasUsage := int64(*substate.Result.GasUsed)
		if maxInstCount < txGasUsage {
			maxInstCount = txGasUsage
		}
	}
	return &DeltaSemantics{
		MaxGas:       EXT_DEFAULT_MAX_GAS,
		MaxInstCount: maxInstCount,

		RecordedTrace:      trace,
		RecordedAllocCount: SubstatePreAllocCount(substate),

		SubstrateTrace:     evm.StateDB.(*state.StateDB).ResearchEVMTrace,
		AllocCount:         0,
		UsedEVM:            false,
		SubstrateResult:    "",
		SubstrateErr:       nil,
		SubstrateInstCount: 0,
	}
}

func (delta *DeltaSemantics) CheckOutOfGas() bool {
	recTrace := delta.RecordedTrace
	subTrace := delta.SubstrateTrace

	i := len(subTrace.CallTraces) - 1
	if i >= len(recTrace.CallTraces) {
		return false
	}

	recErr := recTrace.CallTraces[i].Err
	if recErr == nil || recErr.Error() != ErrOutOfGas.Error() {
		return false
	}

	if recTrace.SstoreCount <= subTrace.SstoreCount && // number of SSTOREs
		len(recTrace.EventTraces) <= len(subTrace.EventTraces) && // number of LOGs
		delta.RecordedAllocCount <= delta.AllocCount { // number of addresses
		return true
	}
	return false
}

// SubstatePreAllocCount counts number of unique addresses in input and output
// substates except the coinbase address.
func SubstatePreAllocCount(substate *research.Substate) int {
	addrSet := make(map[common.Address]struct{})
	for _, entry := range substate.GetInputAlloc().GetAlloc() {
		addr := research.BytesToAddress(entry.GetAddress())
		if addr != nil {
			addrSet[*addr] = struct{}{}
		}
	}
	for _, entry := range substate.GetOutputAlloc().GetAlloc() {
		addr := research.BytesToAddress(entry.GetAddress())
		if addr != nil {
			addrSet[*addr] = struct{}{}
		}
	}
	// exclude coinbase from AllocCount
	coinbase := research.BytesToAddress(substate.GetBlockEnv().GetCoinbase())
	if coinbase != nil {
		delete(addrSet, *coinbase)
	}
	return len(addrSet)
}

func (ds *DeltaSemantics) SstoreCount() int64 {
	return ds.SubstrateTrace.SstoreCount
}

func (ds *DeltaSemantics) EventCount() int {
	return len(ds.SubstrateTrace.EventTraces)
}

var ErrDeltaNoGasFound = errors.New("no next gas record found")

func GetGasRetInst(recCallTrace, subCallTrace *research.EVMCallTrace) (uint64, error) {
	idx := len(subCallTrace.GasInstRet)
	if idx >= len(recCallTrace.GasInstRet) {
		return 0, ErrDeltaNoGasFound
	}
	gas := recCallTrace.GasInstRet[idx]
	return gas, nil
}
