package main

import (
   "os"
   "strings"
   "fmt"

   "github.com/seonghojj/substrate_parser/parser"
   "github.com/seonghojj/substrate_parser/ast"
)

func visitAST(l *parser.Lexer, w ast.Worker) {
   ret := parser.ItemParse(l)
   if ret != 0 {
      //println(l.GetPos())
      panic(l.GetError() + " in ItemParse()")
   }
   ast.Act(w, l.Root)
}

func compareTokenSeq(file *os.File) bool {
   origToken := parser.NewLexer(file, false).GetTokenSequence()

   file.Seek(0, 0)
   u := ast.Unparser{}
   visitAST(parser.NewLexer(file, false), &u)
   unparsedToken := parser.NewLexer(strings.NewReader(u.GetUnparsed()), false).GetTokenSequence()

   if false {
      //fmt.Printf(unparsedToken)
      o,_ := os.Create("original.txt"); defer o.Close()
      u,_ := os.Create("unparsed.txt"); defer u.Close()
      fmt.Fprintf(o, origToken)
      fmt.Fprintf(u, unparsedToken)
   }

   return origToken == unparsedToken
}

func main() {
   file, err := os.Open(os.Args[1])
   if err != nil {
      panic(err)
   }
   defer file.Close()

   if true {
      equal := compareTokenSeq(file)
      if !equal {
	 panic("Different token sequences")
      }
   } else {
      p := ast.Printer{}
      visitAST(parser.NewLexer(file, false), &p)
      fmt.Printf("%s", p.GetTree())
   }
}
