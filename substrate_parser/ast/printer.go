package ast

import (
    "fmt"
    "strings"
)

var buffer string

func (p *Printer) writeBuffer(s string) {
    buffer += s
}
func (p *Printer) write(s string, pos Pos) {
    for i := 0; i < Depth*indent; i++ {
        if i%indent == 0 {
            p.tree += fmt.Sprintf("|")
        } else {
            p.tree += fmt.Sprintf(" ")
        }
    }
    if buffer != "" {
        p.tree += fmt.Sprintf("   %s\n", buffer)
        buffer = ""
        for i := 0; i < Depth*indent; i++ {
            if i%indent == 0 {
                p.tree += fmt.Sprintf("|")
            } else {
                p.tree += fmt.Sprintf(" ")
            }
        }
    }
    if pos.line == 0 {
        p.tree += fmt.Sprintf(" %s\n", s)
    } else {
        p.tree += fmt.Sprintf("%s (%d,%d)\n", s, pos.line, pos.column)
    }
}

type Printer struct {
    tree string
}

func (p *Printer) GetTree() string {
    return p.tree
}

func (p *Printer) printBasicType(n *BasicTypeNode) (str string) {
    return "type '" + n.Val + "'"
}
func (p *Printer) printArrayType(n *ArrayTypeNode) (str string, dep bool) {
    dep = true
    typ, s := n.Typ, n.IsStorage
    Act(p, typ)
    str = "[]"
    if s {
        str += " storage"
    }
    return
}
func (p *Printer) printSubexpr(n *SubexprNode) (str string) {
    if n.IsUnary {
        if n.Op != "" {
            str = "UnaryExpr"
        }
        return
    }
    switch n.Op {
    case "=":
        str = "AssignExpr"
    case "||", "&&":
        str = "LogicalExpr"
    case "==", "!=":
        str = "EqualityExpr"
    case "<", ">", "<=", ">=":
        str = "CompareExpr"
    case "|", "^", "&":
        str = "BitwiseExpr"
    case "<<", ">>":
        str = "ShiftExpr"
    case "+", "-", "*", "/", "%", "**":
        str = "ArithmeticExpr"
    }
    return
}
func (p *Printer) printLvalue(n *LvalueNode) (str string) {
    switch n.LvalueType {
    case TypeAlloc:
        str = "LvalueAllocation"
    case TypeCall:
        str = "LvalueCall"
    case TypeAccess:
        str = "LvalueMemberAccess"
    case TypePrimary:
        str = "LvaluePrimary"
    case TypeCast:
        str = "LvalueCastExpr"
    case TypeExpr:
        str = "LvalueExpression"
    case TypeType:
        str = "LvalueTypeName"
    default:
        panic("????")
    }
    return
}
func (p *Printer) printPrimary(n *PrimaryNode) (str string) {
    typ, val := n.Typ, n.Val
    switch typ {
    case TypeMESSAGE:
        str = "Message " + val
    case TypeGOTOADDRESS:
        str = "GOTOADDRESS '" + val + "'"
    case TypeID:
        str = "ID '" + val + "'"
    case TypeHEX:
        str = "HEX '" + val + "'"
    case TypeNUM:
        str = "NUM '" + val + "'"
    default:
        str = "'" + val + "'"
    }
    return
}
func (p *Printer) printToken(n *TokenNode) (str string) {
    return n.Val
}

func (p *Printer) printNode(n AstNode) {
    var dep bool
    typ := strings.Split(strings.Split(fmt.Sprintf("%T", n), ".")[1], "Node")[0]
    if typ == "Primary" {
        typ = p.printPrimary(n.(*PrimaryNode))
    } else if typ == "BasicType" {
        typ = p.printBasicType(n.(*BasicTypeNode))
    } else if typ == "ArrayType" {
        typ, dep = p.printArrayType(n.(*ArrayTypeNode))
    } else if typ == "Subexpr" {
        typ = p.printSubexpr(n.(*SubexprNode))
    } else if typ == "Lvalue" {
        typ = p.printLvalue(n.(*LvalueNode))
    } else if typ == "Token" {
        typ = p.printToken(n.(*TokenNode))
    }

    if typ != "" {
        if !dep {
            p.write(typ, n.Pos())
        } else {
            p.writeBuffer(typ)
        }
    }
}

func (p *Printer) ActBaseNode(n *BaseNode) (v Value) {
    panic("Act on BaseNode")
    return
}
func (p *Printer) ActSubstrate(n *SubstrateNode) (v Value) {
    p.tree = ""
    Depth = 0
    p.printNode(n)
    Depth++
    if n.SList != nil {
        Act(p, n.SList)
    }
    if n.EList != nil {
        Act(p, n.EList)
    }
    if n.FList != nil {
        Act(p, n.FList)
    }
    Depth--
    return
}
func (p *Printer) ActStorageDeclList(l *StorageDeclList) (v Value) {
    for _, n := range l.List {
        Act(p, n)
    }
    return
}
func (p *Printer) ActStorageDecl(n *StorageDeclNode) (v Value) {
    p.printNode(n)
    Depth++
    Act(p, n.Typ)
    Act(p, n.Id)
    p.writeBuffer(string(n.Comment))
    Depth--
    return
}
func (p *Printer) ActStructFieldList(l *StructFieldList) (v Value) {
    for _, n := range l.List {
        Act(p, n)
    }
    return
}
func (p *Printer) ActStructFieldNode(n *StructFieldNode) (v Value) {
    p.printNode(n)
    Depth++
    Act(p, n.Typ)
    Act(p, n.Id)
    Depth--
    return
}
func (p *Printer) ActStructDefList(l *StructDefList) (v Value) {
    for _, n := range l.List {
        Act(p, n)
    }
    return
}
func (p *Printer) ActStructDefNode(n *StructDefNode) (v Value) {
    p.printNode(n)
    Depth++
    Act(p, n.Typ)
    Act(p, n.Fields)
    Depth--
    return
}
func (p *Printer) ActEventDeclList(l *EventDeclList) (v Value) {
    for _, n := range l.List {
        Act(p, n)
    }
    return
}
func (p *Printer) ActEventDecl(n *EventDeclNode) (v Value) {
    p.printNode(n)
    Depth++
    Act(p, n.Id)
    if n.TypList != nil {
        Act(p, n.TypList)
    }
    Depth--
    return
}
func (p *Printer) ActFunctionDefList(l *FunctionDefList) (v Value) {
    for _, n := range l.List {
        Act(p, n)
    }
    return
}
func (p *Printer) ActFunctionDef(n *FunctionDefNode) (v Value) {
    p.printNode(n)
    Depth++
    Act(p, n.Name)
    Act(p, n.Args)
    Act(p, n.Acc)
    if n.Pay != nil {
        Act(p, n.Pay)
    }
    Act(p, n.Sb)
    Depth--
    return
}
func (p *Printer) ActFormalArgList(l *FormalArgList) (v Value) {
    for _, n := range l.List {
        Act(p, n)
    }
    return
}
func (p *Printer) ActFormalArgNode(n *FormalArgNode) (v Value) {
    p.printNode(n)
    Depth++
    if n.Typ != nil {
        Act(p, n.Typ)
    }
    Act(p, n.Id)
    Depth--
    return
}
func (p *Printer) ActTypeNameList(l *TypeNameList) (v Value) {
    for _, n := range l.List {
        Act(p, n)
    }
    return
}
func (p *Printer) ActTypeName(n *TypeNameNode) (v Value) {
    switch n.Internal.(type) {
    case *BasicTypeNode:
        Act(p, n.Internal.(*BasicTypeNode))
    case *MapTypeNode:
        Act(p, n.Internal.(*MapTypeNode))
    case *StructArgTypeNode:
        Act(p, n.Internal.(*StructArgTypeNode))
    case *ArrayTypeNode:
        Act(p, n.Internal.(*ArrayTypeNode))
    case *LibraryTypeNode:
        Act(p, n.Internal.(*LibraryTypeNode))
    default:
        Act(p, unimpl)
    }
    return
}
func (p *Printer) ActBasicType(n *BasicTypeNode) (v Value) {
    p.printNode(n)
    //Depth++
    //p.actNodeType(n.Typ)
    //Act(p, &TokenNode{Val: n.Val})
    //Depth--
    return
}
func (p *Printer) ActMapType(n *MapTypeNode) (v Value) {
    p.printNode(n)
    Depth++
    Act(p, n.From)
    Act(p, n.To)
    Depth--
    return
}
func (p *Printer) ActStructArgType(n *StructArgTypeNode) (v Value) {
    p.printNode(n)
    Depth++
    Act(p, n.List)
    Depth--
    return
}
func (p *Printer) ActArrayType(n *ArrayTypeNode) (v Value) {
    p.printNode(n)
    Depth++
    //Act(p, n.Typ)
    if n.Alloc != nil {
        Act(p, n.Alloc)
    }
    //p.actBool(n.IsStorage)
    Depth--
    return
}
func (p *Printer) ActLibraryType(n *LibraryTypeNode) (v Value) {
    p.printNode(n)
    Depth++
    if n.Lvalue.Lval != nil {
        Act(p, n.Lvalue.Lval)
        Act(p, &TokenNode{Val: n.Lvalue.Member})
    } else {
        Act(p, n.Lvalue.Primary)
    }
    Depth--
    return
}
func (p *Printer) ActStmtBlock(n *StmtBlockNode) (v Value) {
    p.printNode(n)
    Depth++
    Act(p, n.List)
    Depth--
    return
}
func (p *Printer) ActStmtList(l *StmtList) (v Value) {
    for _, n := range l.List {
        Act(p, n)
    }
    return
}
func (p *Printer) ActStmt(n *StmtNode) (v Value) {
    switch n.Internal.(type) {
    case *IfStmtNode:
        Act(p, n.Internal.(*IfStmtNode))
    case *WhileNode:
        Act(p, n.Internal.(*WhileNode))
    case *DoWhileNode:
        Act(p, n.Internal.(*DoWhileNode))
    case *StmtBlockNode:
        Act(p, n.Internal.(*StmtBlockNode))
    case *ContinueStmtNode:
        Act(p, n.Internal.(*ContinueStmtNode))
    case *BreakStmtNode:
        Act(p, n.Internal.(*BreakStmtNode))
    case *ReturnStmtNode:
        Act(p, n.Internal.(*ReturnStmtNode))
    case *RequireStmtNode:
        Act(p, n.Internal.(*RequireStmtNode))
    case *ThrowStmtNode:
        Act(p, n.Internal.(*ThrowStmtNode))
    case *EmitStmtNode:
        Act(p, n.Internal.(*EmitStmtNode))
    case *GotoStmtNode:
        Act(p, n.Internal.(*GotoStmtNode))
    case *CalcAssignStmtNode:
        Act(p, n.Internal.(*CalcAssignStmtNode))
    case *ExprStmtNode:
        Act(p, n.Internal.(*ExprStmtNode))
    default:
        Act(p, unimpl)
    }
    return
}
func (p *Printer) ActIfStmt(n *IfStmtNode) (v Value) {
    Act(p, n.IfN)
    if n.Elif != nil {
        Act(p, n.Elif)
    }
    if n.Els != nil {
        Act(p, n.Els)
    }
    return
}
func (p *Printer) ActIf(n *IfNode) (v Value) {
    p.printNode(n)
    Depth++
    Act(p, n.Expr)
    Act(p, n.Sb)
    Depth--
    return
}
func (p *Printer) ActElse(n *ElseNode) (v Value) {
    p.printNode(n)
    Depth++
    Act(p, n.Sb)
    Depth--
    return
}
func (p *Printer) ActElseIfList(l *ElseIfList) (v Value) {
    for _, n := range l.List {
        Act(p, n)
    }
    return
}
func (p *Printer) ActElseIf(n *ElseIfNode) (v Value) {
    p.printNode(n)
    Depth++
    Act(p, n.Expr)
    Act(p, n.Sb)
    Depth--
    return
}
func (p *Printer) ActWhile(n *WhileNode) (v Value) {
    p.printNode(n)
    Depth++
    Act(p, n.Expr)
    Act(p, n.Stmt)
    Depth--
    return
}
func (p *Printer) ActDoWhile(n *DoWhileNode) (v Value) {
    p.printNode(n)
    Depth++
    Act(p, n.Stmt)
    Act(p, n.Expr)
    Depth--
    return
}
func (p *Printer) ActContinueStmt(n *ContinueStmtNode) (v Value) {
    p.printNode(n)
    return
}
func (p *Printer) ActBreakStmt(n *BreakStmtNode) (v Value) {
    p.printNode(n)
    return
}
func (p *Printer) ActReturnStmt(n *ReturnStmtNode) (v Value) {
    p.printNode(n)
    Depth++
    Act(p, n.Exprs)
    Depth--
    return
}
func (p *Printer) ActRequireStmt(n *RequireStmtNode) (v Value) {
    p.printNode(n)
    Depth++
    Act(p, n.Exprs)
    Depth--
    return
}
func (p *Printer) ActThrowStmt(n *ThrowStmtNode) (v Value) {
    p.printNode(n)
    Depth++
    Act(p, n.Exprs)
    Depth--
    return
}
func (p *Printer) ActEmitStmt(n *EmitStmtNode) (v Value) {
    p.printNode(n)
    Depth++
    Act(p, n.Expr)
    Depth--
    return
}
func (p *Printer) ActGotoStmt(n *GotoStmtNode) (v Value) {
    p.printNode(n)
    Depth++
    Act(p, n.Addrs)
    Depth--
    return
}
func (p *Printer) ActCalcAssignStmt(n *CalcAssignStmtNode) (v Value) {
    p.printNode(n)
    Depth++
    Act(p, n.Lvalue)
    Act(p, &TokenNode{Val: n.Op})
    Act(p, n.Expr)
    Depth--
    return
}
func (p *Printer) ActExprStmt(n *ExprStmtNode) (v Value) {
    p.printNode(n)
    Depth++
    Act(p, n.Expr)
    Depth--
    return
}
func (p *Printer) ActCastExpression(n *CastExpressionNode) (v Value) {
    p.printNode(n)
    Depth++
    Act(p, n.Typ)
    Act(p, n.Expr)
    Depth--
    return
}
func (p *Printer) ActExternalCallExpr(n *ExternalCallExprNode) (v Value) {
    p.printNode(n)
    Depth++
    Act(p, n.Lvalue)
    Act(p, &TokenNode{Val: n.CallId})
    Act(p, n.ExprList)
    Depth--
    return
}
func (p *Printer) ActExtSpecifierList(l *ExtSpecifierList) (v Value) {
    for _, n := range l.List {
        Act(p, n)
    }
    return
}
func (p *Printer) ActExtSpecifier(n *ExtSpecifierNode) (v Value) {
    p.printNode(n)
    Depth++
    Act(p, &TokenNode{Val: n.Id})
    Act(p, n.Expr)
    Depth--
    return
}
func (p *Printer) ActExpressionList(l *ExpressionList) (v Value) {
    for _, n := range l.List {
        Act(p, n)
    }
    return
}
func (p *Printer) ActExpression(n *ExpressionNode) (v Value) {
    Act(p, n.Subexpr)
    return
}
func (p *Printer) ActSubexpr(n *SubexprNode) (v Value) {
    p.printNode(n)
    Depth++
    if n.Exprs != nil { //assignExpr
        Act(p, n.Exprs)
        Act(p, &TokenNode{Val: n.Op})
        Act(p, n.Right)
    } else if !n.IsUnary {
        Act(p, n.Left)
        Act(p, &TokenNode{Val: n.Op})
        Act(p, n.Right)
    } else {
        Act(p, &TokenNode{Val: n.Op})
        if n.Op == "" {
            Depth--
        }
        Act(p, n.Rvalue)
        if n.Op == "" {
            Depth++
        }
    }
    Depth--
    return
}
func (p *Printer) ActRvalue(n *RvalueNode) (v Value) {
    if n.Cast != nil {
        Act(p, n.Cast)
    } else if n.ExtCall != nil {
        Act(p, n.ExtCall)
        Act(p, n.ExtSpec)
    } else if n.Lvalue != nil {
        Act(p, n.Lvalue)
    } else {
        Act(p, unimpl)
    }
    return
}
func (p *Printer) ActLvalue(n *LvalueNode) (v Value) {
    p.printNode(n)
    Depth++
    switch n.LvalueType {
    case TypeAlloc:
        if n.Lval != nil {
            Act(p, n.Lval)
        } else {
            Act(p, &TokenNode{Val: n.AllocLoc})
        }
        Act(p, n.AllocSize)
    case TypeCall:
        Act(p, n.Lval)
        Act(p, n.ExprList)
    case TypeAccess:
        Act(p, n.Lval)
        Act(p, &TokenNode{Val: n.Member})
    case TypePrimary:
        Act(p, n.Primary)
    case TypeCast:
        Act(p, n.Cast)
    case TypeExpr:
        Act(p, n.Expr)
    case TypeType:
        Act(p, n.Typ)
    default:
        Act(p, unimpl)
    }
    Depth--
    return
}
func (p *Printer) ActAllocSize(n *AllocSizeNode) (v Value) {
    p.printNode(n)
    Depth++
    Act(p, n.From)
    if n.Length != nil {
        Act(p, n.Length)
    }
    if n.To != nil {
        Act(p, n.To)
    }
    Depth--
    return
}
func (p *Printer) ActPrimaryList(l *PrimaryList) (v Value) {
    for _, n := range l.List {
        Act(p, n)
    }
    return
}
func (p *Printer) ActPrimary(n *PrimaryNode) (v Value) {
    p.printNode(n)
    return
}
func (p *Printer) ActToken(n *TokenNode) (v Value) {
    p.printNode(n)
    return
}
func (p *Printer) ActNodeType(t NodeType) (v Value) {
    return
}
func (p *Printer) ActBool(b bool) (v Value) {
    return
}

func (p *Printer) ActFallbackStmt(n *FallbackStmtNode) (v Value) {
    return
}
