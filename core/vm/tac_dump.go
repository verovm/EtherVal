package vm

import (
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/tac_parser"
)

// dump define, for each TAC code, a function that prints out the opcode and args in
// humanreadable format

const TraceTAC bool = false

func DumpProgram(in *TACProgram, out io.Writer) {
	for i := range in.Functions {
		dumpFunc(&in.Functions[i], out)
		fmt.Fprintf(out, "\n")
	}
}

func dumpFunc(in *TACFunction, out io.Writer) {
	jumptable := GetTACJumptable()
	fmt.Fprintf(out, "%s ", in.Name)
	if in.IsPublic {
		//TODO
	} else {
		fmt.Fprintf(out, "( ")
		for _, v := range in.Args {
			fmt.Fprintf(out, "%s ", in.RenderVar(v))
		}
		fmt.Fprintf(out, ")")
	}
	fmt.Fprintf(out, "\n")
	for pc := 0; pc < len(in.Insts); {
		fmt.Fprintf(out, "%d:\t", pc)
		jumptable[in.Insts[pc]].dump(&pc, in, out)
	}
}

func dump_throw(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "THROW\n")
	*pc += 1
}

func dump_stop(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "STOP\n")
	*pc += 1
}

func dump_address(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = ADDRESS\n", in.RenderVar(in.Insts[*pc+1]))
	*pc += 2
}

func dump_origin(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = ORIGIN\n", in.RenderVar(in.Insts[*pc+1]))
	*pc += 2
}

func dump_caller(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = CALLER\n", in.RenderVar(in.Insts[*pc+1]))
	*pc += 2
}

func dump_callvalue(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = CALLVALUE\n", in.RenderVar(in.Insts[*pc+1]))
	*pc += 2
}

func dump_calldatasize(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = CALLDATASIZE\n", in.RenderVar(in.Insts[*pc+1]))
	*pc += 2
}

func dump_codesize(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = CODESIZE\n", in.RenderVar(in.Insts[*pc+1]))
	*pc += 2
}

func dump_gasprice(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = GASPRICE\n", in.RenderVar(in.Insts[*pc+1]))
	*pc += 2
}

func dump_returndatasize(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = RETURNDATASIZE\n", in.RenderVar(in.Insts[*pc+1]))
	*pc += 2
}

func dump_coinbase(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = COINBASE\n", in.RenderVar(in.Insts[*pc+1]))
	*pc += 2
}

func dump_timestamp(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = TIMESTAMP\n", in.RenderVar(in.Insts[*pc+1]))
	*pc += 2
}

func dump_number(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = NUMBER\n", in.RenderVar(in.Insts[*pc+1]))
	*pc += 2
}

func dump_difficulty(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = DIFFICULTY\n", in.RenderVar(in.Insts[*pc+1]))
	*pc += 2
}

func dump_gaslimit(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = GASLIMIT\n", in.RenderVar(in.Insts[*pc+1]))
	*pc += 2
}

func dump_msize(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = MSIZE\n", in.RenderVar(in.Insts[*pc+1]))
	*pc += 2
}

func dump_gas(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = GAS\n", in.RenderVar(in.Insts[*pc+1]))
	*pc += 2
}

func dump_chainid(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = CHAINID\n", in.RenderVar(in.Insts[*pc+1]))
	*pc += 2
}

func dump_selfbalance(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = SELFBALANCE\n", in.RenderVar(in.Insts[*pc+1]))
	*pc += 2
}

func dump_iszero(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = ISZERO %s\n", in.RenderVar(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]))
	*pc += 3
}

func dump_balance(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = BALANCE %s\n", in.RenderVar(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]))
	*pc += 3
}

func dump_calldataload(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = CALLDATALOAD %s\n", in.RenderVar(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]))
	*pc += 3
}

func dump_extcodesize(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = EXTCODESIZE %s\n", in.RenderVar(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]))
	*pc += 3
}

func dump_extcodehash(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = EXTCODEHASH %s\n", in.RenderVar(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]))
	*pc += 3
}

func dump_blockhash(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = BLOCKHASH %s\n", in.RenderVar(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]))
	*pc += 3
}

func dump_mload(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = MLOAD %s\n", in.RenderVar(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]))
	*pc += 3
}

func dump_sload(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = SLOAD %s\n", in.RenderVar(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]))
	*pc += 3
}

func dump_jump(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "JUMP %s\n", in.RenderAddress(in.Insts[*pc+1]))
	*pc += 2
}

func dump_jump_var(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "JUMP_VAR %s\n", in.RenderVar(in.Insts[*pc+1]))
	*pc += 2
}

func dump_selfdestruct(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "SELFDESTRUCT %s\n", in.RenderVar(in.Insts[*pc+1]))
	*pc += 2
}

func dump_not(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = NOT %s\n", in.RenderVar(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]))
	*pc += 3
}

func dump_add(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = ADD %s %s\n", in.RenderVar(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]), in.RenderVar(in.Insts[*pc+3]))
	*pc += 4
}

func dump_mul(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = MUL %s %s\n", in.RenderVar(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]), in.RenderVar(in.Insts[*pc+3]))
	*pc += 4
}

func dump_sub(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = SUB %s %s\n", in.RenderVar(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]), in.RenderVar(in.Insts[*pc+3]))
	*pc += 4
}

func dump_div(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = DIV %s %s\n", in.RenderVar(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]), in.RenderVar(in.Insts[*pc+3]))
	*pc += 4
}

func dump_sdiv(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = SDIV %s %s\n", in.RenderVar(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]), in.RenderVar(in.Insts[*pc+3]))
	*pc += 4
}

func dump_mod(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = MOD %s %s\n", in.RenderVar(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]), in.RenderVar(in.Insts[*pc+3]))
	*pc += 4
}

func dump_smod(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = SMOD %s %s\n", in.RenderVar(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]), in.RenderVar(in.Insts[*pc+3]))
	*pc += 4
}

func dump_addmod(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = ADDMOD %s %s %s\n", in.RenderVar(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]), in.RenderVar(in.Insts[*pc+3]), in.RenderVar(in.Insts[*pc+4]))
	*pc += 5
}

func dump_mulmod(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = MULMOD %s %s %s\n", in.RenderVar(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]), in.RenderVar(in.Insts[*pc+3]), in.RenderVar(in.Insts[*pc+4]))
	*pc += 5
}

func dump_exp(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = EXP %s %s\n", in.RenderVar(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]), in.RenderVar(in.Insts[*pc+3]))
	*pc += 4
}

func dump_signextend(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = SIGNEXTEND %s %s\n", in.RenderVar(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]), in.RenderVar(in.Insts[*pc+3]))
	*pc += 4
}

func dump_lt(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = LT %s %s\n", in.RenderVar(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]), in.RenderVar(in.Insts[*pc+3]))
	*pc += 4
}

func dump_gt(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = GT %s %s\n", in.RenderVar(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]), in.RenderVar(in.Insts[*pc+3]))
	*pc += 4
}

func dump_slt(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = SLT %s %s\n", in.RenderVar(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]), in.RenderVar(in.Insts[*pc+3]))
	*pc += 4
}

func dump_sgt(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = SGT %s %s\n", in.RenderVar(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]), in.RenderVar(in.Insts[*pc+3]))
	*pc += 4
}

func dump_eq(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = EQ %s %s\n", in.RenderVar(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]), in.RenderVar(in.Insts[*pc+3]))
	*pc += 4
}

func dump_and(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = AND %s %s\n", in.RenderVar(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]), in.RenderVar(in.Insts[*pc+3]))
	*pc += 4
}

func dump_or(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = OR %s %s\n", in.RenderVar(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]), in.RenderVar(in.Insts[*pc+3]))
	*pc += 4
}

func dump_xor(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = XOR %s %s\n", in.RenderVar(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]), in.RenderVar(in.Insts[*pc+3]))
	*pc += 4
}

func dump_byte(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = BYTE %s %s\n", in.RenderVar(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]), in.RenderVar(in.Insts[*pc+3]))
	*pc += 4
}

func dump_shl(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = SHL %s %s\n", in.RenderVar(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]), in.RenderVar(in.Insts[*pc+3]))
	*pc += 4
}

func dump_shr(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = SHR %s %s\n", in.RenderVar(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]), in.RenderVar(in.Insts[*pc+3]))
	*pc += 4
}

func dump_sar(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = SAR %s %s\n", in.RenderVar(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]), in.RenderVar(in.Insts[*pc+3]))
	*pc += 4
}

func dump_sha3(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = SHA3 %s %s\n", in.RenderVar(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]), in.RenderVar(in.Insts[*pc+3]))
	*pc += 4
}

func dump_mstore(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "MSTORE %s %s\n", in.RenderVar(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]))
	*pc += 3
}

func dump_mstore8(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "MSTORE8 %s %s\n", in.RenderVar(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]))
	*pc += 3
}

func dump_sstore(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "SSTORE %s %s\n", in.RenderVar(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]))
	*pc += 3
}

func dump_jumpi(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "JUMPI %s %s\n", in.RenderAddress(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]))
	*pc += 3
}

func dump_jumpi_var(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "JUMPI_VAR %s %s\n", in.RenderVar(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]))
	*pc += 3
}

func dump_revert(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "REVERT %s %s\n", in.RenderVar(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]))
	*pc += 3
}

func dump_return(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "RETURN %s %s\n", in.RenderVar(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]))
	*pc += 3
}

func dump_log0(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "LOG0 %s %s\n", in.RenderVar(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]))
	*pc += 3
}

func dump_calldatacopy(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "CALLDATACOPY %s %s %s\n", in.RenderVar(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]), in.RenderVar(in.Insts[*pc+3]))
	*pc += 4
}

func dump_codecopy(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "CODECOPY %s %s %s\n", in.RenderVar(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]), in.RenderVar(in.Insts[*pc+3]))
	*pc += 4
}

func dump_returndatacopy(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "RETURNDATACOPY %s %s %s\n", in.RenderVar(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]), in.RenderVar(in.Insts[*pc+3]))
	*pc += 4
}

func dump_log1(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "LOG1 %s %s %s\n", in.RenderVar(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]), in.RenderVar(in.Insts[*pc+3]))
	*pc += 4
}

func dump_create(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = CREATE %s %s %s\n", in.RenderVar(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]), in.RenderVar(in.Insts[*pc+3]), in.RenderVar(in.Insts[*pc+4]))
	*pc += 5
}

func dump_extcodecopy(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = EXTCODECOPY %s %s %s\n", in.RenderVar(in.Insts[*pc+1]), in.RenderVar(in.Insts[*pc+2]), in.RenderVar(in.Insts[*pc+3]), in.RenderVar(in.Insts[*pc+4]))
	*pc += 5
}

func dump_log2(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "LOG2 ")
	for i := 0; i < 4; i++ {
		fmt.Fprintf(out, "%s ", in.RenderVar(in.Insts[*pc+i+1]))
	}
	fmt.Fprintf(out, "\n")
	*pc += 5
}

func dump_log3(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "LOG3 ")
	for i := 0; i < 5; i++ {
		fmt.Fprintf(out, "%s ", in.RenderVar(in.Insts[*pc+i+1]))
	}
	fmt.Fprintf(out, "\n")
	*pc += 6
}

func dump_log4(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "LOG4 ")
	for i := 0; i < 6; i++ {
		fmt.Fprintf(out, "%s ", in.RenderVar(in.Insts[*pc+i+1]))
	}
	fmt.Fprintf(out, "\n")
	*pc += 7
}

func dump_call(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = CALL ", in.RenderVar(in.Insts[*pc+1]))
	*pc += 1
	for i := 0; i < 7; i++ {
		fmt.Fprintf(out, "%s ", in.RenderVar(in.Insts[*pc+i+1]))
	}
	fmt.Fprintf(out, "\n")
	*pc += 8
}

func dump_callcode(pc *int, in *TACFunction, out io.Writer) {
	dest, gas, addr, value, argsOffset, argsSize, retOffset, retSize :=
		in.RenderVar(in.Insts[*pc+1]),
		in.RenderVar(in.Insts[*pc+2]),
		in.RenderVar(in.Insts[*pc+3]),
		in.RenderVar(in.Insts[*pc+4]),
		in.RenderVar(in.Insts[*pc+5]),
		in.RenderVar(in.Insts[*pc+6]),
		in.RenderVar(in.Insts[*pc+7]),
		in.RenderVar(in.Insts[*pc+8])

	fmt.Fprintf(out, "%s = CALLCODE %s %s %s %s %s %s %s\n",
		dest, gas, addr, value, argsOffset, argsSize, retOffset, retSize)
	*pc += 9
}

func dump_delegatecall(pc *int, in *TACFunction, out io.Writer) {
	dest, gas, addr, argsOffset, argsSize, retOffset, retSize :=
		in.RenderVar(in.Insts[*pc+1]),
		in.RenderVar(in.Insts[*pc+2]),
		in.RenderVar(in.Insts[*pc+3]),
		in.RenderVar(in.Insts[*pc+4]),
		in.RenderVar(in.Insts[*pc+5]),
		in.RenderVar(in.Insts[*pc+6]),
		in.RenderVar(in.Insts[*pc+7])

	fmt.Fprintf(out, "%s = DELEGATECALL %s %s %s %s %s %s\n",
		dest, gas, addr, argsOffset, argsSize, retOffset, retSize)
	*pc += 8
}

func dump_create2(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "%s = CREATE2 ", in.RenderVar(in.Insts[*pc+1]))
	*pc += 1
	for i := 0; i < 4; i++ {
		fmt.Fprintf(out, "%s ", in.RenderVar(in.Insts[*pc+i+1]))
	}
	fmt.Fprintf(out, "\n")
	*pc += 5
}

func dump_staticcall(pc *int, in *TACFunction, out io.Writer) {
	dest, gas, addr, argsOffset, argsSize, retOffset, retSize :=
		in.RenderVar(in.Insts[*pc+1]),
		in.RenderVar(in.Insts[*pc+2]),
		in.RenderVar(in.Insts[*pc+3]),
		in.RenderVar(in.Insts[*pc+4]),
		in.RenderVar(in.Insts[*pc+5]),
		in.RenderVar(in.Insts[*pc+6]),
		in.RenderVar(in.Insts[*pc+7])

	fmt.Fprintf(out, "%s = STATICCALL %s %s %s %s %s %s\n",
		dest, gas, addr, argsOffset, argsSize, retOffset, retSize)
	*pc += 8
}

func dump_callprivate(pc *int, in *TACFunction, out io.Writer) {
	num_dest := in.Insts[*pc+1]
	*pc += 2
	for i := 0; i < num_dest; i++ {
		fmt.Fprintf(out, "%s ", in.RenderVar(in.Insts[*pc+i]))
	}
	*pc += num_dest
	target_func := in.Insts[*pc]
	fmt.Fprintf(out, "= CALLPRIVATE %s ", in.Prog.RenderFunction(target_func))
	*pc += 1
	num_args := len(in.Prog.Functions[target_func].Args)
	for i := 0; i < num_args; i++ {
		fmt.Fprintf(out, "%s ", in.RenderVar(in.Insts[*pc+i]))
	}
	fmt.Fprintf(out, "\n")
	*pc += num_args
}

func dump_returnprivate(pc *int, in *TACFunction, out io.Writer) {
	num_ret := in.Insts[*pc+1]
	fmt.Fprintf(out, "RETURNPRIVATE ")
	*pc += 2
	for i := 0; i < num_ret; i++ {
		fmt.Fprintf(out, "%s ", in.RenderVar(in.Insts[*pc+i]))
	}
	fmt.Fprintf(out, "\n")
	*pc += num_ret
}

func dump_phi_start(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "PHI_START (%d):\n", in.Insts[*pc+1])

	numOfPHI := in.Insts[*pc+1]
	if numOfPHI == 0 {
		*pc += 3
		return
	}
	*pc += 2 // move to first PHI
	for i := 0; i < numOfPHI; i++ {
		dump_phi(pc, in, out)
	}
	*pc += 1 // skip PHI_END
}

func dump_phi(pc *int, in *TACFunction, out io.Writer) {
	num_choices := in.Insts[*pc+1]
	fmt.Fprintf(out, "%s = PHI ", in.RenderVar(in.Insts[*pc+2]))
	*pc += 3
	for i := 0; i < num_choices; i++ {
		fmt.Fprintf(out, "%s ", in.RenderVar(in.Insts[*pc+i]))
	}
	fmt.Fprintf(out, "\n")
	*pc = *pc + num_choices
}

func dump_illphi(pc *int, in *TACFunction, out io.Writer) {
	panic(tac_parser.ErrTACIllPhiFound)
}

func dump_phi_end(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "PHI_END\n")
	*pc += 1
}

func dump_block_flag(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "function %s, basic block %s\n", in.Name, in.BlockByIndex[*pc])
	*pc += 1
}

func dump_const(pc *int, in *TACFunction, out io.Writer) {
	fmt.Fprintf(out, "CONST %s\n", in.RenderConst(in.Insts[*pc+1]))
	*pc += 2
}
