package vm

import (
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"golang.org/x/crypto/sha3"

	"crypto/md5"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/seonghojj/substrate_parser/ast"

	"sync"
	"sync/atomic"
)

const memLast = 64 //MEM[0x40]

type ValueType int

type memoryRef struct {
	id              string
	typ             ValueType
	dataPtr, length *uint256.Int
	index           uint64
}

func (ref *memoryRef) SetIndex(index uint64) *memoryRef {
	// if index >= ref.length.Uint64() {
	//  println("id:", ref.id, ", length:", ref.length.Uint64(), ", index:", index)
	//  panic("invalid index in SetIndex")
	// }
	return &memoryRef{ref.id, ref.typ, ref.dataPtr, ref.length, index}
}

type memAccess struct {
	id     *memoryRef
	member string
}

type storageRef struct {
	kind      ValueType
	index     *uint256.Int
	bit_start uint8
	bit_end   uint8
	mapping   string
}
type storAccess struct {
	id     storageRef
	member string
}

// storage of array or mapping type using struct
type storVar struct {
	name *ast.LvalueNode
	typ  string //written by extractStorType()
	hash *uint256.Int
}

// storage with field
type storStruct struct {
	obj       *storVar
	field     uint64
	fieldType string
	hash      *uint256.Int
	bit_start uint64
	bit_end   uint64
}

type structDecl struct {
	field2type map[string]string
	size       uint64
}

type castExpr struct {
	castType string
	val      *uint256.Int
}

type inputArgRef struct { //bytes, string in function argument
	id          string
	typ         string
	ptr, length uint64
}
type inputArgAccess struct {
	id     *inputArgRef
	member string
}

type callReturn struct {
	success bool
	retVal  *uint256.Int //?
}
type specifier struct {
	value, gas *uint256.Int
}
type arrayType struct {
	typ     string
	storage bool
}
type eventDef struct {
	eventId, args string
	signature     string
}

type callCtx_s struct {
	actualArgs  map[string]*inputArgRef
	passedArgs  []NodeValue
	localStruct map[string]*memoryRef //memory reference for struct
	localVar    map[string]*uint256.Int
	lVarType    map[string]ValueType
	memoryVar   map[string]*memoryRef //memory reference (e.g. v0.data & v0.length)
	storageVar  map[string]storageRef
	storageNum  uint64 //number of storage declaration
	structInfo  map[string]structDecl
	memory      *Memory
	selector    *uint256.Int
	isSelector  bool
	isPublic    bool
	contract    *Contract
	Failure     string
	eventList   []eventDef
	CMap        *Coverage
	CheckCvg    bool
	terminate   int32
	panicMsg    string
	callDepth   int
	actDepth    int
}

type SubstrateInterpreter struct {
	evm *EVM
	cfg Config

	hasher    crypto.KeccakState // Keccak256 hasher instance shared across opcodes
	hasherBuf common.Hash        // Keccak256 hasher result array shared aross opcodes

	readOnly   bool   // Whether to throw on stateful modifications
	returnData []byte // Last CALL's return data for subsequent reuse

	callContexts []*callCtx_s
	framePtr     *callCtx_s

	Runnable bool

	currNode ast.AstNode

	Delta *DeltaSemantics
}

const (
	useDeltaGas  = true
	useDeltaLOG  = true
	useDeltaFMP  = true
	useDeltaProp = true
)

func NewSubstrateInterpreter(evm *EVM, cfg Config) *SubstrateInterpreter {
	return &SubstrateInterpreter{
		evm:          evm,
		callContexts: make([]*callCtx_s, 0, 1024),
		cfg:          cfg,
		Runnable:     true,

		Delta: evm.Delta,
	}
}

func getMD5(code []byte) string {
	md5result := md5.Sum([]byte(code))
	return hex.EncodeToString(md5result[:])
}

// combine flags into a 2-bit state (high: bit 1, low: bit 0)
func makeState(high, low bool) int {
	state := 0
	if high {
		state |= 2
	}
	if low {
		state |= 1
	}
	return state
}

// /////////////////////////////
// ///COPIED interpreter.go/////
// /////////////////////////////
func (in *SubstrateInterpreter) Run(contract *Contract, input []byte, readOnly bool) (ret []byte, err error) {
	in.Delta = in.evm.Delta

	if checkEMI {
		printDebug(in.evm.Context.BlockNumber.String()+", "+getMD5(contract.Code)+","+contract.Address().String(), true)
		printDebug(strconv.Itoa(in.evm.depth)+", 0x"+hex.EncodeToString(input), true)
	}
	defer func() {
		if checkEMI {
			if err != nil && in.framePtr.contract.AstRoot != nil {
				l, c := in.currNode.GetPos()
				printDebug(fmt.Sprintf("%s in %d,%d", err.Error(), l, c), true)
			}
			for i, ct := range in.Delta.RecordedTrace.CallTraces {
				if ct.Err != nil {
					printDebug(fmt.Sprint("EVM call ", i, ": ", ct.Err.Error()), true)
				}
			}
		}
	}()

	// Increment the call depth which is restricted to 1024
	in.evm.depth++
	in.Delta.SubstrateTrace.CallTraces[in.evm.Config.IsolationIndex].Depth = int32(in.evm.depth)

	defer func() { in.evm.depth-- }()

	// Make sure the readOnly is only set if we aren't in readOnly yet.
	// This makes also sure that the readOnly flag isn't removed for child calls.
	if readOnly && !in.readOnly {
		in.readOnly = true
		defer func() { in.readOnly = false }()
	}

	// Reset the previous call's return data. It's unimportant to preserve the old buffer
	// as every returning call will return new data anyway.
	in.returnData = nil

	// Don't bother with the execution if there's no code.
	//if len(contract.Code) == 0 {
	//   return nil, nil
	//}

	contract.Input = input
	if len(input) != 0 && len(input) < 32 {
		pad := make([]byte, 32-len(input))
		contract.Input = append(input, pad...)
	} else {
		contract.Input = input
	}

	selector := uint256.NewInt(0)
	var (
		mem         = NewMemory() // bound memory
		callContext = &callCtx_s{
			memory:      mem,
			localStruct: make(map[string]*memoryRef),
			localVar:    make(map[string]*uint256.Int),
			lVarType:    make(map[string]ValueType),
			memoryVar:   make(map[string]*memoryRef),
			storageVar:  make(map[string]storageRef),
			structInfo:  make(map[string]structDecl),
			actualArgs:  make(map[string]*inputArgRef),
			passedArgs:  nil,
			selector:    selector,
			isSelector:  true,
			isPublic:    false,
			contract:    contract,
			Failure:     "",
			eventList:   make([]eventDef, 0, 4),
			CMap:        nil,
			CheckCvg:    false,
			panicMsg:    "",
			callDepth:   0,
			actDepth:    0,
		}
		callPanicMsg = ""
	)

	in.framePtr = callContext
	in.callContexts = append(in.callContexts, callContext)
	callContext.memory.Resize(4096)
	callContext.memory.Set32(memLast, uint256.NewInt(0x80)) //MEM[0x40] = 0x80
	if in.evm.depth == 1 {
		in.framePtr.CMap = getCMap(contract.Address())
	}
	inputLen := len(input)
	if inputLen >= 4 {
		selector.SetBytes(input[0:4])
	}
	in.hasher = sha3.NewLegacyKeccak256().(crypto.KeccakState)

	var output string
	wg := sync.WaitGroup{}
	wg.Add(1)

	defer func() {
		atomic.StoreInt32(&in.framePtr.terminate, 1)

		wg.Wait() //Wait until go routine for ActSubstrate ends
		for in.framePtr != callContext {
			//Wait until all internal calls terminate
		}
		in.framePtr.panicMsg = callPanicMsg + in.framePtr.panicMsg
		if len(in.callContexts) > 1 {
			if callPanicMsg != "" {
				in.callContexts[len(in.callContexts)-2].panicMsg += "|" + in.framePtr.panicMsg
			}
			in.callContexts = in.callContexts[:len(in.callContexts)-1]
			in.framePtr = in.callContexts[len(in.callContexts)-1]
		}
		v := recover()

		if len(in.callContexts) == 1 {
			callTrace := in.Delta.RecordedTrace.CallTraces[in.evm.Config.IsolationIndex]

			hexInput := "0x"
			if len(input) > 4 {
				hexInput += hex.EncodeToString(input[0:4])
			} else {
				hexInput += hex.EncodeToString(input)
			}
			date := ""
			if in.framePtr.contract.AstRoot != nil {
				date = in.framePtr.contract.AstRoot.Date
			}
			output = fmt.Sprintf("%s,%d,%d,%s,%s,%s,", in.evm.Context.BlockNumber.String(), in.evm.StateDB.TxIndex(), in.evm.Config.IsolationIndex,
				getMD5(contract.Code), date, hexInput)
			if v != nil {
				output += strings.Replace(fmt.Sprint(v), ",", "", -1)
			}
			output += in.framePtr.panicMsg
			output += fmt.Sprintf(",%d,%d,%d,%d,%02b,%02b",
				len(in.evm.StateDB.(*state.StateDB).ResearchEVMTrace.CallTraces), len(in.Delta.RecordedTrace.CallTraces),
				in.Delta.SubstrateTrace.SstoreCount, in.Delta.RecordedTrace.SstoreCount,
				makeState(in.checkOutOfGas(), in.Delta.IsEVMOutOfGas(in.evm.Config.IsolationIndex)),
				makeState(err == nil, callTrace.Err == nil))
			in.Delta.SubstrateResult = output

			//TODO:For comparison
			output += fmt.Sprintf(",%d,%d", contract.Gas-callTrace.LeftOverGas, len(contract.Code))
			in.Delta.SubstrateResult = output

			contract.UseGas(contract.Gas - callTrace.LeftOverGas)
			in.evm.StateDB.AddRefund(callTrace.RefundGas - in.evm.StateDB.GetRefund())

			////
			//fmt.Println(in.Delta.SubstrateTrace.CallTraces[in.cfg.IsolationIndex].SHA3IndexList, in.Delta.SubstrateTrace.CallTraces[in.cfg.IsolationIndex].SHA3HashToIndex)
			//fmt.Println(in.evm.Config.EVMTrace.CallTraces[in.cfg.IsolationIndex].SHA3IndexList, in.evm.Config.EVMTrace.CallTraces[in.cfg.IsolationIndex].SHA3HashToIndex)
		}

	}()

	queue := make(chan struct {
		r []byte
		e error
	}, 1)
	go func() {
		defer wg.Done()
		defer func() {
			v := recover()
			t := atomic.LoadInt32(&in.framePtr.terminate)
			if t == 1 {
				//Unintended termination
			} else if t == 2 { //Intended termination (e.g., out-of-gas error in EVM)
				v = nil
				if useDeltaGas {
					err = in.Delta.RecordedTrace.CallTraces[in.evm.Config.IsolationIndex].Err
				}
			}
			if v != nil {
				callPanicMsg = strings.Replace(fmt.Sprint(v), ",", "", -1)
				err = ErrExecutionReverted

				if getCheckCoverage() {
					addErrorCount(in.framePtr)
				}
				//buf := make([]byte, 1024)
				//stackSize := runtime.Stack(buf, false)
				//fmt.Printf("Stack trace:\n%s\n", string(buf[:stackSize]))
			}
			queue <- struct {
				r []byte
				e error
			}{ret, err}
		}()
		if in.framePtr.contract.AstRoot == nil {
			in.Panic("Error in parsing", in.currNode, false)
		}

		//Get fuzz input
		if false && len(input) > 4 {
			finput, err := fuzzInput(contract.AstRoot, hex.EncodeToString(input[0:4]), in.evm.Context.BlockNumber.Uint64(), uint64(in.evm.StateDB.TxIndex()))
			if err != nil {
				in.Panic(err.Error(), in.currNode, false)
			}
			if finput != nil {
				copy(contract.Input[4:], finput)
			}
		}

		r := in.ActSubstrate(in.framePtr.contract.AstRoot).(NodeValue)
		in.checkOutOfGas()
		if !r.IsEmpty() {
			if r.Type() == ValueRevert {
				err = ErrExecutionReverted
			} else if r.Type() == ValueInvalid {
				err = ErrInvalidJump
			} else {
				switch r.value.(type) {
				case []byte:
					ret = r.value.([]byte)
				case []*uint256.Int:
					list := r.value.([]*uint256.Int)
					for _, v := range list {
						elem := v.Bytes32()
						ret = append(ret[:], elem[:]...)
					}
				case *uint256.Int:
					bytes32 := r.value.(*uint256.Int).Bytes32()
					ret = bytes32[:]
				}
			}
		}
	}()
	select {
	case result := <-queue:
		return result.r, result.e
	case <-time.After(time.Second * 5):
		in.Panic("Endless loop", in.currNode, false)
		return nil, nil
	}
}
func (in *SubstrateInterpreter) CanRun(code []byte) bool {
	return in.Runnable
}

func revert(ret []byte) NodeValue {
	return NodeValue{kind: ValueRevert, value: ret}
}

func (in *SubstrateInterpreter) getHash(a []byte, traceSHA3 bool) *uint256.Int {
	in.hasher.Reset()
	in.hasher.Write(a)
	in.hasher.Read(in.hasherBuf[:])

	// check deviation of SHA3 compared to EVM trace
	if traceSHA3 && in.evm.Config.EVMTrace != nil {
		if in.checkOutOfGas() {
			return nil
		}
		if in.evm.checkSHA3Deviation(in.cfg.IsolationIndex, in.hasherBuf) {
			line, _ := in.currNode.GetPos()
			in.Panic("Deviated in SHA3 at line "+strconv.Itoa(line), in.currNode, false)
		}
		// substrate-evm-trace: add SHA3 instruction return values
		in.Delta.SubstrateTrace.CallTraces[in.cfg.IsolationIndex].AddSHA3InstRet(in.hasherBuf)
	}

	return uint256.NewInt(0).SetBytes(in.hasherBuf[:])
}

func (in *SubstrateInterpreter) keccak256(args NodeValue) *uint256.Int {
	paddedBytesByType := func(val *uint256.Int, str string) (key []byte) {
		if strings.Contains(str, "uint") {
			uintSize, _ := strconv.Atoi(str[4:])
			key = val.PaddedBytes(uintSize / 8)
		} else if strings.Contains(str, "address") {
			key = val.PaddedBytes(20)
		} else if strings.Contains(str, "bytes") {
			bytesSize, _ := strconv.Atoi(str[5:])
			key = val.PaddedBytes(32)
			key = key[0:bytesSize]
		} else {
			in.Panic("unknown type in keccak", in.currNode, false)
		}
		return
	}
	var hashKey []byte
	if args.Type() == ValueExprList {
		exprs := args.value.([]NodeValue)
		for _, expr := range exprs {
			var key []byte
			if expr.Type() == ValueCastExpr {
				cast := expr.value.(*castExpr)
				key = paddedBytesByType(cast.val, cast.castType)
			} else if expr.Type() == ValueString {
				//String literal does not need padding
				str, _ := strconv.Unquote(`"` + expr.value.(string) + `"`)
				key = []byte(str)
			} else if expr.Type() == ValueStorId {
				stor, _ := in.getStorageRef(expr)
				key = paddedBytesByType(in.getUint256(expr), stor.mapping)
			} else {
				key = in.getUint256(expr).PaddedBytes(32)
			}
			hashKey = append(hashKey, key...)
		}
	} else if args.Type() == ValueString {
		hashKey = []byte(args.value.(string))
	} else if args.Type() == ValueAccess {
		if mem, ok := args.value.(*memAccess); ok {
			if mem.member != "data" {
				in.Panic("mem.member is not data", in.currNode, false)
			}
			if mem.id.length == nil {
				in.Panic("length of memAccess is not known", in.currNode, false)
			}
			hashKey = in.framePtr.memory.GetPtr(int64(mem.id.dataPtr.Uint64()), int64(mem.id.length.Uint64()))
		} else {
			in.Panic("GGGGGGGGGGG", in.currNode, false)
		}
	} else {
		tmp := in.getUint256(args).Bytes32()
		hashKey = tmp[:]
	}
	printDebug("hashkey: "+hex.EncodeToString(hashKey), false)

	return in.getHash(hashKey, true)
}

func abiEncodeErrorMessage(errorMessage string) []byte {
	errorABI := `[{"name":"Error","type":"function","inputs":[{"type":"string"}]}]`
	parsedABI, err := abi.JSON(strings.NewReader(errorABI))
	if err != nil {
		return nil
	}

	data, err := parsedABI.Pack("Error", errorMessage)
	if err != nil {
		return nil
	}

	return data
}

func (in *SubstrateInterpreter) CallIntrinsic(callId string, args NodeValue) NodeValue {
	switch callId {
	case "__DEBUG":
		if args.Type() == ValueExprList {
			println("debug: exprlist not yet")
		} else {
			if in.getUint256(args).Hex() == "0xabcdef" {
				in.framePtr.memory.Ext_Dump()
			}
			l, c := in.currNode.GetPos()
			fmt.Printf("debug(%d,%d): %s\n", l, c, in.getUint256(args).Hex())
		}
	case "require":
		if args.Type() == ValueExprList {
			exprs := args.value.([]NodeValue)
			if in.getUint256(exprs[0]).IsZero() {
				if str, ok := exprs[1].value.([]byte); ok {
					return revert(str)
				} else {
					return revert(in.getUint256(exprs[1]).Bytes())
				}
			}
		} else {
			if in.getUint256(args).IsZero() {
				return revert(nil)
			}
		}
	case "revert":
		//TODO: revert with return?
		return revert(nil)
	case "Panic":
		panicSelector := []byte{0x4e, 0x48, 0x7b, 0x71} //selector of Panic(uint256)
		panicRet := append(panicSelector, in.getUint256(args).PaddedBytes(32)...)
		return revert(panicRet)
	case "assert":
		if in.getUint256(args).IsZero() {
			return revert(nil)
		}
	case "CALLDATACOPY":
		exprs := args.value.([]NodeValue)
		if len(exprs) != 3 {
			in.Panic("Wrong number of arguments in CALLDATACOPY", in.currNode, true)
		}
		dest, offset, size := in.getUint256(exprs[0]).Uint64(), in.getUint256(exprs[1]).Uint64(), in.getUint256(exprs[2]).Uint64()
		var copydata []byte
		if uint64(len(in.framePtr.contract.Input)) < offset+size {
			//awkward behavior, such as CALLDATACOPY(v0.data, msg.data.length, N), exists in EVM bytecode
			if uint64(len(in.framePtr.contract.Input)) == offset {
				copydata = make([]byte, size)
			} else {
				in.Panic("Invalid calldata size to load", in.currNode, false)
			}
		} else {
			copydata = in.framePtr.contract.Input[offset : offset+size]
		}
		if acc, ok := exprs[0].value.(*memAccess); ok {
			if acc.id.length == nil {
				acc.id.length = in.getUint256(exprs[2])
			}
		}
		merr := in.framePtr.memory.Ext_Set(dest, size, copydata, in.Delta.MaxGas)
		if merr != nil {
			in.Panic(merr.Error(), in.currNode, false)
		}
	case "RETURNDATASIZE":
		return NodeValue{kind: ValueNum, value: uint256.NewInt(uint64(len(in.returnData)))}
	case "EXTCODEHASH":
		addr := common.BytesToAddress(in.getUint256(args).Bytes())
		hash := in.evm.StateDB.GetCodeHash(addr)
		return NodeValue{kind: ValueNum, value: uint256.NewInt(0).SetBytes(hash.Bytes())}
	case "sha256hash":
		exprList := []NodeValue{
			{kind: ValueNum, value: uint256.NewInt(1)},
			{kind: ValueNum, value: in.keccak256(args)},
		}
		return NodeValue{kind: ValueExprList, value: exprList}
	case "keccak256":
		return NodeValue{kind: ValueNum, value: in.keccak256(args)}
	case "ecrecover":
		exprs := args.value.([]NodeValue)
		if len(exprs) != 4 {
			in.Panic("Arguments in ecrecover are not four", in.currNode, false)
		}
		hash := in.getUint256(exprs[0]).Bytes()
		v := in.getUint256(exprs[1]).PaddedBytes(32)
		r := in.getUint256(exprs[2]).PaddedBytes(32)
		s := in.getUint256(exprs[3]).PaddedBytes(32)
		sig := append(append(v, r...), s...)

		//addr, _, err := in.evm.StaticCall(in.framePtr.contract, common.HexToAddress("0x1"), append(hash, sig...), in.framePtr.contract.Gas)
		in.evm.StaticCall(in.framePtr.contract, common.HexToAddress("0x1"), append(hash, sig...), in.framePtr.contract.Gas)
		ecr := &ecrecover{}
		addr, err := ecr.Run(append(hash, sig...))
		succ := uint64(0)
		if err == nil {
			succ = 1
		}
		exprList := []NodeValue{
			{kind: ValueNum, value: uint256.NewInt(succ)},
			{kind: ValueAddress, value: uint256.NewInt(0).SetBytes(addr)},
		}
		return NodeValue{kind: ValueExprList, value: exprList}
	case "MSIZE":
		return NodeValue{kind: ValueNum, value: uint256.NewInt(0).SetBytes(in.framePtr.memory.GetPtr(memLast, 32))}
	case "Error":
		var bytes []byte
		if args.Type() == ValueString {
			bytes = abiEncodeErrorMessage(args.value.(string))
		} else if args.Type() == ValueBytes {
			bytes = args.value.([]byte)
		} else if args.Type() == ValueExprList { //TODO: Check if there is any meaningful second args
			exprs := args.value.([]NodeValue)
			bytes = abiEncodeErrorMessage(exprs[0].value.(string))
		} else {
			xxx := in.getUint256(args).Bytes()
			bytes = abiEncodeErrorMessage(string(xxx))
		}
		if bytes == nil {
			in.Panic("ABI Encoding failed in Error()", in.currNode, false)
		}
		return NodeValue{kind: ValueBytes, value: bytes}
	case "byte":
		exprs := args.value.([]NodeValue)
		if len(exprs) != 2 {
			in.Panic("Arguments in byte() are not two", in.currNode, false)
		}
		bytes := in.getUint256(exprs[0]).Bytes()
		index := in.getUint256(exprs[1]).Uint64()
		return NodeValue{kind: ValueNum, value: uint256.NewInt(0).SetBytes1([]byte{bytes[index]})}
	case "selfdestruct":
		destructAddr := in.framePtr.contract.Address()
		balance := in.evm.StateDB.GetBalance(destructAddr)
		beneficiary := in.getUint256(args).Bytes20()

		in.evm.StateDB.AddBalance(common.Address(beneficiary), balance)
		in.evm.StateDB.SelfDestruct(destructAddr)
	case "RETURNDATACOPY":
		//do nothing
	default:
		in.Panic("Unimplemented intrinsic "+callId, in.currNode, false)
		return NodeValue{}
	}
	return NodeValue{}
}

var checkEMI, checkLoad bool

func (in *SubstrateInterpreter) checkOutOfGas() bool {
	preAlloc := in.evm.StateDB.(*state.StateDB).ResearchPreAlloc
	in.Delta.AllocCount = len(preAlloc)
	// exclude coinbase from AllocCount
	if _, exist := preAlloc[in.evm.Context.Coinbase]; exist {
		in.Delta.AllocCount--
	}

	if in.Delta.CheckOutOfGas(in.evm.Config.IsolationIndex) {
		atomic.StoreInt32(&in.framePtr.terminate, 2)
		return true
	}
	return false
}

func (in *SubstrateInterpreter) sload(loc *uint256.Int) *uint256.Int {
	val := in.evm.StateDB.GetState(in.framePtr.contract.Address(), common.Hash(loc.Bytes32()))

	str := fmt.Sprintf("INT SLOAD: address %s, loc: %s, value: %s, depth: %d", in.framePtr.contract.Address().Hex(), loc.Hex(), val.Hex(), in.evm.depth)
	printDebug(str, checkEMI && checkLoad)

	return uint256.NewInt(0).SetBytes(val.Bytes())
}

func (in *SubstrateInterpreter) sstore(loc, val *uint256.Int) {
	if in.checkOutOfGas() {
		return
	}
	defer func() {
		in.checkOutOfGas()
	}()

	// check deviation of storage compared to EVM trace
	if in.evm.Config.EVMTrace != nil {
		if in.evm.checkSstoreDeviation(in.framePtr.contract.Address(), loc, val) {
			line, _ := in.currNode.GetPos()
			in.Panic("Deviated in sstore at line "+strconv.Itoa(line), in.currNode, false)
		}
	}

	in.evm.StateDB.SetState(in.framePtr.contract.Address(), common.Hash(loc.Bytes32()), common.Hash(val.Bytes32()))

	t := fmt.Sprintf("%s,%s,%s", in.framePtr.contract.Address(), loc.Hex(), val.Hex())
	in.Delta.SubstrateTrace.AddSstoreTrace(t)
	in.Delta.SubstrateTrace.IncSstoreCount()

	str := fmt.Sprintf("INT: address %s, loc: %s, value: %s, depth: %d", in.framePtr.contract.Address().Hex(), loc.Hex(), common.Hash(val.Bytes32()).Hex(), in.evm.depth)
	printDebug(str, checkEMI)
}

func (in *SubstrateInterpreter) sloadRef(loc storageRef) *uint256.Int {
	index := uint256.NewInt(0).Set(loc.index)
	val := in.evm.StateDB.GetState(in.framePtr.contract.Address(), common.Hash(index.Bytes32()))
	bytes := val.Bytes()[31-loc.bit_end : 32-loc.bit_start]

	str := fmt.Sprintf("INR SLOAD: address %s, loc: %s, value: %s, depth: %d", in.framePtr.contract.Address().Hex(), index.Hex(), val.Hex(), in.evm.depth)
	printDebug(str, checkEMI && checkLoad)
	return uint256.NewInt(0).SetBytes(bytes)
}
func (in *SubstrateInterpreter) sstoreRef(loc storageRef, val *uint256.Int) {
	if in.checkOutOfGas() {
		return
	}
	defer func() {
		in.checkOutOfGas()
	}()
	index := uint256.NewInt(0).Set(loc.index)
	sloadBytes := in.evm.StateDB.GetState(in.framePtr.contract.Address(), common.Hash(index.Bytes32()))
	valBytes := val.Bytes32()
	copy(sloadBytes[31-loc.bit_end:32-loc.bit_start], valBytes[31-(loc.bit_end-loc.bit_start):32])
	mergedVal := uint256.NewInt(0).SetBytes(sloadBytes[:])

	// check deviation of storage compared to EVM trace
	if in.evm.Config.EVMTrace != nil {
		if in.evm.checkSstoreDeviation(in.framePtr.contract.Address(), index, mergedVal) {
			line, _ := in.currNode.GetPos()
			in.Panic("Deviated in sstoreRef at line "+strconv.Itoa(line), in.currNode, false)
		}
	}

	in.evm.StateDB.SetState(in.framePtr.contract.Address(), common.Hash(index.Bytes32()), common.Hash(mergedVal.Bytes32()))

	t := fmt.Sprintf("%s,%s,%s", in.framePtr.contract.Address(), index.Hex(), mergedVal.Hex())
	in.Delta.SubstrateTrace.AddSstoreTrace(t)
	in.Delta.SubstrateTrace.IncSstoreCount()

	str := fmt.Sprintf("INR: address %s, loc: %s, value: %s, depth: %d", in.framePtr.contract.Address().Hex(), index.Hex(), mergedVal.Hex(), in.evm.depth)
	printDebug(str, checkEMI)
}

func (in *SubstrateInterpreter) getUint256(v NodeValue) (u *uint256.Int) {
	switch v.Type() {
	case ValueNum:
		u = v.Uint256()
	case ValueBytes32:
		u = v.Uint256()
	case ValueId:
		u, ok := in.framePtr.localVar[v.GetId()]
		if !ok {
			u = uint256.NewInt(0)
			in.framePtr.localVar[v.GetId()] = u
		}
		return u
	case ValueStorId:
		stor, _ := in.getStorageRef(v)
		if stor.kind == ValueBasicType || stor.kind == ValueArrayType {
			u = in.sloadRef(stor)
		} else {
			panic("unknown storage type")
		}
	case ValueHex:
		if tmp, ok := v.value.(string); ok {
			if trimmed := "0x" + strings.TrimLeft(tmp[2:], "0"); trimmed != tmp {
				if trimmed == "0x" {
					trimmed = "0x0"
				}
				tmp = trimmed
			}
			uu, err := uint256.FromHex(tmp)
			if err != nil {
				in.Panic("Wrong type info of hex digit "+err.Error(), in.currNode, false)
			}
			u = uu
		} else {
			u = v.value.(*uint256.Int)
		}
	case ValueMem:
		allocInfo := v.value.([]NodeValue)
		offset, length, size := allocInfo[0], allocInfo[1], allocInfo[2]
		var memBytes []byte
		var merr error
		if length.IsEmpty() && size.IsEmpty() {
			//tmp := in.framePtr.memory.GetPtr(int64(offset.Uint256()[0]), 32)
			num := in.getUint256(offset).Uint64()
			memBytes, merr = in.framePtr.memory.Ext_GetPtr(int64(num), 32, in.Delta.MaxGas)
		} else if !length.IsEmpty() && size.IsEmpty() {
			p := in.getUint256(offset).Uint64()
			l := in.getUint256(length).Uint64()
			memBytes, merr = in.framePtr.memory.Ext_GetPtr(int64(p), int64(l), in.Delta.MaxGas)
		} else {
			p := in.getUint256(offset).Uint64()
			s := in.getUint256(size).Uint64()
			memBytes, merr = in.framePtr.memory.Ext_GetPtr(int64(p), int64(s-p), in.Delta.MaxGas)
		}
		if merr != nil {
			in.Panic(merr.Error(), in.currNode, false)
		}
		u = uint256.NewInt(0).SetBytes(memBytes)
	case ValueStorage:
		allocInfo := v.value.([]NodeValue) //TODO: get only loc?
		loc, length, size := allocInfo[0], allocInfo[1], allocInfo[2]
		if length.IsEmpty() && size.IsEmpty() {
			u = in.sload(in.getUint256(loc))
		} else {
			panic("size & length is not empty in STORAGE") //TODO: remove length, size if it never happens
		}
	case ValueReturn:
		//TODO: is v.value always *uint256.Int?
		switch v.value.(type) {
		case *uint256.Int:
			u = v.value.(*uint256.Int)
		default:
			u = uint256.NewInt(0)
			panic("unknown type in ValueReturn")
		}
	case ValueMemRef, ValueLocalStruct:
		ref := v.value.(*memoryRef)
		//TODO: need assertion for index?
		if ref.index == ^uint64(0) {
			//println(ref.id, ref.dataPtr.Uint64(), ref.index, ref.length.Uint64())
			panic("index not set in ValueMemRef")
		}
		//unit of index is 32-byte
		tmp := in.framePtr.memory.GetPtr(int64(ref.dataPtr.Uint64()+ref.index), 32)
		u = uint256.NewInt(0).SetBytes(tmp)
	case ValueStorRef:
		hash := v.value.(*storVar).hash
		u = in.sload(hash)
	case ValueStructId:
		ref := in.framePtr.localStruct[v.GetId()]
		u = uint256.NewInt(0).Set(ref.dataPtr)
	case ValueStruct:
		stor := v.value.(*storStruct)
		if stor.obj == nil {
			in.Panic("stor.obj is nil", nil, false)
		}
		u = in.sload(stor.hash)
		if !(stor.bit_start > stor.bit_end) {
			bytes32 := u.Bytes32()
			u = uint256.NewInt(0).SetBytes(bytes32[31-stor.bit_end : 32-stor.bit_start])
		}
	case ValueAccess:
		//TODO: access type which is not ValueMemRef?
		switch v.value.(type) {
		case *memAccess:
			acc := v.value.(*memAccess)
			if acc.member == "data" {
				u = acc.id.dataPtr
			} else if acc.member == "length" {
				u = acc.id.length
			} else {
				in.Panic("unknown member in ValueAccess "+acc.member, in.currNode, false)
			}
			if u == nil {
				in.Panic("unknown member in ValueAccess "+acc.id.id+"."+acc.member, in.currNode, false)
			}
		case *storAccess:
			acc := v.value.(*storAccess)
			if acc.member == "length" {
				u = in.sloadRef(acc.id)
			} else if acc.member == "data" {
				u = in.getHash(uint256.NewInt(0).Set(acc.id.index).PaddedBytes(32), true)
			} else if acc.member == "code.size" {
				addr := in.sloadRef(acc.id)
				u = uint256.NewInt(uint64(in.evm.StateDB.GetCodeSize(addr.Bytes20())))
			} else {
				in.Panic("unknown member in ValueAccess "+acc.member, in.currNode, false)
			}
		case *uint256.Int:
			acc := v.value.(*uint256.Int)
			u = uint256.NewInt(0).Set(acc)
		}
	case ValueCallReturn:
		succ := v.value.(callReturn).success
		if succ {
			u = uint256.NewInt(1)
		} else {
			u = uint256.NewInt(0)
		}
	case ValueArgId:
		/*arg, ok := in.framePtr.actualArgs[v.value.(string)]
		if !ok {
			//println(v.value.(string))
			panic("non-existent argument")
		}
		u = uint256.NewInt(0).SetUint64(arg.ptr)*/
		//Seems like the syntax for ArgId itself has changed..
		//varg0.data => points varg0's dataPtr. varg0 => 32 where the actual data starts
		u = uint256.NewInt(32)
	case ValueAddress:
		addr, ok := v.value.(common.Address)
		if ok {
			u = uint256.NewInt(0).SetBytes20(addr[:])
		} else {
			u = v.value.(*uint256.Int)
		}
	case ValueMemId:
		ref, _ := in.getMemoryRef(v)
		//Seems like the syntax for MemId itself has changed..
		//v0.data => points v0's dataPtr. v0 => dataPtr-32
		if ref.index == ^uint64(0) {
			u = uint256.NewInt(0).Sub(ref.dataPtr, uint256.NewInt(32))
		} else {
			u = uint256.NewInt(0).SetBytes(in.framePtr.memory.GetPtr(int64(ref.dataPtr.Uint64()+ref.index), 32))
		}
		//Old version
		//tmp := in.framePtr.memory.GetPtr(int64(ref.dataPtr.Uint64()), int64(ref.length.Uint64()))
		//u = uint256.NewInt().SetBytes(tmp)
		//printDebug("MemId:" + v.GetId() + ", " + hex.EncodeToString(tmp), false)
	case ValueMsgSender:
		u = uint256.NewInt(0).SetBytes20(in.framePtr.contract.CallerAddress[:])
	case ValueCastExpr:
		u = uint256.NewInt(0).Set(v.value.(*castExpr).val)
	case ValueString:
		str, _ := strconv.Unquote(`"` + v.value.(string) + `"`)
		pad := make([]byte, 32)
		bytes := append([]byte(str), pad[:32-len(str)]...)
		u = uint256.NewInt(0).SetBytes(bytes)
	default:
		in.Panic("unknown type in getUint256() ("+mapping[v.Type()]+")", in.currNode, false)
		u = uint256.NewInt(0)
	}
	return
}
func (in *SubstrateInterpreter) setUint256(id NodeValue, val *uint256.Int) {
	switch id.Type() {
	case ValueId:
		in.framePtr.localVar[id.GetId()] = val
	case ValueMem8:
		allocInfo := id.value.([]NodeValue)
		offset := allocInfo[0]
		merr := in.framePtr.memory.Ext_Set(in.getUint256(offset).Uint64(), 1, val.Bytes(), in.Delta.MaxGas)
		if merr != nil {
			in.Panic(merr.Error(), in.currNode, false)
		}
	case ValueMem:
		allocInfo := id.value.([]NodeValue)
		offset, length, size := in.getUint256(allocInfo[0]), allocInfo[1], allocInfo[2]
		//memLastPtr := uint256.NewInt(0).SetBytes(in.framePtr.memory.GetPtr(memLast, 32))
		if length.IsEmpty() && size.IsEmpty() {
			merr := in.framePtr.memory.Ext_Set32(offset.Uint64(), val, in.Delta.MaxGas)
			if merr != nil {
				in.Panic(merr.Error(), in.currNode, false)
			}
		} else if !length.IsEmpty() && size.IsEmpty() {
			merr := in.framePtr.memory.Ext_Set(offset.Uint64(), in.getUint256(length).Uint64(), val.Bytes(), in.Delta.MaxGas)
			if merr != nil {
				in.Panic(merr.Error(), in.currNode, false)
			}
		} else {
			panic("MEM size: Not implemented") //TODO
		}
	case ValueStorage:
		allocInfo := id.value.([]NodeValue) //TODO: get only loc?
		loc, length, size := allocInfo[0], allocInfo[1], allocInfo[2]
		if length.IsEmpty() && size.IsEmpty() {
			in.sstore(in.getUint256(loc), val)
		} else {
			//TODO: remove length, size if it never happens
			panic("size & length is not empty in STORAGE")
		}
	case ValueMemRef, ValueLocalStruct:
		ref := id.value.(*memoryRef)
		//TODO: need assertion for index?
		if ref.index == ^uint64(0) {
			//println(ref.id, ref.dataPtr.Uint64(), ref.index, ref.length.Uint64())
			panic("index not set in ValueMemRef")
		}
		offset := ref.dataPtr.Uint64() + ref.index
		merr := in.framePtr.memory.Ext_Set32(offset, val, in.Delta.MaxGas)
		if merr != nil {
			in.Panic(merr.Error(), in.currNode, false)
		}
	case ValueStorId:
		stor, _ := in.getStorageRef(id)
		if stor.kind == ValueBasicType {
			in.sstoreRef(stor, val)
		} else {
			panic("unknown storage id")
		}
	case ValueStorRef:
		hash := id.value.(*storVar).hash
		in.sstore(hash, val)
	case ValueStruct:
		stor := id.value.(*storStruct)
		if stor.obj == nil {
			in.Panic("stor.obj is nil", nil, false)
		}
		if !(stor.bit_start > stor.bit_end) {
			sloadBytes := in.evm.StateDB.GetState(in.framePtr.contract.Address(), common.Hash(stor.hash.Bytes32()))
			valBytes := val.Bytes32()
			copy(sloadBytes[31-stor.bit_end:32-stor.bit_start], valBytes[31-(stor.bit_end-stor.bit_start):32])
			val = val.SetBytes(sloadBytes[:])
		}
		in.sstore(stor.hash, val)
	case ValueAccess:
		switch id.value.(type) {
		case *storAccess:
			acc := id.value.(*storAccess)
			if acc.member != "length" || acc.id.kind != ValueArrayType {
				panic("storage member is not length")
			}
			in.sstoreRef(acc.id, val)
		case *uint256.Int:
			tmp := id.value.(*uint256.Int)
			tmp.Set(val)
		}
	case ValueTerminate:
		//Do nothing
	default:
		in.Panic("unknown id type "+mapping[id.Type()], in.currNode, true)
	}
}

func (in *SubstrateInterpreter) getVariableList(exprs NodeValue) (vl []NodeValue) {
	processExprByType := func(expr NodeValue) {
		printDebug(mapping[expr.Type()], false)
		if expr.Type() == ValueMemId {
			memRef, _ := in.getMemoryRef(expr)
			memRef.id = ""
			vl = append(vl, NodeValue{kind: ValueMemRef, value: memRef})
		} else if expr.Type() == ValueString || expr.Type() == ValueBytes {
			if str, ok := expr.value.(string); ok {
				expr.value = []byte(str)
			}
			vl = append(vl, NodeValue{kind: expr.Type(), value: expr.value})
		} else if expr.Type() == ValueMemBytes {
			//vl = append(vl, expr)
			in.Panic("membytes without type", nil, false)
		} else if expr.Type() == ValueMemRef || expr.Type() == ValueLocalStruct {
			vl = append(vl, NodeValue{kind: expr.Type(), value: expr.value})
			//in.Panic("expr type is valuememref", nil, false)
		} else if expr.Type() == ValueCallReturn {
			cr := expr.value.(callReturn)
			var succ, retVal NodeValue
			if cr.success {
				succ.SetValue(ValueNum, uint256.NewInt(1))
			} else {
				succ.SetValue(ValueNum, uint256.NewInt(0))
			}
			//TODO: check return bytes
			/*bytes := in.framePtr.memory.GetPtr(int64(cr.retPtr.Uint64()), int64(cr.length.Uint64()))
			retVal.SetValue(ValueBytes, bytes)*/
			retVal.SetValue(ValueNum, cr.retVal)
			vl = append(vl, succ)
			vl = append(vl, retVal)
		} else if expr.Type() == ValueId {
			typ := in.framePtr.lVarType[expr.GetId()]
			if typ == ValueStorRef || typ == ValueStorId {
				typ = ValueNum
			}
			vl = append(vl, NodeValue{kind: typ, value: in.getUint256(expr)})
		} else if expr.Type() == ValueStorId || expr.Type() == ValueStorRef || expr.Type() == ValueStorage {
			vl = append(vl, NodeValue{kind: ValueNum, value: in.getUint256(expr)})
		} else if expr.Type() == ValueArgId {
			arg := in.framePtr.actualArgs[expr.value.(string)]
			var typ ValueType
			if arg.typ == "address" {
				typ = ValueAddress
			} else if arg.typ == "bytes32" {
				typ = ValueBytes32
			} else {
				//TODO: could be other types
				typ = ValueNum
			}
			vl = append(vl, NodeValue{kind: typ, value: in.getUint256(expr)})
		} else if expr.Type() == ValueStructId {
			vl = append(vl, NodeValue{kind: ValueLocalStruct, value: in.framePtr.localStruct[expr.GetId()]})
		} else if expr.Type() == ValueLocalStruct {
			vl = append(vl, NodeValue{kind: expr.Type(), value: expr.value.(*memoryRef)})
		} else if expr.Type() == ValueStruct {
			vl = append(vl, NodeValue{kind: expr.Type(), value: expr.value.(*storStruct)})
		} else if expr.Type() == ValueMem {
			allocInfo := expr.value.([]NodeValue)
			for i, al := range allocInfo {
				if al.Type() == EMPTY {
					continue
				}
				allocInfo[i] = NodeValue{kind: ValueNum, value: in.getUint256(al)}
			}
			vl = append(vl, NodeValue{kind: expr.Type(), value: allocInfo})
			//vl = append(vl, NodeValue{kind: expr.Type(), value: expr.value})
		} else if expr.Type() == ValueCastExpr {
			vl = append(vl, NodeValue{kind: expr.Type(), value: expr.value})
		} else if _, ok := expr.value.([]byte); ok {
			vl = append(vl, NodeValue{kind: expr.Type(), value: expr.value})
		} else {
			if expr.Type() == EMPTY {
				expr.kind = ValueNum
			}
			vl = append(vl, NodeValue{kind: expr.Type(), value: in.getUint256(expr)})
		}
	}

	vl = make([]NodeValue, 0, 20)
	if exprs.Type() == ValueExprList || exprs.Type() == ValueReturn {
		list := exprs.value.([]NodeValue)
		for _, expr := range list {
			processExprByType(expr)
		}
	} else {
		processExprByType(exprs)
	}
	return
}

func (in *SubstrateInterpreter) setVariableList(lvals NodeValue, rvals NodeValue) {
	r := in.getVariableList(rvals)
	for i, l := range lvals.value.([]NodeValue) {
		if len(r) <= i { //left-side has more variables than right-side
			r = append(r, NodeValue{kind: ValueNum, value: uint256.NewInt(0)})
		}
		printDebug(mapping[r[i].kind], false)
		if r[i].kind == ValueNum || r[i].kind == ValueAddress || r[i].kind == ValueBytes32 || r[i].kind == ValueHex {
			if val, ok := r[i].value.(*uint256.Int); ok {
				in.framePtr.lVarType[l.GetId()] = r[i].Type()
				in.framePtr.localVar[l.GetId()] = val
			} else {
				v := NodeValue{kind: ValueMemRef, value: in.makeMemoryRef(l.GetId(), r[i].Type(), r[i].value.([]byte))}
				in.setNewMemoryRef(v)
				in.framePtr.lVarType[l.GetId()] = ValueMemId
			}
		} else if r[i].kind == ValueString || r[i].kind == ValueBytes {
			if str, ok := r[i].value.(string); ok {
				r[i].value = []byte(str)
			}
			bytes := r[i].value.([]byte)
			v := NodeValue{kind: r[i].kind, value: in.makeMemoryRef(l.GetId(), r[i].kind, bytes)}
			in.setNewMemoryRef(v)
			in.framePtr.lVarType[l.GetId()] = ValueMemId
		} else if r[i].kind == ValueMemBytes {
			bytes := r[i].value.([]byte)
			v := NodeValue{kind: ValueMemRef, value: in.makeMemoryRef(l.GetId(), in.framePtr.lVarType[l.GetId()], bytes)}
			in.setNewMemoryRef(v)
			in.framePtr.lVarType[l.GetId()] = ValueMemId
		} else if r[i].kind == ValueMemRef || r[i].kind == ValueLocalStruct {
			ref := r[i].value.(*memoryRef)
			ref.id = l.GetId()
			in.framePtr.lVarType[l.GetId()] = r[i].kind
			in.framePtr.memoryVar[l.GetId()] = ref
		} else if r[i].kind == ValueReturn {
			in.framePtr.lVarType[l.GetId()] = r[i].kind
			in.framePtr.localVar[l.GetId()] = r[i].value.(*uint256.Int)
		} else if r[i].kind == ValueCastExpr {
			castType := r[i].value.(*castExpr).castType
			if castType == "address" {
				in.framePtr.lVarType[l.GetId()] = ValueAddress
			} else if castType == "bytes32" {
				in.framePtr.lVarType[l.GetId()] = ValueBytes32
			} else if castType[0:4] == "uint" {
				in.framePtr.lVarType[l.GetId()] = ValueNum
			}
			in.framePtr.localVar[l.GetId()] = r[i].value.(*castExpr).val
		} else if r[i].kind == ValueStruct || r[i].kind == ValueMem || r[i].kind == ValueAccess {
			in.framePtr.lVarType[l.GetId()] = ValueNum
			in.framePtr.localVar[l.GetId()] = in.getUint256(r[i])
		} else if r[i].kind == ValueLocalStruct {
			in.framePtr.lVarType[l.GetId()] = r[i].kind
			in.framePtr.localStruct[l.GetId()] = r[i].value.(*memoryRef)
		} else {
			in.Panic("unexpected r[i].kind "+mapping[r[i].kind], nil, false)
		}
	}
}

func (in *SubstrateInterpreter) getArgumentRef(v NodeValue) (*inputArgRef, bool) {
	ref, exist := in.framePtr.actualArgs[v.value.(string)]
	return ref, exist
}
func (in *SubstrateInterpreter) setArgumentRef(id string, ref *inputArgRef) {
	in.framePtr.actualArgs[id] = ref
}

func (in *SubstrateInterpreter) getMemBytes(ref *memoryRef) (b []byte) {
	offset := int64(ref.dataPtr.Uint64())
	length := int64((ref.length.Uint64()))
	if ref.typ == ValueNum {
		length = length * 32
	}
	//TODO: not sure length is x32. maybe need to check type
	return in.framePtr.memory.GetPtr(offset, length)
}
func (in *SubstrateInterpreter) getFreeMemPtr(byteLen uint64, update bool) *uint256.Int {
	currLast := uint256.NewInt(0).SetBytes(in.framePtr.memory.GetPtr(memLast, 32))
	if update && useDeltaFMP {
		newLast := uint256.NewInt(0).AddUint64(currLast, byteLen)
		in.framePtr.memory.Set32(memLast, newLast)
	}
	return currLast
}
func (in *SubstrateInterpreter) makeMemoryRef(id string, typ ValueType, bytes []byte) *memoryRef {
	length := uint256.NewInt(uint64(len(bytes)))
	ptr := in.getFreeMemPtr(uint64(len(bytes))+32, true)
	dataPtr := uint256.NewInt(0).AddUint64(ptr, 32)

	merr := in.framePtr.memory.Ext_Set(dataPtr.Uint64(), length.Uint64(), bytes, in.Delta.MaxGas)
	if merr != nil {
		in.Panic(merr.Error(), in.currNode, false)
	}
	if typ == ValueNum {
		length.Rsh(length, 5)
	}
	return &memoryRef{id: id, typ: typ, dataPtr: dataPtr, length: length, index: ^uint64(0)}
}
func (in *SubstrateInterpreter) getMemoryRef(v NodeValue) (*memoryRef, bool) {
	ref, exist := in.framePtr.memoryVar[v.value.(string)]
	return ref, exist
}
func (in *SubstrateInterpreter) setNewMemoryRef(v NodeValue) {
	ref := v.value.(*memoryRef)
	in.framePtr.memoryVar[ref.id] = ref //mapping
	if ref.typ == ValueArrayType {
		merr := in.framePtr.memory.Ext_Set32(ref.dataPtr.Uint64()-32, ref.length, in.Delta.MaxGas) //put array length in memory
		if merr != nil {
			in.Panic(merr.Error(), in.currNode, false)
		}
	}
}

func (in *SubstrateInterpreter) getStorageRef(v NodeValue) (storageRef, bool) {
	ref, exist := in.framePtr.storageVar[v.value.(string)]
	return ref, exist
}
func (in *SubstrateInterpreter) setNewStorageVar(id string, ref storageRef) {
	in.framePtr.storageVar[id] = ref
}

func (in *SubstrateInterpreter) doUnaryOp(l NodeValue, op string) NodeValue {
	empty, val := uint256.NewInt(0), in.getUint256(l)
	result := uint256.NewInt(0)
	switch op {
	case "+":
		result.Add(empty, val)
	case "-":
		result.Sub(empty, val)
	case "!":
		if val.IsZero() {
			result.SetUint64(1)
		} else {
			result.SetUint64(0)
		}
	case "~":
		result.Not(val)
	}
	return NodeValue{kind: ValueNum, value: result}
}
func (in *SubstrateInterpreter) doBinaryOp(left, right NodeValue, op string) NodeValue {
	l, r := in.getUint256(left), in.getUint256(right)
	result := uint256.NewInt(0)
	switch op {
	case "+":
		result.Add(l, r)
	case "-":
		result.Sub(l, r)
	case "*":
		result.Mul(l, r)
	case "/":
		result.Div(l, r)
	case "%":
		result.Mod(l, r)
	case "**":
		result.Exp(l, r)
	case "<<":
		result.Lsh(l, uint(r.Uint64()))
	case ">>":
		result.Rsh(l, uint(r.Uint64()))
	case "&":
		result.And(l, r)
	case "^":
		result.Xor(l, r)
	case "|":
		result.Or(l, r)
	case "<":
		if l.Lt(r) {
			result.SetUint64(1)
		} else {
			result.SetUint64(0)
		}
	case ">":
		if l.Gt(r) {
			result.SetUint64(1)
		} else {
			result.SetUint64(0)
		}
	case "<=":
		if l.Lt(r) || l.Eq(r) {
			result.SetUint64(1)
		} else {
			result.SetUint64(0)
		}
	case ">=":
		if l.Gt(r) || l.Eq(r) {
			result.SetUint64(1)
		} else {
			result.SetUint64(0)
		}
	case "==":
		if l.Eq(r) {
			result.SetUint64(1)
		} else {
			result.SetUint64(0)
		}
	case "!=":
		if !l.Eq(r) {
			result.SetUint64(1)
		} else {
			result.SetUint64(0)
		}
	case "&&":
		if !l.IsZero() && !r.IsZero() {
			result.SetUint64(1)
		} else {
			result.SetUint64(0)
		}
	case "||":
		if !l.IsZero() || !r.IsZero() {
			result.SetUint64(1)
		} else {
			result.SetUint64(0)
		}
	}
	return NodeValue{kind: ValueNum, value: result}
}
