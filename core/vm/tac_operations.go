package vm

import (
	"io"

	"github.com/ethereum/go-ethereum/tac_parser"
)

type TACOperation struct {
	execution func(pc *int, engine *TACInterpreter, ctxt *TACContext) ([]byte, error)
	dump      func(pc *int, in *TACFunction, out io.Writer)
	name      string
	returns   bool // determines whether the operations sets the return data content
	reverts   bool // determines whether the operation reverts state (implicitly halts)
	halts     bool // indicates whether the operation should halt further execution
}

type TACJumpTable [256]*TACOperation

func GetTACJumptable() *TACJumpTable {
	return &TACJumpTable{
		tac_parser.TAC_THROW: {
			execution: exec_throw,
			dump:      dump_throw,
			name:      "TAC_THROW",
			halts:     true,
		},
		tac_parser.TAC_STOP: {
			execution: exec_stop,
			dump:      dump_stop,
			name:      "TAC_STOP",
			halts:     true,
		},
		tac_parser.TAC_ADDRESS: {
			execution: exec_address,
			dump:      dump_address,
			name:      "TAC_ADDRESS",
		},
		tac_parser.TAC_ORIGIN: {
			execution: exec_origin,
			dump:      dump_origin,
			name:      "TAC_ORIGIN",
		},
		tac_parser.TAC_CALLER: {
			execution: exec_caller,
			dump:      dump_caller,
			name:      "TAC_CALLER",
		},
		tac_parser.TAC_CALLVALUE: {
			execution: exec_callvalue,
			dump:      dump_callvalue,
			name:      "TAC_CALLVALUE",
		},
		tac_parser.TAC_CALLDATASIZE: {
			execution: exec_calldatasize,
			dump:      dump_calldatasize,
			name:      "TAC_CALLDATASIZE",
		},
		tac_parser.TAC_CODESIZE: {
			execution: exec_codesize,
			dump:      dump_codesize,
			name:      "TAC_CODESIZE",
		},
		tac_parser.TAC_GASPRICE: {
			execution: exec_gasprice,
			dump:      dump_gasprice,
			name:      "TAC_GASPRICE",
		},
		tac_parser.TAC_RETURNDATASIZE: {
			execution: exec_returndatasize,
			dump:      dump_returndatasize,
			name:      "TAC_RETURNDATASIZE",
		},
		tac_parser.TAC_COINBASE: {
			execution: exec_coinbase,
			dump:      dump_coinbase,
			name:      "TAC_COINBASE",
		},
		tac_parser.TAC_TIMESTAMP: {
			execution: exec_timestamp,
			dump:      dump_timestamp,
			name:      "TAC_TIMESTAMP",
		},
		tac_parser.TAC_NUMBER: {
			execution: exec_number,
			dump:      dump_number,
			name:      "TAC_NUMBER",
		},
		tac_parser.TAC_DIFFICULTY: {
			execution: exec_difficulty,
			dump:      dump_difficulty,
			name:      "TAC_DIFFICULTY",
		},
		tac_parser.TAC_GASLIMIT: {
			execution: exec_gaslimit,
			dump:      dump_gaslimit,
			name:      "TAC_GASLIMIT",
		},
		tac_parser.TAC_MSIZE: {
			execution: exec_msize,
			dump:      dump_msize,
			name:      "TAC_MSIZE",
		},
		tac_parser.TAC_GAS: {
			execution: exec_gas,
			dump:      dump_gas,
			name:      "TAC_GAS",
		},
		tac_parser.TAC_CHAINID: {
			execution: exec_chainid,
			dump:      dump_chainid,
			name:      "TAC_CHAINID",
		},
		tac_parser.TAC_SELFBALANCE: {
			execution: exec_selfbalance,
			dump:      dump_selfbalance,
			name:      "TAC_SELFBALANCE",
		},
		tac_parser.TAC_ISZERO: {
			execution: exec_iszero,
			dump:      dump_iszero,
			name:      "TAC_ISZERO",
		},
		tac_parser.TAC_BALANCE: {
			execution: exec_balance,
			dump:      dump_balance,
			name:      "TAC_BALANCE",
		},
		tac_parser.TAC_CALLDATALOAD: {
			execution: exec_calldataload,
			dump:      dump_calldataload,
			name:      "TAC_CALLDATALOAD",
		},
		tac_parser.TAC_EXTCODESIZE: {
			execution: exec_extcodesize,
			dump:      dump_extcodesize,
			name:      "TAC_EXTCODESIZE",
		},
		tac_parser.TAC_EXTCODEHASH: {
			execution: exec_extcodehash,
			dump:      dump_extcodehash,
			name:      "TAC_EXTCODEHASH",
		},
		tac_parser.TAC_BLOCKHASH: {
			execution: exec_blockhash,
			dump:      dump_blockhash,
			name:      "TAC_BLOCKHASH",
		},
		tac_parser.TAC_MLOAD: {
			execution: exec_mload,
			dump:      dump_mload,
			name:      "TAC_MLOAD",
		},
		tac_parser.TAC_SLOAD: {
			execution: exec_sload,
			dump:      dump_sload,
			name:      "TAC_SLOAD",
		},
		tac_parser.TAC_JUMP: {
			execution: exec_jump,
			dump:      dump_jump,
			name:      "TAC_JUMP",
		},
		tac_parser.TAC_JUMP_VAR: {
			execution: exec_jump_var,
			dump:      dump_jump_var,
			name:      "TAC_JUMP_VAR",
		},
		tac_parser.TAC_SELFDESTRUCT: {
			execution: exec_selfdestruct,
			dump:      dump_selfdestruct,
			name:      "TAC_SELFDESTRUCT",
			halts:     true,
		},
		tac_parser.TAC_NOT: {
			execution: exec_not,
			dump:      dump_not,
			name:      "TAC_NOT",
		},
		tac_parser.TAC_ADD: {
			execution: exec_add,
			dump:      dump_add,
			name:      "TAC_ADD",
		},
		tac_parser.TAC_MUL: {
			execution: exec_mul,
			dump:      dump_mul,
			name:      "TAC_MUL",
		},
		tac_parser.TAC_SUB: {
			execution: exec_sub,
			dump:      dump_sub,
			name:      "TAC_SUB",
		},
		tac_parser.TAC_DIV: {
			execution: exec_div,
			dump:      dump_div,
			name:      "TAC_DIV",
		},
		tac_parser.TAC_SDIV: {
			execution: exec_sdiv,
			dump:      dump_sdiv,
			name:      "TAC_SDIV",
		},
		tac_parser.TAC_MOD: {
			execution: exec_mod,
			dump:      dump_mod,
			name:      "TAC_MOD",
		},
		tac_parser.TAC_SMOD: {
			execution: exec_smod,
			dump:      dump_smod,
			name:      "TAC_SMOD",
		},
		tac_parser.TAC_ADDMOD: {
			execution: exec_addmod,
			dump:      dump_addmod,
			name:      "TAC_ADDMOD",
		},
		tac_parser.TAC_MULMOD: {
			execution: exec_mulmod,
			dump:      dump_mulmod,
			name:      "TAC_MULMOD",
		},
		tac_parser.TAC_EXP: {
			execution: exec_exp,
			dump:      dump_exp,
			name:      "TAC_EXP",
		},
		tac_parser.TAC_SIGNEXTEND: {
			execution: exec_signextend,
			dump:      dump_signextend,
			name:      "TAC_SIGNEXTEND",
		},
		tac_parser.TAC_LT: {
			execution: exec_lt,
			dump:      dump_lt,
			name:      "TAC_LT",
		},
		tac_parser.TAC_GT: {
			execution: exec_gt,
			dump:      dump_gt,
			name:      "TAC_GT",
		},
		tac_parser.TAC_SLT: {
			execution: exec_slt,
			dump:      dump_slt,
			name:      "TAC_SLT",
		},
		tac_parser.TAC_SGT: {
			execution: exec_sgt,
			dump:      dump_sgt,
			name:      "TAC_SGT",
		},
		tac_parser.TAC_EQ: {
			execution: exec_eq,
			dump:      dump_eq,
			name:      "TAC_EQ",
		},
		tac_parser.TAC_AND: {
			execution: exec_and,
			dump:      dump_and,
			name:      "TAC_AND",
		},
		tac_parser.TAC_OR: {
			execution: exec_or,
			dump:      dump_or,
			name:      "TAC_OR",
		},
		tac_parser.TAC_XOR: {
			execution: exec_xor,
			dump:      dump_xor,
			name:      "TAC_XOR",
		},
		tac_parser.TAC_BYTE: {
			execution: exec_byte,
			dump:      dump_byte,
			name:      "TAC_BYTE",
		},
		tac_parser.TAC_SHL: {
			execution: exec_shl,
			dump:      dump_shl,
			name:      "TAC_SHL",
		},
		tac_parser.TAC_SHR: {
			execution: exec_shr,
			dump:      dump_shr,
			name:      "TAC_SHR",
		},
		tac_parser.TAC_SAR: {
			execution: exec_sar,
			dump:      dump_sar,
			name:      "TAC_SAR",
		},
		tac_parser.TAC_SHA3: {
			execution: exec_sha3,
			dump:      dump_sha3,
			name:      "TAC_SHA3",
		},
		tac_parser.TAC_MSTORE: {
			execution: exec_mstore,
			dump:      dump_mstore,
			name:      "TAC_MSTORE",
		},
		tac_parser.TAC_MSTORE8: {
			execution: exec_mstore8,
			dump:      dump_mstore8,
			name:      "TAC_MSTORE8",
		},
		tac_parser.TAC_SSTORE: {
			execution: exec_sstore,
			dump:      dump_sstore,
			name:      "TAC_SSTORE",
		},
		tac_parser.TAC_JUMPI: {
			execution: exec_jumpi,
			dump:      dump_jumpi,
			name:      "TAC_JUMPI",
		},
		tac_parser.TAC_JUMPI_VAR: {
			execution: exec_jumpi_var,
			dump:      dump_jumpi_var,
			name:      "TAC_JUMPI_VAR",
		},
		tac_parser.TAC_REVERT: {
			execution: exec_revert,
			dump:      dump_revert,
			name:      "TAC_REVERT",
			returns:   true,
			reverts:   true,
		},
		tac_parser.TAC_RETURN: {
			execution: exec_return,
			dump:      dump_return,
			name:      "TAC_RETURN",
			halts:     true,
		},
		tac_parser.TAC_LOG0: {
			execution: makeTACLog(0),
			dump:      dump_log0,
			name:      "TAC_LOG0",
		},
		tac_parser.TAC_CALLDATACOPY: {
			execution: exec_calldatacopy,
			dump:      dump_calldatacopy,
			name:      "TAC_CALLDATACOPY",
		},
		tac_parser.TAC_CODECOPY: {
			execution: exec_codecopy,
			dump:      dump_codecopy,
			name:      "TAC_CODECOPY",
		},
		tac_parser.TAC_RETURNDATACOPY: {
			execution: exec_returndatacopy,
			dump:      dump_returndatacopy,
			name:      "TAC_RETURNDATACOPY",
		},
		tac_parser.TAC_LOG1: {
			execution: makeTACLog(1),
			dump:      dump_log1,
			name:      "TAC_LOG1",
		},
		tac_parser.TAC_CREATE: {
			execution: exec_create,
			dump:      dump_create,
			name:      "TAC_CREATE",
			returns:   true,
		},
		tac_parser.TAC_ILLPHI: {
			execution: exec_illphi,
			dump:      dump_illphi,
			name:      "TAC_ILLPHI",
		},
		tac_parser.TAC_PHI_START: {
			execution: exec_phi_start,
			dump:      dump_phi_start,
			name:      "TAC_PHI_START",
		},
		tac_parser.TAC_PHI: {
			execution: nil, // actual instruction is handled by PHI_START
			dump:      dump_phi,
			name:      "TAC_PHI",
		},
		tac_parser.TAC_PHI_END: {
			execution: nil, // actual instruction is handled by PHI_START
			dump:      dump_phi_end,
			name:      "TAC_PHI_END",
		},
		tac_parser.TAC_BLOCK_FLAG: {
			execution: exec_block_flag,
			dump:      dump_block_flag,
			name:      "TAC_BLOCK_FLAG",
		},
		tac_parser.TAC_EXTCODECOPY: {
			execution: exec_extcodecopy,
			dump:      dump_extcodecopy,
			name:      "TAC_EXTCODECOPY",
		},
		tac_parser.TAC_LOG2: {
			execution: makeTACLog(2),
			dump:      dump_log2,
			name:      "TAC_LOG2",
		},
		tac_parser.TAC_LOG3: {
			execution: makeTACLog(3),
			dump:      dump_log3,
			name:      "TAC_LOG3",
		},
		tac_parser.TAC_LOG4: {
			execution: makeTACLog(4),
			dump:      dump_log4,
			name:      "TAC_LOG4",
		},
		tac_parser.TAC_CALL: {
			execution: exec_call,
			dump:      dump_call,
			name:      "TAC_CALL",
			returns:   true,
		},
		tac_parser.TAC_CALLCODE: {
			execution: exec_callcode,
			dump:      dump_callcode,
			name:      "TAC_CALLCODE",
			returns:   true,
		},
		tac_parser.TAC_DELEGATECALL: {
			execution: exec_delegatecall,
			dump:      dump_delegatecall,
			name:      "TAC_DELEGATECALL",
			returns:   true,
		},
		tac_parser.TAC_CREATE2: {
			execution: exec_create2,
			dump:      dump_create2,
			name:      "TAC_CREATE2",
			returns:   true,
		},
		tac_parser.TAC_STATICCALL: {
			execution: exec_staticcall,
			dump:      dump_staticcall,
			name:      "TAC_STATICCALL",
			returns:   true,
		},
		tac_parser.TAC_CALLPRIVATE: {
			execution: exec_callprivate,
			dump:      dump_callprivate,
			name:      "TAC_CALLPRIVATE",
		},
		tac_parser.TAC_RETURNPRIVATE: {
			execution: exec_returnprivate,
			dump:      dump_returnprivate,
			name:      "TAC_RETURNPRIVATE",
		},
		tac_parser.TAC_CONST: {
			execution: exec_const,
			dump:      dump_const,
			name:      "TAC_CONST",
		},
	}
}
