//go:generate goyacc -p Item -o parser.go parser.go.y
package parser

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/ethereum/go-ethereum/substrate_parser/ast"
)

var buffer, prevBuffer ItemSymType

type position struct {
   line int
   column int
}

func (i *ItemSymType) String() string {
   return fmt.Sprintf("%q", i.val)
}

func isDigit(c rune) bool {
   return unicode.IsDigit(c)
}
func isHexDigit(c rune) bool {
   if isDigit(c) || ('a' <= c && c <= 'f') || ('A' <= c && c <= 'F') {
      return true
   }
   return false
}
func isCharacter(c rune) bool {
   if c == '$' || c == '_' || ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z') || c == '@' {
      return true
   }
   return false
}
func isLegalSymbol(c rune) bool {
   return strings.Contains(":;.,()[]{}~!/%<>&^|=+-*/?", string(c))
}
func isUintType(str string) bool {
   if strings.HasPrefix(str, "uint") {
      for _, c := range str[4:] {
         if !isDigit(c) {
            return false
         }
      }
      return true
   }
   return false
}

type Lexer struct {
   reader *bufio.Reader
   err error
   pos position
   debug bool
   Root *ast.SubstrateNode
}
func NewLexer(reader io.Reader, d bool) *Lexer {
   return &Lexer{
      reader:   bufio.NewReader(reader),
      pos:      position{line: 1, column: 0},
      debug: 	d,
   }
}

func (l *Lexer) GetPos() (int, int) {
   return l.pos.line, l.pos.column
}

func (l *Lexer) next() rune {
   r, _, err := l.reader.ReadRune()
   if err != nil {
      if err == io.EOF {
         return eof
      }
      l.Error(err.Error())
   }
   l.pos.column++
   return r
}

func (l *Lexer) rewind() {
   if err := l.reader.UnreadRune(); err != nil {
      l.Error(err.Error())
   }
   l.pos.column--
}

func (l *Lexer) peek() rune {
   r, err := l.reader.Peek(1)
   if err != nil {
      if err == io.EOF {
	 return eof
      }
      l.Error(err.Error())
   }
   return rune(r[0])
}

func (l *Lexer) peekN(n int) string {
   r, _ := l.reader.Peek(n)
   return string(r)
}

func (l *Lexer) debug_message(msg string) {
   if l.debug {
      fmt.Printf(msg)
   }
}

func (l *Lexer) Error(err string) {
   l.err = errors.New(err)
}
func (l *Lexer) GetError() string {
   return l.err.Error()
}
func errMsg (err string, pos position, val string, r rune) string{
   return fmt.Sprintf("%s at %d,%d\t%s, %c\n", err, pos.line, pos.column, val, r)
}

func (l *Lexer) Lex(curr *ItemSymType) int {
   for{
      r := l.peek()
      if r == eof {
         curr.typ = -1
         curr.val = ""
         curr.pos = l.pos
         return eof
      }
      switch {
      case r == '\n':
         l.pos.line++
         l.pos.column = 0
         l.next()
         continue
      case r == '"' || r == '\'':
         curr.pos = l.pos
         curr.typ, curr.val = l.lexString()
         buffer = *curr
      case isLegalSymbol(r):
         curr.pos = l.pos
         curr.typ, curr.val = l.lexSymbol()
         if curr.typ == COMMENT {
            if curr.pos.column < 2 || curr.val[0:2] == "/*" {
               continue
            }
            buffer = *curr
         }
         if curr.val == "?" {
            buffer = *curr
         }
      case isDigit(r):
         curr.pos = l.pos
         if l.peekN(2) == "0x"{
            curr.typ, curr.val = l.lexHex()
         } else {
            curr.typ = NUM
            curr.val = l.lexNUM()
         }
         buffer = *curr
      case isCharacter(r):
         curr.pos = l.pos
         curr.val = l.lexID()
         if curr.val == "True" || curr.val == "False" {
            curr.typ = BOOLLITERAL
         } else if isUintType(curr.val) {
            curr.typ = UINT
         } else if contains(intType, curr.val) {
            curr.typ = INT
         } else if contains(byteType, curr.val) {
            curr.typ = BYTE
         } else if strings.HasPrefix(curr.val, "struct") {
            curr.typ = STRUCT
         } else if typ, ok := keyword[curr.val]; ok {
            //special occasion where keyword is used for ID
            if curr.val == "emit" && l.peek() == '(' {
               curr.typ = ID
            } else if curr.val == "array" && l.peek() != '[' {
               curr.typ = ID
            } else {
               curr.typ = typ
            }
         } else {
            curr.typ = ID
         }
         if !(curr.val == "storage") &&			//Avoid keyword "storage" is taken as ID
            !(curr.val == "len" && l.peek() == ' ') { 	//Handle rare occasion where "len" is used as ID
            prevBuffer = buffer
            buffer = *curr
         }
      case unicode.IsSpace(r):
         l.next()
         continue
      default:
         l.next()
      }

      l.debug_message(fmt.Sprintf("%d,%d\t%d:%s\n", (*curr).pos.line, curr.pos.column, curr.typ, curr.val))
      return curr.typ
   }
}
func (l *Lexer) lexSymbol() (typ int, literal string) {
   r := l.next()
   literal = literal + string(r)

   switch l.peek() {
   case '/':
      if r == '/' {
         literal = literal + string(l.next())
      }
   case '=':
      switch r {
      case '+', '-', '*', '/', '<', '>', '=', '!':
         literal = literal + string(l.next())
      }
   case '*':
      if r == '*' {
         literal = literal + string(l.next())
      } else if r == '/' {
         literal = literal + string(l.next())
      }
   case '<':
      if r == '<' {
         literal = literal + string(l.next())
      }
   case '>':
      if r == '>' || r == '=' {
         literal = literal + string(l.next())
      }
   case '&':
      if r == '&' {
         literal = literal + string(l.next())
      }
   case '|':
      if r == '|' {
         literal = literal + string(l.next())
      }
   }

   var ok bool
   if typ, ok = operator[literal]; !ok {
      typ = int(r)
   }
   if typ == COMMENT {
      for{
         r := l.next()
         if r == eof || r == '\n' {
            l.rewind()
            break
         } else if r == '*' && l.next() == '/' {
            literal = literal + "*/"
            break
         } else {
            literal = literal + string(r)
         }
      }
   }

   return typ, literal
}
func (l *Lexer) lexString() (typ int, literal string) {
   typ = MESSAGE
   open := l.next()
   literal = literal + string(open)
   var prev rune
   for {
      r := l.next()
      if r == eof {
         l.Error(errMsg("unterminated quote", l.pos, literal, r))
      }

      literal = literal + string(r)
      if r == open && prev != '\\' {
         return
      }
      prev = r
   }
}
func (l *Lexer) lexNUM() (literal string) {
   for {
      r := l.next()
      if r == eof {
         return
      }

      if isDigit(r) {
         literal = literal + string(r)
      } else if unicode.IsSpace(r) || !isCharacter(r) {
         l.rewind()
         return
      } else {
         l.Error(errMsg("unexpected character", l.pos, literal, r))
      }
   }
}
func (l *Lexer) lexHex() (typ int, literal string) {
   typ = HEX
   l.next()
   l.next()
   literal = "0x"
   var r, prev rune
   for {                //TODO:make it handle only [0-9a-f]+, not [0-9a-f]*
      prev = r
      r = l.peek()
      if r == eof {
         return
      }

      if isHexDigit(r) {
         literal = literal + string(l.next())
      } else if (prev == '0' && r == 'x') {
         typ = GOTOADDRESS
         literal = literal + string(l.next())
      } else {
         return
      }
   }
}
func (l *Lexer) lexID() (literal string) {
   for {
      r := l.peek()
      if r == eof {
         return
      }

      if isCharacter(r) || isDigit(r) {
         literal = literal + string(l.next())
      } else if (r == ' ' && literal == "else") {
         if l.peekN(3) == " if" {
            literal = literal + string(l.next())
            literal = literal + string(l.next())
            literal = literal + string(l.next())
         }
         break
      } else if (r == '.' && (literal == "msg" || literal == "code")) {
         literal = literal + string(l.next())
      } else {
         break
      }
   }
   return
}

func (l *Lexer) GetTokenSequence() string {
   var item ItemSymType
   seq := ""
   for {
      l.Lex(&item)
      if item.typ == eof {
	 break
      }
      seq += item.val + "\n"
   }
   return seq
}

const eof = -1
var operator = map[string]int {
   "//":        COMMENT,
   "/*":        COMMENT,
   "+":         ADD,
   "-":         SUB,
   "~":         NEG,
   "!":         NOT,

   "**":        EXP,

   "*":         MUL,
   "/":         DIV,
   "%":         MOD,

   "<<":        SL,
   ">>":        SR,

   "&":         AND,
   "^":         XOR,
   "|":         OR,

   "<":         LT,
   ">":         GT,
   "<=":        LE,
   ">=":        GE,

   "==":        EQ,
   "!=":        NEQ,

   "&&":        LOGICALAND,

   "||":        LOGICALOR,

   "=":         ASSIGN,
   "+=":        ASSIGNADD,
   "-=":        ASSIGNSUB,
   "*=":        ASSIGNMUL,
   "/=":        ASSIGNDIV,

   "=>":        ARROW,
}

func contains(types []string, str string) bool {
   for _, v := range types {
      if v == str {
         return true
      }
   }
   return false
}
var uintType = []string {
   "uint", "uint8", "uint16", "uint24", "uint32", "uint40", "uint48", "uint56", "uint64",
   "uint72", "uint80", "uint88", "uint96", "uint104", "uint112", "uint120", "uint128",
   "uint136", "uint144", "uint152", "uint160", "uint168", "uint176", "uint184", "uint192",
   "uint200", "uint208", "uint216", "uint224", "uint232", "uint240", "uint248", "uint256", "uint299",
}
var intType = []string {
   "int", "int8", "int16", "int24", "int32", "int40", "int48", "int56", "int64",
   "int72", "int80", "int88", "int96", "int104", "int112", "int120", "int128",
   "int136", "int144", "int152", "int160", "int168", "int176", "int184", "int192",
   "int200", "int208", "int216", "int224", "int232", "int240", "int248", "int256",
}
var byteType = []string {
   "bytes", "bytes1", "bytes2", "bytes3", "bytes4", "bytes5", "bytes6", "bytes7",
   "bytes8", "bytes9", "bytes10", "bytes11", "bytes12", "bytes13", "bytes14", "bytes15",
   "bytes16", "bytes17", "bytes18", "bytes19", "bytes20", "bytes21", "bytes22", "bytes23",
   "bytes24", "bytes25", "bytes26", "bytes27", "bytes28", "bytes29", "bytes30", "bytes31", "bytes32",
}

var keyword = map[string]int {
   "string":            STRING,
   "array":             ARRAY,
   "address":           ADDRESS,
   "bool":              BOOL,
   "storage":           STORAGET,
   "function":          FUNCTION,
   "MEM":               MEM,
   "STORAGE":           STORAGE,
   "if":                IF,
   "else if":           ELSEIF,
   "else":              ELSE,
   "emit":              EMIT,
   "while":             WHILE,
   "do":                DO,
   "break":             BREAK,
   "throw":             THROW,
   "continue":          CONTINUE,
   "return":            RETURN,
   "require":           REQUIRE,
   "goto":              GOTO,
   "msg.gas":           MSGGAS,
   "msg.value":         MSGVAL,
   "msg.data":          MSGDATA,
   "msg.sender":        MSGSENDER,
   "this":              THIS,
   "public":            PUBLIC,
   "private":           PRIVATE,
   "payable":           PAYABLE,
   "nonPayable":        NONPAYABLE,
   "new":               NEW,
   "mapping":           MAPPING,
}
//intrinsics
//   CREATE
//   CREATE2
//   CALLDATACOPY
//   RETURNDATASIZE
//   EXTCODECOPY
//   EXTCODEHASH
//   block
