package vm

import (
	"github.com/holiman/uint256"
	"github.com/ethereum/go-ethereum/substrate_parser/ast"
)

const (
	EMPTY ValueType = iota
	ValueTrue
	ValueNum
	ValueBytes32
	ValueString
	ValueBytes
	ValueId
	ValueHex
	ValueNumList
	ValueExprList
	ValueArgument
	ValueMem8
	ValueMem
	ValueStorage
	ValueAlloc
	ValueContinue
	ValueBreak
	ValueReturn
	ValueMsgVal
	ValueMsgData
	ValueMsgGas
	ValueMsgSender
	ValueAddress
	ValueNewMem
	ValueNewStruct
	ValueArgId
	ValueMemId
	ValueStorId
	ValueMemRef
	ValueStorRef
	ValueAccess
	ValueRevert
	ValueInvalid
	ValueTerminate
	ValueBasicType
	ValueMapType
	ValueStructArgType
	ValueArrayType
	ValueLibType
	ValueSpecifier
	ValueCallReturn
	ValueStruct
	ValueCastExpr
	ValueMemBytes
	ValueLocalStruct
	ValueStructId
)

var mapping = map[ValueType]string{
	EMPTY:            "EMPTY",
	ValueTrue:        "ValueTrue",
	ValueNum:         "ValueNum",
	ValueBytes32:     "ValueBytes32",
	ValueString:      "ValueString",
	ValueBytes:       "ValueBytes",
	ValueId:          "ValueId",
	ValueHex:         "ValueHex",
	ValueNumList:     "ValueNumList",
	ValueExprList:    "ValueExprList",
	ValueArgument:    "ValueArgument",
	ValueMem8:        "ValueMem8",
	ValueMem:         "ValueMem",
	ValueStorage:     "ValueStorage",
	ValueAlloc:       "ValueAlloc",
	ValueContinue:    "ValueContinue",
	ValueBreak:       "ValueBreak",
	ValueReturn:      "ValueReturn",
	ValueMsgVal:      "ValueMsgVal",
	ValueMsgData:     "ValueMsgData",
	ValueMsgGas:      "ValueMsgGas",
	ValueMsgSender:   "ValueMsgSender",
	ValueNewMem:      "ValueNewMem",
	ValueMemRef:      "ValueMemRef",
	ValueStorRef:     "ValueStorRef",
	ValueArgId:       "ValueArgId",
	ValueMemId:       "ValueMemId",
	ValueStorId:      "ValueStorId",
	ValueAccess:      "ValueAccess",
	ValueAddress:     "ValueAddress",
	ValueRevert:      "ValueRevert",
	ValueInvalid:     "ValueInvalid",
	ValueTerminate:   "ValueTerminate",
	ValueBasicType:   "ValueBasicType",
	ValueMapType:     "ValueMapType",
	ValueArrayType:   "ValueArrayType",
	ValueLibType:     "ValueLibType",
	ValueSpecifier:   "ValueSpecifier",
	ValueCallReturn:  "ValueCallReturn",
	ValueStruct:      "ValueStruct",
	ValueCastExpr:    "ValueCastExpr",
	ValueMemBytes:    "ValueMemBytes",
	ValueLocalStruct: "ValueLocalStruct",
	ValueStructId:    "ValueStructId",
}

var intrinsics = []string{
	"__DEBUG",
	"assert",
	"revert",
	"Panic",
	"CALLDATACOPY",
	"RETURNDATASIZE",
	"EXTCODEHASH",
	"keccak256",
	"sha256hash",
	"ecrecover",
	"MSIZE",
	"Error",
	"byte",
	"selfdestruct",
	"RETURNDATACOPY",
}

func isIntrinsic(id string) bool {
	for _, str := range intrinsics {
		if id == str {
			return true
		}
	}
	return false
}

type NodeValue struct {
	ast.Value
	kind  ValueType
	value interface{}
}

func (v *NodeValue) True() bool {
	return v.kind == ValueTrue
}
func (v *NodeValue) IsEmpty() bool {
	return v.kind == EMPTY
}
func (v *NodeValue) Type() ValueType {
	return v.kind
}
func (v *NodeValue) SetValue(k ValueType, val interface{}) {
	v.kind = k
	v.value = val
}
func (v *NodeValue) GetSig() (string, ValueType) {
	//TODO: need both GetSig() and GetId()?
	if v.kind != ValueId && v.kind != ValueHex {
		panic("Not ID nor HEX")
		//return "", EMPTY
	}
	return v.value.(string), v.kind
}
func (v *NodeValue) GetId() string {
	if v.kind != ValueId && v.kind != ValueArgId && v.kind != ValueMemId && v.kind != ValueStructId && v.kind != ValueStorId {
		panic("Not ID " + mapping[v.kind])
		//return ""
	}
	return v.value.(string)
}
func (v *NodeValue) Uint64() uint64 {
	if v.kind != ValueNum {
		println("Not uint256  ", v.kind, v.value.(string))
		return uint256.NewInt(0).Uint64()
	}
	return v.value.(*uint256.Int).Uint64()
}
func (v *NodeValue) Uint256() *uint256.Int {
	if v.kind != ValueNum && v.kind != ValueBytes32 {
		println("Not uint256  ", v.kind, v.value.(string))
		return uint256.NewInt(0)
	}
	return v.value.(*uint256.Int)
}

func (v *NodeValue) String(in *SubstrateInterpreter) string {
	str := mapping[v.Type()] + "| "
	switch v.Type() {
	case EMPTY:
		str = "Empty value"
	case ValueNum:
		str = v.value.(*uint256.Int).String()
	case ValueId:
		str = v.value.(string) + ": " + in.getUint256(*v).String()
	case ValueExprList:
		list := v.value.([]NodeValue)
		for _, n := range list {
			str += n.value.(string) + ": " + in.getUint256(n).String() + " "
		}
	default:
		val := v.value.(NodeValue)
		str += mapping[val.Type()]
	}
	return str
}

func printDebug(msg string, f bool) {
	if f {
		println(msg)
	}
}
func unparser(n interface{}) string {
	p := &ast.Unparser{}
	ast.Act(p, n.(ast.AstNode))
	str := p.GetUnparsed()
	return str[0:]
}
