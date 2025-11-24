package vm

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"time"

	// "fmt"
	// "encoding/hex"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/research"
	"github.com/ethereum/go-ethereum/tac_parser"
	"github.com/holiman/uint256"
	"github.com/urfave/cli/v2"
)

type TACProgram = tac_parser.TACProgram
type TACFunction = tac_parser.TACFunction

var (
	ErrTACThrow       = errors.New("TAC THROW statement was executed")
	ErrTACNoTac       = errors.New("no TAC found in substratedir")
	ErrTACIllPhiExec  = errors.New("TAC ILLPHI statement was executed")
	ErrTACIllJump     = errors.New("illegal jump destination to nonexistent TAC block")
	ErrTACNoCallTrace = errors.New("no next call trace found")
	ErrTACNoGasTrace  = ErrDeltaNoGasFound
	ErrTACTimeout     = errors.New("TAC interpreter reached transaction timeout")
	ErrTACPanic       = errors.New("TAC interpreter raised unexpected runtime panic")
	ErrTACParseError  = errors.New("TAC parsing error")
)

// check if the error return should be treated as global error (TAC error)
// Those error should be passed directly back to testing client to be reported
// ErrTACThrow is not a TAC global error - it is raised only for a (nested) contract.
// Contract caller will set
func IsTACGlobalError(err error) bool {
	return err == ErrTACNoTac || err == ErrTACIllPhiExec ||
		err == ErrTACIllJump || err == ErrTACNoCallTrace || err == ErrTACNoGasTrace ||
		err == ErrTACTimeout || err == ErrTACPanic || err == ErrTACParseError
}

var (
	tacTimeoutValue = float64(0.5)
	TacTimeoutFlag  = &cli.Float64Flag{
		Name:  "tac-timeout",
		Usage: "TAC transaction timeout in seconds, non-positive value to disable",
		Value: tacTimeoutValue,
	}

	tacMaxInstCountValue = int64(0)
	TacMaxInstCountFlag  = &cli.Int64Flag{
		Name:  "tac-max-inst-count",
		Usage: "TAC transaction max instruction count, non-positive value to disable",
		Value: tacMaxInstCountValue,
	}

	// If --tac-gas-inst-count is true, TAC interpreter should use
	// the largest value between (1) --tac-max-inst-count value from user
	// and (2) tx gas usage recorded in the substate.
	tacGasInstCountValue = false
	TacGasInstCountFlag  = &cli.BoolFlag{
		Name:  "tac-gas-inst-count",
		Usage: "Use tx gas usage as lower bound of TAC transaction max instruction count",
		Value: tacGasInstCountValue,
	}
)

func SetTacFlags(ctx *cli.Context) {
	tacTimeoutValue = ctx.Float64(TacTimeoutFlag.Name)
	tacMaxInstCountValue = ctx.Int64(TacMaxInstCountFlag.Name)
	tacGasInstCountValue = ctx.Bool(TacGasInstCountFlag.Name)

	fmt.Printf("tac-interpreter: --tac-timeout=%v, --tac-max-inst-count=%v, --tac-gas-inst-count=%t\n", tacTimeoutValue, tacMaxInstCountValue, tacGasInstCountValue)
}

// TACInterpreter implements a vm.Interpreter interface
type TACInterpreter struct {
	evm        *EVM
	jumptable  *TACJumpTable
	hasher     crypto.KeccakState // Keccak256 hasher instance shared across opcodes
	hasherBuf  common.Hash        // Keccak256 hasher result array shared aross opcodes
	readOnly   bool               // Whether to throw on stateful modifications
	returnData []byte             // Last CALL's return data for subsequent reuse
	frames     []*TACFrame

	abort atomic.Int32

	Delta *DeltaSemantics
}

const (
	TAC_NO_ABORT       int32 = iota // No error - continue execution
	TAC_ABORT_TIMEOUT               // Intended termination - TAC interpreter timeout
	TAC_ABORT_OUTOFGAS              // Intended termination - out-of-gas error
)

// return current frame
func (in *TACInterpreter) GetFrame() *TACFrame {
	return in.frames[len(in.frames)-1]
}

// return contract under current frame
func (in *TACInterpreter) GetContract() *Contract {
	return in.GetFrame().contract
}

// return memory under current frame
func (in *TACInterpreter) GetMemory() *Memory {
	return in.GetFrame().memory
}

// return current running program
func (in *TACInterpreter) GetProg() *TACProgram {
	return &(in.GetFrame().prog)
}

func (in *TACInterpreter) CanRun([]byte) bool {
	//TODO: only runs code that can be analysized by Gigahorse
	return true
}

const TACResultHeader = "md5,addr,func,callTAC,callEVM,sstoreTAC,sstoreEVM,maxInst,tacInst"

func TACResultRow(code []byte, addr common.Address, input []byte, delta *DeltaSemantics) string {
	funcHex := "0x"
	if len(input) > 4 {
		funcHex += hex.EncodeToString(input[0:4])
	} else {
		funcHex += hex.EncodeToString(input)
	}

	vals := []string{
		getMD5(code),
		addr.Hex(),
		funcHex,
		fmt.Sprintf("%v", len(delta.SubstrateTrace.CallTraces)),
		fmt.Sprintf("%v", len(delta.RecordedTrace.CallTraces)),
		fmt.Sprintf("%v", delta.SubstrateTrace.SstoreCount),
		fmt.Sprintf("%v", delta.RecordedTrace.SstoreCount),
		fmt.Sprintf("%v", delta.MaxInstCount),
		fmt.Sprintf("%v", delta.SubstrateInstCount),
	}
	row := strings.Join(vals, ",")

	return row
}

// implements vm.Interpreter.Run
// However, instead of executing `contract.code`,
// we execute our own binary code translated from Gigahorse, which is `prog`.
// `prog` should be injected into the TACInterpreter at previous stage.
func (in *TACInterpreter) Run(contract *Contract, input []byte, readOnly bool) ([]byte, error) {
	in.Delta = in.evm.Delta

	var (
		newFrame *TACFrame
		err      error
	)
	in.evm.depth++
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
	if len(contract.Code) == 0 {
		return nil, nil
	}

	contract.Input = input

	contractCalled := len(in.Delta.SubstrateTrace.CallTraces)
	if contractCalled > len(in.Delta.RecordedTrace.CallTraces) {
		in.Delta.SubstrateErr = ErrTACNoCallTrace
		return nil, ErrTACNoCallTrace
	}

	newFrame = &TACFrame{
		contract:     contract,
		prog:         *contract.TACProg,
		memory:       NewMemory(),
		recCallTrace: in.Delta.RecordedTrace.CallTraces[contractCalled-1],
		subCallTrace: in.Delta.SubstrateTrace.CallTraces[contractCalled-1],
	}

	in.frames = append(in.frames, newFrame)

	defer func() {
		callTrace := newFrame.recCallTrace
		contract.UseGas(contract.Gas - callTrace.LeftOverGas)
		in.evm.StateDB.AddRefund(callTrace.RefundGas - in.evm.StateDB.GetRefund())
	}()

	var res []byte
	if (research.GetContractIso() || in.evm.depth <= 1) && tacTimeoutValue > 0 {
		doneChan := make(chan struct{}, 1)
		go func() {
			defer func() {
				// When this.execute_func paniced
				if r := recover(); r != nil {
					res = nil
					err = ErrTACPanic
					// When TraceTAC is set, print stacktrace
					if TraceTAC {
						fmt.Printf("\nTAC panic: %v\n\n%s\n", r, debug.Stack())
					}
				}
				doneChan <- struct{}{}
				close(doneChan)
			}()
			res, err = in.execute_func(in.GetProg().Entry, NewContext(in.GetProg().Entry, nil))
		}()

		select {
		case <-doneChan:
			break
		case <-time.After(time.Duration(tacTimeoutValue * float64(time.Second))):
			in.Delta.SubstrateErr = ErrTACTimeout
			in.abort.Store(TAC_ABORT_TIMEOUT)
			in.evm.Cancel()
			<-doneChan
			res, err = nil, ErrTACTimeout
		}
	} else {
		res, err = in.execute_func(in.GetProg().Entry, NewContext(in.GetProg().Entry, nil))
	}

	in.frames = in.frames[:len(in.frames)-1] //pop frame

	if IsTACGlobalError(err) {
		in.Delta.SubstrateErr = err
	}

	return res, err
}

func NewTACInterpreter(evm *EVM, vmConfig Config) *TACInterpreter {
	return &TACInterpreter{
		evm:       evm,
		jumptable: GetTACJumptable(),

		Delta: evm.Delta,
	}
}

func (in *TACInterpreter) execute_func(f *TACFunction, ctx *TACContext) ([]byte, error) {
	var (
		res []byte
		err error
	)
	// Execute a given function, until
	// 1. RETURNPRIVATE which set ctx.functionStop to true and return from the function (not the contract)
	// 2. STOP, RETURN, SELFDESTRUCT which halts the whole contract
	// 3. Error occurred
	for pc := 0; pc < len(f.Insts); {
		if in.checkOutOfGas() {
			res, err = nil, ErrOutOfGas
		}

		if in.Delta.MaxInstCount > 0 && in.Delta.SubstrateInstCount >= in.Delta.MaxInstCount {
			in.Delta.SubstrateErr = ErrTACTimeout
			in.abort.Store(TAC_ABORT_TIMEOUT)
			in.evm.Cancel()
			res, err = nil, ErrTACTimeout
		}

		switch a := in.abort.Load(); a {
		case TAC_NO_ABORT:
			if in.evm.abort.Load() {
				if in.Delta.SubstrateErr == nil {
					return nil, errStopToken
				}
				return nil, in.Delta.SubstrateErr
			}
		case TAC_ABORT_TIMEOUT: // Unintended termination: timeout
			return nil, ErrTACTimeout
		case TAC_ABORT_OUTOFGAS: // Intended termination: out-of-gas error
			return nil, ErrOutOfGas
		default:
			return nil, fmt.Errorf("unexpected abort value (%v)", a)
		}

		in.Delta.SubstrateInstCount++
		operation := in.jumptable[ctx.get_inst(pc)]
		if TraceTAC {
			tmp_pc := pc
			out := os.Stdout
			if operation.name != "TAC_BLOCK_FLAG" {
				fmt.Fprintf(out, "  ")
			}
			operation.dump(&tmp_pc, f, out)
		}

		// If the operation is valid, enforce and write restrictions
		if in.readOnly && in.evm.chainRules.IsByzantium {
			// If the interpreter is operating in readonly mode, make sure no
			// state-modifying operation is performed. The 3rd stack item
			// for a call operation is the value. Transferring value from one
			// account to the others means the state is modified and should also
			// return with an error.

			// TODO: handles readonly
			// if operation.writes || (op == CALL && stack.Back(2).Sign() != 0) {
			// 	return nil, ErrWriteProtection
			// }
		}
		res, err = operation.execution(&pc, in, ctx)
		ctx.increase_cycle()
		if operation.returns {
			in.returnData = common.CopyBytes(res)
		}

		frame := in.GetFrame()

		switch {
		case ctx.functionStop:
			return res, err
		case err != nil:
			return nil, err
		case operation.reverts:
			frame.reverts = true
		case operation.halts:
			frame.halts = true
		}

		switch {
		case frame.reverts:
			return res, ErrExecutionReverted
		case frame.halts:
			return res, nil
		}
	}
	return res, err
}

// THROW instruction in TAC corresponding to INVALID instruction in EVM
func exec_throw(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	//this is a gigahorse exception
	return nil, ErrTACThrow
}

// stop execution
func exec_stop(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	return nil, nil
}

// address of the executing contract
func exec_address(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	address := new(uint256.Int).SetBytes(in.GetContract().Address().Bytes())
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	dest.Set(ctx.cycle, address)
	*pc += 2
	return nil, nil
}

// transaction origin address
func exec_origin(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	address := new(uint256.Int).SetBytes(in.evm.Origin.Bytes())
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	dest.Set(ctx.cycle, address)
	*pc += 2
	return nil, nil
}

// message caller address
func exec_caller(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	address := new(uint256.Int).SetBytes(in.GetContract().Caller().Bytes())
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	dest.Set(ctx.cycle, address)
	*pc += 2
	return nil, nil
}

// message funds in wei
func exec_callvalue(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	value := in.GetContract().value.Clone()
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	dest.Set(ctx.cycle, value)
	*pc += 2
	return nil, nil
}

// message data length in bytes
func exec_calldatasize(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	size := new(uint256.Int).SetUint64(uint64(len(in.GetContract().Input)))
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	dest.Set(ctx.cycle, size)
	*pc += 2
	return nil, nil
}

// length of the executing contract's code in bytes
func exec_codesize(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	size := new(uint256.Int).SetUint64(uint64(len(in.GetContract().Code)))
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	dest.Set(ctx.cycle, size)
	*pc += 2
	return nil, nil
}

// gas price of the executing transaction, in wei per unit of gas
func exec_gasprice(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	price, _ := uint256.FromBig(in.evm.GasPrice)
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	dest.Set(ctx.cycle, price)
	*pc += 2
	return nil, nil
}

// the size of the returned data from the last external call, in bytes
func exec_returndatasize(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	dest.SetUint64(ctx.cycle, uint64(len(in.returnData)))
	*pc += 2
	return nil, nil
}

// address of the current block's miner
func exec_coinbase(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	address := new(uint256.Int).SetBytes(in.evm.Context.Coinbase.Bytes())
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	dest.Set(ctx.cycle, address)
	*pc += 2
	return nil, nil
}

// current block's Unix timestamp in seconds
func exec_timestamp(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	dest.SetUint64(ctx.cycle, in.evm.Context.Time)
	*pc += 2
	return nil, nil
}

// current block's number
func exec_number(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	dest.SetFromBig(ctx.cycle, in.evm.Context.BlockNumber)
	*pc += 2
	return nil, nil
}

// current block's difficulty
func exec_difficulty(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	dest.SetFromBig(ctx.cycle, in.evm.Context.Difficulty)
	*pc += 2
	return nil, nil
}

// current block's gas limit
func exec_gaslimit(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	dest.SetUint64(ctx.cycle, in.evm.Context.GasLimit)
	*pc += 2
	return nil, nil
}

// size of memory for this contract execution, in bytes
func exec_msize(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	dest.SetUint64(ctx.cycle, uint64(in.GetMemory().Len()))
	*pc += 2
	return nil, nil
}

// remaining gas
func exec_gas(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	// Read gas values from GasInstRet
	const UseGasTrace = true

	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	gas := in.GetContract().Gas

	frame := in.GetFrame()

	if UseGasTrace {
		var gerr error
		gas, gerr = GetGasRetInst(frame.recCallTrace, frame.subCallTrace)
		if gerr != nil {
			return nil, ErrTACNoGasTrace
		}
	}

	frame.subCallTrace.AddGasInstRet(gas)
	dest.SetUint64(ctx.cycle, gas)
	// fmt.Printf("Gas %d\n", gas)
	*pc += 2
	return nil, nil
}

func exec_chainid(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	dest.SetFromBig(ctx.cycle, in.evm.chainConfig.ChainID)
	*pc += 2
	return nil, nil
}

func exec_selfbalance(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	dest.Set(ctx.cycle, in.evm.StateDB.GetBalance(in.GetContract().Address()))
	*pc += 2
	return nil, nil
}

// check (u)int256 is zero
func exec_iszero(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	arg0 := ctx.get_var(ctx.get_inst(*pc + 2))
	if arg0.val.IsZero() {
		dest.SetOne(ctx.cycle)
	} else {
		dest.Clear(ctx.cycle)
	}
	*pc += 3
	return nil, nil
}

// address balance in wei
func exec_balance(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	address := common.Address(ctx.get_var(ctx.get_inst(*pc + 2)).val.Bytes20())
	dest.Set(ctx.cycle, in.evm.StateDB.GetBalance(address))
	*pc += 3
	return nil, nil
}

// reads a (u)int256 from message data
func exec_calldataload(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	arg0 := ctx.get_var(ctx.get_inst(*pc + 2))
	if offset, overflow := arg0.val.Uint64WithOverflow(); !overflow {
		data, merr := Ext_getData(in.GetContract().Input, offset, 32, in.Delta.MaxGas)
		if merr != nil {
			return nil, merr
		}
		dest.SetBytes(ctx.cycle, data)
	} else {
		dest.Clear(ctx.cycle)
	}
	*pc += 3
	return nil, nil
}

// length of the contract bytecode at addr, in bytes
func exec_extcodesize(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	address := common.Address(ctx.get_var(ctx.get_inst(*pc + 2)).val.Bytes20())
	size := uint64(in.evm.StateDB.GetCodeSize(address))
	dest.SetUint64(ctx.cycle, size)
	*pc += 3
	return nil, nil
}

// hash of the contract bytecode at addr, see EIP-1052
func exec_extcodehash(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	address := common.Address(ctx.get_var(ctx.get_inst(*pc + 2)).val.Bytes20())
	if in.evm.StateDB.Empty(address) {
		dest.Clear(ctx.cycle)
	} else {
		dest.SetBytes(ctx.cycle, in.evm.StateDB.GetCodeHash(address).Bytes())
	}
	*pc += 3
	return nil, nil
}

// hash of the specific block, only valid for the 256 most recent blocks, excluding the current one
func exec_blockhash(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	// blockhash was recorded and stroed in substateDB.Env.BlockHashes
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	arg0 := ctx.get_var(ctx.get_inst(*pc + 2))
	num64, overflow := arg0.val.Uint64WithOverflow()

	defer func() { *pc += 3 }()

	if overflow {
		dest.Clear(ctx.cycle)
		return nil, nil
	}

	var upper, lower uint64
	upper = in.evm.Context.BlockNumber.Uint64()

	if upper < 257 {
		lower = 0
	} else {
		lower = upper - 256
	}
	if num64 >= lower && num64 < upper {
		dest.SetBytes(ctx.cycle, in.evm.Context.GetHash(num64).Bytes())
	} else {
		dest.Clear(ctx.cycle)
	}
	return nil, nil
}

// reads a (u)int256 from memory
func exec_mload(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	arg0 := ctx.get_var(ctx.get_inst(*pc + 2))
	offset := int64(arg0.val.Uint64())
	data, merr := in.GetMemory().Ext_GetPtr(offset, 32, in.Delta.MaxGas)
	if merr != nil {
		return nil, merr
	}
	dest.SetBytes(ctx.cycle, data)
	*pc += 3
	return nil, nil
}

// reads a (u)int256 from storage
func exec_sload(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	arg0 := ctx.get_var(ctx.get_inst(*pc + 2))
	hash := common.Hash(arg0.val.Bytes32())
	val := in.evm.StateDB.GetState(in.GetContract().Address(), hash)
	dest.SetBytes(ctx.cycle, val.Bytes())
	*pc += 3

	if checkEMI && checkLoad {
		t := fmt.Sprintf("address %s, loc: %s, value: %s, depth: %d, pc: %v", in.GetContract().Address().Hex(), hash.String(), val.Hex(), in.evm.depth, *pc)
		fmt.Println("TAC SLOAD:", t)
	}

	return nil, nil
}

// unconditional jump
func exec_jump(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	if ctx.get_inst(*pc+1) == -1 {
		return nil, in.errInvalidJump()
	}
	*pc = ctx.get_inst(*pc + 1)
	return nil, nil
}

func exec_jump_var(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	target := ctx.get_var(ctx.get_inst(*pc + 1))

	function := ctx.Function()
	baseAddr := target.val.Hex()

	// First search from successors of the block
	succ := function.UniqueJumpVarSucc[*pc]
	addr := -1
	for _, name := range succ {
		if tac_parser.EqualBaseAddr(baseAddr, name) {
			*pc = function.BlockByName[name]
			return nil, nil
		}
	}

	// Search if the base address is unique within the function
	addr, ok := ctx.Function().UniqueBlockAddr[baseAddr]
	if ok {
		*pc = addr
		return nil, nil
	}

	return nil, in.errInvalidJump()
}

// destroys the contract and sends all funds to addr.
func exec_selfdestruct(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	addr := ctx.get_var(ctx.get_inst(*pc + 1))
	balance := in.evm.StateDB.GetBalance(in.GetContract().Address())
	in.evm.StateDB.AddBalance(common.Address(addr.val.Bytes20()), balance)
	in.evm.StateDB.SelfDestruct(in.GetContract().Address())
	return nil, nil
}

func exec_not(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	arg0 := ctx.get_var(ctx.get_inst(*pc + 2))
	dest.val.Not(&arg0.val)
	*pc += 3
	return nil, nil
}

func exec_add(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	arg0 := ctx.get_var(ctx.get_inst(*pc + 2))
	arg1 := ctx.get_var(ctx.get_inst(*pc + 3))
	dest.Add(ctx.cycle, &arg0.val, &arg1.val)
	*pc += 4
	return nil, nil
}

func exec_mul(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	arg0 := ctx.get_var(ctx.get_inst(*pc + 2))
	arg1 := ctx.get_var(ctx.get_inst(*pc + 3))
	dest.Mul(ctx.cycle, &arg0.val, &arg1.val)
	*pc += 4
	return nil, nil
}

func exec_sub(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	arg0 := ctx.get_var(ctx.get_inst(*pc + 2))
	arg1 := ctx.get_var(ctx.get_inst(*pc + 3))
	dest.Sub(ctx.cycle, &arg0.val, &arg1.val)
	*pc += 4
	return nil, nil
}

func exec_div(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	arg0 := ctx.get_var(ctx.get_inst(*pc + 2))
	arg1 := ctx.get_var(ctx.get_inst(*pc + 3))
	dest.Div(ctx.cycle, &arg0.val, &arg1.val)
	*pc += 4
	return nil, nil
}

func exec_sdiv(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	arg0 := ctx.get_var(ctx.get_inst(*pc + 2))
	arg1 := ctx.get_var(ctx.get_inst(*pc + 3))
	dest.SDiv(ctx.cycle, &arg0.val, &arg1.val)
	*pc += 4
	return nil, nil
}

func exec_mod(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	arg0 := ctx.get_var(ctx.get_inst(*pc + 2))
	arg1 := ctx.get_var(ctx.get_inst(*pc + 3))
	dest.Mod(ctx.cycle, &arg0.val, &arg1.val)
	*pc += 4
	return nil, nil
}

func exec_smod(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	arg0 := ctx.get_var(ctx.get_inst(*pc + 2))
	arg1 := ctx.get_var(ctx.get_inst(*pc + 3))
	dest.SMod(ctx.cycle, &arg0.val, &arg1.val)
	*pc += 4
	return nil, nil
}

func exec_addmod(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	arg0 := ctx.get_var(ctx.get_inst(*pc + 2))
	arg1 := ctx.get_var(ctx.get_inst(*pc + 3))
	arg2 := ctx.get_var(ctx.get_inst(*pc + 4))
	if arg2.val.IsZero() {
		// evm source code checks zero, no sure why
		dest.Clear(ctx.cycle)
	} else {
		dest.AddMod(ctx.cycle, &arg0.val, &arg1.val, &arg2.val)
	}

	*pc += 5
	return nil, nil
}

func exec_mulmod(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	arg0 := ctx.get_var(ctx.get_inst(*pc + 2))
	arg1 := ctx.get_var(ctx.get_inst(*pc + 3))
	arg2 := ctx.get_var(ctx.get_inst(*pc + 4))
	if arg2.val.IsZero() {
		// evm source code checks zero, no sure why
		dest.Clear(ctx.cycle)
	} else {
		dest.MulMod(ctx.cycle, &arg0.val, &arg1.val, &arg2.val)
	}

	*pc += 5
	return nil, nil
}

func exec_exp(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	base := ctx.get_var(ctx.get_inst(*pc + 2))
	exponent := ctx.get_var(ctx.get_inst(*pc + 3))
	dest.Exp(ctx.cycle, &base.val, &exponent.val)
	*pc += 4
	return nil, nil
}

func exec_signextend(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	arg0 := ctx.get_var(ctx.get_inst(*pc + 2))
	arg1 := ctx.get_var(ctx.get_inst(*pc + 3))
	dest.ExtendSign(ctx.cycle, &arg1.val, &arg0.val)
	*pc += 4
	return nil, nil
}

func exec_lt(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	arg0 := ctx.get_var(ctx.get_inst(*pc + 2))
	arg1 := ctx.get_var(ctx.get_inst(*pc + 3))

	if arg0.val.Lt(&arg1.val) {
		dest.SetOne(ctx.cycle)
	} else {
		dest.Clear(ctx.cycle)
	}
	*pc += 4
	return nil, nil
}

func exec_gt(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	arg0 := ctx.get_var(ctx.get_inst(*pc + 2))
	arg1 := ctx.get_var(ctx.get_inst(*pc + 3))

	if arg0.val.Gt(&arg1.val) {
		dest.SetOne(ctx.cycle)
	} else {
		dest.Clear(ctx.cycle)
	}
	*pc += 4
	return nil, nil
}
func exec_slt(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	arg0 := ctx.get_var(ctx.get_inst(*pc + 2))
	arg1 := ctx.get_var(ctx.get_inst(*pc + 3))

	if arg0.val.Slt(&arg1.val) {
		dest.SetOne(ctx.cycle)
	} else {
		dest.Clear(ctx.cycle)
	}
	*pc += 4
	return nil, nil
}

func exec_sgt(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	arg0 := ctx.get_var(ctx.get_inst(*pc + 2))
	arg1 := ctx.get_var(ctx.get_inst(*pc + 3))

	if arg0.val.Sgt(&arg1.val) {
		dest.SetOne(ctx.cycle)
	} else {
		dest.Clear(ctx.cycle)
	}
	*pc += 4
	return nil, nil
}

func exec_eq(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	arg0 := ctx.get_var(ctx.get_inst(*pc + 2))
	arg1 := ctx.get_var(ctx.get_inst(*pc + 3))

	if arg0.val.Eq(&arg1.val) {
		dest.SetOne(ctx.cycle)
	} else {
		dest.Clear(ctx.cycle)
	}
	*pc += 4
	return nil, nil
}

func exec_and(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	arg0 := ctx.get_var(ctx.get_inst(*pc + 2))
	arg1 := ctx.get_var(ctx.get_inst(*pc + 3))

	dest.And(ctx.cycle, &arg0.val, &arg1.val)
	*pc += 4
	return nil, nil
}

func exec_or(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	arg0 := ctx.get_var(ctx.get_inst(*pc + 2))
	arg1 := ctx.get_var(ctx.get_inst(*pc + 3))

	dest.Or(ctx.cycle, &arg0.val, &arg1.val)
	*pc += 4
	return nil, nil
}

func exec_xor(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	arg0 := ctx.get_var(ctx.get_inst(*pc + 2))
	arg1 := ctx.get_var(ctx.get_inst(*pc + 3))

	dest.Xor(ctx.cycle, &arg0.val, &arg1.val)
	*pc += 4
	return nil, nil
}

// ith byte of (u)int256 x, counting from most significant byte
func exec_byte(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	arg0 := ctx.get_var(ctx.get_inst(*pc + 2))
	arg1 := ctx.get_var(ctx.get_inst(*pc + 3))

	newVal := arg1.val
	newVal.Byte(&arg0.val)
	dest.Assign(ctx.cycle, newVal)
	*pc += 4
	return nil, nil
}

// 256-bit shift left
func exec_shl(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	shift := ctx.get_var(ctx.get_inst(*pc + 2))
	value := ctx.get_var(ctx.get_inst(*pc + 3))

	if shift.val.LtUint64(256) {
		dest.Lsh(ctx.cycle, &value.val, uint(shift.val.Uint64()))
	} else {
		dest.Clear(ctx.cycle)
	}
	*pc += 4
	return nil, nil
}

// 256-bit shift right
func exec_shr(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	shift := ctx.get_var(ctx.get_inst(*pc + 2))
	value := ctx.get_var(ctx.get_inst(*pc + 3))

	if shift.val.LtUint64(256) {
		dest.Rsh(ctx.cycle, &value.val, uint(shift.val.Uint64()))
	} else {
		dest.Clear(ctx.cycle)
	}
	*pc += 4
	return nil, nil
}

// int256 shift right
func exec_sar(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	shift := ctx.get_var(ctx.get_inst(*pc + 2))
	value := ctx.get_var(ctx.get_inst(*pc + 3))

	defer func() { *pc += 4 }()

	if shift.val.GtUint64(256) {
		if value.val.Sign() > 0 {
			dest.Clear(ctx.cycle)
			return nil, nil
		} else {
			dest.SetAllOne(ctx.cycle)
			return nil, nil
		}
	}

	dest.SRsh(ctx.cycle, &value.val, uint(shift.val.Uint64()))
	return nil, nil
}

// keccak256 encoding: hash = keccak256(memory[offset:offset+length])
func exec_sha3(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	dest := ctx.get_var(ctx.get_inst(*pc + 1))
	offset := ctx.get_var(ctx.get_inst(*pc + 2))
	length := ctx.get_var(ctx.get_inst(*pc + 3))
	data, merr := in.GetMemory().Ext_GetPtr(int64(offset.val.Uint64()), int64(length.val.Uint64()), in.Delta.MaxGas)
	if merr != nil {
		return nil, merr
	}

	if in.hasher == nil {
		in.hasher = crypto.NewKeccakState()
	} else {
		in.hasher.Reset()
	}

	in.hasher.Write(data)
	in.hasher.Read(in.hasherBuf[:])

	evm := in.evm
	if evm.Config.EnablePreimageRecording {
		evm.StateDB.AddPreimage(in.hasherBuf, data)
	}

	dest.SetBytes(ctx.cycle, in.hasherBuf[:])
	*pc += 4
	return nil, nil
}

// writes a (u)int256 to memory
func exec_mstore(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	offset := ctx.get_var(ctx.get_inst(*pc + 1))
	value := ctx.get_var(ctx.get_inst(*pc + 2))

	merr := in.GetMemory().Ext_Set32(offset.val.Uint64(), &value.val, in.Delta.MaxGas)
	if merr != nil {
		return nil, merr
	}

	*pc += 3
	return nil, nil
}

// writes a uint8 to memory
func exec_mstore8(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	offset := ctx.get_var(ctx.get_inst(*pc + 1))
	value := ctx.get_var(ctx.get_inst(*pc + 2))

	merr := in.GetMemory().Ext_Set1(offset.val.Uint64(), byte(value.val.Uint64()), in.Delta.MaxGas)
	if merr != nil {
		return nil, merr
	}
	*pc += 3
	return nil, nil
}

func exec_sstore(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	loc := ctx.get_var(ctx.get_inst(*pc + 1))
	val := ctx.get_var(ctx.get_inst(*pc + 2))
	in.evm.StateDB.SetState(in.GetContract().Address(),
		common.Hash(loc.val.Bytes32()), common.Hash(val.val.Bytes32()))
	*pc += 3

	if checkEMI {
		fmt.Printf("TAC: address %s, loc: %s, value: %s, depth: %d, pc: %v\n", in.GetContract().Address().Hex(), loc.val.String(), common.Hash(val.val.Bytes32()).Hex(), in.evm.depth, *pc)
	}
	t := fmt.Sprintf("address %s, pc: %v", in.GetContract().Address().Hex(), *pc)

	in.Delta.SubstrateTrace.AddSstoreTrace(t)
	in.Delta.SubstrateTrace.IncSstoreCount()

	return nil, nil
}

func exec_jumpi(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	target := ctx.get_inst(*pc + 1)
	cond := ctx.get_var(ctx.get_inst(*pc + 2))
	if !cond.val.IsZero() {
		if target == -1 {
			return nil, in.errInvalidJump()
		}
		*pc = target
	} else {
		*pc += 3
	}
	return nil, nil
}

func exec_jumpi_var(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	target := ctx.get_var(ctx.get_inst(*pc + 1))
	cond := ctx.get_var(ctx.get_inst(*pc + 2))

	if cond.val.IsZero() {
		*pc += 3
		return nil, nil
	}

	function := ctx.Function()
	baseAddr := target.val.Hex()

	// First search from successors of the block
	succ := function.UniqueJumpVarSucc[*pc]
	addr := -1
	for _, name := range succ {
		if tac_parser.EqualBaseAddr(baseAddr, name) {
			*pc = function.BlockByName[name]
			return nil, nil
		}
	}

	// Search if the base address is unique within the function
	addr, ok := ctx.Function().UniqueBlockAddr[baseAddr]
	if ok {
		*pc = addr
		return nil, nil
	}

	return nil, in.errInvalidJump()
}

// reverts with return data
func exec_revert(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	offset := ctx.get_var(ctx.get_inst(*pc + 1))
	value := ctx.get_var(ctx.get_inst(*pc + 2))
	ret, merr := in.GetMemory().Ext_GetPtr(int64(offset.val.Uint64()), int64(value.val.Uint64()), in.Delta.MaxGas)
	if merr != nil {
		return nil, merr
	}

	return ret, ErrExecutionReverted
}

func exec_return(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	offset := ctx.get_var(ctx.get_inst(*pc + 1))
	value := ctx.get_var(ctx.get_inst(*pc + 2))
	ret, merr := in.GetMemory().Ext_GetPtr(int64(offset.val.Uint64()), int64(value.val.Uint64()), in.Delta.MaxGas)
	if merr != nil {
		return nil, merr
	}

	return ret, nil
}

// copy message data
// memory[destOffset:destOffset+length] = msg.data[offset:offset+length]
func exec_calldatacopy(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	destOffset := ctx.get_var(ctx.get_inst(*pc + 1))
	srcOffset := ctx.get_var(ctx.get_inst(*pc + 2))
	length := ctx.get_var(ctx.get_inst(*pc + 3))

	srcOffset64, overflow := srcOffset.val.Uint64WithOverflow()
	if overflow {
		srcOffset64 = 0xffffffffffffffff
	}

	destOffset64 := destOffset.val.Uint64()
	length64 := length.val.Uint64()

	data, merr := Ext_getData(in.GetContract().Input, srcOffset64, length64, in.Delta.MaxGas)
	if merr != nil {
		return nil, merr
	}
	merr = in.GetMemory().Ext_Set(destOffset64, length64, data, in.Delta.MaxGas)
	if merr != nil {
		return nil, merr
	}

	*pc += 4
	return nil, nil
}

// copy executing contract's bytecode
// memory[destOffset:destOffset+length] = address(this).code[offset:offset+length]
func exec_codecopy(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	destOffset := ctx.get_var(ctx.get_inst(*pc + 1))
	srcOffset := ctx.get_var(ctx.get_inst(*pc + 2))
	length := ctx.get_var(ctx.get_inst(*pc + 3))

	srcOffset64, overflow := srcOffset.val.Uint64WithOverflow()
	if overflow {
		srcOffset64 = math.MaxUint64
	}

	destOffset64 := destOffset.val.Uint64()
	length64 := length.val.Uint64()

	codeCopy, merr := Ext_getData(in.GetContract().Code, srcOffset64, length64, in.Delta.MaxGas)
	if merr != nil {
		return nil, merr
	}
	merr = in.GetMemory().Ext_Set(destOffset64, length64, codeCopy, in.Delta.MaxGas)
	if merr != nil {
		return nil, merr
	}

	*pc += 4
	return nil, nil
}

// copy returned data
// memory[destOffset:destOffset+length] = RETURNDATA[offset:offset+length]
func exec_returndatacopy(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	destOffset := ctx.get_var(ctx.get_inst(*pc + 1))
	srcOffset := ctx.get_var(ctx.get_inst(*pc + 2))
	length := ctx.get_var(ctx.get_inst(*pc + 3))

	srcOffset64, overflow := srcOffset.val.Uint64WithOverflow()
	if overflow {
		return nil, ErrReturnDataOutOfBounds
	}

	end := new(uint256.Int).Add(&srcOffset.val, &length.val)
	end64, overflow := end.Uint64WithOverflow()

	if overflow || uint64(len(in.returnData)) < end64 {
		return nil, ErrReturnDataOutOfBounds
	}

	destOffset64 := destOffset.val.Uint64()
	length64 := length.val.Uint64()

	merr := in.GetMemory().Ext_Set(destOffset64, length64,
		in.returnData[srcOffset64:end64], in.Delta.MaxGas)
	if merr != nil {
		return nil, merr
	}

	*pc += 4
	return nil, nil
}

func exec_illphi(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	return nil, ErrTACIllPhiExec
}

func exec_phi_start(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	numOfPHI := ctx.get_inst(*pc + 1)
	if numOfPHI == 0 {
		*pc += 3
		return nil, nil
	}
	*pc += 2 // move to first PHI
	// phi should be executed in parallel, so we copy the whole value array (can be improved)
	copiedRegister := ctx.registers
	for i := 0; i < numOfPHI; i++ {
		numOfChoice := ctx.get_inst(*pc + 1)
		dest := ctx.get_var(ctx.get_inst(*pc + 2))
		*pc += 3 // move to first choice
		// choose the most recent updated one
		recentIdx := ctx.get_inst(*pc)
		recentVar := ctx.get_var(recentIdx)
		for j := 1; j < numOfChoice; j++ {
			choiceIdx := ctx.get_inst(*pc + j)
			choiceVar := ctx.get_var(choiceIdx)
			if choiceVar.cycle > recentVar.cycle {
				recentIdx = choiceIdx
				recentVar = choiceVar
			}
		}
		dest.Set(ctx.cycle, &copiedRegister[recentIdx].val)
		*pc += numOfChoice
	}
	*pc += 1 // skip PHI_END
	return nil, nil
}

func exec_block_flag(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	// This has no effect
	*pc += 1
	return nil, nil
}

// copy contract's bytecode
// memory[destOffset:destOffset+length] = address(addr).code[offset:offset+length]
func exec_extcodecopy(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	arg0 := ctx.get_var(ctx.get_inst(*pc + 1))
	address := common.Address(arg0.val.Bytes20())
	destOffset := ctx.get_var(ctx.get_inst(*pc + 2))
	srcOffset := ctx.get_var(ctx.get_inst(*pc + 3))
	length := ctx.get_var(ctx.get_inst(*pc + 4))

	srcOffset64, overflow := srcOffset.val.Uint64WithOverflow()
	if overflow {
		srcOffset64 = 0xffffffffffffffff
	}

	length64 := length.val.Uint64()
	destOffset64 := destOffset.val.Uint64()

	codeCopy, merr := Ext_getData(in.evm.StateDB.GetCode(address), srcOffset64, length64, in.Delta.MaxGas)
	if merr != nil {
		return nil, merr
	}
	merr = in.GetMemory().Ext_Set(destOffset64, length64, codeCopy, in.Delta.MaxGas)
	if merr != nil {
		return nil, merr
	}

	*pc += 5
	return nil, nil
}

func makeTACLog(size int) func(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	return func(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
		topics := make([]common.Hash, size)
		offset, length := ctx.get_var(ctx.get_inst(*pc+1)), ctx.get_var(ctx.get_inst(*pc+2))
		*pc += 3
		for i := 0; i < size; i++ {
			topics[i] = common.Hash(ctx.get_var(ctx.get_inst(*pc + i)).val.Bytes32())
		}
		d, merr := in.GetMemory().Ext_GetCopy(int64(offset.val.Uint64()), int64(length.val.Uint64()), in.Delta.MaxGas)
		if merr != nil {
			return nil, merr
		}

		if checkEMI {
			fmt.Print("TAC LOG:")
			for _, t := range topics {
				fmt.Print(t.String(), ",")
			}
			fmt.Println("\nLOG Data:", hex.EncodeToString(d))
		}

		in.Delta.SubstrateTrace.AddEventTrace(topics, d)

		in.evm.StateDB.AddLog(&types.Log{
			Address:     in.GetContract().Address(),
			Topics:      topics,
			Data:        d,
			BlockNumber: in.evm.Context.BlockNumber.Uint64(),
		})
		*pc += size
		return nil, nil
	}
}

// calls a method in another contract
// success, memory[retOffset:retOffset+retLength] =
// address(addr).call.gas(gas).value(value) (memory[argsOffset:argsOffset+argsLength])
func exec_call(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	success := ctx.get_var(ctx.get_inst(*pc + 1))

	// NOTE: EVM does not omit the first argument (gas).
	// It passes gas to gasCall function and update evm.callGasTemp before opCall.
	//gas := in.evm.callGasTemp

	gasVal, addr, valueVal, argsOffset, argsSize, retOffset, retSize :=
		ctx.get_var(ctx.get_inst(*pc+2)),
		ctx.get_var(ctx.get_inst(*pc+3)),
		ctx.get_var(ctx.get_inst(*pc+4)),
		ctx.get_var(ctx.get_inst(*pc+5)),
		ctx.get_var(ctx.get_inst(*pc+6)),
		ctx.get_var(ctx.get_inst(*pc+7)),
		ctx.get_var(ctx.get_inst(*pc+8))

	gas := gasVal.val.Uint64()
	toAddr := common.Address(addr.val.Bytes20())
	value := valueVal.val.Clone()
	args, merr := in.GetMemory().Ext_GetPtr(int64(argsOffset.val.Uint64()), int64(argsSize.val.Uint64()), in.Delta.MaxGas)
	if merr != nil {
		return nil, merr
	}

	if in.readOnly && !value.IsZero() {
		return nil, ErrWriteProtection
	}
	if !value.IsZero() {
		gas += params.CallStipend
	}

	if gas > in.Delta.MaxGas {
		return nil, ErrOutOfGas
	}

	ret, _, err := in.evm.Call(in.GetContract(), toAddr, args, gas, value)

	if err != nil {
		success.Clear(ctx.cycle)
	} else {
		success.SetOne(ctx.cycle)
	}
	if err == nil || err == ErrExecutionReverted {
		merr := in.GetMemory().Ext_Set(retOffset.val.Uint64(), retSize.val.Uint64(), ret, in.Delta.MaxGas)
		if merr != nil {
			return nil, merr
		}
	}

	in.returnData = ret

	if _, ok := in.evm.interpreter.(*EVMInterpreter); ok {
		if len(in.evm.StateDB.GetCode(toAddr)) != 0 {
			in.Delta.UsedEVM = true
		}
	}
	if _, ok := in.evm.interpreter.(*TACInterpreter); ok {
		if IsTACGlobalError(err) {
			return nil, err
		}
	}

	*pc += 9
	return ret, nil
}

// Callcode differs from call in the sense it executes the given address with the
// caller as context
func exec_callcode(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	success := ctx.get_var(ctx.get_inst(*pc + 1))

	// NOTE: EVM does not omit the first argument (gas).
	// It passes gas to gasCall function and update evm.callGasTemp before opCall.
	//gas := in.evm.callGasTemp

	gasVal, addr, valueVal, argsOffset, argsSize, retOffset, retSize :=
		ctx.get_var(ctx.get_inst(*pc+2)),
		ctx.get_var(ctx.get_inst(*pc+3)),
		ctx.get_var(ctx.get_inst(*pc+4)),
		ctx.get_var(ctx.get_inst(*pc+5)),
		ctx.get_var(ctx.get_inst(*pc+6)),
		ctx.get_var(ctx.get_inst(*pc+7)),
		ctx.get_var(ctx.get_inst(*pc+8))

	gas := gasVal.val.Uint64()
	toAddr := common.Address(addr.val.Bytes20())
	value := valueVal.val.Clone()
	args, merr := in.GetMemory().Ext_GetPtr(int64(argsOffset.val.Uint64()), int64(argsSize.val.Uint64()), in.Delta.MaxGas)
	if merr != nil {
		return nil, merr
	}

	if !value.IsZero() {
		gas += params.CallStipend
	}

	if gas > in.Delta.MaxGas {
		return nil, ErrOutOfGas
	}

	ret, _, err := in.evm.CallCode(in.GetContract(), toAddr, args, gas, value)

	if err != nil {
		success.Clear(ctx.cycle)
	} else {
		success.SetOne(ctx.cycle)
	}
	if err == nil || err == ErrExecutionReverted {
		merr := in.GetMemory().Ext_Set(retOffset.val.Uint64(), retSize.val.Uint64(), ret, in.Delta.MaxGas)
		if merr != nil {
			return nil, merr
		}
	}

	in.returnData = ret

	if _, ok := in.evm.interpreter.(*EVMInterpreter); ok {
		if len(in.evm.StateDB.GetCode(toAddr)) != 0 {
			in.Delta.UsedEVM = true
		}
	}
	if _, ok := in.evm.interpreter.(*TACInterpreter); ok {
		if IsTACGlobalError(err) {
			return nil, err
		}
	}

	*pc += 9
	return ret, nil
}

// calls a method in another contract, using the storage of the current contract
// success, memory[retOffset:retOffset+retLength] =
// address(addr).delegatecall.gas(gas) (memory[argsOffset:argsOffset+argsLength])
func exec_delegatecall(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	success := ctx.get_var(ctx.get_inst(*pc + 1))

	// NOTE: EVM does not omit the first argument (gas).
	// It passes gas to gasCall function and update evm.callGasTemp before opCall.
	//gas := in.evm.callGasTemp

	gasVal, addr, argsOffset, argsSize, retOffset, retSize :=
		ctx.get_var(ctx.get_inst(*pc+2)),
		ctx.get_var(ctx.get_inst(*pc+3)),
		ctx.get_var(ctx.get_inst(*pc+4)),
		ctx.get_var(ctx.get_inst(*pc+5)),
		ctx.get_var(ctx.get_inst(*pc+6)),
		ctx.get_var(ctx.get_inst(*pc+7))

	gas := gasVal.val.Uint64()
	toAddr := common.Address(addr.val.Bytes20())
	args, merr := in.GetMemory().Ext_GetPtr(int64(argsOffset.val.Uint64()), int64(argsSize.val.Uint64()), in.Delta.MaxGas)
	if merr != nil {
		return nil, merr
	}

	if gas > in.Delta.MaxGas {
		return nil, ErrOutOfGas
	}

	ret, _, err := in.evm.DelegateCall(in.GetContract(), toAddr, args, gas)

	if err != nil {
		success.Clear(ctx.cycle)
	} else {
		success.SetOne(ctx.cycle)
	}
	if err == nil || err == ErrExecutionReverted {
		merr := in.GetMemory().Ext_Set(retOffset.val.Uint64(), retSize.val.Uint64(), ret, in.Delta.MaxGas)
		if merr != nil {
			return nil, merr
		}
	}

	in.returnData = ret

	if _, ok := in.evm.interpreter.(*EVMInterpreter); ok {
		if len(in.evm.StateDB.GetCode(toAddr)) != 0 {
			in.Delta.UsedEVM = true
		}
	}
	if _, ok := in.evm.interpreter.(*TACInterpreter); ok {
		if IsTACGlobalError(err) {
			return nil, err
		}
	}

	*pc += 8
	return ret, nil
}

func exec_staticcall(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	success := ctx.get_var(ctx.get_inst(*pc + 1))

	// NOTE: EVM does not omit the first argument (gas).
	// It passes gas to gasCall function and update evm.callGasTemp before opCall.
	//gas := in.evm.callGasTemp

	gasVal, addr, argsOffset, argsSize, retOffset, retSize :=
		ctx.get_var(ctx.get_inst(*pc+2)),
		ctx.get_var(ctx.get_inst(*pc+3)),
		ctx.get_var(ctx.get_inst(*pc+4)),
		ctx.get_var(ctx.get_inst(*pc+5)),
		ctx.get_var(ctx.get_inst(*pc+6)),
		ctx.get_var(ctx.get_inst(*pc+7))

	gas := gasVal.val.Uint64()
	toAddr := common.Address(addr.val.Bytes20())
	args, merr := in.GetMemory().Ext_GetPtr(int64(argsOffset.val.Uint64()), int64(argsSize.val.Uint64()), in.Delta.MaxGas)
	if merr != nil {
		return nil, merr
	}

	if gas > in.Delta.MaxGas {
		return nil, ErrOutOfGas
	}

	ret, _, err := in.evm.StaticCall(in.GetContract(), toAddr, args, gas)

	if err != nil {
		success.Clear(ctx.cycle)
	} else {
		success.SetOne(ctx.cycle)
	}
	if err == nil || err == ErrExecutionReverted {
		merr := in.GetMemory().Ext_Set(retOffset.val.Uint64(), retSize.val.Uint64(), ret, in.Delta.MaxGas)
		if merr != nil {
			return nil, merr
		}
	}

	in.returnData = ret

	if _, ok := in.evm.interpreter.(*EVMInterpreter); ok {
		if len(in.evm.StateDB.GetCode(toAddr)) != 0 {
			in.Delta.UsedEVM = true
		}
	}
	if _, ok := in.evm.interpreter.(*TACInterpreter); ok {
		if IsTACGlobalError(err) {
			return nil, err
		}
	}

	*pc += 8
	return ret, nil
}

// creates a child contract
// dest = new memory[offset:offset+length].value(value)
func exec_create(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	dest, value, offset, length :=
		ctx.get_var(ctx.get_inst(*pc+1)),
		ctx.get_var(ctx.get_inst(*pc+2)),
		ctx.get_var(ctx.get_inst(*pc+3)),
		ctx.get_var(ctx.get_inst(*pc+4))

	*pc += 5

	gas := in.GetContract().Gas
	input, merr := in.GetMemory().Ext_GetCopy(int64(offset.val.Uint64()), int64(length.val.Uint64()), in.Delta.MaxGas)
	if merr != nil {
		return nil, merr
	}

	if in.evm.chainRules.IsEIP150 {
		gas -= gas / 64
	}

	// no need to do gas modification in gvm
	// in.GetContract().UseGas(gas)

	var (
		res       []byte
		addr      common.Address
		returnGas uint64
		err       error
		suberr    error
		bigVal    = uint256.NewInt(0)
	)
	if !value.val.IsZero() {
		bigVal = value.val.Clone()
	}

	if gas > in.Delta.MaxGas {
		return nil, ErrOutOfGas
	}

	res, addr, returnGas, suberr = in.evm.Create(in.GetContract(), input, gas, bigVal)

	if _, ok := in.evm.interpreter.(*EVMInterpreter); ok {
		in.Delta.UsedEVM = true
	}
	if _, ok := in.evm.interpreter.(*TACInterpreter); ok {
		if IsTACGlobalError(err) {
			return nil, err
		}
	}

	suberrStr := ""
	if suberr != nil {
		suberrStr = suberr.Error()
	}

	// Modify return value based on the returned error. If the ruleset is
	// homestead we must check for CodeStoreOutOfGasError (homestead only
	// rule) and treat as an error, if the ruleset is frontier we must
	// ignore this error and pretend the operation was successful.
	if in.evm.chainRules.IsHomestead && suberrStr == ErrCodeStoreOutOfGas.Error() {
		dest.Clear(ctx.cycle)
	} else if suberr != nil && suberrStr != ErrCodeStoreOutOfGas.Error() {
		dest.Clear(ctx.cycle)
	} else {
		dest.SetBytes(ctx.cycle, addr.Bytes())
	}

	in.GetContract().Gas += returnGas

	if suberrStr == ErrExecutionReverted.Error() {
		return res, nil
	}

	return nil, nil
}

// creates a child contract with a deterministic address, see EIP-1014
// addr = new memory[offset:offset+length].value(value)
func exec_create2(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	dest, value, offset, length, salt :=
		ctx.get_var(ctx.get_inst(*pc+1)),
		ctx.get_var(ctx.get_inst(*pc+2)),
		ctx.get_var(ctx.get_inst(*pc+3)),
		ctx.get_var(ctx.get_inst(*pc+4)),
		ctx.get_var(ctx.get_inst(*pc+5))

	*pc += 6

	gas := in.GetContract().Gas
	input, merr := in.GetMemory().Ext_GetCopy(int64(offset.val.Uint64()), int64(length.val.Uint64()), in.Delta.MaxGas)
	if merr != nil {
		return nil, merr
	}

	if in.evm.chainRules.IsEIP150 {
		gas -= gas / 64
	}

	// no need to do gas modification in gvm
	// in.GetContract().UseGas(gas)

	var (
		res       []byte
		addr      common.Address
		returnGas uint64
		err       error
		suberr    error
		bigVal    = uint256.NewInt(0)
	)
	if !value.val.IsZero() {
		bigVal = value.val.Clone()
	}

	if gas > in.Delta.MaxGas {
		return nil, ErrOutOfGas
	}

	res, addr, returnGas, suberr = in.evm.Create2(in.GetContract(), input, gas, bigVal, &salt.val)

	if _, ok := in.evm.interpreter.(*EVMInterpreter); ok {
		in.Delta.UsedEVM = true
	}
	if _, ok := in.evm.interpreter.(*TACInterpreter); ok {
		if IsTACGlobalError(err) {
			return nil, err
		}
	}

	suberrStr := ""
	if suberr != nil {
		suberrStr = suberr.Error()
	}

	if suberr != nil {
		dest.Clear(ctx.cycle)
	} else {
		dest.SetBytes(ctx.cycle, addr.Bytes())
	}

	in.GetContract().Gas += returnGas

	if suberrStr == ErrExecutionReverted.Error() {
		return res, nil
	}

	return nil, nil
}

func exec_callprivate(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	numDest := ctx.get_inst(*pc + 1)
	*pc += 2 // Move to first dest
	var destIds []int
	for i := 0; i < numDest; i++ {
		destIds = append(destIds, ctx.get_inst(*pc+i))
	}
	*pc += numDest // move to target func
	targetFunctionId := ctx.get_inst(*pc)
	targetFunction := in.GetProg().Functions[targetFunctionId]
	*pc += 1 // Move to first arg
	var args []uint256.Int
	for i := range targetFunction.Args {
		argId := ctx.get_inst(*pc + i)
		args = append(args, (ctx.get_var(argId).val))
	}
	var newCtxt *TACContext
	newCtxt = NewContext(&targetFunction, &args)
	newCtxt.PrevContext = ctx
	*pc += len(targetFunction.Args)
	// prepare space to receive return data
	ctx.PrivateReturnData = make([]uint256.Int, numDest)
	res, err := in.execute_func(&targetFunction, newCtxt)
	for idx, dest := range destIds {
		ctx.get_var(dest).Assign(ctx.cycle, (ctx.PrivateReturnData)[idx])
	}
	return res, err
}

func exec_returnprivate(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	numRet := ctx.get_inst(*pc + 1)
	*pc += 2 // Move to first ret variable
	for i := 0; i < numRet; i++ {
		ctx.PrevContext.PrivateReturnData[i] = ctx.get_var(ctx.get_inst(*pc + i)).val
	}
	*pc += numRet
	ctx.functionStop = true
	return nil, nil
}

func exec_const(pc *int, in *TACInterpreter, ctx *TACContext) ([]byte, error) {
	// Update cycle for PHI selection
	c := ctx.get_var(ctx.get_inst(*pc + 1))
	c.UpdateCycle(ctx.cycle)
	*pc += 2
	return nil, nil
}

func (in *TACInterpreter) checkOutOfGas() bool {
	preAlloc := in.evm.StateDB.(*state.StateDB).ResearchPreAlloc
	in.Delta.AllocCount = len(preAlloc)
	// exclude coinbase from AllocCount
	if _, exist := preAlloc[in.evm.Context.Coinbase]; exist {
		in.Delta.AllocCount--
	}

	if in.Delta.CheckOutOfGas() {
		in.abort.Store(TAC_ABORT_OUTOFGAS)
		return true
	}
	return false
}

// errInvalidJump returns ErrInvalidJump if the transaction raised
// ErrInvalidJump, otherwise returns TACIllJump.
func (in *TACInterpreter) errInvalidJump() error {
	err := in.GetFrame().recCallTrace.Err
	if err != nil && err.Error() == ErrInvalidJump.Error() {
		return ErrInvalidJump
	}
	return ErrTACIllJump
}
