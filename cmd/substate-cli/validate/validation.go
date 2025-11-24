package validate

import (
	"bytes"
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/research"
	"google.golang.org/protobuf/proto"
)

var DEBUG bool

// DeletedAccounts returns a set of addresses of deleted accounts
func DeletedAccounts(substate *research.Substate) map[common.Address]struct{} {
	deleted := make(map[common.Address]struct{})
	for _, entry := range substate.InputAlloc.Alloc {
		addr := *research.BytesToAddress(entry.Address)
		deleted[addr] = struct{}{}
	}
	for _, entry := range substate.OutputAlloc.Alloc {
		addr := *research.BytesToAddress(entry.Address)
		delete(deleted, addr)
	}
	return deleted
}

// EquivalentStateDBAlloc returns true if sdb contains all values from alloc.
func EquivalentStateDBAlloc(sdb *state.StateDB, alloc *research.Substate_Alloc) bool {
	// Read values from StateDB with addresses from Alloc
	for _, entry := range alloc.Alloc {
		addr := *research.BytesToAddress(entry.Address)
		a := entry.Account
		if sdb.GetNonce(addr) != *a.Nonce {
			if DEBUG {
				fmt.Fprintf(
					os.Stdout, "%s NONCE NOT EQUAL %v %v\n",
					addr.Hex(), sdb.GetNonce(addr), *a.Nonce,
				)
			}
			return false
		}
		if !sdb.GetBalance(addr).Eq(research.BytesToUint256(a.Balance)) {
			if DEBUG {
				fmt.Fprintf(
					os.Stdout, "%s BALANCE NOT EQUAL %s %s\n",
					addr.Hex(), sdb.GetBalance(addr).Hex(), research.BytesToUint256(a.Balance).Hex(),
				)
			}
			return false
		}
		if !bytes.Equal(sdb.GetCode(addr), a.GetCode()) {
			if DEBUG {
				fmt.Fprintf(
					os.Stdout, "%s CODE NOT EQUAL MD5 %s %s\n",
					addr.Hex(), research.Md5HashHex(sdb.GetCode(addr)), research.Md5HashHex(a.GetCode()),
				)
			}
			return false
		}
		for _, pair := range a.Storage {
			key := *research.BytesToHash(pair.Key)
			value := *research.BytesToHash(pair.Value)
			if sdb.GetState(addr, key) != value {
				if DEBUG {
					fmt.Fprintf(
						os.Stdout, "%s STORAGE %s NOT EQUAL %s %s\n",
						addr.Hex(), research.BytesToHash(pair.Key).Hex(),
						sdb.GetState(addr, key).Hex(), value.Hex(),
					)
				}
				return false
			}
		}
	}

	return true
}

// EquivalentSideEffects returns true if created, changed, or deleted
// account information and storage values are equal.
func EquivalentSideEffects(x, y *research.Substate) bool {
	// x and y must have same list of deleted accounts
	xd := DeletedAccounts(x)
	yd := DeletedAccounts(y)
	if len(xd) != len(yd) {
		return false
	}
	for addr := range yd {
		if xd[addr] != yd[addr] {
			return false
		}
	}

	idb, _ := state.New(types.EmptyRootHash, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
	idb.LoadSubstate(x)
	idb.LoadSubstate(y)
	idb.SetTxContext(common.Hash{}, 0)

	// Updatet StateDB with x.OutputAlloc and compare with y.OutputAlloc
	xdb := idb.Copy()
	xdb.LoadSubstate(&research.Substate{
		InputAlloc: x.OutputAlloc,
		BlockEnv:   x.BlockEnv,
	})
	for addr := range xd {
		xdb.SelfDestruct(addr)
	}
	xdb.SetTxContext(common.Hash{}, 0)
	xdbeqy := EquivalentStateDBAlloc(xdb, y.OutputAlloc)
	if !xdbeqy {
		return false
	}

	// Update StateDB with y.OutputAlloc and compare with x.OutputAlloc
	ydb := idb.Copy()
	ydb.LoadSubstate(&research.Substate{
		InputAlloc: y.OutputAlloc,
		BlockEnv:   y.BlockEnv,
	})
	for addr := range yd {
		ydb.SelfDestruct(addr)
	}
	ydb.SetTxContext(common.Hash{}, 0)
	ydbeqx := EquivalentStateDBAlloc(ydb, x.OutputAlloc)
	if !ydbeqx {
		return false
	}

	return true
}

// EqualStorageKeys returns true when x.OutputAlloc and y.OutputAlloc have
// the same list of account addresses and storage keys.
func EqualStorageKeys(x, y *research.Substate) bool {
	xalloc := x.OutputAlloc.Alloc
	yalloc := y.OutputAlloc.Alloc
	if len(xalloc) != len(yalloc) {
		return false
	}

	for i, xentry := range xalloc {
		yentry := yalloc[i]
		if !bytes.Equal(xentry.Address, yentry.Address) {
			return false
		}

		xstorage := xentry.Account.Storage
		ystorage := yentry.Account.Storage
		if len(xstorage) != len(ystorage) {
			return false
		}
		for j, xpair := range xstorage {
			ypair := ystorage[j]
			if !bytes.Equal(xpair.Key, ypair.Key) {
				return false
			}
		}
	}

	return true
}

// EquivalentAlloc returns eqAlloc and matchedSA.
// eqAlloc is true if x and y have the same side effects. Side effects are
// newly created, modified, or deleted accounts and changed storage values.
// matchedSA is true if x and y have the same list of storage keys accessed.
func EquivalentAlloc(x, y *research.Substate) (eqAlloc bool, matchedSA bool) {
	eqAlloc = proto.Equal(x.InputAlloc, y.InputAlloc) && proto.Equal(x.OutputAlloc, y.OutputAlloc)
	if !eqAlloc {
		// if eqAlloc is false, check with relaxed conditions
		eqAlloc = EquivalentSideEffects(x, y)
	}

	matchedSA = EqualStorageKeys(x, y)

	return eqAlloc, matchedSA
}

func debugSubstateResult(substate *research.Substate, replaySubstate *research.Substate) {
	resultA := substate.Result
	resultB := replaySubstate.Result
	if resultA.Status != nil && resultB.Status != nil {
		if *resultA.Status != *resultB.Status {
			fmt.Printf("status not eq: substate: %v replay: %v\n", *resultA.Status, *resultB.Status)
		} else {
			fmt.Println("status eq", *resultA.Status)
		}
	} else {
		fmt.Println("status is nil")
	}
	if !bytes.Equal(resultA.Bloom, resultB.Bloom) {
		fmt.Println("bloom not eq")
		//fmt.Printf("substate: %v\n", resultA.Bloom)
		//fmt.Printf("replay  : %v\n", resultB.Bloom)
	} else {
		fmt.Println("bloom eq")
	}
	if resultA.GasUsed != nil && resultB.Status != nil {
		if *resultA.GasUsed != *resultB.GasUsed {
			fmt.Printf("gas not eq: substate: %d replay: %d\n", *resultA.GasUsed, *resultB.GasUsed)
		} else {
			fmt.Println("gas eq")
		}
	} else {
		fmt.Println("gas used is nil")
	}
	allLogEq := true
	if len(resultA.Logs) == len(resultB.Logs) {
		for i := 0; i < len(resultA.Logs); i++ {
			if !bytes.Equal(resultA.Logs[i].Address, resultB.Logs[i].Address) {
				fmt.Println("log address at index", i, "differ")
				allLogEq = false
			}
			if !bytes.Equal(resultA.Logs[i].Data, resultB.Logs[i].Data) {
				fmt.Printf("Log[%v] data not eq\n", i)
				fmt.Printf("substate: %v\n", resultA.Logs[i].Data)
				fmt.Printf("replay  : %v\n", resultB.Logs[i].Data)
				allLogEq = false
			}
			topicLenA := len(resultA.Logs[i].Topics)
			topicLenB := len(resultB.Logs[i].Topics)
			if topicLenA == topicLenB {
				for j := 0; j < len(resultA.Logs[i].Topics); j++ {
					if len(resultA.Logs[i].Topics[j]) == len(resultB.Logs[i].Topics[j]) {
						if !bytes.Equal(resultA.Logs[i].Topics[j], resultB.Logs[i].Topics[j]) {
							fmt.Printf("Log[%v] Topic[%v] not eq\n", i, j)
							fmt.Printf("substate: %v\n", resultA.Logs[i].Topics[j])
							fmt.Printf("replay  : %v\n", resultB.Logs[i].Topics[j])
							allLogEq = false
						}
					} else {
						fmt.Printf("Log[%v] len(Topic[%v]) not eq\n", i, j)
						allLogEq = false
					}
				}
			} else {
				fmt.Printf("Logs[%v] len(Topics) not eq: substate = %v replay = %v\n", i, topicLenA, topicLenB)
				allLogEq = false
			}
		}
	} else {
		fmt.Printf("len(Log) not eq: substate = %v replay = %v\n", len(resultA.Logs), len(resultB.Logs))
		allLogEq = false
	}
	if allLogEq {
		fmt.Println("all log eq")
	}
}
