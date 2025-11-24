package tac_parser

// Parser parse the input into a CFG-like IR, called AstProgram
// CFG IR is not directly runnable, but easy to be handled by our transformer

import (
	"fmt"
	"strings"

	"github.com/holiman/uint256"
)

// AstVariable is either a variable or a constant
// Xihu: Because I don't know if we need the constant information or not later for execution
// For now I'll just keep both
type AstVariable struct {
	name string
	val  string
	typ  VarType
}

type VarType int

const (
	CONSTANT VarType = iota
	VARIABLE
)

type ConstSymPair struct {
	Index int
	Value uint256.Int
}

// AstStatement is a single statement
type AstStatement struct {
	name      string
	address   string
	operation TokenType
	args      []AstVariable
	// phi_mapping  []Pair // phi_mapping is a pair of (string, AstVariable)
	num_assignee int
	block_name   string
}

// AstBlock is a basic control block representation
type AstBlock struct {
	name        string
	address     string
	predecessor []string
	successor   []string
	statements  []AstStatement
}

// AstFunction is a function representation
type AstFunction struct {
	name      string
	args      []string
	blocks    []AstBlock
	is_public bool
}

func (this *AstFunction) get_block(name string) *AstBlock {
	for idx, block := range this.blocks {
		if block.name == name {
			return &this.blocks[idx]
		}
	}
	return nil
}

// AstProgram
type AstProgram struct {
	functions []AstFunction
}

type Parser struct {
	tokens []Token
	cur    int // current cursor (not yet read)
}

func ParseProgram(input string) *AstProgram {
	lexer := Lex(input)
	parser := &Parser{
		tokens: lexer.drain(),
		cur:    0,
	}
	return parser.parse_program()
}

func (p *Parser) next() (Token, bool) {
	if p.cur >= len(p.tokens) {
		return Token{EOF, "", 0, 0}, false
	}
	tok := p.tokens[p.cur]
	p.cur++
	return tok, true
}

// peek peek the ith item
func (p *Parser) peek(i int) (Token, bool) {
	if p.cur+i >= len(p.tokens) {
		return Token{EOF, "", 0, 0}, false
	}
	return p.tokens[p.cur+i], true
}

func (p *Parser) expect(expected []TokenType) Token {
	next, ok := p.next()
	if !ok {
		panic("No more token")
	}
	for _, ele := range expected {
		if next.typ == ele {
			return next
		}
	}
	panic(fmt.Sprintf("Unexpected %s", next.String()))
}

func (p *Parser) parse_program() *AstProgram {
	program := &AstProgram{}
	for next, ok := p.peek(0); ok && next.typ == FUNCTION; next, ok = p.peek(0) {
		program.functions = append(program.functions, p.parse_function())
	}
	// sanity check, token is exhausted
	if next, ok := p.peek(0); ok {
		// throw error
		panic(fmt.Sprintf("Program is not exhausted! Got %s", next.String()))
	}
	return program
}

// parse_list parses a list of args of type 'arg_types', enclosed by 'start' and 'end'
// seperated by comma
func (p *Parser) parse_list(start, end TokenType, arg_types []TokenType) []string {
	p.expect([]TokenType{start})
	var argument_list []string
	for peek_next, ok := p.peek(0); ok && peek_next.typ != end; peek_next, ok = p.peek(0) {
		next := p.expect(arg_types)
		argument_list = append(argument_list, next.val)
		if peek_next, ok = p.peek(0); ok && peek_next.typ != end {
			p.expect([]TokenType{COMMA})
		}
	}
	p.expect([]TokenType{end})
	return argument_list
}

func (p *Parser) parse_function() AstFunction {
	function := AstFunction{
		is_public: false,
	}
	p.expect([]TokenType{FUNCTION})

	// hack fix: some keywords can also be a function name
	tokenTypes := []TokenType{IDENT, HEX_NUM}
	for _, tokenType := range keywords {
		tokenTypes = append(tokenTypes, tokenType)
	}
	function_name := p.expect(tokenTypes)
	function.name = function_name.val
	// if function name does not start with 0x, it is in db and has an extra signature
	if !strings.HasPrefix(function.name, "0x") && function.name != "__function_selector__" {
		// we need to include the signature as part of the function name
		// which is enclosed by a parenthesis
		p.expect([]TokenType{LEFT_PAREN})
		function.name += "("
		for parenthesisLevel := 1; parenthesisLevel != 0; {
			next, _ := p.next()
			if next.typ == LEFT_PAREN {
				parenthesisLevel++
			} else if next.typ == RIGHT_PAREN {
				parenthesisLevel--
			}
			function.name += next.val
		}
	}
	function.args = p.parse_list(LEFT_PAREN, RIGHT_PAREN, []TokenType{IDENT})
	if tok := p.expect([]TokenType{PUBLIC, PRIVATE}); tok.typ == PUBLIC {
		function.is_public = true
	} else {
		function.is_public = false
	}
	p.expect([]TokenType{LEFT_BRAC})
	for next, _ := p.peek(0); next.typ != RIGHT_BRAC; next, _ = p.peek(0) {
		function.blocks = append(function.blocks, p.parse_block())
	}
	p.expect([]TokenType{RIGHT_BRAC})
	return function
}

func (p *Parser) parse_block() AstBlock {
	var block AstBlock
	p.expect([]TokenType{BEGIN})
	p.expect([]TokenType{BLOCK})
	block.name = p.expect([]TokenType{HEX_NUM, IDENT}).val
	block.address = BaseAddr(block.name)

	// prev=[..] ,
	p.expect([]TokenType{PREV})
	p.expect([]TokenType{ASSIGN})
	block.predecessor = p.parse_list(LEFT_SQUARE_BRAC, RIGHT_SQUARE_BRAC, []TokenType{HEX_NUM, IDENT})

	p.expect([]TokenType{COMMA})

	// succ=[..]
	p.expect([]TokenType{SUCC})
	p.expect([]TokenType{ASSIGN})
	block.successor = p.parse_list(LEFT_SQUARE_BRAC, RIGHT_SQUARE_BRAC, []TokenType{HEX_NUM, IDENT})

	for next, _ := p.peek(0); next.typ != BEGIN && next.typ != RIGHT_BRAC; next, _ = p.peek(0) {
		statement := p.parse_statement()
		statement.block_name = block.name
		block.statements = append(block.statements, statement)
	}
	return block
}

func (p *Parser) parse_statement() AstStatement {
	statement := AstStatement{
		num_assignee: 0,
	}
	// 0xabc :
	// TODO: This should be BaseAddr, but so far, statement.address is untouched
	// during transformer and evaluation
	statement.name = p.expect([]TokenType{IDENT, HEX_NUM}).val
	statement.address = BaseAddr(statement.name)
	p.expect([]TokenType{SEMI_COLON})

	next, _ := p.peek(0)
	var num_args int
	if next.typ == IDENT {
		// There can be more than one assignee for CALLPRIVATE
		for peek_next, ok := p.peek(0); ok && peek_next.typ != ASSIGN; peek_next, ok = p.peek(0) {
			if statement.num_assignee > 0 {
				p.expect([]TokenType{COMMA})
			}
			statement.args = append(statement.args, p.parse_variable())
			statement.num_assignee++
		}
		p.expect([]TokenType{ASSIGN})
		statement.operation, num_args = p.parse_operation()
		if statement.operation == PHI {
			with_brace := false
			if peek_next, ok := p.peek(0); ok && peek_next.typ == LEFT_BRAC {
				p.expect([]TokenType{LEFT_BRAC})
				with_brace = true
			}
			statement.args = append(statement.args, p.parse_variable())
			for {
				if peek_next, ok := p.peek(0); ok && peek_next.typ == COMMA {
					p.expect([]TokenType{COMMA})
					statement.args = append(statement.args, p.parse_variable())
				} else {
					break
				}
			}
			if with_brace {
				p.expect([]TokenType{RIGHT_BRAC})
			}
			return statement
		}
	} else {
		statement.operation, num_args = p.parse_operation()
	}
	if num_args != 0 {
		statement.args = append(statement.args, p.parse_variable())
		for {
			if peek_next, ok := p.peek(0); ok && peek_next.typ == COMMA {
				p.expect([]TokenType{COMMA})
				statement.args = append(statement.args, p.parse_variable())
			} else {
				break
			}
		}
	}
	return statement
}

func (p *Parser) parse_variable() AstVariable {
	var variable AstVariable
	variable.name = p.expect([]TokenType{IDENT}).val

	if next, _ := p.peek(0); next.typ == LEFT_PAREN { // a constant variable
		p.expect([]TokenType{LEFT_PAREN})
		variable.val = p.expect([]TokenType{HEX_NUM}).val
		p.expect([]TokenType{RIGHT_PAREN})
		variable.typ = CONSTANT
		return variable
	}

	variable.typ = VARIABLE
	return variable
}

func (p *Parser) parse_operation() (TokenType, int) {
	tok, _ := p.next()
	if tok.typ < CONST {
		panic(fmt.Sprintf("Unknown operation %s", tok.String()))
	}
	if tok.typ >= CONST && tok.typ <= SELFBALANCE {
		return tok.typ, 0
	}
	return tok.typ, -1
}

// BaseAddr computes the base address from block names (base address + suffix)
// When addresses overlap, Gigahorse rewrite as {address}+{headblock} to differentiate.
// The reason (I guess) is although within the function there is no conflict, Datalog cannot
// tell the differences when doing analysis
//
// However, this causes some inconsistences between constant values and block addresses.
// As operations like jump still uses {address} not {address}+{headblock}
//
// This function simply extract the {address} part
func BaseAddr(name string) string {
	if strings.Index(name, "0x") != 0 {
		panic("BaseAddr assumes address name string starts with 0x")
	}

	name = name[2:]
	name = strings.Split(name, "B")[0]
	name = strings.Split(name, "0x")[0]
	name = strings.Split(name, "_")[0]
	name = "0x" + name

	return name
}

func EqualBaseAddr(n1 string, n2 string) bool {
	return BaseAddr(n1) == BaseAddr(n2)
}
