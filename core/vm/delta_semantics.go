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
	MaxGas uint64

	// Information recorded from EVM
	RecordedTrace      *research.EVMTrace
	RecordedAllocCount int

	// SubstrateInterpreter and TACInterpreter
	SubstrateTrace  *research.EVMTrace
	AllocCount      int
	UsedEVM         bool
	SubstrateResult string
	SubstrateErr    error

	TACStats // Embedding struct only for TAC interpreter
}

type TACStats struct {
	// TACInterpreter instruction count
	TACMaxInstCount int64 // TACInterpreter instruction count limit
	TACInstCount    int64 // TACInterpreter dynamic instruction count

	// TACInterpreter patched instructions
	TACStopCount            int64
	TACFallthroughStopCount int64 // fallthrough semantics patch
	// JUMP
	TACJumpCount            int64
	TACFallthroughJumpCount int64 // fallthrough semantics patch
	TACAmbiguousJumpCount   int64 // ambiguous jump patch
	// THROW
	TACThrowCount            int64
	TACFallthroughThrowCount int64 // fallthrough semantics patch
	// CONST
	TACConstCount        int64
	TACReorderConstCount int64 // reordering patch
	// PHI
	TACPhiCount                int64
	TACReorderPhiCount         int64 // reordering patch
	TACAmbiguousPhiCount       int64 // phi patch
	TACAmbiguousPhiChoiceCount int64 // count ambiguous PHI choices at runtime

	// Deviation point for debugging decompiler
	TACDeviation string
}

func NewDeltaSemantics(evm *EVM, substate *research.Substate, trace *research.EVMTrace) *DeltaSemantics {
	tacMaxInstCount := tacMaxInstCountValue
	if tacGasInstCountValue {
		txGasUsage := int64(*substate.Result.GasUsed)
		if tacMaxInstCount < txGasUsage {
			tacMaxInstCount = txGasUsage
		}
	}

	return &DeltaSemantics{
		MaxGas: EXT_DEFAULT_MAX_GAS,

		RecordedTrace:      trace,
		RecordedAllocCount: SubstatePreAllocCount(substate),

		SubstrateTrace:  evm.StateDB.(*state.StateDB).ResearchEVMTrace,
		AllocCount:      0,
		UsedEVM:         false,
		SubstrateResult: "",
		SubstrateErr:    nil,

		TACStats: TACStats{
			TACMaxInstCount: tacMaxInstCount,
		},
	}
}

func (delta *DeltaSemantics) IsEVMOutOfGas(index int) bool {
	recErr := delta.RecordedTrace.CallTraces[index].Err
	if recErr == nil || recErr.Error() != ErrOutOfGas.Error() {
		return false
	} else {
		return true
	}
}

func (delta *DeltaSemantics) CheckOutOfGas(currIdx int) bool {
	recTrace := delta.RecordedTrace
	subTrace := delta.SubstrateTrace

	if len(subTrace.CallTraces) > len(recTrace.CallTraces) ||
		recTrace.CallTraces[currIdx].Depth != subTrace.CallTraces[currIdx].Depth {
		return false
	}

	recErr := recTrace.CallTraces[currIdx].Err
	if recErr == nil || recErr.Error() != ErrOutOfGas.Error() {
		return false
	}

	// Check for equivalent sub-calltree by callLen
	callLen := currIdx + 1
	for callLen < len(recTrace.CallTraces) {
		if recTrace.CallTraces[callLen].Depth <= subTrace.CallTraces[currIdx].Depth {
			break
		}
		callLen++
	}
	if len(subTrace.CallTraces) != callLen {
		return false
	}

	if recTrace.SstoreCount <= subTrace.SstoreCount && // number of SSTOREs
		len(recTrace.EventTraces) <= len(subTrace.EventTraces) { // number of LOGs
		//delta.RecordedAllocCount <= delta.AllocCount { // number of addresses

		if currIdx >= len(recTrace.CallTraces) || currIdx >= len(subTrace.CallTraces) {
			return false
		}

		// traces of isolated transactions
		recCt := recTrace.CallTraces[currIdx]
		subCt := subTrace.CallTraces[currIdx]

		// check subset of SHA3 hashes for redundant or reordered SLOADs
		if len(recCt.SHA3HashToIndex) > len(subCt.SHA3HashToIndex) {
			return false
		}
		for h := range recCt.SHA3HashToIndex {
			if _, ok := subCt.SHA3HashToIndex[h]; !ok {
				return false
			}
		}

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
