package vm

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/research"

	"github.com/ethereum/go-ethereum/ContractFuzzer/fuzz"
	"github.com/ethereum/go-ethereum/substrate_parser/ast"
)

type Coverage fuzz.Coverage

var use_CF = false

func getCheckCoverage() bool {
	return fuzz.CheckCoverage()
}

func getCMap(addr common.Address) *Coverage {
	if !fuzz.CheckCoverage() {
		return nil
	}
	c := fuzz.GetCoverageMap(fuzz.Addr(addr))
	return (*Coverage)(c)
}

func putBranch(n ast.StmtInterface, fp *callCtx_s) {
	if fp.CheckCvg && fp.CMap != nil {
		fuzz.PutBranch(n, (*fuzz.Coverage)(fp.CMap))
	}
}

func addErrorCount(fp *callCtx_s) {
	if fp.CheckCvg && fp.CMap != nil {
		c := (*fuzz.Coverage)(fp.CMap)
		c.AddErrorCount()
	}
}

func fuzzInput(root *ast.SubstrateNode, sel string, block, tx uint64) ([]byte, error) {
	if !fuzz.GetFlag() {
		return nil, nil
	}

	a := ast.SubstrateAbi{}
	//a.SetAbis(contract.AstRoot)
	fabi := a.GetAbi(root, sel)
	if fabi == nil {
		return nil, errors.New("Failed to find function to fuzz")
	}

	if use_CF {
		//Use ContractFuzzer to fuzz input
		//TODO: Combine SubstateMap input with GetFuzzInput
		return fuzz.GetFuzzInput(fabi), nil
	}
	//Use substate inputs to fuzz
	var fuzzed []byte
	if fuzz.TxMap != nil {
		s, e := fuzz.SubstateMap[fuzz.TxKey(block, tx)].(*research.Substate)
		if !e {
			fuzz.SubstateMap[fuzz.TxKey(block, tx)] = research.GetSubstate(block, int(tx))
			s = research.GetSubstate(block, int(tx))
		}
		substate1 := s
		fmt.Println("Substate1:", block, tx)

		//Find randome transaction with same function_selector of the contract
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		for {
			k := r.Intn(len(fuzz.TxMap))
			tx = ^uint64(0)
			for b, txlist := range fuzz.TxMap {
				if k == 0 {
					if len(txlist) != 0 {
						k = r.Intn(len(txlist))
						tx = txlist[k]
						block = b
						break
					}
				}
				k--
			}
			if tx == ^uint64(0) {
				continue
			}
			s, e = fuzz.SubstateMap[fuzz.TxKey(block, tx)].(*research.Substate)
			if !e {
				fuzz.SubstateMap[fuzz.TxKey(block, tx)] = research.GetSubstate(block, int(tx))
				s = research.GetSubstate(block, int(tx))
			}
			if s.TxMessage.GetData() != nil {
				if bytes.EqualFold(s.TxMessage.GetData()[0:4], substate1.TxMessage.GetData()[0:4]) {
					break
				}
			}
		}
		substate2 := s
		fmt.Println("Substate2:", block, tx)

		//TODO: divide substate2.Message by fabi.Inputs. add more substateN by the number of parameters.
		fmt.Println(hex.EncodeToString(substate1.TxMessage.GetData()))
		fmt.Println(hex.EncodeToString(substate2.TxMessage.GetData()))
		fmt.Println(len(fabi.Inputs))
		fmt.Println(fabi.Inputs)

		fmt.Println("Start")
		var args abi.Arguments
		for _, input := range fabi.Inputs {
			t, _ := abi.NewType(input.Name, "", nil)
			args = append(args, abi.Argument{Type: t})
		}

		fmt.Println("Unpacking")
		unpacked, err := args.UnpackValues(substate1.TxMessage.GetData()[4:])
		if err != nil {
			fmt.Println(err)
			//fmt.Println(len(in.framePtr.contract.Input[4:]), hex.EncodeToString(in.framePtr.contract.Input[4:]))
		}
		fmt.Println("For", len(unpacked))
		for i, b := range unpacked {
			fmt.Printf("%d, %T\n", i, b)
			byt, err := hex.DecodeString(fmt.Sprintf("%x\n", b))
			//bi := b.(*big.Int)
			//fmt.Printf("%x\n", bi)

			//byt, err := GetBytes(b)
			if err != nil {
				fmt.Println(err)
			}
			fmt.Println(hex.EncodeToString(byt))
		}
	} else {
		println("Fuzzed nil")
	}

	/*
		var argIds []NodeValue
		var types []string
		for _, n := range l.List {
			types = append(types, n.Typ.String())
			argIds = append(argIds, in.ActFormalArgNode(n).(NodeValue))
		}*/

	return fuzzed, nil
}

func GetBytes(key interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(key)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
