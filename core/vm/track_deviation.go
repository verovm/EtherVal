package vm

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
)

const PRINT_DEVIATION = false

// checks if the SSTORE operation deviates from the EVM trace and returns true if there is a deviation
func (evm *EVM) checkSstoreDeviation(addr common.Address, loc *uint256.Int, val *uint256.Int) (exist bool) {
	evmTrace := evm.Config.EVMTrace
	intrTrace := evm.Delta.SubstrateTrace
	intr := fmt.Sprintf("%s,%s,%s", addr.Hex(), loc.Hex(), val.Hex())

	defer func() {
		if PRINT_DEVIATION && exist {
			fmt.Println("Deviated SSTORE at", evm.Context.BlockNumber.String(), evm.StateDB.TxIndex(), evm.Config.IsolationIndex)
			if intrTrace.SstoreCount >= evmTrace.SstoreCount {
				fmt.Println("EVM trace has fewer SSTORE operations than substrate trace")
			} else {
				fmt.Printf("%s\n%s\n", intr, evmTrace.SstoreTrace[intrTrace.SstoreCount])
			}
		}
	}()

	if intrTrace.SstoreCount >= evmTrace.SstoreCount {
		return true
	}
	if intr != evmTrace.SstoreTrace[intrTrace.SstoreCount] {
		return true
	}

	return false
}

// checks if the LOG operation deviates from the EVM trace and returns true if there is a deviation
func (evm *EVM) checkLogDeviation(topics []common.Hash, data []byte) (exist bool) {
	evmTrace := evm.Config.EVMTrace
	intrTrace := evm.Delta.SubstrateTrace
	evmLog := evmTrace.EventTraces[len(intrTrace.EventTraces)]

	defer func() {
		if PRINT_DEVIATION && exist {
			fmt.Println("Deviated LOG at", evm.Context.BlockNumber.String(), evm.StateDB.TxIndex(), evm.Config.IsolationIndex)
			if len(intrTrace.EventTraces) >= len(evmTrace.EventTraces) {
				fmt.Println("EVM trace has fewer LOG operations than substrate trace")
			} else {
				tmp := types.Log{Address: common.Address{}, Topics: evmLog.Topics,
					Data: evmLog.Data, BlockNumber: evm.Context.BlockNumber.Uint64()}
				evmByte, _ := json.MarshalIndent(tmp, "", "    ")
				tmp = types.Log{Address: common.Address{}, Topics: topics,
					Data: data, BlockNumber: evm.Context.BlockNumber.Uint64()}
				sbtByte, _ := json.MarshalIndent(tmp, "", "    ")
				fmt.Println(string(evmByte), string(sbtByte))
			}
		}
	}()

	if len(intrTrace.EventTraces) >= len(evmTrace.EventTraces) {
		return true
	}

	if len(topics) != len(evmLog.Topics) {
		return true
	}

	for i, topic := range topics {
		if evmLog.Topics[i].Cmp(topic) != 0 {
			return true
		}
	}

	if !bytes.Equal(evmLog.Data, data) {
		return true
	}

	return false
}

// checks if the LOG operation deviates from the EVM trace and returns true if there is a deviation
func (evm *EVM) checkCallDeviation(callee common.Address, input []byte, value *uint256.Int, isDelegate bool) (exist bool) {
	evmCallTraces := evm.Config.EVMTrace.CallTraces
	intrCallTraces := evm.Delta.SubstrateTrace.CallTraces
	evmCall := evmCallTraces[len(intrCallTraces)]

	defer func() {
		if PRINT_DEVIATION && exist {
			fmt.Println("Deviated CALL at", evm.Context.BlockNumber.String(), evm.StateDB.TxIndex(), evm.Config.IsolationIndex)
			if len(intrCallTraces) >= len(evmCallTraces) {
				fmt.Println("EVM trace has fewer CALL operations than substrate trace")
			} else {
				fmt.Printf("%v\n%v\n", evmCall.Callee.Hex(), callee.Hex())
			}
		}
	}()

	if len(intrCallTraces) >= len(evmCallTraces) {
		return true
	}

	if evmCall.Callee.Cmp(callee) != 0 {
		return true
	}

	return false
}

const CHECK_SHA3_ORDER = false

// checks if the SHA3 operation deviates from the EVM trace and returns true if there is a deviation
func (evm *EVM) checkSHA3Deviation(isoIndex int, hash common.Hash) (exist bool) {
	evmCt := evm.Config.EVMTrace.CallTraces[isoIndex]
	intrCt := evm.Delta.SubstrateTrace.CallTraces[isoIndex]

	defer func() {
		if PRINT_DEVIATION && exist {
			fmt.Println(evm.Context.BlockNumber.String(), evm.StateDB.TxIndex(), evm.Config.IsolationIndex)
			fmt.Printf("Deviated SHA3:\n%v\n%v\n%v\n", evmCt.SHA3IndexToHash, intrCt.SHA3IndexToHash, hash.Hex())
		}
	}()

	if _, exist := intrCt.SHA3HashToIndex[hash]; exist {
		return false
	}

	if CHECK_SHA3_ORDER { // check order of SHA3 operations
		lastIndex := -1
		for _, idx := range intrCt.SHA3IndexList {
			if idx > lastIndex {
				lastIndex = idx
			}
		}
		if evmCt.SHA3IndexToHash[lastIndex+1].Cmp(hash) != 0 {
			return true
		}
	} else { // check only existence of SHA3 operations
		if _, exist := evmCt.SHA3HashToIndex[hash]; !exist {
			return true
		}
	}

	//fmt.Printf("SHA3: %v\n%v\n%v\n", hash.Hex(), evmCt.SHA3IndexToHash, intrCt.SHA3HashToIndex)
	return false
}

func (evm *EVM) CheckRemainingDeviation(isoIndex int) (msg string, exist bool) {
	evmTrace := evm.Config.EVMTrace
	intrTrace := evm.Delta.SubstrateTrace

	if len(evmTrace.SstoreTrace) != len(intrTrace.SstoreTrace) {
		return fmt.Sprintf("sstore %d %d", len(evmTrace.SstoreTrace), len(intrTrace.SstoreTrace)), true
	}
	if len(evmTrace.EventTraces) != len(intrTrace.EventTraces) {
		return fmt.Sprintf("event %d %d", len(evmTrace.EventTraces), len(intrTrace.EventTraces)), true
	}
	if len(evmTrace.CallTraces) != len(intrTrace.CallTraces) {
		return fmt.Sprintf("call %d %d", len(evmTrace.CallTraces), len(intrTrace.CallTraces)), true
	}
	if len(evmTrace.CallTraces[isoIndex].SHA3HashToIndex) != len(intrTrace.CallTraces[isoIndex].SHA3HashToIndex) {
		return fmt.Sprintf("sha3 %d %d", len(evmTrace.CallTraces[isoIndex].SHA3HashToIndex), len(intrTrace.CallTraces[isoIndex].SHA3HashToIndex)), true
	}
	return "", false
}
