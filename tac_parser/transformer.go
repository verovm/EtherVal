package tac_parser

import (
	"fmt"
	"strings"

	"github.com/holiman/uint256"
)

// TODO: alternative policies for PHI
const (
	EnablePatch_ParameterName = true
	EnablePatch_Reorder       = true
	EnablePatch_Fallthrough   = true
	EnablePatch_PHI           = true // true: most recently assigned var, false: always first arg
	EnablePatch_AmbiguousJump = true
)

// Transformer encode AstProgram into an executable format TACProgram
// Each TACProgram contains several TACFunctions
// Each TACFunctions contains a linear bytecode

// Constants are mocked with variable
// There are two types of argument, either a variable or an address
// types are static and should be determined by operation semantic

var ErrTACIllPhiFound = fmt.Errorf("TAC ILLPHI statement should not exist in TAC")

type Uint256 = uint256.Int

type TACFunction struct {
	// execution info
	Insts    []int
	ConstSym []ConstSymPair // const values (idx, val)
	Args     []int
	IsPublic bool
	Prog     *TACProgram
	NumVars  int
	Name     string

	// auxiliary info
	BlockByName       map[string]int   // map block name to index in the bytecode
	BlockByIndex      map[int]string   // map block index to name
	UniqueBlockAddr   map[string]int   // if block base address is unique, map it to index in the bytecode
	UniqueJumpVarSucc map[int][]string // JUMP_VAR instruction idx to list of successor block names
	VariableEncoding  map[string]int   // map variable name into an idx

	// Track instructions with semantic issues
	AmbiguousJumpAddr map[int]bool      // Addresses of ambiguous jump instructions (regardless of patches)
	VariableBlock     map[string]string // Variable definitions to block names
	AmbiguousPhiAddr  map[int]bool      // Addresses of ambiguous PHI instructions (regardless of patches)
	ReorderConstAddr  map[int]bool      // Addresses of CONST instructions to reorder
	ReorderPhiAddr    map[int]bool      // Addresses of PHI instructions to reorder

	// Track instructions added from fallthrough semantics patch
	FallthroughStopAddr  map[int]bool // Addresses of STOP from fallthrough semantics patch
	FallthroughJumpAddr  map[int]bool // Addresses of JUMP from fallthrough semantics patch
	FallthroughThrowAddr map[int]bool // Addresses of THROW from fallthrough semantics patch

	// Track choices that make PHI instructions ambiguous
	AmbiguousPhiChoice map[int]map[int]bool // map[phiAddr][regIndex]

	bookkeeping map[int]*AstStatement // bookkeeping all instructions that need to be filled later

	AstFunc *AstFunction
}

func newTACFunction(prog *TACProgram, astFunc *AstFunction) TACFunction {
	return TACFunction{
		IsPublic: false,
		Prog:     prog,

		BlockByName:       make(map[string]int),
		BlockByIndex:      make(map[int]string),
		UniqueBlockAddr:   make(map[string]int),
		UniqueJumpVarSucc: make(map[int][]string),
		VariableEncoding:  make(map[string]int),

		AmbiguousJumpAddr: make(map[int]bool),
		VariableBlock:     make(map[string]string),
		AmbiguousPhiAddr:  make(map[int]bool),
		ReorderConstAddr:  make(map[int]bool),
		ReorderPhiAddr:    make(map[int]bool),

		FallthroughStopAddr:  make(map[int]bool),
		FallthroughJumpAddr:  make(map[int]bool),
		FallthroughThrowAddr: make(map[int]bool),

		AmbiguousPhiChoice: make(map[int]map[int]bool),

		bookkeeping: make(map[int]*AstStatement),

		AstFunc: astFunc,
	}
}

type TACProgram struct {
	Entry     *TACFunction
	Functions []TACFunction

	FunctionEncoding map[string]int // map function address into an idx

	Ast *AstProgram
}

func newTACProgram(ast *AstProgram) *TACProgram {
	return &TACProgram{
		FunctionEncoding: make(map[string]int),

		Ast: ast,
	}
}

func (f *TACFunction) add_var(name string) int {
	if id, ok := f.VariableEncoding[name]; ok {
		return id
	}
	f.VariableEncoding[name] = f.NumVars
	f.NumVars++
	return f.NumVars - 1
}

// never fails, if element already exists, return its idx
func (f *TACFunction) encode_var(astVar *AstVariable) int {
	var idx int
	if typ := astVar.typ; typ == CONSTANT {
		if idx, ok := f.VariableEncoding[astVar.name]; ok {
			return idx
		}
		// add constant into constant symbol
		idx = f.add_var(astVar.name)
		// trim leading 0 from hex number, this is just to make the parser to be more forgivable
		hex := astVar.val[2:]
		for len(hex) > 1 && hex[0] == '0' {
			hex = hex[1:]
		}
		hex = "0x" + hex
		val, err := uint256.FromHex(hex)
		if err != nil {
			panic(fmt.Errorf("cannot transfer to hex from %s", hex))
		}
		f.ConstSym = append(f.ConstSym, ConstSymPair{Index: idx, Value: *val})
	} else {
		// for variable, simply encode it
		idx = f.add_var(astVar.name)
	}
	return idx
}

func (prog *TACProgram) GetFunctions() []TACFunction {
	return prog.Functions
}

// never fails, if element already exists, return its idx
func (prog *TACProgram) encode_function(name string) {
	if _, ok := prog.FunctionEncoding[name]; ok {
		return
	}
	prog.FunctionEncoding[name] = len(prog.FunctionEncoding)
}

// panic if elment cannot be found
func (prog *TACProgram) decode_function(name string) int {
	if idx, ok := prog.FunctionEncoding[name]; !ok {
		panic(fmt.Errorf("error in decode_function, cannot find function %s", name))
	} else {
		return idx
	}
}

func shiftVarDecls(ast *AstProgram) *AstProgram {
	// Shift all variable declarations (CONST, PHI, ILL_PHI) to the start of the block
	// Because sometimes gigahorse insert them at random positions
	// When they are inserted at the end of the block, this can mess up other
	// semantic assumption.

	isVariableDecl := func(s *AstStatement) bool {
		op := s.operation
		return op == CONST || op == PHI || op == ILLPHI
	}

	for func_idx := range ast.functions {
		function := ast.functions[func_idx]
		for block_idx := range function.blocks {
			block := &function.blocks[block_idx]
			varDeclStmt := make([]AstStatement, 0)
			others := make([]AstStatement, 0)

			for i, stmt := range block.statements {
				if isVariableDecl(&stmt) {
					varDeclStmt = append(varDeclStmt, stmt)

					// Track var decl stmt when reorder patch will change their addresses
					if j := len(varDeclStmt) - 1; i != j {
						// Infeasible to analyze mutability of struct from arrays and pointers
						// Just set patch_reorder flag for all possible statement instances
						stmt.patch_reorder = true
						block.statements[i].patch_reorder = true
						varDeclStmt[j].patch_reorder = true
					}
				} else {
					others = append(others, stmt)
				}
			}

			if EnablePatch_Reorder {
				block.statements = append(varDeclStmt, others...)
			}
		}
	}
	return ast
}

func addMissingStop(ast *AstProgram) *AstProgram {
	// A block with no succ must end with an halt operation.
	// However, this is not always the case in TAC, there are missing halt operation.
	// e.g. Gigahorse people mistranslate the function selector
	// in the if-else chain that selects the correct function
	// they forgot to put a STOP after selecting the function
	// causing the selected function + all functions afterwards being called.
	// We have to manually insert a STOP after each call

	notHalt := func(s *AstStatement) bool {
		op := s.operation
		return op != STOP && op != RETURN && op != REVERT &&
			op != SELFDESTRUCT && op != THROW && op != RETURNPRIVATE
	}

	for func_idx := range ast.functions {
		function := ast.functions[func_idx]
		for block_idx := range function.blocks {
			block := &function.blocks[block_idx]
			if len(block.successor) != 0 {
				continue
			}
			if len(block.statements) == 0 || notHalt(&block.statements[len(block.statements)-1]) {
				block.statements = append(block.statements, AstStatement{
					address:      "auxiliary",
					operation:    STOP,
					num_assignee: 0,
					block_name:   block.name,
				})
			}
		}
	}
	return ast
}

func addMissingJump(ast *AstProgram) *AstProgram {
	// This transformer try to fix 2 things.
	// 1. A broken fallthrough semantic
	// 2. A missing jump at the end of (empty) block
	for func_idx := range ast.functions {
		function := ast.functions[func_idx]
		for block_idx := range function.blocks {
			block := &function.blocks[block_idx]
			// if block has only one succ and last statement is not jump,
			// than a fix may needed
			if len(block.successor) == 1 {
				if len(block.statements) == 0 || block.statements[len(block.statements)-1].operation != JUMP {
					should_fall_to := BaseAddr(block.successor[0])
					if block_idx == len(function.blocks)-1 || should_fall_to != function.blocks[block_idx+1].address {
						arg := AstVariable{
							name: should_fall_to,
							val:  should_fall_to,
							typ:  CONSTANT,
						}
						block.statements = append(block.statements, AstStatement{
							address:      "auxiliary",
							operation:    JUMP,
							args:         []AstVariable{arg},
							num_assignee: 0,
							block_name:   block.name,
						})
					}
				}
			}

			// if block has two succes, then it must ends with a jump/jumpi with a dest d1
			// the next block must be succ \ d1, otherwise a fix is needed
			// by observation, the first succ is always the fallthrough one
			if len(block.successor) == 2 {
				// assuming non-empty block, as empty block with two succ cannot be fixed anyway
				lastStmt := block.statements[len(block.statements)-1]
				if lastStmt.operation != JUMP && lastStmt.operation != JUMPI {
					// If last statement does not end with a JUMP or JUMPI
					// we cannot determine the fallthrough semantic, insert an artificial THROW
					// TODO specialized throw for error report
					block.statements = append(block.statements, AstStatement{
						address:      "auxiliary",
						operation:    THROW,
						args:         []AstVariable{},
						num_assignee: 0,
						block_name:   block.name,
					})
				} else {
					var fall int
					if lastStmt.args[0].typ == VARIABLE {
						continue // can't determine if it s a indirect jump
					}
					jump_to := lastStmt.args[0].val
					if EqualBaseAddr(jump_to, block.successor[0]) {
						fall = 1
					} else {
						fall = 0
					}
					should_fall_to := BaseAddr(block.successor[fall])
					if block_idx == len(function.blocks)-1 || should_fall_to != function.blocks[block_idx+1].address {
						arg := AstVariable{
							name: should_fall_to,
							val:  should_fall_to,
							typ:  CONSTANT,
						}
						block.statements = append(block.statements, AstStatement{
							address:      "auxiliary",
							operation:    JUMP,
							args:         []AstVariable{arg},
							num_assignee: 0,
							block_name:   block.name,
						})
					}
				}
			}
		}
	}
	return ast
}

func TransformProgram(ast *AstProgram) *TACProgram {
	// Patch: Shift CONST and PHI to the start of the block
	ast = shiftVarDecls(ast)

	if EnablePatch_Fallthrough {
		// Patch: Add missing STOP for blocks with empty succ
		ast = addMissingStop(ast)
	}

	if EnablePatch_Fallthrough {
		// Patch: Add missing JUMP to fix fallthrough semantics and empty blocks
		ast = addMissingJump(ast)
	}

	prog := newTACProgram(ast)

	// Find entry function, always placed at 0
	for idx := range ast.functions {
		function := ast.functions[idx]
		if function.name == "__function_selector__" {
			prog.encode_function(function.blocks[0].address)
		}
	}

	// Encode others
	for idx := range ast.functions {
		function := ast.functions[idx]
		prog.encode_function(function.blocks[0].address)
	}

	prog.Functions = make([]TACFunction, len(ast.functions))

	// transform each function
	for idx := range ast.functions {
		function := ast.functions[idx]
		ret := transform_function(&function, prog)
		prog.Functions[prog.decode_function(function.blocks[0].address)] = ret
	}

	prog.Entry = &prog.Functions[0]

	return prog
}

// transform_function flatten the graph layout into a linear one.
func transform_function(astFunc *AstFunction, prog *TACProgram) TACFunction {
	function := newTACFunction(prog, astFunc)
	function.Name = astFunc.name
	// encode each arguments for function
	astFunc.is_public = function.IsPublic
	for _, arg := range astFunc.args {
		if EnablePatch_ParameterName {
			// Patch: Fix wrong parameter names
			if !strings.HasPrefix(arg, "v") {
				arg = render_var(arg)
			}
		}
		function.Args = append(function.Args, function.add_var(arg))
	}
	var (
		order           []*AstBlock
		visited         = make(map[*AstBlock]bool)
		get_visit_order func(cur *AstBlock)
	)

	get_visit_order = func(cur *AstBlock) {
		order = append(order, cur)
		for _, block_addr := range cur.successor {
			block := astFunc.get_block(block_addr)
			if is_visited := (visited)[block]; !is_visited {
				(visited)[block] = true
				get_visit_order(block)
			}
		}
	}

	visited[&astFunc.blocks[0]] = true
	get_visit_order(&astFunc.blocks[0])

	// variable definitions and block names
	for _, block := range order {
		for _, stmt := range block.statements {
			for _, arg := range stmt.args[:stmt.num_assignee] {
				function.VariableBlock[arg.name] = block.name
			}
		}
	}

	// transform blocks
	for _, block := range order {
		function.add_block(block)
	}

	// Count block base addresses
	countBlockBaseAddr := make(map[string]int)
	for name := range function.BlockByName {
		countBlockBaseAddr[BaseAddr(name)]++
	}

	// Compute UniqueBlockAddr
	for name, idx := range function.BlockByName {
		if addr := BaseAddr(name); countBlockBaseAddr[addr] == 1 {
			function.UniqueBlockAddr[addr] = idx
		}
	}

	// Fill up address encoding
	for pc, stmt := range function.bookkeeping {
		switch opcode := function.Insts[pc]; opcode {
		case TAC_JUMP, TAC_JUMPI:
			destIdx := pc + 1
			function.Insts[destIdx] = -1

			block := astFunc.get_block(stmt.block_name)
			baseAddr := stmt.args[0].val

			// Track ambiguous jump instructions
			if countBlockBaseAddr[baseAddr] >= 2 {
				function.AmbiguousJumpAddr[pc] = true
			}
			for i, name := range block.successor {
				if opcode == TAC_JUMPI && i == 0 {
					// Ignore fallthrough edge of JUMPI
					continue
				}
				if countBlockBaseAddr[BaseAddr(name)] >= 2 {
					function.AmbiguousJumpAddr[pc] = true
				}
			}

			if EnablePatch_AmbiguousJump {
				// Patch: Fix ambiguous jump addresses with succ
				dests := []int{}
				for _, name := range block.successor {
					if strings.HasPrefix(name, baseAddr) {
						idx := function.BlockByName[name]
						dests = append(dests, idx)
					}
				}
				if len(dests) == 1 {
					function.Insts[destIdx] = dests[0]
					continue
				}
			}

			// If not found, search in UniqueBlockAddr
			if idx, ok := function.UniqueBlockAddr[baseAddr]; ok {
				function.Insts[destIdx] = idx
				continue
			}
		case TAC_JUMP_VAR, TAC_JUMPI_VAR:
			// Track ambiguous jump var instructions
			block := astFunc.get_block(stmt.block_name)
			for i, name := range block.successor {
				if opcode == TAC_JUMPI_VAR && i == 0 {
					// Ignore fallthrough edge of JUMPI_VAR
					continue
				}
				if countBlockBaseAddr[BaseAddr(name)] >= 2 {
					function.AmbiguousJumpAddr[pc] = true
				}
			}

			if EnablePatch_AmbiguousJump {
				// Patch: Fix ambiguous jump var addresses with succ
				countSuccBaseAddr := make(map[string]int)
				for _, name := range block.successor {
					countSuccBaseAddr[BaseAddr(name)]++
				}
				for name, count := range countSuccBaseAddr {
					if count == 1 {
						function.UniqueJumpVarSucc[pc] = append(function.UniqueJumpVarSucc[pc], name)
					}
				}
			}
		default:
			panic("Fatal in bookkeeping, unmatched operation")
		}
	}
	return function
}

func (f *TACFunction) add_block(block *AstBlock) {
	// log block start location
	f.BlockByName[block.name] = len(f.Insts)
	f.BlockByIndex[len(f.Insts)] = block.name

	f.Insts = append(f.Insts, TAC_BLOCK_FLAG)

	// Convert numbered ILLPHI to PHI to allow execution.
	for i, stmt := range block.statements {
		if ILLPHI1 <= stmt.operation && stmt.operation <= ILLPHI5 {
			block.statements[i].operation = PHI
		}
	}

	// collect all CONST and PHI statements
	var const_statements []*AstStatement
	var phi_statements []*AstStatement

	// Track reordering CONST and PHI statements while collecting
	for i, stmt := range block.statements {
		if stmt.operation == CONST {
			const_statements = append(const_statements, &block.statements[i])
			if j := len(const_statements) - 1; i != j {
				stmt.patch_reorder = true
				block.statements[i].patch_reorder = true
				const_statements[j].patch_reorder = true
			}
		}
	}
	for i, stmt := range block.statements {
		if stmt.operation == PHI {
			phi_statements = append(phi_statements, &block.statements[i])
			if j := len(phi_statements) - 1; i != len(const_statements)+j {
				stmt.patch_reorder = true
				block.statements[i].patch_reorder = true
				phi_statements[j].patch_reorder = true
			}
		}
	}

	if EnablePatch_Reorder {
		// insert all found CONST statements at the start of the block
		if len(const_statements) > 0 {
			for i := range const_statements {
				trans_const(f, const_statements[i])
			}
		}

		// insert all found PHI statements at the start of the block
		if len(phi_statements) > 0 {
			trans_phi_statements(f, phi_statements)
		}

		// translate all other statements
		for i, stmt := range block.statements {
			if stmt.operation != CONST && stmt.operation != PHI {
				transformer_dispatcher(f, &block.statements[i])
			}
		}
	} else {
		// transform statements in order without shifting
		for i, stmt := range block.statements {
			switch stmt.operation {
			case CONST:
				trans_const(f, &block.statements[i])
			case PHI:
				trans_phi_statements(f, []*AstStatement{&block.statements[i]})
			default:
				transformer_dispatcher(f, &block.statements[i])
			}
		}
	}

}

func trans_phi_statements(f *TACFunction, phi_statements []*AstStatement) {
	// Surround PHI statements with PHI_START and PHI_END
	if len(phi_statements) > 0 {
		f.Insts = append(f.Insts, TAC_PHI_START)
		f.Insts = append(f.Insts, len(phi_statements))

		// translate phis
		for i := range phi_statements {
			trans_phi(f, phi_statements[i])
		}

		f.Insts = append(f.Insts, TAC_PHI_END)
	}
}

func trans_phi(f *TACFunction, s *AstStatement) {
	phiAddr := len(f.Insts)
	// Track ambiguous PHI instructions
	argBlockCount := make(map[string]int)
	ambiArgBlock := make(map[string]bool)
	for _, arg := range s.args[1:] {
		b := f.VariableBlock[arg.name]
		argBlockCount[b]++
	}
	for b, count := range argBlockCount {
		if count >= 2 {
			ambiArgBlock[b] = true
		}
	}
	for _, is_ambi := range ambiArgBlock {
		if is_ambi {
			f.AmbiguousPhiAddr[phiAddr] = true
			break
		}
	}

	// Track PHI instructions to reorder
	f.ReorderPhiAddr[phiAddr] = s.patch_reorder
	// Initialize map for ambiguous PHI arguments
	f.AmbiguousPhiChoice[phiAddr] = make(map[int]bool)

	// New format: PHI num_choice dest v1 v2 .. vn
	f.Insts = append(f.Insts, TAC_PHI)
	// only single choice without patching
	f.Insts = append(f.Insts, 1)
	f.Insts = append(f.Insts, f.encode_var(&s.args[0])) // dest
	f.Insts = append(f.Insts, f.encode_var(&s.args[1])) // first choice
	// Track if first choice is ambiguous
	if arg := s.args[1]; ambiArgBlock[f.VariableBlock[arg.name]] {
		choiceIdx := f.Insts[len(f.Insts)-1]
		f.AmbiguousPhiChoice[phiAddr][choiceIdx] = true
	}

	if EnablePatch_PHI {
		// Patch: Allow multiple choices for PHI nodes
		// TAC interpreter will choose the most recently assigned variable
		f.Insts[len(f.Insts)-3] = len(s.args) - 1
		for _, arg := range s.args[2:] {
			f.Insts = append(f.Insts, f.encode_var(&arg))
			// Track ambiguous PHI choices
			if ambiArgBlock[f.VariableBlock[arg.name]] {
				choiceIdx := f.Insts[len(f.Insts)-1]
				f.AmbiguousPhiChoice[phiAddr][choiceIdx] = true
			}
		}
	}
}

func trans_illphi(f *TACFunction, s *AstStatement) {
	panic(ErrTACIllPhiFound)
}

func trans_const(f *TACFunction, s *AstStatement) {
	// Track CONST instructions to reorder
	f.ReorderConstAddr[len(f.Insts)] = s.patch_reorder

	// We need to execute CONST to update variable cycles for PHI
	f.Insts = append(f.Insts, TAC_CONST)
	f.push_variables(1, s.args)
}

func trans_throw(f *TACFunction, s *AstStatement) {
	// Track THROW from fallthrough semantics patch
	if s.address == "auxiliary" {
		f.FallthroughThrowAddr[len(f.Insts)] = true
	}
	f.Insts = append(f.Insts, TAC_THROW)
}

func trans_stop(f *TACFunction, s *AstStatement) {
	// Track STOP from fallthrough semantics patch
	if s.address == "auxiliary" {
		f.FallthroughStopAddr[len(f.Insts)] = true
	}
	f.Insts = append(f.Insts, TAC_STOP)
}

func trans_address(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_ADDRESS)
	f.push_variables(1, s.args)
}

func trans_origin(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_ORIGIN)
	f.push_variables(1, s.args)
}

func trans_caller(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_CALLER)
	f.push_variables(1, s.args)
}

func trans_callvalue(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_CALLVALUE)
	f.push_variables(1, s.args)
}

func trans_calldatasize(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_CALLDATASIZE)
	f.push_variables(1, s.args)
}

func trans_codesize(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_CODESIZE)
	f.push_variables(1, s.args)
}

func trans_gasprice(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_GASPRICE)
	f.push_variables(1, s.args)
}

func trans_returndatasize(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_RETURNDATASIZE)
	f.push_variables(1, s.args)
}

func trans_coinbase(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_COINBASE)
	f.push_variables(1, s.args)
}

func trans_timestamp(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_TIMESTAMP)
	f.push_variables(1, s.args)
}

func trans_number(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_NUMBER)
	f.push_variables(1, s.args)
}

func trans_difficulty(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_DIFFICULTY)
	f.push_variables(1, s.args)
}

func trans_gaslimit(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_GASLIMIT)
	f.push_variables(1, s.args)
}

func trans_msize(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_MSIZE)
	f.push_variables(1, s.args)
}

func trans_gas(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_GAS)
	f.push_variables(1, s.args)
}

func trans_chainid(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_CHAINID)
	f.push_variables(1, s.args)
}

func trans_selfbalance(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_SELFBALANCE)
	f.push_variables(1, s.args)
}

func trans_iszero(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_ISZERO)
	f.push_variables(2, s.args)
}

func trans_balance(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_BALANCE)
	f.push_variables(2, s.args)
}

func trans_calldataload(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_CALLDATALOAD)
	f.push_variables(2, s.args)
}

func trans_extcodesize(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_EXTCODESIZE)
	f.push_variables(2, s.args)
}

func trans_extcodehash(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_EXTCODEHASH)
	f.push_variables(2, s.args)
}

func trans_blockhash(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_BLOCKHASH)
	f.push_variables(2, s.args)
}

func trans_mload(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_MLOAD)
	f.push_variables(2, s.args)
}

func trans_sload(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_SLOAD)
	f.push_variables(2, s.args)
}

// jump target(address)
func trans_jump(f *TACFunction, s *AstStatement) {
	// jump target can be a constant or a variable (rare)
	// To differentiate them during runtime, we introduce JUMP (assume constant) and JUMP_VAR
	// For constant, we fill up the static information after first pass of transformation.
	// For variable, we look it up during runtime
	// Because 1. gives more debug info when target is constant
	// 2. performance (doesn't actually matter)
	f.bookkeeping[len(f.Insts)] = s

	if s.args[0].typ == CONSTANT {
		f.Insts = append(f.Insts, TAC_JUMP)
		f.push_variables(1, s.args)
	} else {
		f.Insts = append(f.Insts, TAC_JUMP_VAR)
		f.push_variables(1, s.args)
	}
}

func trans_selfdestruct(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_SELFDESTRUCT)
	f.push_variables(1, s.args)
}

func trans_not(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_NOT)
	f.push_variables(2, s.args)
}

func trans_add(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_ADD)
	f.push_variables(3, s.args)
}

func trans_mul(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_MUL)
	f.push_variables(3, s.args)
}

func trans_sub(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_SUB)
	f.push_variables(3, s.args)
}

func trans_div(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_DIV)
	f.push_variables(3, s.args)
}

func trans_sdiv(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_SDIV)
	f.push_variables(3, s.args)
}

func trans_mod(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_MOD)
	f.push_variables(3, s.args)
}

func trans_smod(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_SMOD)
	f.push_variables(3, s.args)
}

func trans_addmod(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_ADDMOD)
	f.push_variables(4, s.args)
}

func trans_mulmod(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_MULMOD)
	f.push_variables(4, s.args)
}

func trans_exp(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_EXP)
	f.push_variables(3, s.args)
}

func trans_signextend(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_SIGNEXTEND)
	f.push_variables(3, s.args)
}

func trans_lt(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_LT)
	f.push_variables(3, s.args)
}

func trans_gt(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_GT)
	f.push_variables(3, s.args)
}

func trans_slt(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_SLT)
	f.push_variables(3, s.args)
}

func trans_sgt(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_SGT)
	f.push_variables(3, s.args)
}

func trans_eq(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_EQ)
	f.push_variables(3, s.args)
}

func trans_and(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_AND)
	f.push_variables(3, s.args)
}

func trans_or(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_OR)
	f.push_variables(3, s.args)
}

func trans_xor(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_XOR)
	f.push_variables(3, s.args)
}

func trans_byte(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_BYTE)
	f.push_variables(3, s.args)
}

func trans_shl(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_SHL)
	f.push_variables(3, s.args)
}

func trans_shr(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_SHR)
	f.push_variables(3, s.args)
}

func trans_sar(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_SAR)
	f.push_variables(3, s.args)
}

func trans_sha3(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_SHA3)
	f.push_variables(3, s.args)
}

func trans_mstore(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_MSTORE)
	f.push_variables(2, s.args)
}

func trans_mstore8(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_MSTORE8)
	f.push_variables(2, s.args)
}

func trans_sstore(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_SSTORE)
	f.push_variables(2, s.args)
}

// JUMPI target(address) cond(var)
func trans_jumpi(f *TACFunction, s *AstStatement) {
	// Track JUMP from fallthrough semantics patch
	if s.address == "auxiliary" {
		f.FallthroughJumpAddr[len(f.Insts)] = true
	}
	// see comment in trans_jump
	f.bookkeeping[len(f.Insts)] = s
	if s.args[0].typ == CONSTANT {
		f.Insts = append(f.Insts, TAC_JUMPI)
		f.push_variables(2, s.args)
	} else {
		f.Insts = append(f.Insts, TAC_JUMPI_VAR)
		f.push_variables(2, s.args)
	}
}

func trans_revert(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_REVERT)
	f.push_variables(2, s.args)
}

func trans_return(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_RETURN)
	f.push_variables(2, s.args)
}

func trans_log0(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_LOG0)
	f.push_variables(2, s.args)
}

func trans_calldatacopy(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_CALLDATACOPY)
	f.push_variables(3, s.args)
}

func trans_codecopy(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_CODECOPY)
	f.push_variables(3, s.args)
}

func trans_returndatacopy(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_RETURNDATACOPY)
	f.push_variables(3, s.args)
}

func trans_log1(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_LOG1)
	f.push_variables(3, s.args)
}

func trans_create(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_CREATE)
	f.push_variables(4, s.args)
}

func trans_extcodecopy(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_EXTCODECOPY)
	f.push_variables(4, s.args)
}

func trans_log2(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_LOG2)
	f.push_variables(4, s.args)
}

func trans_log3(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_LOG3)
	f.push_variables(5, s.args)
}

func trans_log4(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_LOG4)
	f.push_variables(6, s.args)
}

func trans_call(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_CALL)
	f.push_variables(8, s.args)
}

func trans_callcode(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_CALLCODE)
	f.push_variables(8, s.args)
}

func trans_delegatecall(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_DELEGATECALL)
	f.push_variables(7, s.args)
}

func trans_create2(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_CREATE2)
	f.push_variables(5, s.args)
}

func trans_staticcall(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_STATICCALL)
	f.push_variables(7, s.args)
}

// callprivate numOfDest [dest...] target(address) [args...]
func trans_callprivate(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_CALLPRIVATE)
	f.Insts = append(f.Insts, s.num_assignee)
	f.push_variables(s.num_assignee, s.args)
	f.Insts = append(f.Insts, f.Prog.decode_function(s.args[s.num_assignee].val))
	f.push_variables(len(s.args)-1-s.num_assignee, s.args[s.num_assignee+1:])
}

// returnprivate numOfRets var
func trans_returnprivate(f *TACFunction, s *AstStatement) {
	f.Insts = append(f.Insts, TAC_RETURNPRIVATE)
	f.Insts = append(f.Insts, len(s.args)-1) // first arg is the return address, which we omit here.
	f.push_variables(len(s.args)-1, s.args[1:])
}

func (f *TACFunction) push_variables(num_args int, vars []AstVariable) {
	i := 0
	for ; i < num_args; i++ {
		f.Insts = append(f.Insts, f.encode_var(&vars[i]))
	}
}

/*
From gigahorse-toolchain/visualizeout.py file:

def render_var(var: str):

	if var in tac_variable_value:
	    return f"v{var.replace('0x', '')}({tac_variable_value[var]})"
	else:
	    return f"v{var.replace('0x', '')}
*/
func render_var(name string) string {
	return "v" + strings.ReplaceAll(name, "0x", "")
}

func transformer_dispatcher(f *TACFunction, s *AstStatement) {
	switch op := s.operation; op {
	case PHI:
		panic("transformation of PHI should be done before transformer_dispatcher")
	case ILLPHI:
		trans_illphi(f, s)
	case CONST:
		trans_const(f, s)
	case THROW:
		trans_throw(f, s)
	case STOP:
		trans_stop(f, s)
	case ADDRESS:
		trans_address(f, s)
	case ORIGIN:
		trans_origin(f, s)
	case CALLER:
		trans_caller(f, s)
	case CALLVALUE:
		trans_callvalue(f, s)
	case CALLDATASIZE:
		trans_calldatasize(f, s)
	case CODESIZE:
		trans_codesize(f, s)
	case GASPRICE:
		trans_gasprice(f, s)
	case RETURNDATASIZE:
		trans_returndatasize(f, s)
	case COINBASE:
		trans_coinbase(f, s)
	case TIMESTAMP:
		trans_timestamp(f, s)
	case NUMBER:
		trans_number(f, s)
	case DIFFICULTY:
		trans_difficulty(f, s)
	case GASLIMIT:
		trans_gaslimit(f, s)
	case MSIZE:
		trans_msize(f, s)
	case GAS:
		trans_gas(f, s)
	case CHAINID:
		trans_chainid(f, s)
	case SELFBALANCE:
		trans_selfbalance(f, s)
	case ISZERO:
		trans_iszero(f, s)
	case BALANCE:
		trans_balance(f, s)
	case CALLDATALOAD:
		trans_calldataload(f, s)
	case EXTCODESIZE:
		trans_extcodesize(f, s)
	case EXTCODEHASH:
		trans_extcodehash(f, s)
	case BLOCKHASH:
		trans_blockhash(f, s)
	case MLOAD:
		trans_mload(f, s)
	case SLOAD:
		trans_sload(f, s)
	case JUMP:
		trans_jump(f, s)
	case SELFDESTRUCT:
		trans_selfdestruct(f, s)
	case NOT:
		trans_not(f, s)
	case ADD:
		trans_add(f, s)
	case MUL:
		trans_mul(f, s)
	case SUB:
		trans_sub(f, s)
	case DIV:
		trans_div(f, s)
	case SDIV:
		trans_sdiv(f, s)
	case MOD:
		trans_mod(f, s)
	case SMOD:
		trans_smod(f, s)
	case ADDMOD:
		trans_addmod(f, s)
	case MULMOD:
		trans_mulmod(f, s)
	case EXP:
		trans_exp(f, s)
	case SIGNEXTEND:
		trans_signextend(f, s)
	case LT:
		trans_lt(f, s)
	case GT:
		trans_gt(f, s)
	case SLT:
		trans_slt(f, s)
	case SGT:
		trans_sgt(f, s)
	case EQ:
		trans_eq(f, s)
	case AND:
		trans_and(f, s)
	case OR:
		trans_or(f, s)
	case XOR:
		trans_xor(f, s)
	case BYTE:
		trans_byte(f, s)
	case SHL:
		trans_shl(f, s)
	case SHR:
		trans_shr(f, s)
	case SAR:
		trans_sar(f, s)
	case SHA3:
		trans_sha3(f, s)
	case MSTORE:
		trans_mstore(f, s)
	case MSTORE8:
		trans_mstore8(f, s)
	case SSTORE:
		trans_sstore(f, s)
	case JUMPI:
		trans_jumpi(f, s)
	case REVERT:
		trans_revert(f, s)
	case RETURN:
		trans_return(f, s)
	case LOG0:
		trans_log0(f, s)
	case CALLDATACOPY:
		trans_calldatacopy(f, s)
	case CODECOPY:
		trans_codecopy(f, s)
	case RETURNDATACOPY:
		trans_returndatacopy(f, s)
	case LOG1:
		trans_log1(f, s)
	case CREATE:
		trans_create(f, s)
	case EXTCODECOPY:
		trans_extcodecopy(f, s)
	case LOG2:
		trans_log2(f, s)
	case LOG3:
		trans_log3(f, s)
	case LOG4:
		trans_log4(f, s)
	case CALL:
		trans_call(f, s)
	case CALLCODE:
		trans_callcode(f, s)
	case DELEGATECALL:
		trans_delegatecall(f, s)
	case CREATE2:
		trans_create2(f, s)
	case STATICCALL:
		trans_staticcall(f, s)
	case CALLPRIVATE:
		trans_callprivate(f, s)
	case RETURNPRIVATE:
		trans_returnprivate(f, s)
	default:
		panic(fmt.Errorf("unknown operation %d", op))
	}
}

// IMPORTANT: Order of enum values is important
const (
	// 0 operands
	TAC_THROW = iota
	TAC_STOP
	TAC_ADDRESS
	TAC_ORIGIN
	TAC_CALLER
	TAC_CALLVALUE
	TAC_CALLDATASIZE
	TAC_CODESIZE
	TAC_GASPRICE
	TAC_RETURNDATASIZE
	TAC_COINBASE
	TAC_TIMESTAMP
	TAC_NUMBER
	TAC_DIFFICULTY
	TAC_GASLIMIT
	TAC_MSIZE
	TAC_GAS
	TAC_CHAINID
	TAC_SELFBALANCE
	TAC_BLOCK_FLAG
	TAC_PHI_END // Mark end of PHI

	// 1 operand
	TAC_ISZERO
	TAC_BALANCE
	TAC_CALLDATALOAD
	TAC_EXTCODESIZE
	TAC_EXTCODEHASH
	TAC_BLOCKHASH
	TAC_MLOAD
	TAC_SLOAD
	TAC_JUMP
	TAC_SELFDESTRUCT
	TAC_NOT
	TAC_PHI_START // Mark start of PHI
	TAC_CONST     // To update cycles of constants for PHI
	TAC_JUMP_VAR

	// 2 opearnds
	TAC_ADD
	TAC_MUL
	TAC_SUB
	TAC_DIV
	TAC_SDIV
	TAC_MOD
	TAC_SMOD
	TAC_ADDMOD
	TAC_MULMOD
	TAC_EXP
	TAC_SIGNEXTEND
	TAC_LT
	TAC_GT
	TAC_SLT
	TAC_SGT
	TAC_EQ
	TAC_AND
	TAC_OR
	TAC_XOR
	TAC_BYTE
	TAC_SHL
	TAC_SHR
	TAC_SAR
	TAC_SHA3
	TAC_MSTORE
	TAC_MSTORE8
	TAC_SSTORE
	TAC_JUMPI
	TAC_REVERT
	TAC_RETURN
	TAC_LOG0
	TAC_JUMPI_VAR

	// 3 operands
	TAC_CALLDATACOPY
	TAC_CODECOPY
	TAC_RETURNDATACOPY
	TAC_LOG1

	// 4 operands
	TAC_CREATE
	TAC_EXTCODECOPY
	TAC_LOG2

	// 5 operands
	TAC_LOG3
	TAC_CREATE2

	// 6 operands
	TAC_LOG4

	// 7 operands
	TAC_DELEGATECALL
	TAC_STATICCALL

	// 8 operands
	TAC_CALL
	TAC_CALLCODE

	// PHI N dest x1 x2 ... xN
	TAC_PHI // PHI dest e v, assign dest with value in v if incoming edge is e

	// CALLPRIVATE N x1 x2 ... xN F a1 a2 ... a_len(F.Args)
	TAC_CALLPRIVATE

	// RETURNPRIVATE N x1 x2 ... xN
	TAC_RETURNPRIVATE

	TAC_ILLPHI // Should not have illphi
)
