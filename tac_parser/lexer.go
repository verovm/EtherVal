package tac_parser

// Lexer scan the input into a list of tokens.

/**
 * TODO:
 * 1. A toString for tokentype (just for the sake of debugging)
 */

import (
	"fmt"
	"os"
	"strings"
	"unicode/utf8"
)

type TokenType int

type Token struct {
	typ  TokenType
	val  string
	pos  int
	line int
}

func (tok Token) String() string {
	str := ""
	switch tok.typ {
	case IDENT:
		str += "IDENT: "
	case HEX_NUM:
		str += "HEX: "
	}
	str = str + fmt.Sprintf("%s at %d:%d", tok.val, tok.line, tok.pos)
	return str
}

type stateFn func(*lexer) stateFn

type lexer struct {
	input      string     // input string
	start      int        // start pos of current token
	cur        int        // current pos on the input (not yet read)
	width      int        // current pos on the line
	prev_width int        // cache for backup
	line       int        // line scanning
	tokens     chan Token // token channel
}

var keywords = map[string]TokenType{
	"FUNCTION":       FUNCTION,
	"PRIVATE":        PRIVATE,
	"PUBLIC":         PUBLIC,
	"PREV":           PREV,
	"SUCC":           SUCC,
	"BEGIN":          BEGIN,
	"BLOCK":          BLOCK,
	"STOP":           STOP,
	"ADD":            ADD,
	"MUL":            MUL,
	"SUB":            SUB,
	"DIV":            DIV,
	"SDIV":           SDIV,
	"MOD":            MOD,
	"SMOD":           SMOD,
	"ADDMOD":         ADDMOD,
	"MULMOD":         MULMOD,
	"EXP":            EXP,
	"SIGNEXTEND":     SIGNEXTEND,
	"LT":             LT,
	"GT":             GT,
	"SLT":            SLT,
	"SGT":            SGT,
	"EQ":             EQ,
	"ISZERO":         ISZERO,
	"AND":            AND,
	"OR":             OR,
	"XOR":            XOR,
	"NOT":            NOT,
	"BYTE":           BYTE,
	"SHL":            SHL,
	"SHR":            SHR,
	"SAR":            SAR,
	"SHA3":           SHA3,
	"ADDRESS":        ADDRESS,
	"BALANCE":        BALANCE,
	"ORIGIN":         ORIGIN,
	"CALLER":         CALLER,
	"CALLVALUE":      CALLVALUE,
	"CALLDATALOAD":   CALLDATALOAD,
	"CALLDATASIZE":   CALLDATASIZE,
	"CALLDATACOPY":   CALLDATACOPY,
	"CODESIZE":       CODESIZE,
	"CODECOPY":       CODECOPY,
	"GASPRICE":       GASPRICE,
	"EXTCODESIZE":    EXTCODESIZE,
	"EXTCODECOPY":    EXTCODECOPY,
	"RETURNDATASIZE": RETURNDATASIZE,
	"RETURNDATACOPY": RETURNDATACOPY,
	"EXTCODEHASH":    EXTCODEHASH,
	"BLOCKHASH":      BLOCKHASH,
	"COINBASE":       COINBASE,
	"TIMESTAMP":      TIMESTAMP,
	"NUMBER":         NUMBER,
	"DIFFICULTY":     DIFFICULTY,
	"GASLIMIT":       GASLIMIT,
	"MLOAD":          MLOAD,
	"MSTORE":         MSTORE,
	"MSTORE8":        MSTORE8,
	"SLOAD":          SLOAD,
	"SSTORE":         SSTORE,
	"JUMP":           JUMP,
	"JUMPI":          JUMPI,
	"MSIZE":          MSIZE,
	"GAS":            GAS,
	"CHAINID":        CHAINID,
	"SELFBALANCE":    SELFBALANCE,
	"LOG0":           LOG0,
	"LOG1":           LOG1,
	"LOG2":           LOG2,
	"LOG3":           LOG3,
	"LOG4":           LOG4,
	"CREATE":         CREATE,
	"CALL":           CALL,
	"CALLCODE":       CALLCODE,
	"RETURN":         RETURN,
	"DELEGATECALL":   DELEGATECALL,
	"CREATE2":        CREATE2,
	"STATICCALL":     STATICCALL,
	"REVERT":         REVERT,
	"SELFDESTRUCT":   SELFDESTRUCT,
	"CONST":          CONST,
	"THROW":          THROW,
	"PHI":            PHI,
	"RETURNPRIVATE":  RETURNPRIVATE,
	"CALLPRIVATE":    CALLPRIVATE,
	"ILL_PHI":        ILLPHI,
	"ILLPHI1":        ILLPHI1,
	"ILLPHI2":        ILLPHI2,
	"ILLPHI3":        ILLPHI3,
	"ILLPHI4":        ILLPHI4,
	"ILLPHI5":        ILLPHI5,
}

// next returns the next char in the input.
func (l *lexer) next() rune {
	if l.cur >= len(l.input) {
		return EOF
	}
	r, _ := utf8.DecodeRuneInString(l.input[l.cur:])
	l.cur++
	l.prev_width = l.width
	if r == '\n' {
		l.line++
		l.width = 0
	} else {
		l.width++
	}
	return r
}

// peek returns the next char, without consuming
func (l *lexer) peek() rune {
	r := l.next()
	if r != EOF {
		l.backup()
	}
	return r
}

func (l *lexer) backup() {
	l.cur--
	l.width = l.prev_width
	if l.input[l.cur] == '\n' {
		l.line--
	}
}

func (l *lexer) content() string {
	return l.input[l.start:l.cur]
}

// drain scan the whole input and return an array
// Xihu: I wanted to do a fancy channel implementation.
// But later I found the nature of channel makes it hard to do peek action in the parser
// As you cannot put token back into channel.. (And some kind of local caching is required)
func (l *lexer) drain() []Token {
	var tokens []Token
	for x := range l.tokens {
		tokens = append(tokens, x)
	}
	return tokens
}

// emit passes an item back to the client.
func (l *lexer) emit(t TokenType) {
	l.tokens <- Token{t, l.input[l.start:l.cur], l.width, l.line}
	l.start = l.cur
}

func (l *lexer) emitIgnore() {
	l.start = l.cur
}

// accept consumes the next rune if it's from the valid set.
func (l *lexer) accept(valid string) bool {
	if strings.ContainsRune(valid, l.next()) {
		return true
	}
	l.backup()
	return false
}

// acceptRun consumes a run of runes from the valid set.
func (l *lexer) acceptRun(valid string) {
	for strings.ContainsRune(valid, l.next()) {
	}
	l.backup()
}

var delimiters = map[rune]TokenType{
	'{': LEFT_BRAC,
	'}': RIGHT_BRAC,
	'[': LEFT_SQUARE_BRAC,
	']': RIGHT_SQUARE_BRAC,
	',': COMMA,
	':': SEMI_COLON,
	'(': LEFT_PAREN,
	')': RIGHT_PAREN,
	'=': ASSIGN,
}

// isSpace reports whether r is a space character.
func isSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\r' || r == '\n'
}

func isDelimiter(r rune) bool {
	return r == '{' || r == '}' || r == '(' || r == ')' || r == ':' || r == ',' || r == '[' || r == ']' || r == '=' || r == '-' || isSpace(r)
}

func isHex(str string) bool {
	if !strings.HasPrefix(str, "0X") {
		return false
	}

	valid := "1234567890ABCDEF"
	for _, char := range str[2:] {
		if !strings.ContainsRune(valid, char) {
			return false
		}
	}

	return true
}

// scan untils meets a delimiter
func (l *lexer) scanUntilDelimeter() bool {
	for l.peek() != EOF && !isDelimiter(l.peek()) {
		l.next()
	}

	// emit the scanned token
	str := strings.ToUpper(l.content())

	if tok, ok := keywords[str]; ok {
		// a keyword
		l.emit(tok)
	} else if isHex(str) {
		// a hex
		l.emit(HEX_NUM)
	} else if len(str) > 0 {
		// a IDENT
		l.emit(IDENT)
	} /* else: empty content, omit */

	// scan and emit the delimiter
	r := l.next()

	// special case, omit horizontal line
	if r == '=' && l.peek() == '=' {
		for l.peek() == '=' {
			l.next()
		}
		l.emitIgnore()
		return true
	}

	// ->
	if r == '-' && l.peek() == '>' {
		l.next()
		l.emit(RIGHT_ARROW)
		return true
	}

	if tok, ok := delimiters[r]; ok {
		l.emit(tok)
	} else if r == EOF {
		return false
	} else if isSpace(r) {
		l.emitIgnore()
	} else {
		// error
		fmt.Fprintf(os.Stderr, "Cannot recognize delimiter %s", string(r))
		return false
	}
	return true
}

// lex creates a new scanner for the input string.
func Lex(input string) *lexer {
	l := &lexer{
		input:  input,
		start:  0,
		cur:    0,
		line:   1,
		tokens: make(chan Token),
	}
	go l.run()
	return l
}

// run runs the state machine for the lexer.
func (l *lexer) run() {
	for l.scanUntilDelimeter() == true {
	}
	close(l.tokens)
}

// Enum for lexer
const (
	/* Order Important */
	EOF               = -1
	HEX_NUM TokenType = iota
	IDENT

	/** Delimiters **/
	COMMA
	SEMI_COLON
	RIGHT_ARROW
	LEFT_BRAC
	RIGHT_BRAC
	LEFT_PAREN
	RIGHT_PAREN
	LEFT_SQUARE_BRAC
	RIGHT_SQUARE_BRAC
	ASSIGN

	/** misc keyword */
	FUNCTION
	PRIVATE
	PUBLIC
	PREV
	SUCC
	BEGIN
	BLOCK

	/** op keywords **/

	/** nullary */
	CONST
	THROW
	STOP
	ADDRESS
	ORIGIN
	CALLER
	CALLVALUE
	CALLDATASIZE
	CODESIZE
	GASPRICE
	RETURNDATASIZE
	COINBASE
	TIMESTAMP
	NUMBER
	DIFFICULTY
	GASLIMIT
	MSIZE
	GAS
	CHAINID
	SELFBALANCE

	/** unary */
	ISZERO
	BALANCE
	CALLDATALOAD
	EXTCODESIZE
	EXTCODEHASH
	BLOCKHASH
	MLOAD
	SLOAD
	JUMP
	SELFDESTRUCT
	NOT

	/** Binary */
	ADD
	MUL
	SUB
	DIV
	SDIV
	MOD
	SMOD
	ADDMOD
	MULMOD
	EXP
	SIGNEXTEND
	LT
	GT
	SLT
	SGT
	EQ
	AND
	OR
	XOR
	BYTE
	SHL
	SHR
	SAR
	SHA3
	MSTORE
	MSTORE8
	SSTORE
	JUMPI
	REVERT
	RETURN
	LOG0

	/** Ternary operator */
	CALLDATACOPY
	CODECOPY
	RETURNDATACOPY
	LOG1
	CREATE

	/** n-ary operator*/
	PHI
	EXTCODECOPY
	LOG2
	LOG3
	LOG4
	CALL
	CALLCODE
	DELEGATECALL
	CREATE2
	STATICCALL
	CALLPRIVATE
	RETURNPRIVATE
	ILLPHI
	ILLPHI1 // PHI does not define exactly one variable.
	ILLPHI2 // PHI uses variables undefined in the function.
	ILLPHI3 // PHI is ambiguous - variables defined in the same block.
	ILLPHI4 // PHI is not placed at a join block. (#predecessors < 2)
	ILLPHI5 // PHI is not placed on any of the dominance frontiers.
)
