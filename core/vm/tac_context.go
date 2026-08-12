package vm

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/research"
	"github.com/ethereum/go-ethereum/tac_parser"
	"github.com/holiman/uint256"
)

// Contract frame (per ContractCall)
type TACFrame struct {
	prog     TACProgram
	memory   *Memory // global program memory
	contract *Contract

	reverts bool
	halts   bool

	recCallTrace *research.EVMCallTrace
	subCallTrace *research.EVMCallTrace
}

type TACRegister struct {
	val   uint256.Int
	cycle uint64
}

// runtime execution context (per function call)
type TACContext struct {
	registers  []TACRegister
	cycle      uint64
	block_flag int

	functionStop      bool          // if the current function should stop execution
	function          *TACFunction  // pointer to the corrsponding function
	PrivateReturnData []uint256.Int // return value from RETURNPRIVATE
	PrevContext       *TACContext   // when RETURNPRIVATE, write return data to prevContext.privateReturnData
}

func NewContext(f *tac_parser.TACFunction, args *[]uint256.Int) *TACContext {
	ctx := &TACContext{
		registers:    make([]TACRegister, f.NumVars),
		cycle:        0,
		block_flag:   -1,
		functionStop: false,
		function:     f,
		PrevContext:  nil,
	}

	// initialize args if any
	if args != nil && len(*args) != len(f.Args) {
		panic(fmt.Errorf("calling function with unmatched arguments"))
	}

	// public function does not require argument passing (they use the msg data)
	if !f.IsPublic {
		for idx, id := range f.Args {
			ctx.registers[id].val = (*args)[idx]
		}
	}

	// initialize constants
	for _, sym := range f.ConstSym {
		ctx.registers[sym.Index].val = sym.Value
	}

	return ctx
}

func (ctx *TACContext) GetFuncBlockNames() string {

	fname := ctx.function.Name
	if idx := strings.IndexAny(fname, ",("); idx != -1 {
		fname = fname[:idx]
	}
	fname = strings.TrimSpace(fname)

	bname := ctx.function.BlockByIndex[ctx.block_flag]

	return "func " + fname + " block " + bname
}

func (ctx *TACContext) get_var(idx int) *TACRegister {
	return &ctx.registers[idx]
}

func (ctx *TACContext) get_inst(idx int) int {
	return ctx.function.Insts[idx]
}

func (ctx *TACContext) increase_cycle() {
	ctx.cycle += 1
}

func (ctx *TACContext) Function() *TACFunction {
	return ctx.function
}

func (r *TACRegister) UpdateCycle(cycle uint64) {
	r.cycle = cycle
}

func (r *TACRegister) Assign(cycle uint64, a uint256.Int) {
	r.cycle = cycle
	r.val = a
}

// A series of wrappers for uint256
func (r *TACRegister) Set(cycle uint64, a *uint256.Int) {
	r.cycle = cycle
	r.val.Set(a)
}

func (r *TACRegister) SetUint64(cycle uint64, a uint64) {
	r.cycle = cycle
	r.val.SetUint64(a)
}

func (r *TACRegister) SetBytes(cycle uint64, a []byte) {
	r.cycle = cycle
	r.val.SetBytes(a)
}

func (r *TACRegister) SetFromBig(cycle uint64, a *big.Int) {
	r.cycle = cycle
	r.val.SetFromBig(a)
}

func (r *TACRegister) SetOne(cycle uint64) {
	r.cycle = cycle
	r.val.SetOne()
}

func (r *TACRegister) Clear(cycle uint64) {
	r.cycle = cycle
	r.val.Clear()
}

func (r *TACRegister) Add(cycle uint64, a *uint256.Int, b *uint256.Int) {
	r.cycle = cycle
	r.val.Add(a, b)
}

func (r *TACRegister) Mul(cycle uint64, a *uint256.Int, b *uint256.Int) {
	r.cycle = cycle
	r.val.Mul(a, b)
}

func (r *TACRegister) Sub(cycle uint64, a *uint256.Int, b *uint256.Int) {
	r.cycle = cycle
	r.val.Sub(a, b)
}

func (r *TACRegister) Div(cycle uint64, a *uint256.Int, b *uint256.Int) {
	r.cycle = cycle
	r.val.Div(a, b)
}

func (r *TACRegister) SDiv(cycle uint64, a *uint256.Int, b *uint256.Int) {
	r.cycle = cycle
	r.val.SDiv(a, b)
}

func (r *TACRegister) Mod(cycle uint64, a *uint256.Int, b *uint256.Int) {
	r.cycle = cycle
	r.val.Mod(a, b)
}

func (r *TACRegister) SMod(cycle uint64, a *uint256.Int, b *uint256.Int) {
	r.cycle = cycle
	r.val.SMod(a, b)
}

func (r *TACRegister) AddMod(cycle uint64, a *uint256.Int, b *uint256.Int, c *uint256.Int) {
	r.cycle = cycle
	r.val.AddMod(a, b, c)
}

func (r *TACRegister) MulMod(cycle uint64, a *uint256.Int, b *uint256.Int, c *uint256.Int) {
	r.cycle = cycle
	r.val.MulMod(a, b, c)
}

func (r *TACRegister) Exp(cycle uint64, a *uint256.Int, b *uint256.Int) {
	r.cycle = cycle
	r.val.Exp(a, b)
}

func (r *TACRegister) ExtendSign(cycle uint64, a *uint256.Int, b *uint256.Int) {
	r.cycle = cycle
	r.val.ExtendSign(a, b)
}

func (r *TACRegister) And(cycle uint64, a *uint256.Int, b *uint256.Int) {
	r.cycle = cycle
	r.val.And(a, b)
}

func (r *TACRegister) Or(cycle uint64, a *uint256.Int, b *uint256.Int) {
	r.cycle = cycle
	r.val.Or(a, b)
}

func (r *TACRegister) Xor(cycle uint64, a *uint256.Int, b *uint256.Int) {
	r.cycle = cycle
	r.val.Xor(a, b)
}

func (r *TACRegister) Lsh(cycle uint64, a *uint256.Int, b uint) {
	r.cycle = cycle
	r.val.Lsh(a, b)
}

func (r *TACRegister) Rsh(cycle uint64, a *uint256.Int, b uint) {
	r.cycle = cycle
	r.val.Rsh(a, b)
}

func (r *TACRegister) SetAllOne(cycle uint64) {
	r.cycle = cycle
	r.val.SetAllOne()
}

func (r *TACRegister) SRsh(cycle uint64, a *uint256.Int, b uint) {
	r.cycle = cycle
	r.val.SRsh(a, b)
}
