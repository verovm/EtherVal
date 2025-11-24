package vm

import (
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"

	"encoding/hex"
	"strconv"
	"strings"

	"github.com/holiman/uint256"

	"github.com/ethereum/go-ethereum/substrate_parser/ast"

	"fmt"
	"sync/atomic"
)

var unimpl *ast.BaseNode

func (in *SubstrateInterpreter) Panic(msg string, n ast.AstNode, terminate bool) {
	if atomic.LoadInt32(&in.framePtr.terminate) == 2 || msg == "Terminate" {
		panic("")
	}
	msg = fmt.Sprintf("%s (calldepth: %d)", msg, in.evm.depth)
	if in.framePtr.Failure == "" {
		in.framePtr.Failure = msg
	}
	if checkEMI {
		if n != nil {
			line, column := n.GetPos()
			fmt.Printf("Block: %d\n", in.evm.Context.BlockNumber)
			fmt.Printf("line: %d, column: %d\n", line, column)
			println(msg)
			//log.Printf("Stack trace:%s\n", debug.Stack())
		}
	}
	//if terminate {
	if false {
		atomic.StoreInt32(&in.framePtr.terminate, 1)
		panic(msg)
	} else {
		panic(msg)
	}
}

const STACK_MAX_DEPTH = 1 << 18

func (in *SubstrateInterpreter) preAct(n ast.AstNode) {
	if atomic.LoadInt32(&in.framePtr.terminate) != 0 {
		in.Panic("Terminate", n, false)
	}
	in.framePtr.actDepth++
	if in.framePtr.actDepth > STACK_MAX_DEPTH {
		in.Panic("stack overflow by recursion", n, false)
	}
	in.currNode = n
}

func (in *SubstrateInterpreter) getTypeString(n NodeValue) (str string) {
	typ := n.Type()
	if typ == ValueId {
		typ = in.framePtr.lVarType[n.GetId()]
	} else if typ == ValueMemId {
		memRef, _ := in.getMemoryRef(n)
		typ = memRef.typ
	}
	if typ == ValueCastExpr {
		str = n.value.(*castExpr).castType
	} else if typ == ValueAddress {
		str = "address"
	} else if typ == ValueBytes32 {
		str = "bytes32"
	} else if typ == ValueBytes {
		str = "bytes"
	} else if typ == ValueString {
		str = "string"
	} else {
		str = "uint256"
	}
	return
}

func (in *SubstrateInterpreter) ActBaseNode(n *ast.BaseNode) ast.Value {
	in.Panic("Act on BaseNode", n, true)
	return NodeValue{}
}
func (in *SubstrateInterpreter) ActSubstrate(n *ast.SubstrateNode) ast.Value {
	in.currNode = n
	if n.SList != nil {
		for _, n := range n.SList.List {
			in.ActStorageDecl(n)
		}
	}
	if n.StList != nil {
		for _, n := range n.StList.List {
			in.ActStructDefNode(n)
		}
	}
	if n.EList != nil {
		for _, n := range n.EList.List {
			in.ActEventDecl(n)
		}
	}
	/*if n.FList != nil {
	     ast.Act(in, n.FList)
	  }
	*/
	for _, f := range n.FList.List {
		if f.Name.Val == "__function_selector__" || (f.Name.Val == "function_selector" && len(f.Args.List) != 0) {
			return in.ActFunctionDef(f)
		}
	}
	return NodeValue{}
}
func (in *SubstrateInterpreter) ActStorageDeclList(l *ast.StorageDeclList) ast.Value {
	in.currNode = l
	//not used
	for _, n := range l.List {
		ast.Act(in, n)
	}
	return NodeValue{}
}
func extractStorageRef(k ValueType, c ast.CommentNode) storageRef {
	bit_s := uint8(0)
	bit_e := uint8(31)

	comment := string(c)
	slice := strings.Split(strings.Split(comment, "[")[1], "]")
	str := slice[0][2:]
	if len(str)%2 != 0 {
		str = "0" + str
	}
	bytes, _ := hex.DecodeString(str)
	index := uint256.NewInt(0).SetBytes(bytes)
	if slice[1] != "" {
		slice = strings.Split(strings.Split(slice[1], "bytes")[1], "to")
		bs, _ := strconv.ParseUint(slice[0][1:len(slice[0])-1], 0, 10)
		be, _ := strconv.ParseUint(slice[1][1:], 0, 10)
		bit_s = uint8(bs)
		bit_e = uint8(be)
	}
	return storageRef{k, index, bit_s, bit_e, ""}
}
func (in *SubstrateInterpreter) ActStorageDecl(n *ast.StorageDeclNode) ast.Value {
	in.currNode = n
	typ := in.ActTypeName(n.Typ).(NodeValue)
	id := in.ActPrimary(n.Id).(NodeValue)
	ref := extractStorageRef(typ.kind, n.Comment)
	ref.mapping = n.Typ.String()
	in.setNewStorageVar(id.GetId(), ref)
	in.framePtr.storageNum += 1
	//return NodeValue{kind: ValueStorId, value: ref}
	return NodeValue{}
}
func (in *SubstrateInterpreter) ActStructFieldList(l *ast.StructFieldList) ast.Value {
	in.currNode = l
	in.Panic("not used", l, true)
	return NodeValue{}
}
func (in *SubstrateInterpreter) ActStructFieldNode(n *ast.StructFieldNode) ast.Value {
	return NodeValue{}
}
func (in *SubstrateInterpreter) ActStructDefList(l *ast.StructDefList) ast.Value {
	in.currNode = l
	in.Panic("not used", l, true)
	return NodeValue{}
}
func (in *SubstrateInterpreter) ActStructDefNode(n *ast.StructDefNode) ast.Value {
	var lastField string
	f2t := make(map[string]string)
	for _, field := range n.Fields.List {
		f2t[field.Id.Val] = field.Typ.String()
		lastField = field.Id.Val
	}
	fieldSize := strings.Split(strings.Split(lastField, "field")[1], "_")
	pos, _ := strconv.ParseUint(fieldSize[0], 10, 64)
	in.framePtr.structInfo[n.Typ.String()] = structDecl{f2t, pos + 1}
	return NodeValue{}
}
func (in *SubstrateInterpreter) ActEventDeclList(l *ast.EventDeclList) ast.Value {
	in.currNode = l
	//not used
	for _, n := range l.List {
		ast.Act(in, n)
	}
	return NodeValue{}
}
func (in *SubstrateInterpreter) ActEventDecl(n *ast.EventDeclNode) ast.Value {
	in.currNode = n

	sig := n.Id.Val + "("
	args := ""
	if n.TypList != nil {
		for i, t := range n.TypList.List {
			args += t.String()
			if i < len(n.TypList.List)-1 {
				args += ","
			}
		}
	}
	sig += args + ")"
	e := eventDef{eventId: n.Id.Val, args: args, signature: in.getHash([]byte(sig)).Hex()}
	in.framePtr.eventList = append(in.framePtr.eventList, e)

	return NodeValue{}
}
func (in *SubstrateInterpreter) ActFunctionDefList(l *ast.FunctionDefList) ast.Value {
	in.currNode = l
	in.Panic("not used", l, true)
	//for _, n := range l.List {
	//   ast.Act(in, n)
	//}
	return NodeValue{}
}
func (in *SubstrateInterpreter) ActFunctionDef(n *ast.FunctionDefNode) ast.Value {
	in.currNode = n
	//ast.Act(in, n.Sig)

	/*if n.Name.Val != "__function_selector__" {
		p := ast.Printer{}
		ast.Act(&p, n)
		println(p.GetTree())
	}*/
	tmp := in.ActFormalArgList(n.Args).(NodeValue)
	if tmp.Type() == ValueTerminate {
		return tmp
	}
	ast.Act(in, n.Acc)
	if n.Pay != nil {
		ast.Act(in, n.Pay)
	}
	tmp = in.ActStmtBlock(n.Sb).(NodeValue)
	return tmp
}
func (in *SubstrateInterpreter) ActFormalArgList(l *ast.FormalArgList) ast.Value {
	in.currNode = l
	if len(in.framePtr.contract.Input) == 0 && in.framePtr.passedArgs == nil {
		return NodeValue{}
	}

	var argIds []NodeValue
	var types []string
	for _, n := range l.List {
		if n.Typ == nil {
			///////TODO: in case of function_selector, type is omitted for the argument. Separate mechanism might be needed.
			types = append(types, "bytes4")
		} else {
			types = append(types, n.Typ.String())
		}
		argIds = append(argIds, in.ActFormalArgNode(n).(NodeValue))
	}
	if !in.framePtr.isSelector {
		if in.framePtr.passedArgs == nil {
			in.unpackInput(types, argIds)
		} else {
			if len(l.List) != len(in.framePtr.passedArgs) /*&& !in.framePtr.isSelector */ {
				//for _, arg := range in.framePtr.passedArgs {
				//	println(mapping[arg.kind])
				//}
				in.Panic(fmt.Sprintf("Number of formal args does not match: %d %d", len(l.List), len(in.framePtr.passedArgs)), l, true)
			}
			in.setVariableList(NodeValue{value: argIds}, NodeValue{kind: ValueExprList, value: in.framePtr.passedArgs})
		}
	}
	return NodeValue{}
}

// TODO: remove Node
func (in *SubstrateInterpreter) ActFormalArgNode(n *ast.FormalArgNode) ast.Value {
	in.preAct(n)
	defer func() { in.framePtr.actDepth-- }()

	vType := ValueId

	if n.Id != nil {
		//ast.Act(in, n.Typ)
		//TODO: put proper value type based on n.Typ
		var typStr string
		if n.Typ != nil {
			typ := in.ActTypeName(n.Typ).(NodeValue)
			typStr = typ.value.(string)
		} else {
			typStr = "uint256"
		}
		//TODO: revise after type cast is implemented
		if in.framePtr.isSelector && n.Id.Val == "function_selector" {
			var size uint64
			if strings.Contains(typStr, "uint") {
				str := strings.Split(typStr, "uint")[1]
				size, _ = strconv.ParseUint(str, 10, 64)
			} else if strings.Contains(typStr, "bytes") {
				size = 256
			}
			input := uint256.NewInt(0).SetBytes(in.framePtr.contract.Input[0:32])
			in.framePtr.localVar["function_selector"] = input.Rsh(input, 256-uint(size))
			//in.framePtr.selector.Rsh(in.framePtr.selector, 256-uint(size))
		}
	}

	return NodeValue{kind: vType, value: n.Id.Val}
}
func (in *SubstrateInterpreter) ActTypeNameList(l *ast.TypeNameList) ast.Value {
	in.preAct(l)
	defer func() { in.framePtr.actDepth-- }()

	for _, n := range l.List {
		ast.Act(in, n)
	}
	return NodeValue{}
}
func (in *SubstrateInterpreter) ActTypeName(n *ast.TypeNameNode) ast.Value {
	in.preAct(n)
	defer func() { in.framePtr.actDepth-- }()

	switch n.Internal.(type) {
	case *ast.BasicTypeNode:
		return in.ActBasicType(n.Internal.(*ast.BasicTypeNode))
	case *ast.MapTypeNode:
		return in.ActMapType(n.Internal.(*ast.MapTypeNode))
	case *ast.StructArgTypeNode:
		return in.ActStructArgType(n.Internal.(*ast.StructArgTypeNode))
	case *ast.ArrayTypeNode:
		return in.ActArrayType(n.Internal.(*ast.ArrayTypeNode))
	case *ast.LibraryTypeNode:
		return in.ActLibraryType(n.Internal.(*ast.LibraryTypeNode))
	default:
		ast.Act(in, unimpl)
	}
	return NodeValue{}
}
func (in *SubstrateInterpreter) ActBasicType(n *ast.BasicTypeNode) ast.Value {
	in.preAct(n)
	defer func() { in.framePtr.actDepth-- }()

	return NodeValue{kind: ValueBasicType, value: n.Val}
}
func (in *SubstrateInterpreter) ActMapType(n *ast.MapTypeNode) ast.Value {
	in.preAct(n)
	defer func() { in.framePtr.actDepth-- }()

	return NodeValue{kind: ValueMapType, value: n.String()}
}
func (in *SubstrateInterpreter) ActStructArgType(n *ast.StructArgTypeNode) ast.Value {
	in.preAct(n)
	defer func() { in.framePtr.actDepth-- }()

	return NodeValue{kind: ValueStructArgType, value: n.String()}
}
func (in *SubstrateInterpreter) ActArrayType(n *ast.ArrayTypeNode) ast.Value {
	in.preAct(n)
	defer func() { in.framePtr.actDepth-- }()

	return NodeValue{kind: ValueArrayType, value: n.String()}
}
func (in *SubstrateInterpreter) ActLibraryType(n *ast.LibraryTypeNode) ast.Value {
	in.preAct(n)
	defer func() { in.framePtr.actDepth-- }()

	if n.Lvalue.Lval != nil {
		ast.Act(in, n.Lvalue.Lval)
		ast.Act(in, &ast.TokenNode{Val: n.Lvalue.Member})
	} else {
		ast.Act(in, n.Lvalue.Primary)
	}
	return NodeValue{}
}

func (in *SubstrateInterpreter) ActFallbackStmt(n *ast.FallbackStmtNode) ast.Value {
	return in.CallIntrinsic("revert", NodeValue{})
}

func (in *SubstrateInterpreter) ActStmtBlock(n *ast.StmtBlockNode) ast.Value {
	in.preAct(n)
	defer func() { in.framePtr.actDepth-- }()

	return in.ActStmtList(n.List)
}
func (in *SubstrateInterpreter) ActStmtList(l *ast.StmtList) ast.Value {
	in.preAct(l)
	defer func() { in.framePtr.actDepth-- }()

	for _, n := range l.List {
		result := in.ActStmt(n).(NodeValue)
		if !result.IsEmpty() { //TODO: must distinguish kind
			return result
		}
	}
	return NodeValue{}
}
func (in *SubstrateInterpreter) ActStmt(n *ast.StmtNode) ast.Value {
	in.preAct(n)
	defer func() { in.framePtr.actDepth-- }()

	var v NodeValue
	switch n.Internal.(type) {
	case *ast.IfStmtNode:
		v = in.ActIfStmt(n.Internal.(*ast.IfStmtNode)).(NodeValue)
	case *ast.WhileNode:
		v = in.ActWhile(n.Internal.(*ast.WhileNode)).(NodeValue)
	case *ast.DoWhileNode:
		v = in.ActDoWhile(n.Internal.(*ast.DoWhileNode)).(NodeValue)
	case *ast.StmtBlockNode:
		v = in.ActStmtBlock(n.Internal.(*ast.StmtBlockNode)).(NodeValue)
	case *ast.ContinueStmtNode:
		v = in.ActContinueStmt(n.Internal.(*ast.ContinueStmtNode)).(NodeValue)
	case *ast.BreakStmtNode:
		v = in.ActBreakStmt(n.Internal.(*ast.BreakStmtNode)).(NodeValue)
	case *ast.ReturnStmtNode:
		v = in.ActReturnStmt(n.Internal.(*ast.ReturnStmtNode)).(NodeValue)
	case *ast.RequireStmtNode:
		v = in.ActRequireStmt(n.Internal.(*ast.RequireStmtNode)).(NodeValue)
	case *ast.ThrowStmtNode:
		v = in.ActThrowStmt(n.Internal.(*ast.ThrowStmtNode)).(NodeValue)
	case *ast.EmitStmtNode:
		v = in.ActEmitStmt(n.Internal.(*ast.EmitStmtNode)).(NodeValue)
	case *ast.GotoStmtNode:
		v = in.ActGotoStmt(n.Internal.(*ast.GotoStmtNode)).(NodeValue)
	case *ast.CalcAssignStmtNode:
		v = in.ActCalcAssignStmt(n.Internal.(*ast.CalcAssignStmtNode)).(NodeValue)
	case *ast.ExprStmtNode:
		v = in.ActExprStmt(n.Internal.(*ast.ExprStmtNode)).(NodeValue)
	case *ast.FallbackStmtNode:
		v = in.ActFallbackStmt(n.Internal.(*ast.FallbackStmtNode)).(NodeValue)
	default:
		in.Panic("Not implemented stmt: "+unparser(n.Internal), n, false)
		v = in.ActBaseNode(unimpl).(NodeValue)
	}
	return v
}
func (in *SubstrateInterpreter) ActIfStmt(n *ast.IfStmtNode) ast.Value {
	in.preAct(n)
	defer func() { in.framePtr.actDepth-- }()
	putBranch(n, in.framePtr)

	result := in.ActIf(n.IfN).(NodeValue)
	if !result.IsEmpty() {
		if result.True() {
			return NodeValue{}
		} else {
			return result
		}
	}
	if n.Elif != nil {
		for _, e := range n.Elif.List {
			result = in.ActElseIf(e).(NodeValue)
			if !result.IsEmpty() {
				if result.True() {
					return NodeValue{}
				} else {
					return result
				}
			}
		}
	}
	if n.Els != nil {
		result = in.ActElse(n.Els).(NodeValue)
	}
	return result
}
func (in *SubstrateInterpreter) ActIf(n *ast.IfNode) ast.Value {
	in.preAct(n)
	defer func() { in.framePtr.actDepth-- }()

	cond := in.ActExpression(n.Expr).(NodeValue)
	if !in.getUint256(cond).IsZero() {
		n.SetCovered(true)
		sbResult := in.ActStmtBlock(n.Sb).(NodeValue)
		if sbResult.IsEmpty() {
			return NodeValue{kind: ValueTrue, value: nil}
		} else {
			return sbResult
		}
	} else {
		n.SetElseCovered(true)
	}
	return NodeValue{}
}
func (in *SubstrateInterpreter) ActElse(n *ast.ElseNode) ast.Value {
	in.preAct(n)
	defer func() { in.framePtr.actDepth-- }()

	n.SetCovered(true)
	return in.ActStmtBlock(n.Sb).(NodeValue)
}
func (in *SubstrateInterpreter) ActElseIfList(l *ast.ElseIfList) ast.Value {
	in.preAct(l)
	defer func() { in.framePtr.actDepth-- }()

	in.Panic("not used", l, true)
	//for _, n := range l.List {
	//   ast.Act(in, n)
	//}
	return NodeValue{}
}
func (in *SubstrateInterpreter) ActElseIf(n *ast.ElseIfNode) ast.Value {
	in.preAct(n)
	defer func() { in.framePtr.actDepth-- }()

	cond := in.ActExpression(n.Expr).(NodeValue)
	if !in.getUint256(cond).IsZero() {
		n.SetCovered(true)
		sbResult := in.ActStmtBlock(n.Sb).(NodeValue)
		if sbResult.IsEmpty() {
			return NodeValue{kind: ValueTrue, value: nil}
		} else {
			return sbResult
		}
	}
	return NodeValue{}
}
func (in *SubstrateInterpreter) ActWhile(n *ast.WhileNode) ast.Value {
	putBranch(n, in.framePtr)
	in.preAct(n)
	defer func() { in.framePtr.actDepth-- }()

	cond := in.ActExpression(n.Expr).(NodeValue)
	for ; !in.getUint256(cond).IsZero(); cond = in.ActExpression(n.Expr).(NodeValue) {
		if atomic.LoadInt32(&in.framePtr.terminate) != 0 {
			in.Panic("Terminate", n, false)
		}
		n.SetCovered(true)
		sbResult := in.ActStmt(n.Stmt).(NodeValue)
		if !sbResult.IsEmpty() {
			if sbResult.Type() == ValueBreak {
				break
			} else if sbResult.Type() == ValueContinue {
				continue
			} else if sbResult.Type() == ValueReturn || sbResult.Type() == ValueRevert || sbResult.Type() == ValueNum {
				return sbResult
			} else {
				in.Panic("Unknown statement value: "+mapping[sbResult.Type()], n, true)
			}
		}
	}
	n.SetOutCovered(true)
	return NodeValue{}
}
func (in *SubstrateInterpreter) ActDoWhile(n *ast.DoWhileNode) ast.Value {
	putBranch(n, in.framePtr)
	in.preAct(n)
	defer func() { in.framePtr.actDepth-- }()

	for {
		if atomic.LoadInt32(&in.framePtr.terminate) != 0 {
			in.Panic("Terminate", n, false)
		}
		n.SetCovered(true)
		sbResult := in.ActStmt(n.Stmt).(NodeValue)
		if !sbResult.IsEmpty() {
			if sbResult.Type() == ValueBreak {
				break
			} else if sbResult.Type() == ValueContinue {
			} else if sbResult.Type() == ValueReturn {
				return sbResult
			} else {
				in.Panic("Unknown statement value: "+mapping[sbResult.Type()], n, true)
			}
		}
		cond := in.ActExpression(n.Expr).(NodeValue)
		if in.getUint256(cond).IsZero() {
			break
		}
	}
	n.SetOutCovered(true)
	return NodeValue{}
}
func (in *SubstrateInterpreter) ActContinueStmt(n *ast.ContinueStmtNode) ast.Value {
	in.preAct(n)
	defer func() { in.framePtr.actDepth-- }()

	return NodeValue{kind: ValueContinue, value: nil}
}
func (in *SubstrateInterpreter) ActBreakStmt(n *ast.BreakStmtNode) ast.Value {
	in.preAct(n)
	defer func() { in.framePtr.actDepth-- }()

	return NodeValue{kind: ValueBreak, value: nil}
}

// //////////////////////////
func (in *SubstrateInterpreter) ActReturnStmt(n *ast.ReturnStmtNode) ast.Value {
	in.preAct(n)
	defer func() { in.framePtr.actDepth-- }()

	exprs := in.ActExpressionList(n.Exprs).(NodeValue)
	var v interface{}
	kind := ValueReturn
	switch exprs.Type() {
	case ValueExprList:
		if !in.framePtr.isPublic {
			kind = ValueExprList
		}
		v = in.getVariableList(exprs)
	case ValueMemId:
		ref, _ := in.getMemoryRef(exprs)
		v = in.getMemBytes(ref)
	case ValueStructId:
		kind = ValueStructId
		v = in.framePtr.localStruct[exprs.GetId()]
	default:
		if !in.framePtr.isPublic {
			kind = ValueNum
		}
		v = in.getUint256(exprs)
	}
	return NodeValue{kind: kind, value: v}
}
func (in *SubstrateInterpreter) ActRequireStmt(n *ast.RequireStmtNode) ast.Value {
	in.preAct(n)
	defer func() { in.framePtr.actDepth-- }()

	args := in.ActExpressionList(n.Exprs).(NodeValue)
	return in.CallIntrinsic("require", args)
}
func (in *SubstrateInterpreter) ActThrowStmt(n *ast.ThrowStmtNode) ast.Value {
	in.preAct(n)
	defer func() { in.framePtr.actDepth-- }()

	ast.Act(in, n.Exprs)
	return NodeValue{}
}
func (in *SubstrateInterpreter) ActEmitStmt(n *ast.EmitStmtNode) ast.Value {
	in.preAct(n)
	defer func() { in.framePtr.actDepth-- }()

	defer func() {
		in.checkOutOfGas()
	}()

	if useDelta {
		if in.Delta.EventCount() == len(in.Delta.RecordedTrace.EventTraces) {
			in.Panic("event number does not match: "+strconv.Itoa(in.Delta.EventCount())+" "+strconv.Itoa(len(in.Delta.RecordedTrace.EventTraces)), n, false)
		}
	}

	var exprList NodeValue
	var eventId string
	if n.Expr.Rvalue == nil {
		//emit statement has incorrect syntax. (e.g., ~0xff..ff & (0xff..00 & msg.data[0x0])(arglist) )
		left := in.ActSubexpr(n.Expr.Left).(NodeValue)
		right := in.ActLvalue(n.Expr.Right.Rvalue.Lvalue.Lval).(NodeValue)
		eventId = in.getUint256(in.doBinaryOp(left, right, n.Expr.Op)).Hex()
		exprList = in.ActExpressionList(n.Expr.Right.Rvalue.Lvalue.ExprList).(NodeValue)
	} else {
		event := n.Expr.Rvalue.Lvalue
		eventId = event.Lval.String()
		if eventId[:2] != "0x" {
			for _, e := range in.framePtr.eventList {
				if e.eventId != eventId {
					continue
				}
				if len(event.ExprList.List) == 0 && e.args == "" {
					eventId = e.signature
					break
				} else if len(event.ExprList.List) == len(strings.Split(e.args, ",")) {
					//judging the signature by types is more likely to fail
					eventId = e.signature
					break
				}
			}
		}
		if eventId[:2] != "0x" {
			in.Panic("incorrect event emit", n, false)
		}
		exprList = in.ActExpressionList(event.ExprList).(NodeValue)
	}

	recordedEvent := in.Delta.RecordedTrace.EventTraces[in.Delta.EventCount()]
	topicSize := len(recordedEvent.Topics)
	topics := make([]common.Hash, topicSize)
	args := in.getVariableList(exprList)

	topics[0] = common.HexToHash(eventId)
	if !useDelta {
		topicSize = 1
	}
	for i := 0; i < topicSize-1; i++ {
		tmp := in.getUint256(args[i]).Bytes32()
		topics[i+1] = common.BytesToHash(tmp[:])
	}

	var abis abi.Arguments
	var argList []interface{}
	for i := topicSize - 1; i < len(args); i++ {
		printDebug(mapping[args[i].Type()], false)
		if args[i].Type() == ValueMemBytes || args[i].Type() == ValueBytes || args[i].Type() == ValueString {
			t, _ := abi.NewType("bytes", "", nil)
			abis = append(abis, abi.Argument{Type: t})
			argList = append(argList, args[i].value.([]byte))
		} else if args[i].Type() == ValueMemRef {
			m := args[i].value.(*memoryRef)
			if m.index == ^uint64(0) {
				if m.typ == ValueNum {
					t, _ := abi.NewType("uint256[]", "", nil)
					abis = append(abis, abi.Argument{Type: t})
					uintBytes := in.getMemBytes(m)
					var uintArr []*big.Int
					for i := 0; i < len(uintBytes); i += 32 {
						uintArr = append(uintArr, new(big.Int).SetBytes(uintBytes[i:i+32]))
					}
					argList = append(argList, uintArr)
				} else {
					t, _ := abi.NewType("bytes", "", nil)
					abis = append(abis, abi.Argument{Type: t})
					argList = append(argList, in.getMemBytes(m))
				}
			} else {
				t, _ := abi.NewType("uint", "", nil)
				abis = append(abis, abi.Argument{Type: t})
				argList = append(argList, in.getUint256(args[i]).ToBig())
			}
		} else if _, ok := args[i].value.([]byte); ok {
			t, _ := abi.NewType("uint256[]", "", nil)
			abis = append(abis, abi.Argument{Type: t})
			uintBytes := args[i].value.([]byte)
			var uintArr []*big.Int
			for i := 0; i < len(uintBytes); i += 32 {
				uintArr = append(uintArr, new(big.Int).SetBytes(uintBytes[i:i+32]))
			}
			argList = append(argList, uintArr)
		} else {
			t, _ := abi.NewType("uint", "", nil)
			abis = append(abis, abi.Argument{Type: t})
			argList = append(argList, in.getUint256(args[i]).ToBig())
		}
	}

	packedArgs, err := abis.PackValues(argList)
	if err != nil {
		str := fmt.Sprintf("%s, %s", err.Error(), hex.EncodeToString(packedArgs))
		in.Panic("pack error: "+str, in.currNode, false)
	}

	if useDelta {
		//TODO: why sometime recorded event data is not 32-byte word size
		if len(packedArgs) > len(recordedEvent.Data) {
			packedArgs = packedArgs[:len(recordedEvent.Data)]
		}
	}

	in.evm.StateDB.AddLog(&types.Log{
		Address: in.framePtr.contract.Address(),
		Topics:  topics,
		Data:    packedArgs,
		// This is a non-consensus field, but assigned here because
		// core/state doesn't know the current block number.
		BlockNumber: in.evm.Context.BlockNumber.Uint64(),
	})
	in.Delta.SubstrateTrace.AddEventTrace(topics, packedArgs)

	if checkEMI {
		fmt.Print("INT LOG:")
		for _, t := range topics {
			fmt.Print(t.String(), ",")
		}
		fmt.Println("\nLOG Data:", hex.EncodeToString(packedArgs))
	}

	return NodeValue{}
}
func (in *SubstrateInterpreter) ActGotoStmt(n *ast.GotoStmtNode) ast.Value {
	in.preAct(n)
	defer func() { in.framePtr.actDepth-- }()
	//TODO: decide whether to comment out or not
	//in.Panic("Cannot handle goto address ", n, false)

	ast.Act(in, n.Addrs)
	return NodeValue{}
}
func (in *SubstrateInterpreter) ActCalcAssignStmt(n *ast.CalcAssignStmtNode) ast.Value {
	in.preAct(n)
	defer func() { in.framePtr.actDepth-- }()

	lvalue := in.ActLvalue(n.Lvalue).(NodeValue)
	rvalue := in.ActExpression(n.Expr).(NodeValue)
	result := in.doBinaryOp(lvalue, rvalue, string(n.Op[0]))
	if lvalue.Type() == ValueExprList {
		in.setVariableList(lvalue, rvalue)
	} else {
		in.setUint256(lvalue, in.getUint256(result))
	}
	return NodeValue{}
}
func (in *SubstrateInterpreter) ActExprStmt(n *ast.ExprStmtNode) ast.Value {
	in.preAct(n)
	defer func() { in.framePtr.actDepth-- }()

	ret := in.ActExpression(n.Expr).(NodeValue)
	//TODO: other than ValueRevert/ValueInvalid?
	if ret.Type() == ValueRevert || ret.Type() == ValueInvalid || ret.Type() == ValueTerminate ||
		(ret.Type() == ValueReturn && in.framePtr.callDepth == 0) { //public function returned in function selector
		return ret
	} else {
		return NodeValue{}
	}
}
func (in *SubstrateInterpreter) ActCastExpression(n *ast.CastExpressionNode) ast.Value {
	in.preAct(n)
	defer func() { in.framePtr.actDepth-- }()

	var v NodeValue
	typ := in.ActTypeName(n.Typ).(NodeValue)
	val := in.ActExpression(n.Expr).(NodeValue)
	bytes := in.getUint256(val).Bytes32()

	typeName := typ.value.(string)
	casted := uint256.NewInt(0)
	if typeName == "bool" {
		casted.SetBytes(bytes[:])
	} else if strings.Contains(typeName, "uint") {
		bitLen, err := strconv.Atoi(typeName[4:])
		if err != nil {
			in.Panic(fmt.Sprintf("%s", err), n, true)
		}
		casted.SetBytes(bytes[32-bitLen/8:])
	} else if strings.Contains(typeName, "int") {
		bitLen, err := strconv.Atoi(typeName[3:])
		if err != nil {
			in.Panic(fmt.Sprintf("%s", err), n, true)
		}
		casted.SetBytes(bytes[32-bitLen/8:])
	} else if strings.Contains(typeName, "bytes") && len(typeName) > 5 {
		byteLen, err := strconv.Atoi(typeName[5:])
		if err != nil {
			in.Panic(fmt.Sprintf("%s", err), n, true)
		}
		appendedBytes := append(bytes[:byteLen], make([]byte, 32-byteLen)...)
		casted.SetBytes(appendedBytes)
	} else if typeName == "address" {
		casted.SetBytes(bytes[12:])
	} else {
		//TODO: May need a conversion from bytes[:] to 'string' or 'bytes' type
		casted.SetBytes(bytes[:])
		in.Panic("Not implemented Cast Expression "+typ.value.(string), n, true)
	}
	v.kind = ValueCastExpr
	v.value = &castExpr{typeName, casted}
	return v
}
func (in *SubstrateInterpreter) ActExternalCallExpr(n *ast.ExternalCallExprNode) ast.Value {
	in.preAct(n)
	defer func() { in.framePtr.actDepth-- }()

	//not used
	ast.Act(in, n.Lvalue)
	ast.Act(in, &ast.TokenNode{Val: n.CallId})
	ast.Act(in, n.ExprList)
	return NodeValue{}
}
func (in *SubstrateInterpreter) ActExtSpecifierList(l *ast.ExtSpecifierList) ast.Value {
	in.preAct(l)
	defer func() { in.framePtr.actDepth-- }()

	//not used
	for _, n := range l.List {
		ast.Act(in, n)
	}
	return NodeValue{}
}
func (in *SubstrateInterpreter) ActExtSpecifier(n *ast.ExtSpecifierNode) ast.Value {
	in.preAct(n)
	defer func() { in.framePtr.actDepth-- }()

	//not used
	ast.Act(in, &ast.TokenNode{Val: n.Id})
	ast.Act(in, n.Expr)
	return NodeValue{}
}

func (in *SubstrateInterpreter) ActExpressionList(l *ast.ExpressionList) ast.Value {
	in.preAct(l)
	defer func() { in.framePtr.actDepth-- }()

	var v NodeValue
	if len(l.List) == 1 {
		v = in.ActExpression(l.List[0]).(NodeValue)
	} else {
		arr := make([]NodeValue, 0, 10) //TODO: 10 is enough?
		for _, n := range l.List {
			element := in.ActExpression(n).(NodeValue)
			arr = append(arr, element)
		}
		v.SetValue(ValueExprList, arr)
	}
	return v
}
func (in *SubstrateInterpreter) ActExpression(n *ast.ExpressionNode) ast.Value {
	in.preAct(n)
	defer func() { in.framePtr.actDepth-- }()

	v := in.ActSubexpr(n.Subexpr).(NodeValue)
	return v
}

func (in *SubstrateInterpreter) ActSubexpr(n *ast.SubexprNode) ast.Value {
	in.preAct(n)
	defer func() { in.framePtr.actDepth-- }()

	var v NodeValue
	if n.Exprs != nil { //assignExpr
		if n.Op != "=" {
			in.Panic("not assignExpr", n, true)
		}
		variables := in.ActExpressionList(n.Exprs).(NodeValue)
		rvalue := in.ActSubexpr(n.Right).(NodeValue)
		if rvalue.Type() == ValueRevert {
			return rvalue
		}
		//TODO: optimize if-elses
		if rvalue.Type() == ValueNewMem {
			ref := rvalue.value.(*memoryRef)
			ref.id = variables.GetId()
			v = NodeValue{kind: ValueMemRef, value: ref}
			in.setNewMemoryRef(v)
			in.framePtr.lVarType[variables.GetId()] = ValueMemId
		} else if rvalue.Type() == ValueNewStruct {
			ref := rvalue.value.(*memoryRef)
			ref.id = variables.GetId()
			v = NodeValue{kind: ValueStructId, value: ref}
			in.framePtr.localStruct[variables.GetId()] = ref
			in.framePtr.lVarType[variables.GetId()] = ValueStructId
		} else if variables.Type() == ValueExprList {
			in.setVariableList(variables, rvalue)
			v = variables
		} else {
			if bytes, ok := rvalue.value.([]byte); ok {
				if variables.Type() == ValueMem {
					allocInfo := variables.value.([]NodeValue)
					offset, length, size := allocInfo[0], allocInfo[1], allocInfo[2]
					if !length.IsEmpty() && size.IsEmpty() {
						merr := in.framePtr.memory.Ext_Set(in.getUint256(offset).Uint64(), in.getUint256(length).Uint64(), bytes, in.Delta.MaxGas)
						if merr != nil {
							in.Panic(merr.Error(), in.currNode, false)
						}
					} else {
						in.Panic("Unexpected length and size in ValueMem", n, false)
					}
				} else {
					varType := in.framePtr.lVarType[variables.GetId()]
					if varType == EMPTY {
						varType = ValueNum
					}
					v = NodeValue{kind: ValueMemRef, value: in.makeMemoryRef(variables.GetId(), varType, bytes)}
					in.setNewMemoryRef(v)
					in.framePtr.lVarType[variables.GetId()] = ValueMemId
				}
			} else {
				if rvalue.Type() == ValueExprList {
					//returned multiple values, but only one is used. (e.g., ecrecover)
					rvalue = rvalue.value.([]NodeValue)[0]
				}
				if ref, ok := rvalue.value.(*memoryRef); ok {
					if ref.index == ^uint64(0) {
						//e.g., v0 = v1 = new bytes[](n);
						v = NodeValue{kind: ValueMemRef, value: &memoryRef{id: variables.GetId(), typ: ref.typ, dataPtr: ref.dataPtr, length: ref.length, index: ^uint64(0)}}
						if rvalue.Type() == ValueStructId {
							in.framePtr.localStruct[variables.GetId()] = ref
							in.framePtr.lVarType[variables.GetId()] = ValueStructId
						} else {
							in.setNewMemoryRef(v)
							in.framePtr.lVarType[variables.GetId()] = ValueMemId
						}
						return v
					}
				}
				in.setUint256(variables, in.getUint256(rvalue))
				if variables.Type() == ValueId {
					if rvalue.Type() == ValueId {
						in.framePtr.lVarType[variables.GetId()] = in.framePtr.lVarType[rvalue.GetId()]
					} else if rvalue.Type() == ValueCastExpr {
						castType := rvalue.value.(*castExpr).castType
						if castType == "address" {
							in.framePtr.lVarType[variables.GetId()] = ValueAddress
						} else if castType == "bytes32" {
							in.framePtr.lVarType[variables.GetId()] = ValueBytes32
						} else if castType == "bool" {
							in.framePtr.lVarType[variables.GetId()] = ValueNum
						} else if castType[0:4] == "uint" {
							in.framePtr.lVarType[variables.GetId()] = ValueNum
						} else {
							in.Panic("unexpected type of castType in Subexpr "+castType, n, false)
						}
					} else if rvalue.Type() == ValueStruct || rvalue.Type() == ValueStorId || rvalue.Type() == ValueLocalStruct {
						//TODO: what if the type of storage struct is not uint256?
						in.framePtr.lVarType[variables.GetId()] = ValueNum
					} else {
						in.framePtr.lVarType[variables.GetId()] = rvalue.Type()
					}
				}
			}
			v = variables
		}
	} else if !n.IsUnary {
		left := in.ActSubexpr(n.Left).(NodeValue)
		right := in.ActSubexpr(n.Right).(NodeValue)
		v = in.doBinaryOp(left, right, n.Op)
	} else {
		v = in.ActRvalue(n.Rvalue).(NodeValue)
		if n.Op != "" {
			v = in.doUnaryOp(v, n.Op)
		}
	}
	return v
}

func (in *SubstrateInterpreter) ActRvalue(n *ast.RvalueNode) ast.Value {
	in.preAct(n)
	defer func() { in.framePtr.actDepth-- }()

	var v NodeValue
	if n.Cast != nil { //malloc statement (v0 = new type(...)) also copies the internal data
		//extract data type to copy
		split := strings.Split(unparser(n.Cast.Expr), ".length")
		dataVar := split[0]
		dataType := ValueBytes
		if len(split) == 2 {
			primaryNode := n.Cast.Expr.Subexpr.Rvalue.Lvalue.Lval
			str := unparser(primaryNode)
			dataType = in.framePtr.lVarType[str]
			if dataType == ValueMemId || dataType == ValueMemRef {
				memRef, ok := in.getMemoryRef(NodeValue{value: str})
				if !ok {
					in.Panic("Failed to find memoryRef type of "+str, n, false)
				}
				dataType = memRef.typ
			} else if dataType == EMPTY {
				if str == "msg.data" {
					dataType = ValueBytes
				} else {
					if n.Cast.Typ.String() == "uint256" || n.Cast.Typ.String() == "uint256[]" {
						dataType = ValueNum
					} else {
						dataType = ValueBytes
					}
				}
			} else {
				p := in.ActLvalue(primaryNode).(NodeValue)
				if p.Type() == ValueArgId {
					arg := in.framePtr.actualArgs[p.value.(string)]
					if arg.typ != "bytes" && arg.typ != "string" {
						dataType = ValueNum
					}
				}
			}
		}

		length := in.getUint256(in.ActExpression(n.Cast.Expr).(NodeValue))
		memSize := length.Uint64()
		if n.Cast.Typ.String() == "struct" {
			memSize = memSize << 5
		} else if n.Cast.Typ.String() == "bytes[]" || dataType == ValueString || dataType == ValueBytes {
			memSize = (length.Uint64() + 31) / 32 * 32 //round up to 32-word size
			memSize = memSize + 32
		} else {
			memSize = memSize<<5 + 32
		}
		ptr := in.getFreeMemPtr(memSize, true)
		dataPtr := uint256.NewInt(0).AddUint64(ptr, 32)
		if n.Cast.Typ.String() != "struct" {
			merr := in.framePtr.memory.Ext_Set32(ptr.Uint64(), length, in.Delta.MaxGas)
			if merr != nil {
				in.Panic(merr.Error(), in.currNode, false)
			}
		}

		if len(split) == 2 {
			if dataVar == "msg.data" {
				merr := in.framePtr.memory.Ext_Set(dataPtr.Uint64(), length.Uint64(), in.framePtr.contract.Input, in.Delta.MaxGas)
				if merr != nil {
					in.Panic(merr.Error(), in.currNode, false)
				}
			} else if _, ok := in.framePtr.storageVar[dataVar]; ok {
				//Data will be copied later
			} else if _, ok := in.framePtr.actualArgs[dataVar]; ok {
				//Will have CALLDATACOPY later
			} else if in.framePtr.lVarType[dataVar] == ValueMemId {
				m, ok := in.getMemoryRef(NodeValue{kind: ValueId, value: dataVar})
				if !ok {
					in.Panic("Could not find memoryRef", n, false)
				}
				data := in.framePtr.memory.GetPtr(int64(m.dataPtr.Uint64()), int64(m.length.Uint64()))
				merr := in.framePtr.memory.Ext_Set(dataPtr.Uint64(), length.Uint64(), data, in.Delta.MaxGas)
				if merr != nil {
					in.Panic(merr.Error(), in.currNode, false)
				}
			} else if _, ok := in.framePtr.localVar[dataVar]; ok {
				in.Panic("Incorrect use of length for local variable", n, false)
			} else {
				//Data will be copied later
			}
			v = NodeValue{kind: ValueNewMem, value: &memoryRef{id: "", typ: dataType, dataPtr: dataPtr, length: length, index: ^uint64(0)}}
		} else {
			if n.Cast.Typ.String() == "struct" {
				dataPtr.SubUint64(dataPtr, 32) //struct does not need length field
				v = NodeValue{kind: ValueNewStruct, value: &memoryRef{id: "", typ: dataType, dataPtr: dataPtr, length: length, index: ^uint64(0)}}
			} else {
				v = NodeValue{kind: ValueNewMem, value: &memoryRef{id: "", typ: dataType, dataPtr: dataPtr, length: length, index: ^uint64(0)}}
			}
		}
	} else if n.ExtCall != nil {
		if n.ExtCall.Lvalue.String() == "block" && n.ExtCall.CallId == "blockhash" {
			blockN := in.ActExpressionList(n.ExtCall.ExprList).(NodeValue)
			n := in.getUint256(blockN).Uint64()
			bhash := uint256.NewInt(0).SetBytes(in.evm.Context.GetHash(n).Bytes())
			return NodeValue{kind: ValueNum, value: bhash}
		}
		//get value and gas before call
		callGas := uint64(2300)
		callValue := uint256.NewInt(0)
		for _, n := range n.ExtSpec.List {
			if n.Id == "value" {
				val := in.ActExpression(n.Expr).(NodeValue)
				callValue = in.getUint256(val)
			} else if n.Id == "gas" {
				gas := in.ActExpression(n.Expr).(NodeValue)
				callGas = in.getUint256(gas).Uint64()
			} else {
				in.Panic("unexpected specifier", n, true)
			}
		}

		//get input data from expression list
		isDelegate := false
		input := make([]byte, 0)
		exprList := in.ActExpressionList(n.ExtCall.ExprList).(NodeValue)
		var typList []string
		if n.ExtCall.CallId[0:2] == "0x" {
			//TODO: need to check
			hex, _ := hex.DecodeString(n.ExtCall.CallId)
			input = append(input, hex...)
		} else if n.ExtCall.CallId == "delegatecall" {
			isDelegate = true
			if exprList.Type() == ValueExprList {
				for _, expr := range exprList.value.([]NodeValue) {
					t := in.getTypeString(expr)
					typList = append(typList, t)
				}
			} else {
				t := in.getTypeString(exprList)
				typList = append(typList, t)
			}
		} else if n.ExtCall.CallId == "call" {
			printDebug("call", false)
			if sig, ok := exprList.value.(*uint256.Int); ok {
				hex := sig.Bytes32()
				input = append(input, hex[28:32]...)
				exprList.kind = ValueExprList
				exprList.value = []NodeValue{}
			} else if len(exprList.value.([]NodeValue)) > 1 {
				expr := exprList.value.([]NodeValue)[0]
				hex := in.getUint256(expr).Bytes32()
				input = append(input, hex[28:32]...)
				exprList.value = exprList.value.([]NodeValue)[1:]
				for _, expr := range exprList.value.([]NodeValue) {
					t := in.getTypeString(expr)
					typList = append(typList, t)
				}
			}
		} else {
			signature := n.ExtCall.CallId + "("
			if exprList.Type() == ValueExprList {
				for _, expr := range exprList.value.([]NodeValue) {
					t := in.getTypeString(expr)
					typList = append(typList, t)
					signature += t + ","
				}
			} else {
				t := in.getTypeString(exprList)
				typList = append(typList, t)
				signature += t
			}
			if signature[len(signature)-1] == ',' {
				signature = signature[:len(signature)-1]
			}
			signature += ")"
			hexSig := in.getHash([]byte(signature)).Bytes()
			input = append(input, hexSig[0:4]...)
			printDebug("call signature: "+signature, false)
		}
		var useMEM bool
		var inputArg []interface{}
		for i, expr := range in.getVariableList(exprList) {
			switch expr.Type() {
			case ValueNum, ValueHex, ValueAccess, ValueReturn, ValueLocalStruct:
				inputArg = append(inputArg, in.getUint256(expr).ToBig())
			case ValueAddress:
				bytes := in.getUint256(expr).Bytes32()
				inputArg = append(inputArg, common.BytesToAddress(bytes[:]))
			case ValueString:
				inputArg = append(inputArg, string(expr.value.([]byte)))
			case ValueMemBytes:
				inputArg = append(inputArg, expr.value.([]byte))
			case ValueBytes:
				inputArg = append(inputArg, expr.value.([]byte))
			case ValueBytes32:
				inputArg = append(inputArg, in.getUint256(expr).Bytes32())
			case ValueMem:
				allocInfo := expr.value.([]NodeValue)
				offset, length, size := allocInfo[0], allocInfo[1], allocInfo[2]
				p := int64(in.getUint256(offset).Uint64())
				l := int64(32)
				if length.IsEmpty() && size.IsEmpty() {
				} else if !length.IsEmpty() && size.IsEmpty() {
					l = int64(in.getUint256(length).Uint64())
				} else if length.IsEmpty() && !size.IsEmpty() {
					l = int64(in.getUint256(size).Uint64())
				}
				mem := in.framePtr.memory.GetPtr(p, l)
				if i == 0 || useMEM {
					useMEM = true
					input = append(input, mem...)
				} else {
					inputArg = append(inputArg, uint256.NewInt(0).SetBytes(mem).ToBig())
				}
			case ValueCastExpr:
				cast := expr.value.(*castExpr)
				if cast.castType == "address" {
					inputArg = append(inputArg, common.BytesToAddress(cast.val.Bytes()))
				} else if cast.castType == "bytes32" {
					inputArg = append(inputArg, cast.val.Bytes32())
				} else if cast.castType[0:4] == "uint" {
					bit, _ := strconv.Atoi(cast.castType[4:])
					typeMap := map[int]func(uint64) interface{}{
						8:  func(v uint64) interface{} { return uint8(v) },
						16: func(v uint64) interface{} { return uint16(v) },
						32: func(v uint64) interface{} { return uint32(v) },
						64: func(v uint64) interface{} { return uint64(v) },
					}
					if castFunc, ok := typeMap[bit]; ok {
						inputArg = append(inputArg, castFunc(cast.val.Uint64()))
					} else {
						inputArg = append(inputArg, cast.val.ToBig())
					}
				} else if cast.castType[0:4] == "bool" {
					inputArg = append(inputArg, !cast.val.IsZero())
				} else {
					in.Panic("Unexpected type of castexpr in variable list "+cast.castType, n, false)
				}
			case ValueMemRef:
				ref := expr.value.(*memoryRef)
				if ref.index == ^uint64(0) {
					bytes := in.getMemBytes(ref)
					if ref.typ == ValueString {
						inputArg = append(inputArg, string(bytes))
					} else {
						inputArg = append(inputArg, bytes)
					}
				} else {
					inputArg = append(inputArg, in.getUint256(expr).ToBig())
				}
			case ValueMemId:
				memRef, _ := in.getMemoryRef(expr)
				in.Panic("Unexpected type of MemId in variable list "+expr.GetId()+" "+mapping[memRef.typ], n, false)
			default:
				in.Panic("Unexpected type of expr in variable list "+mapping[expr.Type()], n, false)
			}
		}
		printDebug(fmt.Sprint("types: ", typList), false)
		printDebug(fmt.Sprint("args: ", inputArg), false)

		if !useMEM {
			var args abi.Arguments
			for _, typ := range typList {
				t, _ := abi.NewType(typ, "", nil)
				args = append(args, abi.Argument{Type: t})
			}
			packed, err := args.PackValues(inputArg)
			if err != nil {
				str := fmt.Sprintf("%s, %s", err.Error(), hex.EncodeToString(packed))
				in.Panic("pack error: "+str, in.currNode, false)
			}
			input = append(input, packed...)
		}

		//call
		lvalue := in.ActLvalue(n.ExtCall.Lvalue).(NodeValue)
		callee := in.getUint256(lvalue).Bytes20()

		if false {
			printDebug("call signature: "+n.ExtCall.CallId, true)
			printDebug("call data: "+hex.EncodeToString(input), true)
			if callValue != nil {
				printDebug("call value: "+callValue.Hex(), false)
			}
			printDebug("call gas: "+strconv.Itoa(int(callGas)), true)
			printDebug("External call to "+hex.EncodeToString(callee[:]), true)
			printDebug("MD5: "+getMD5(in.evm.StateDB.GetCode(callee)), true)
		}

		var ret []byte
		var err error
		if isDelegate {
			ret, _, err = in.evm.DelegateCall(in.framePtr.contract, callee, input, callGas)
		} else {
			ret, _, err = in.evm.Call(in.framePtr.contract, callee, input, callGas, callValue)
		}
		in.returnData = make([]byte, len(ret))
		copy(in.returnData, ret)
		printDebug("call return: "+hex.EncodeToString(ret), false)
		if err != nil {
			printDebug("call err: "+err.Error(), false)
		}

		cr := callReturn{success: (err == nil)}
		cr.retVal = uint256.NewInt(0).SetBytes(ret)
		if ret == nil && err == nil {
			//retVal of callee has been omptimized out by the compiler, while the substrate still looks for it
			//TODO: what if the return value is indeed 0? Will it be different from 'ret == nil'
			cr.retVal.SetOne()
		}
		if ret != nil {
			//Set return data into the memory (at MEM[MEM[0x40]])
			ptr := in.getFreeMemPtr(uint64(len(ret)), false)
			merr := in.framePtr.memory.Ext_Set(ptr.Uint64(), uint64(len(ret)), ret, in.Delta.MaxGas)
			if merr != nil {
				in.Panic(merr.Error(), in.currNode, false)
			}
		}
		v = NodeValue{kind: ValueCallReturn, value: cr}

	} else if n.Lvalue != nil {
		v = in.ActLvalue(n.Lvalue).(NodeValue)
	} else {
		ast.Act(in, unimpl)
	}
	return v
}
func (in *SubstrateInterpreter) ActLvalue(n *ast.LvalueNode) ast.Value {
	in.preAct(n)
	defer func() { in.framePtr.actDepth-- }()

	var v NodeValue
	extractStorType := func(str string, t ValueType) (typ string) {
		if t == ValueArrayType {
			typ = str[:len(str)-2]
		} else if t == ValueMapType {
			typ = strings.SplitN(str, "=>", 2)[1]
			typ = typ[1 : len(typ)-1]
		} else if t == ValueStorRef {
			split := strings.SplitN(str, "=>", 2)
			if len(split) > 1 {
				typ = strings.SplitN(str, "=>", 2)[1]
				typ = typ[1 : len(typ)-1]
			} else {
				typ = str[:len(str)-2]
			}
		} else {
			in.Panic("Unexpected ValueType in extractStorType()", n, false)
		}
		return
	}
	switch n.LvalueType {
	case ast.TypeAlloc:
		alloc := in.ActAllocSize(n.AllocSize).(NodeValue).value.([]NodeValue)
		if n.Lval != nil {
			id := in.ActLvalue(n.Lval).(NodeValue)
			switch id.Type() {
			case ValueMsgData:
				msgData := id.value.([]byte)
				index := in.getUint256(alloc[0]).Uint64()
				last := index + 32
				if last > uint64(len(msgData)) {
					last = uint64(len(msgData))
				}
				if index > uint64(len(msgData)) {
					index = 0
					last = 0
				}
				v.SetValue(ValueNum, uint256.NewInt(0).SetBytes(msgData[index:last]))
			case ValueStorId:
				stor, _ := in.getStorageRef(id)
				typ := extractStorType(stor.mapping, stor.kind)
				if stor.kind == ValueArrayType {
					bytes := uint256.NewInt(0).Set(stor.index).Bytes32()
					index := in.getUint256(alloc[0]).Uint64()
					if structInfo, ok := in.framePtr.structInfo[typ]; ok {
						index = index * structInfo.size
					}
					hash := in.getHash(bytes[:])
					hash.Add(hash, uint256.NewInt(index))
					v.SetValue(ValueStorRef, &storVar{n, typ, hash})
				} else if stor.kind == ValueMapType {
					a, b := in.getUint256(alloc[0]).Bytes32(), uint256.NewInt(0).Set(stor.index).Bytes32()
					hash := in.getHash(append(a[:], b[:]...))
					printDebug(hex.EncodeToString(b[:])+", "+hex.EncodeToString(a[:]), false)
					printDebug(hex.EncodeToString(hash.Bytes()), false)
					v.SetValue(ValueStorRef, &storVar{n, typ, hash})
				} else {
					in.Panic("unknown storage type", n, true)
				}
				printDebug(n.String()+": "+typ, false)
			case ValueStorRef:
				lval := id.value.(*storVar)
				//Get storage in 'stor[x][index]'
				//id already contains stor[x]
				if strings.Contains(lval.typ, "mapping") { //mapping type
					slot := lval.hash.Bytes32()
					index := in.getUint256(alloc[0]).Bytes32()
					hash := in.getHash(append(index[:], slot[:]...))
					typ := extractStorType(lval.typ, id.Type())
					v.SetValue(ValueStorRef, &storVar{n, typ, hash})
					printDebug(n.String()+": "+typ, false)
				} else { //array type
					index := in.getUint256(alloc[0])
					hash := in.getHash(lval.hash.PaddedBytes(32))
					hash.Add(hash, index)
					//hash := in.getHash(index.Add(lval.hash, index).PaddedBytes(32))
					typ := extractStorType(lval.typ, id.Type())
					v.SetValue(ValueStorRef, &storVar{n, typ, hash})
					printDebug(n.String()+": "+typ, false)
				}
			case ValueStruct: //'stor.field0[index]'
				var hash *uint256.Int
				lval := id.value.(*storStruct)
				index := in.getUint256(alloc[0])
				if strings.Contains(lval.fieldType, "mapping") { //field type is mapping
					hash = in.getHash(append(index.PaddedBytes(32), lval.hash.PaddedBytes(32)...))
					printDebug(n.String()+", "+hash.Hex(), false)
				} else { //field type is array
					hash = in.getHash(lval.hash.PaddedBytes(32))
					hash.AddUint64(hash, index.Uint64())
				}
				v.SetValue(ValueStorRef, &storVar{n, "N/A", hash})
			case ValueMemId:
				ref, _ := in.getMemoryRef(id)
				index := in.getUint256(alloc[0]).Uint64()
				if ref.typ == ValueNum || ref.typ == ValueAddress {
					index = index * 32
				} else if ref.typ == ValueBytes || ref.typ == ValueString {
				} else if ref.typ == ValueMemRef || ref.typ == ValueLocalStruct {
				} else {
					in.Panic("Check new type[] in getUint256() "+mapping[ref.typ], in.currNode, false)
				}
				ret := ref.SetIndex(index)
				if ret == nil {
					in.Panic("invalid index in SetIndex", n, false)
				}
				v.SetValue(ValueMemRef, ret)
			case ValueMemRef:
				//TODO
				in.Panic("2D memory array not implemented", n, false)
				v.SetValue(ValueTerminate, nil)
			case ValueArgId:
				arg, ok := in.framePtr.actualArgs[id.value.(string)]
				if !ok {
					in.Panic("ArgId for TypeAlloc does not exist "+id.value.(string), n, false)
				}
				index := in.getUint256(alloc[0]).Uint64()
				offset := arg.ptr + index*32
				val := uint256.NewInt(0).SetBytes(in.framePtr.contract.Input[offset : offset+32])
				v.SetValue(ValueNum, val)
			default:
				if id.value.(string) == "MEM8" {
					v.SetValue(ValueMem8, alloc)
				} else {
					in.Panic("unknown type for TypeAlloc "+mapping[id.Type()], n, false)
				}
			}
		} else {
			if n.AllocLoc == "STORAGE" {
				v.SetValue(ValueStorage, alloc)
			} else {
				v.SetValue(ValueMem, alloc)
			}
		}
	case ast.TypeCall:
		//inSelector := false
		tmp := in.ActLvalue(n.Lval).(NodeValue)
		signature, kind := tmp.GetSig()
		args := in.ActExpressionList(n.ExprList).(NodeValue)
		if signature == "fallback" && len(n.ExprList.List) == 0 {
			signature = "function_selector"
		}
		if isIntrinsic(signature) {
			return in.CallIntrinsic(signature, args)
		}
		printDebug("Depth: "+strconv.Itoa(in.evm.depth)+"\nCalling "+signature, false)
		if getCheckCoverage() {
			in.framePtr.CheckCvg = true
		}
		found := false
	searchFunc:
		for _, f := range in.framePtr.contract.AstRoot.FList.List {
			if (kind == ValueId && signature == f.Name.Val && len(n.ExprList.List) == len(f.Args.List)) ||
				(kind == ValueHex && signature == f.Signature()) {
				if kind == ValueId && in.framePtr.isSelector {
					for i, arg := range f.Args.List {
						if unparser(n.ExprList.List[i]) != arg.Typ.String() {
							continue searchFunc
						}
					}
				}
				found = true
				currentArgs := in.framePtr.passedArgs
				currentStruct := in.framePtr.localStruct
				currentVars := in.framePtr.localVar
				currentVarType := in.framePtr.lVarType
				currentMemVars := in.framePtr.memoryVar
				currentVisib := in.framePtr.isPublic

				//Set-up new local variable and actual arguments
				in.framePtr.isPublic = f.Acc.Val == "public"
				if !in.framePtr.isSelector || !in.framePtr.isPublic {
					in.framePtr.passedArgs = make([]NodeValue, 0, 20)
					in.framePtr.passedArgs = append(in.framePtr.passedArgs, in.getVariableList(args)...)
				}
				in.framePtr.isSelector = false
				in.framePtr.localStruct = make(map[string]*memoryRef)
				in.framePtr.localVar = make(map[string]*uint256.Int)
				in.framePtr.lVarType = make(map[string]ValueType)
				in.framePtr.memoryVar = make(map[string]*memoryRef)

				if false {
					printDebug("Internal Call: "+signature, true)
					var tmp string
					for _, arg := range in.framePtr.passedArgs {
						tmp += mapping[arg.kind] + ","
					}
					printDebug("Passed types: "+tmp, true)
					tmp = ""
					for _, arg := range in.framePtr.passedArgs {
						if arg.kind == ValueMemRef {
							tmp += arg.value.(*memoryRef).dataPtr.Hex()
						} else if arg.kind != ValueString && arg.kind != ValueBytes {
							tmp += in.getUint256(arg).Hex()
						} else {
							if arg.kind == ValueString {
								tmp += string(arg.value.([]byte))
							} else {
								tmp += hex.EncodeToString(arg.value.([]byte))
							}
						}
						tmp += ","
					}
					printDebug("Passed values: "+tmp, true)
				}

				in.framePtr.callDepth++
				v = in.ActFunctionDef(f).(NodeValue)
				in.framePtr.callDepth--
				if in.framePtr.callDepth == 0 && v.Type() == EMPTY {
					v.kind = ValueReturn
				}

				//restore call context
				in.framePtr.passedArgs = currentArgs
				in.framePtr.localStruct = currentStruct
				in.framePtr.localVar = currentVars
				in.framePtr.lVarType = currentVarType
				in.framePtr.memoryVar = currentMemVars
				in.framePtr.isPublic = currentVisib
				break
			}
		}
		if !found {
			if signature == "function_selector" && len(n.ExprList.List) == 0 { //implicit fallback()
				return in.CallIntrinsic("revert", NodeValue{})
			}
			in.Panic("Could not find "+signature, n, false)
		}
		//TODO: handle external call?
	case ast.TypeAccess:
		lval := in.ActLvalue(n.Lval).(NodeValue)
		lvalType := lval.Type()
		if lvalType == ValueMsgData {
			if n.Member != "length" { //TODO: remove later
				in.Panic("member of msg.data is not 'length'", n, true)
			}
			length := uint64(len(lval.value.([]byte)))
			v.SetValue(ValueNum, uint256.NewInt(length))
		} else if lvalType == ValueStorId {
			id, exist := in.getStorageRef(lval)
			if exist {
				v.SetValue(ValueAccess, &storAccess{id, n.Member})
			} else {
				in.Panic("Not implemented type of TypeAccess", n, true)
			}
		} else if lvalType == ValueMemId {
			id, exist := in.getMemoryRef(lval)
			if exist {
				v.SetValue(ValueAccess, &memAccess{id, n.Member})
			} else {
				in.Panic("Not implemented type of TypeAccess", n, true)
			}
		} else if lvalType == ValueStructId {
			if n.Member[0:4] != "word" {
				in.Panic("member for local struct is not word: "+n.Member, n, false)
			}
			index, _ := strconv.Atoi(n.Member[4:])
			ls := in.framePtr.localStruct[lval.GetId()]
			lsaccess := &memoryRef{id: ls.id + "." + n.Member, typ: ls.typ, dataPtr: ls.dataPtr, length: ls.length}
			lsaccess.index = uint64(index) << 5
			v.SetValue(ValueLocalStruct, lsaccess)
		} else if n.Member == "code.size" {
			addr := in.getUint256(lval)
			codesize := uint256.NewInt(uint64(in.evm.StateDB.GetCodeSize(common.BytesToAddress(addr.Bytes()))))
			v.SetValue(ValueNum, codesize)
		} else if lvalType == ValueArgId {
			ref, exist := in.getArgumentRef(lval)
			if exist {
				if n.Member == "length" {
					v.SetValue(ValueNum, uint256.NewInt(ref.length))
				} else if n.Member == "data" { //v0.data means a start address of the data of v0
					v.SetValue(ValueNum, uint256.NewInt(ref.ptr))
				} else if strings.Contains(n.Member, "word") { //word0 == first 32-byte, length of string
					///////////////////////////
					//Need to check correctness
					///////////////////////////
					pos, _ := strconv.ParseUint(n.Member[4:], 10, 64)
					offset := ref.ptr + (pos-1)*32
					word := in.framePtr.contract.Input[offset : offset+32]
					v.SetValue(ValueNum, uint256.NewInt(0).SetBytes(word))
				} else {
					in.Panic("member of input argument is not correct", n, true)
				}
			} else {
				in.Panic("Not implemented type of TypeAccess", n, true)
			}
		} else if n.Member == "max" {
			inttype := n.Lval.String()
			bitN := 0
			if inttype[:4] == "uint" {
				bitN, _ = strconv.Atoi(unparser(n.Lval)[4:])
			} else if inttype[:3] == "int" {
				bitN, _ = strconv.Atoi(unparser(n.Lval)[3:])
				bitN -= 1
			} else {
				in.Panic("Unknown type for max "+unparser(n.Lval), n, false)
			}
			max := uint256.NewInt(1)
			max.Lsh(max, uint(bitN)).Sub(max, uint256.NewInt(1)) // 1<<bitN - 1
			v.SetValue(ValueNum, max)
		} else if n.Member == "min" {
			inttype := n.Lval.String()
			bitN := 0
			if inttype[:3] != "int" {
				in.Panic("Unknown type for min "+unparser(n.Lval), n, false)
			}
			bitN, _ = strconv.Atoi(unparser(n.Lval)[3:])
			min := uint256.NewInt(1)
			min.Lsh(min, uint(bitN)-1)
			v.SetValue(ValueNum, min)
		} else if n.Member == "origin" && unparser(n.Lval) == "tx" {
			v.SetValue(ValueNum, uint256.NewInt(0).SetBytes(in.evm.Origin.Bytes()))
		} else if n.Member == "balance" {
			addr := in.getUint256(lval)
			balance := in.evm.StateDB.GetBalance(common.BytesToAddress(addr.Bytes()))
			v.SetValue(ValueNum, balance)
		} else if lvalType == ValueId {
			if lval.GetId() == "block" {
				blockVal := uint256.NewInt(0)
				switch n.Member {
				case "number":
					blockVal, _ = uint256.FromBig(in.evm.Context.BlockNumber)
				case "timestamp":
					blockVal.SetUint64(in.evm.Context.Time)
				case "difficulty":
					blockVal, _ = uint256.FromBig(in.evm.Context.Difficulty)
				case "coinbase":
					blockVal.SetBytes(in.evm.Context.Coinbase.Bytes())
				case "gaslimit":
					blockVal.SetUint64(in.evm.Context.GasLimit)
				default:
					in.Panic("Wrong member for block."+n.Member, n, false)
				}
				v.SetValue(ValueNum, blockVal)
			} else if n.Member == "data" {
				//v0.data is being used without initialization (i.e., without v0 = new bytes[](...);)
				in.Panic("Wrong type for TypeAccess("+mapping[lvalType]+"."+n.Member+")", n, false)
			} else {
				in.Panic("Wrong type for TypeAccess("+mapping[lvalType]+"."+n.Member+")", n, false)
				v.SetValue(ValueTerminate, uint256.NewInt(0))
			}
		} else if lvalType == ValueStorRef {
			id := lval.value.(*storVar)
			if n.Member == "length" {
				if id.typ[len(id.typ)-2:] != "[]" {
					in.Panic("Incorrect use of .length for ValueStorRef", n, false)
				}
				v.SetValue(ValueStorRef, &storVar{n, "N/A", id.hash})
			} else if n.Member == "data" {
				v.SetValue(ValueNum, in.getHash(id.hash.Bytes()))
			} else {
				fieldType := "uint256"
				structInfo, ok := in.framePtr.structInfo[id.typ]
				if ok {
					fieldType = structInfo.field2type[n.Member]
				}
				str := strings.Split(n.Member, "field")[1]
				str2 := strings.Split(str, "_")
				pos, _ := strconv.ParseUint(str2[0], 10, 64)
				bit_start, bit_end := uint64(1), uint64(0) //start > end indicates it is not used
				if len(str2) > 1 {
					bit_start, _ = strconv.ParseUint(str2[1], 10, 64)
					bit_end, _ = strconv.ParseUint(str2[2], 10, 64)
				}
				hash := id.hash.Add(id.hash, uint256.NewInt(pos))
				v.SetValue(ValueStruct, &storStruct{id, pos, fieldType, hash, bit_start, bit_end})
				printDebug("Get "+id.name.String()+"."+n.Member, false)
			}
		} else if lvalType == ValueStruct {
			ss := lval.value.(*storStruct)
			if n.Member == "length" {
				v.SetValue(ValueStorRef, &storVar{n, "N/A", ss.hash})
			} else if n.Member == "data" {
				v.SetValue(ValueStorRef, &storVar{n, "N/A", in.getHash(ss.obj.hash.Bytes())})
			} else {
				//TODO: implement the case like a.field0.field1
				in.Panic("now implementing... (lvalType == ValueStruct)", n, false)
			}
		} else {
			in.Panic("Wrong type for TypeAccess ("+mapping[lvalType]+"."+n.Member+")", n, false)
			v.SetValue(ValueTerminate, uint256.NewInt(0))
		}
	case ast.TypePrimary:
		v = in.ActPrimary(n.Primary).(NodeValue)
	case ast.TypeCast:
		v = in.ActCastExpression(n.Cast).(NodeValue)
	case ast.TypeExpr:
		v = in.ActExpression(n.Expr).(NodeValue)
	case ast.TypeType:
		ast.Act(in, n.Typ)
	default:
		ast.Act(in, unimpl)
	}
	return v
}

func (in *SubstrateInterpreter) ActAllocSize(n *ast.AllocSizeNode) ast.Value {
	in.preAct(n)
	defer func() { in.framePtr.actDepth-- }()

	var v NodeValue
	v.SetValue(ValueAlloc, make([]NodeValue, 3))
	v.value.([]NodeValue)[0] = in.ActExpression(n.From).(NodeValue)
	if n.Length != nil {
		v.value.([]NodeValue)[1] = in.ActExpression(n.Length).(NodeValue)
	}
	if n.To != nil {
		v.value.([]NodeValue)[2] = in.ActExpression(n.To).(NodeValue)
	}
	return v
}
func (in *SubstrateInterpreter) ActPrimaryList(l *ast.PrimaryList) ast.Value {
	in.preAct(l)
	defer func() { in.framePtr.actDepth-- }()

	for _, n := range l.List {
		ast.Act(in, n)
	}
	return NodeValue{}
}
func (in *SubstrateInterpreter) ActPrimary(n *ast.PrimaryNode) ast.Value {
	in.preAct(n)
	defer func() { in.framePtr.actDepth-- }()

	var v NodeValue
	switch n.Typ {
	case ast.TypeID:
		if _, exist := in.framePtr.storageVar[n.Val]; exist {
			v.SetValue(ValueStorId, n.Val)
		} else if _, exist := in.framePtr.memoryVar[n.Val]; exist {
			v.SetValue(ValueMemId, n.Val)
		} else if _, exist := in.framePtr.localStruct[n.Val]; exist {
			v.SetValue(ValueStructId, n.Val)
		} else if _, exist := in.framePtr.actualArgs[n.Val]; exist && in.framePtr.callDepth == 1 {
			v.SetValue(ValueArgId, n.Val)
		} else {
			v.SetValue(ValueId, n.Val)
		}
	case ast.TypeNUM:
		u, _ := strconv.ParseUint(n.Val, 10, 64)
		v.SetValue(ValueNum, uint256.NewInt(u))
	case ast.TypeHEX:
		if len(n.Val) <= 64+4 {
			v.SetValue(ValueHex, n.Val)
		} else {
			decoded, err := hex.DecodeString(n.Val[2:])
			if err != nil {
				in.Panic("TypeHex: "+err.Error(), n, false)
			}
			v.SetValue(ValueBytes, decoded)
		}
	case ast.TypeMSGVAL:
		z := in.framePtr.contract.value
		v.SetValue(ValueNum, z)
	case ast.TypeMSGGAS:
		v.SetValue(ValueNum, uint256.NewInt(in.framePtr.contract.Gas))
	case ast.TypeMSGSENDER:
		v.SetValue(ValueAddress, in.framePtr.contract.CallerAddress)
	case ast.TypeMSGDATA:
		v.SetValue(ValueMsgData, in.framePtr.contract.Input)
	case ast.TypeTHIS:
		v.SetValue(ValueAddress, in.framePtr.contract.Address())
	case ast.TypeGOTOADDRESS:
		in.Panic("Cannot handle goto address ", n, false)
	case ast.TypeSTRING:
		in.Panic("String type not implemented", n, false)
	case ast.TypeMESSAGE:
		msg := n.Val[1 : len(n.Val)-1]
		v.SetValue(ValueString, msg)
	case ast.TypeBOOLLITERAL:
		b := uint256.NewInt(0)
		if n.Val == "True" {
			b.SetOne()
		}
		v.SetValue(ValueNum, b)
	default:
		printDebug(strconv.Itoa(int(n.Typ)), false)
		in.Panic("Not implemented primary type "+strconv.Itoa(int(n.Typ)), n, true)
	}
	return v
}
func (in *SubstrateInterpreter) ActToken(n *ast.TokenNode) ast.Value {
	in.preAct(n)
	defer func() { in.framePtr.actDepth-- }()

	return NodeValue{}
}
func (in *SubstrateInterpreter) ActNodeType(t ast.NodeType) ast.Value {
	return NodeValue{}
}
func (in *SubstrateInterpreter) ActBool(b bool) ast.Value {
	return NodeValue{}
}
