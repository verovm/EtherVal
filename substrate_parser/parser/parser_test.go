package parser

import (
   "os"
   "flag"
   "testing"
   "fmt"
)

var fn = flag.String("fn", "filename", "substrate filename for parser")
func setup() *Lexer {
   file, err := os.Open(*fn)
   if err != nil {
      panic(err)
   }
   return NewLexer(file, false)

}

func TestLexer(t *testing.T) {
   var item ItemSymType
   l := setup()
   for {
      l.Lex(&item)
      if item.typ == eof {
	 break
      }
      l.debug_message(fmt.Sprintf("%4d:%4d  %5d  %s\n", item.pos.line, item.pos.column, item.typ, item.val))
   }
}
func TestParser(t *testing.T) {
   l := setup()
   ItemParse(l)
}
