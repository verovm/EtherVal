package research

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
)

type EVMCallTrace struct {
	Callee   common.Address // Callee addrees for storage and other EVM contexts
	CodeAddr common.Address // Account address to read the bytecode
	CodeHash common.Hash    // Keccak256 hash of bytecode, especially required for CALLCODE and CREATE*

	CallId int32  // Order of calls within a transaction
	CallFn string // "Call", "Create", "DelegateCall", etc.

	Depth       int32  // Depth of call stack
	Ret         []byte // Return data of CALL/CREATE
	RefundGas   uint64 // Gas to be returned at the end of the transaction
	LeftOverGas uint64 // leftOverGas returned by EVM's *Call* and Create* methods
	Err         error  `json:"-"` // nil if the call is successful, ErrExecutionReverted if the call reverted
	ErrMsg      string //
	Reverted    bool   // true if Err is ErrExectuionReverted

	GasInstRet []uint64 // Return values of GAS instructions

	LogInstRet []*types.Log // Return values of LOG* instructions

	JumpInstPos    []uint64      // addresses of JUMP, JUMPI instructions
	JumpInstTarget []uint256.Int // target addresses of JUMP, JUMPI instructinos jumped in runtime
}

func NewEVMCallTrace() *EVMCallTrace {
	return &EVMCallTrace{}
}

func (x *EVMCallTrace) Equal(y *EVMCallTrace) bool {
	if x == y {
		return true
	}

	if (x == nil || y == nil) && x != y {
		return false
	}

	equal := (x.Callee == y.Callee &&
		x.CodeAddr == y.CodeAddr &&
		x.CodeHash == y.CodeHash &&
		x.CallId == y.CallId &&
		x.CallFn == y.CallFn &&
		x.Depth == y.Depth &&
		bytes.Equal(x.Ret, y.Ret) &&
		x.LeftOverGas == y.LeftOverGas &&
		(x.Err == y.Err || (x.Err != nil && y.Err != nil && x.Err.Error() == y.Err.Error())) &&
		x.Reverted == y.Reverted &&
		len(x.GasInstRet) == len(y.GasInstRet) &&
		len(x.LogInstRet) == len(y.LogInstRet) &&
		len(x.JumpInstPos) == len(y.JumpInstPos) &&
		len(x.JumpInstTarget) == len(y.JumpInstTarget))
	if !equal {
		return false
	}

	for i, xg := range x.GasInstRet {
		yg := y.GasInstRet[i]
		if xg != yg {
			return false
		}
	}

	for i, xl := range x.LogInstRet {
		yl := y.LogInstRet[i]

		equal := (xl.Address == yl.Address &&
			len(xl.Topics) == len(yl.Topics) &&
			bytes.Equal(xl.Data, yl.Data))
		if !equal {
			return false
		}

		for i, xt := range xl.Topics {
			yt := yl.Topics[i]
			if xt != yt {
				return false
			}
		}
	}

	for i, xp := range x.JumpInstPos {
		yp := y.JumpInstPos[i]
		if xp != yp {
			return false
		}
	}

	for i, xt := range x.JumpInstTarget {
		yt := y.JumpInstTarget[i]
		if xt != yt {
			return false
		}
	}

	return true
}

func (ct *EVMCallTrace) Copy() *EVMCallTrace {
	ctCopy := NewEVMCallTrace()

	ctCopy.Callee = ct.Callee
	ctCopy.CodeAddr = ct.CodeAddr
	ctCopy.CodeHash = ct.CodeHash

	ctCopy.CallId = ct.CallId
	ctCopy.CallFn = ct.CallFn

	ctCopy.Depth = ct.Depth
	ctCopy.Ret = ct.Ret
	ctCopy.LeftOverGas = ct.LeftOverGas
	ctCopy.Err = ct.Err
	ctCopy.Reverted = ct.Reverted

	ctCopy.GasInstRet = make([]uint64, len(ct.GasInstRet))
	copy(ctCopy.GasInstRet, ct.GasInstRet)

	ctCopy.LogInstRet = make([]*types.Log, len(ct.LogInstRet))
	for i, l := range ct.LogInstRet {
		lCopy := new(types.Log)

		lCopy.Address = l.Address

		lCopy.Topics = make([]common.Hash, len(l.Topics))
		copy(lCopy.Topics, l.Topics)

		lCopy.Data = make([]byte, len(l.Data))
		copy(lCopy.Data, l.Data)

		ctCopy.LogInstRet[i] = lCopy
	}

	ctCopy.JumpInstPos = make([]uint64, len(ct.JumpInstPos))
	copy(ctCopy.JumpInstPos, ct.JumpInstPos)

	ctCopy.JumpInstTarget = make([]uint256.Int, len(ct.JumpInstTarget))
	copy(ctCopy.JumpInstTarget, ct.JumpInstTarget)

	return ctCopy
}

func (ct *EVMCallTrace) AddGasInstRet(gas uint64) {
	ct.GasInstRet = append(ct.GasInstRet, gas)
}

func (ct *EVMCallTrace) AddLogInstRet(log *types.Log) {
	ct.LogInstRet = append(ct.LogInstRet, log)
}

func (ct *EVMCallTrace) AddJumpInstPosTarget(instPos uint64, target uint256.Int) {
	ct.JumpInstPos = append(ct.JumpInstPos, instPos)
	ct.JumpInstTarget = append(ct.JumpInstTarget, *target.Clone())
}

type EVMEventTrace struct {
	Topics []common.Hash
	Data   []byte
}

func (et *EVMTrace) AddEventTrace(topics []common.Hash, data []byte) {
	et.EventTraces = append(et.EventTraces, EVMEventTrace{topics, data})
}

type EVMTrace struct {
	SstoreCount int64
	SstoreTrace []string
	EventTraces []EVMEventTrace
	CallTraces  []*EVMCallTrace
}

func NewEVMTrace() *EVMTrace {
	return &EVMTrace{}
}

func (x *EVMTrace) Equal(y *EVMTrace) bool {
	if x == y {
		return true
	}

	if (x == nil || y == nil) && x != y {
		return false
	}

	equal := (x.SstoreCount == y.SstoreCount &&
		len(x.SstoreTrace) == len(y.SstoreTrace) &&
		len(x.EventTraces) == len(y.EventTraces) &&
		len(x.CallTraces) == len(y.CallTraces))
	if !equal {
		println(len(x.EventTraces), len(y.EventTraces))
		return false
	}

	for i, xt := range x.SstoreTrace {
		yt := y.SstoreTrace[i]
		if xt != yt {
			return false
		}
	}

	for i, xt := range x.CallTraces {
		yt := y.CallTraces[i]
		if !xt.Equal(yt) {
			return false
		}
	}

	return true
}

func (et *EVMTrace) Copy() *EVMTrace {
	etCopy := NewEVMTrace()

	etCopy.SstoreCount = et.SstoreCount

	etCopy.SstoreTrace = make([]string, len(et.SstoreTrace))
	copy(etCopy.SstoreTrace, et.SstoreTrace)

	etCopy.EventTraces = make([]EVMEventTrace, len(et.EventTraces))
	copy(etCopy.EventTraces, et.EventTraces)

	etCopy.CallTraces = make([]*EVMCallTrace, len(et.CallTraces))
	for i, ct := range et.CallTraces {
		etCopy.CallTraces[i] = ct.Copy()
	}

	return etCopy
}

func (et *EVMTrace) IncSstoreCount() {
	et.SstoreCount++
}

func (et *EVMTrace) AddSstoreTrace(t string) {
	et.SstoreTrace = append(et.SstoreTrace, t)
}

func (et *EVMTrace) NextCallTrace() *EVMCallTrace {
	ct := NewEVMCallTrace()
	ct.CallId = int32(len(et.CallTraces))
	et.CallTraces = append(et.CallTraces, ct)
	return ct
}

func HasEVMTrace(block uint64, tx int) bool {
	key := Stage2EVMTraceKey(block, tx)
	has, _ := staticSubstateDB.backend.Has(key)
	return has
}

func GetEVMTrace(block uint64, tx int) *EVMTrace {
	var err error

	key := Stage2EVMTraceKey(block, tx)
	defer func() {
		if err != nil {
			panic(fmt.Errorf("substrate-interpreter: error getting trace of substate %v_%v from substate DB: %v,", block, tx, err))
		}
	}()

	value, err := staticSubstateDB.backend.Get(key)
	if err != nil {
		return nil
	}
	evmTrace := EVMTrace{}

	err = json.Unmarshal(value, &evmTrace)
	if err != nil {
		panic(fmt.Errorf("substrate-interpreter: error decoding trace of substate %v_%v: %v", block, tx, err))
	}
	for _, ct := range evmTrace.CallTraces {
		if ct.ErrMsg != "" {
			ct.Err = errors.New(ct.ErrMsg)
		}
	}

	return &evmTrace
}

func PutEVMTrace(block uint64, tx int, et *EVMTrace) {
	var err error

	key := Stage2EVMTraceKey(block, tx)
	defer func() {
		if err != nil {
			panic(fmt.Errorf("substrate-interpreter: error putting trace of substate %v_%v into substate DB: %v", block, tx, err))
		}
	}()

	for _, ct := range et.CallTraces {
		if ct.Err != nil {
			ct.ErrMsg = ct.Err.Error()
		}
	}
	value, err := json.Marshal(et)
	if err != nil {
		panic(err)
	}

	err = staticSubstateDB.backend.Put(key, value)
	if err != nil {
		panic(err)
	}
}

func DeleteEVMTrace(block uint64, tx int) {
	key := Stage2EVMTraceKey(block, tx)
	err := staticSubstateDB.backend.Delete(key)
	if err != nil {
		panic(err)
	}
}
