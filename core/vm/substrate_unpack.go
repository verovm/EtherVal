package vm

import (
	"fmt"
	"math/big"
	"reflect"
	"regexp"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
)

func preprocessType(str string) string {
	Bool := regexp.MustCompile(`^bool$`)
	Uint := regexp.MustCompile(`^uint\d+$`)
	UintSlice := regexp.MustCompile(`^uint\d+\[\]$`)
	Int := regexp.MustCompile(`^int\d+$`)
	IntSlice := regexp.MustCompile(`^int\d+\[\]$`)
	if Bool.MatchString(str) || Uint.MatchString(str) || Int.MatchString(str) {
		str = "uint256"
	} else if UintSlice.MatchString(str) || IntSlice.MatchString(str) {
		str = "uint256[]"
	}
	return str
}
func splitTopTuple(typeStr string) []string {
	var result []string
	depth, last := 0, 0

	for i, ch := range typeStr {
		switch ch {
		case '(':
			depth++
		case ')':
			depth--
		}
		if ch == ',' && depth == 0 {
			result = append(result, typeStr[last:i])
			last = i + 1
		}
	}
	result = append(result, typeStr[last:])
	return result
}
func parseTuple(tupleStr string) []abi.ArgumentMarshaling {
	elements := splitTopTuple(tupleStr)
	result := make([]abi.ArgumentMarshaling, 0, len(elements))

	for _, elem := range elements {
		elem = strings.TrimSpace(elem)
		if strings.HasPrefix(elem, "(") {
			innerTuple := parseTuple(elem[1 : len(elem)-1])
			result = append(result, abi.ArgumentMarshaling{Type: "tuple", Components: innerTuple})
		} else {
			result = append(result, abi.ArgumentMarshaling{Type: elem})
		}
	}
	return result
}

func (in *SubstrateInterpreter) unpackInput(types []string, argIds []NodeValue) {
	var args abi.Arguments
	for _, typ := range types {
		var t abi.Type
		if strings.HasPrefix(typ, "(") {
			if !strings.HasSuffix(typ, ")") {
				in.Panic("malformed tuple type: missing closing )", in.currNode, false)
			}
			tupleArgs := parseTuple(typ[1 : len(typ)-1])
			t, _ = abi.NewType("tuple", "", tupleArgs)
		} else {
			typ = preprocessType(typ)
			t, _ = abi.NewType(typ, "", nil)
		}
		args = append(args, abi.Argument{Type: t})
	}

	var input []byte
	if in.framePtr.isSelector {
		input = in.framePtr.contract.Input
	} else {
		input = in.framePtr.contract.Input[4:]
	}
	unpacked, err := args.UnpackValues(input)
	if err != nil {
		in.Panic("unpack error: "+err.Error(), in.currNode, false)
	}

	if !in.framePtr.isSelector && len(types) != len(unpacked) {
		in.Panic(fmt.Sprintf("Number of formal args does not match: %d %d", len(types), len(unpacked)), in.currNode, false)
	}

	for i := range types {
		printDebug(fmt.Sprintf("%T: ", unpacked[i])+fmt.Sprint(unpacked[i]), false)
		argId := argIds[i]
		in.framePtr.lVarType[argId.GetId()] = ValueNum
		switch unpacked[i].(type) {
		case common.Address:
			parsed := uint256.NewInt(0).SetBytes(unpacked[i].(common.Address).Bytes())
			in.setUint256(argId, parsed)
			in.framePtr.lVarType[argId.GetId()] = ValueAddress

		case *big.Int:
			parsed, _ := uint256.FromBig(unpacked[i].(*big.Int))
			in.setUint256(argId, parsed)
		case uint8, uint16, uint32, uint64:
			typeName := fmt.Sprintf("%T", unpacked[i])
			parsed := uint256.NewInt(uintHandler[typeName](unpacked[i]))
			in.setUint256(argId, parsed)
		case int8, int16, int32, int64:
			typeName := fmt.Sprintf("%T", unpacked[i])
			parsed := uint256.NewInt(intHandler[typeName](unpacked[i]))
			in.setUint256(argId, parsed)

		case string:
			b, l, _ := args.UnpackPosition(in.framePtr.contract.Input[4:], i)
			ref := inputArgRef{id: argId.GetId(), typ: "string", ptr: uint64(b) + 4, length: uint64(l)}
			in.setArgumentRef(ref.id, &ref)
			in.framePtr.lVarType[argId.GetId()] = ValueString

		case bool:
			if unpacked[i].(bool) {
				in.setUint256(argId, uint256.NewInt(1))
			} else {
				in.setUint256(argId, uint256.NewInt(0))
			}

		default:
			val := reflect.ValueOf(unpacked[i])
			if val.Kind() == reflect.Slice {
				typ := sliceToTypeStr[val.Type()]
				b, l, _ := args.UnpackPosition(in.framePtr.contract.Input[4:], i)
				ref := inputArgRef{id: argId.GetId(), typ: typ, ptr: uint64(b) + 4, length: uint64(l)}
				in.setArgumentRef(ref.id, &ref)
				in.framePtr.lVarType[argId.GetId()] = ValueBytes
			} else if val.Kind() == reflect.Array {
				parsed := arrayToUint256(val, types[i])
				if parsed != nil {
					in.setUint256(argId, parsed)
					in.framePtr.lVarType[argId.GetId()] = ValueBytes32
				} else {
					//case such as uint256[2]. This is not correctly decompiled in substrate yet.
					typ := val.Type().Elem().String()
					b, _, _ := args.UnpackPosition(in.framePtr.contract.Input[4:], i)
					l := val.Len() * 32
					b = b - (l - 32)
					ref := inputArgRef{id: argId.GetId(), typ: typ, ptr: uint64(b) + 4, length: uint64(l)}
					in.setArgumentRef(ref.id, &ref)
					in.framePtr.lVarType[argId.GetId()] = ValueLocalStruct
					in.Panic("Fixed array in unpack", in.currNode, false)
				}
			} else {
				in.Panic("Unknown type for unpacked args"+fmt.Sprintf(" %T", unpacked[i]), in.currNode, false)
			}

		}
	}
}

var uintHandler = map[string]func(interface{}) uint64{
	"uint8":  func(v interface{}) uint64 { return uint64(v.(uint8)) },
	"uint16": func(v interface{}) uint64 { return uint64(v.(uint16)) },
	"uint32": func(v interface{}) uint64 { return uint64(v.(uint32)) },
	"uint64": func(v interface{}) uint64 { return v.(uint64) },
}
var intHandler = map[string]func(interface{}) uint64{
	"int8":  func(v interface{}) uint64 { return uint64(v.(int8)) },
	"int16": func(v interface{}) uint64 { return uint64(v.(int16)) },
	"int32": func(v interface{}) uint64 { return uint64(v.(int32)) },
	"int64": func(v interface{}) uint64 { return uint64(v.(int64)) },
}
var sliceToTypeStr = map[reflect.Type]string{
	reflect.TypeOf([]bool{}):           "bool[]",
	reflect.TypeOf([]uint8{}):          "bytes",
	reflect.TypeOf([32]uint8{}):        "bytes32[]",
	reflect.TypeOf([]common.Address{}): "address[]",
	reflect.TypeOf([]*big.Int{}):       "uint256[]",
	reflect.TypeOf([]string{}):         "string[]",
	//TODO: add more types?
}

func arrayToUint256(uarr reflect.Value, typ string) *uint256.Int {
	parsed := uint256.NewInt(0)
	elemType := uarr.Type().Elem()
	if elemType == reflect.TypeOf(uint8(0)) {
		length := uarr.Len()
		tmp := make([]byte, length)
		for i := 0; i < length; i++ {
			tmp[i] = uint8(uarr.Index(i).Uint())
		}
		if strings.Contains(typ, "bytes") {
			tmp = append(tmp, make([]byte, 32-length)...)
		}
		parsed.SetBytes(tmp)
		return parsed
	}
	return nil
}
