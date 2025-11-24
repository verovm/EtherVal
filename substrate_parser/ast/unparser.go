package ast

type Unparser struct {
    unparsed string
}

func (u *Unparser) printOut(str string) {
    u.unparsed += str
}
func (u *Unparser) putIndent() {
    for i := 0; i < indent*Depth; i++ {
        u.printOut(" ")
    }
}
func (u *Unparser) GetUnparsed() string {
    return u.unparsed
}

func (u *Unparser) ActBaseNode(n *BaseNode) (v Value) {
    panic("Act on BaseNode")
    return
}
func (u *Unparser) ActSubstrate(n *SubstrateNode) (v Value) {
    Depth = 0
    if n.SList != nil {
        Act(u, n.SList)
    }
    u.printOut("\n")
    if n.EList != nil {
        Act(u, n.EList)
    }
    u.printOut("\n")
    if n.FList != nil {
        Act(u, n.FList)
    }
    return
}
func (u *Unparser) ActStorageDeclList(l *StorageDeclList) (v Value) {
    for _, n := range l.List {
        Act(u, n)
    }
    return
}
func (u *Unparser) ActStorageDecl(n *StorageDeclNode) (v Value) {
    Act(u, n.Typ)
    u.printOut(" ")
    Act(u, n.Id)
    u.printOut("; " + string(n.Comment) + "\n")
    return
}
func (u *Unparser) ActStructFieldList(l *StructFieldList) (v Value) {
    for _, n := range l.List {
        Act(u, n)
    }
    return
}
func (u *Unparser) ActStructFieldNode(n *StructFieldNode) (v Value) {
    Act(u, n.Typ)
    u.printOut(" ")
    Act(u, n.Id)
    u.printOut("; ")
    return
}
func (u *Unparser) ActStructDefList(l *StructDefList) (v Value) {
    for _, n := range l.List {
        Act(u, n)
    }
    return
}
func (u *Unparser) ActStructDefNode(n *StructDefNode) (v Value) {
    u.printOut("struct ")
    Act(u, n.Typ)
    u.printOut(" { ")
    Act(u, n.Fields)
    u.printOut("};\n")
    return
}
func (u *Unparser) ActEventDeclList(l *EventDeclList) (v Value) {
    for _, n := range l.List {
        Act(u, n)
    }
    return
}
func (u *Unparser) ActEventDecl(n *EventDeclNode) (v Value) {
    Act(u, n.Id)
    u.printOut("(")
    if n.TypList != nil {
        Act(u, n.TypList)
    }
    u.printOut(");\n")
    return
}
func (u *Unparser) ActFunctionDefList(l *FunctionDefList) (v Value) {
    for i, n := range l.List {
        Act(u, n)
        if i+1 < len(l.List) {
            u.printOut("\n")
        }
    }
    return
}
func (u *Unparser) ActFunctionDef(n *FunctionDefNode) (v Value) {
    u.printOut("function ")
    Act(u, n.Name)
    u.printOut("(")
    Act(u, n.Args)
    u.printOut(") ")
    Act(u, n.Acc)
    if n.Pay != nil {
        u.printOut(" ")
        Act(u, n.Pay)
    }
    Act(u, n.Sb)
    u.printOut("\n")
    return
}
func (u *Unparser) ActFormalArgList(l *FormalArgList) (v Value) {
    for i, n := range l.List {
        Act(u, n)
        if i+1 < len(l.List) {
            u.printOut(", ")
        }
    }
    return
}
func (u *Unparser) ActFormalArgNode(n *FormalArgNode) (v Value) {
    if n.Typ != nil {
        Act(u, n.Typ)
    }
    u.printOut(" ")
    Act(u, n.Id)
    return
}
func (u *Unparser) ActTypeNameList(l *TypeNameList) (v Value) {
    for i, n := range l.List {
        Act(u, n)
        if i+1 < len(l.List) {
            u.printOut(", ")
        }
    }
    return
}
func (u *Unparser) ActTypeName(n *TypeNameNode) (v Value) {
    switch n.Internal.(type) {
    case *BasicTypeNode:
        Act(u, n.Internal.(*BasicTypeNode))
    case *MapTypeNode:
        Act(u, n.Internal.(*MapTypeNode))
    case *StructArgTypeNode:
        Act(u, n.Internal.(*StructArgTypeNode))
    case *ArrayTypeNode:
        Act(u, n.Internal.(*ArrayTypeNode))
    case *LibraryTypeNode:
        Act(u, n.Internal.(*LibraryTypeNode))
    default:
        Act(u, unimpl)
    }
    return
}
func (u *Unparser) ActBasicType(n *BasicTypeNode) (v Value) {
    //u.actNodeType(n.Typ)
    Act(u, &TokenNode{Val: n.Val})
    return
}
func (u *Unparser) ActMapType(n *MapTypeNode) (v Value) {
    u.printOut("mapping (")
    Act(u, n.From)
    u.printOut(" => ")
    Act(u, n.To)
    u.printOut(")")
    return
}
func (u *Unparser) ActStructArgType(n *StructArgTypeNode) (v Value) {
    u.printOut("(")
    Act(u, n.List)
    u.printOut(")")
    return
}
func (u *Unparser) ActArrayType(n *ArrayTypeNode) (v Value) {
    Act(u, n.Typ)
    if n.Alloc != nil {
        Act(u, n.Alloc)
    } else {
        u.printOut("[]")
    }
    //u.actBool(n.isStorage)
    if n.IsStorage {
        u.printOut(" storage")
    }
    return
}
func (u *Unparser) ActLibraryType(n *LibraryTypeNode) (v Value) {
    if n.Lvalue.Lval != nil {
        Act(u, n.Lvalue.Lval)
        u.printOut(".")
        Act(u, &TokenNode{Val: n.Lvalue.Member})
    } else {
        Act(u, n.Lvalue.Primary)
    }
    if n.IsArray {
        u.printOut("[]")
    }
    if n.IsStorage {
        u.printOut(" storage")
    }
    return
}
func (u *Unparser) ActStmtBlock(n *StmtBlockNode) (v Value) {
    u.printOut("{\n")
    Depth++
    Act(u, n.List)
    Depth--
    u.putIndent()
    u.printOut("}")
    return
}
func (u *Unparser) ActStmtList(l *StmtList) (v Value) {
    for _, n := range l.List {
        Act(u, n)
    }
    return
}
func (u *Unparser) ActStmt(n *StmtNode) (v Value) {
    u.putIndent()
    switch n.Internal.(type) {
    case *IfStmtNode:
        Act(u, n.Internal.(*IfStmtNode))
    case *WhileNode:
        Act(u, n.Internal.(*WhileNode))
    case *DoWhileNode:
        Act(u, n.Internal.(*DoWhileNode))
    case *StmtBlockNode:
        Act(u, n.Internal.(*StmtBlockNode))
    case *ContinueStmtNode:
        Act(u, n.Internal.(*ContinueStmtNode))
    case *BreakStmtNode:
        Act(u, n.Internal.(*BreakStmtNode))
    case *ReturnStmtNode:
        Act(u, n.Internal.(*ReturnStmtNode))
    case *RequireStmtNode:
        Act(u, n.Internal.(*RequireStmtNode))
    case *ThrowStmtNode:
        Act(u, n.Internal.(*ThrowStmtNode))
    case *EmitStmtNode:
        Act(u, n.Internal.(*EmitStmtNode))
    case *GotoStmtNode:
        Act(u, n.Internal.(*GotoStmtNode))
    case *CalcAssignStmtNode:
        Act(u, n.Internal.(*CalcAssignStmtNode))
    case *ExprStmtNode:
        Act(u, n.Internal.(*ExprStmtNode))
    default:
        Act(u, unimpl)
    }
    return
}
func (u *Unparser) ActIfStmt(n *IfStmtNode) (v Value) {
    Act(u, n.IfN)
    if n.Elif != nil {
        Act(u, n.Elif)
    }
    if n.Els != nil {
        Act(u, n.Els)
    }
    u.printOut("\n")
    return
}
func (u *Unparser) ActIf(n *IfNode) (v Value) {
    u.printOut("if (")
    Act(u, n.Expr)
    u.printOut(") ")
    Act(u, n.Sb)
    return
}
func (u *Unparser) ActElse(n *ElseNode) (v Value) {
    u.printOut(" else ")
    Act(u, n.Sb)
    return
}
func (u *Unparser) ActElseIfList(l *ElseIfList) (v Value) {
    for _, n := range l.List {
        Act(u, n)
    }
    return
}
func (u *Unparser) ActElseIf(n *ElseIfNode) (v Value) {
    u.printOut(" else if (")
    Act(u, n.Expr)
    u.printOut(") ")
    Act(u, n.Sb)
    return
}
func (u *Unparser) ActWhile(n *WhileNode) (v Value) {
    u.printOut("while (")
    Act(u, n.Expr)
    u.printOut(") ")
    Act(u, n.Stmt)
    return
}
func (u *Unparser) ActDoWhile(n *DoWhileNode) (v Value) {
    u.printOut("do ")
    Act(u, n.Stmt)
    u.printOut(" while (")
    Act(u, n.Expr)
    u.printOut(");\n")
    return
}
func (u *Unparser) ActContinueStmt(n *ContinueStmtNode) (v Value) {
    u.printOut("continue;\n")
    return
}
func (u *Unparser) ActBreakStmt(n *BreakStmtNode) (v Value) {
    u.printOut("break;\n")
    return
}
func (u *Unparser) ActReturnStmt(n *ReturnStmtNode) (v Value) {
    u.printOut("return ")
    Act(u, n.Exprs)
    u.printOut(";\n")
    return
}
func (u *Unparser) ActRequireStmt(n *RequireStmtNode) (v Value) {
    u.printOut("require(")
    Act(u, n.Exprs)
    u.printOut(");\n")
    return
}
func (u *Unparser) ActThrowStmt(n *ThrowStmtNode) (v Value) {
    u.printOut("throw(")
    Act(u, n.Exprs)
    u.printOut(");\n")
    return
}
func (u *Unparser) ActEmitStmt(n *EmitStmtNode) (v Value) {
    u.printOut("emit ")
    Act(u, n.Expr)
    u.printOut(";\n")
    return
}
func (u *Unparser) ActGotoStmt(n *GotoStmtNode) (v Value) {
    u.printOut("goto ")
    Act(u, n.Addrs)
    u.printOut(";\n")
    return
}
func (u *Unparser) ActCalcAssignStmt(n *CalcAssignStmtNode) (v Value) {
    Act(u, n.Lvalue)
    u.printOut(" ")
    Act(u, &TokenNode{Val: n.Op})
    u.printOut(" ")
    Act(u, n.Expr)
    u.printOut(";\n")
    return
}
func (u *Unparser) ActExprStmt(n *ExprStmtNode) (v Value) {
    Act(u, n.Expr)
    u.printOut(";\n")
    return
}
func (u *Unparser) ActCastExpression(n *CastExpressionNode) (v Value) {
    Act(u, n.Typ)
    u.printOut("(")
    Act(u, n.Expr)
    u.printOut(")")
    return
}
func (u *Unparser) ActExternalCallExpr(n *ExternalCallExprNode) (v Value) {
    Act(u, n.Lvalue)
    u.printOut(".")
    Act(u, &TokenNode{Val: n.CallId})
    u.printOut("(")
    Act(u, n.ExprList)
    u.printOut(")")
    return
}
func (u *Unparser) ActExtSpecifierList(l *ExtSpecifierList) (v Value) {
    for _, n := range l.List {
        Act(u, n)
    }
    return
}
func (u *Unparser) ActExtSpecifier(n *ExtSpecifierNode) (v Value) {
    u.printOut(".")
    Act(u, &TokenNode{Val: n.Id})
    u.printOut("(")
    Act(u, n.Expr)
    u.printOut(")")
    return
}
func (u *Unparser) ActExpressionList(l *ExpressionList) (v Value) {
    for i, n := range l.List {
        Act(u, n)
        if i+1 < len(l.List) {
            u.printOut(", ")
        }
    }
    return
}
func (u *Unparser) ActExpression(n *ExpressionNode) (v Value) {
    Act(u, n.Subexpr)
    return
}
func (u *Unparser) ActSubexpr(n *SubexprNode) (v Value) {
    if n.Exprs != nil { //assignExpr
        Act(u, n.Exprs)
        u.printOut(" ")
        Act(u, &TokenNode{Val: n.Op})
        u.printOut(" ")
        Act(u, n.Right)
    } else if !n.IsUnary {
        Act(u, n.Left)
        u.printOut(" ")
        Act(u, &TokenNode{Val: n.Op})
        u.printOut(" ")
        Act(u, n.Right)
    } else {
        if n.Op != "" {
            Act(u, &TokenNode{Val: n.Op})
        }
        Act(u, n.Rvalue)
    }
    return
}
func (u *Unparser) ActRvalue(n *RvalueNode) (v Value) {
    if n.Cast != nil {
        u.printOut("new ")
        Act(u, n.Cast)
    } else if n.ExtCall != nil {
        Act(u, n.ExtCall)
        Act(u, n.ExtSpec)
    } else if n.Lvalue != nil {
        Act(u, n.Lvalue)
    } else {
        Act(u, unimpl)
    }
    return
}
func (u *Unparser) ActLvalue(n *LvalueNode) (v Value) {
    switch n.LvalueType {
    case TypeAlloc:
        if n.Lval != nil {
            Act(u, n.Lval)
        } else {
            Act(u, &TokenNode{Val: n.AllocLoc})
        }
        Act(u, n.AllocSize)
    case TypeCall:
        Act(u, n.Lval)
        u.printOut("(")
        Act(u, n.ExprList)
        u.printOut(")")
    case TypeAccess:
        Act(u, n.Lval)
        u.printOut(".")
        Act(u, &TokenNode{Val: n.Member})
    case TypePrimary:
        Act(u, n.Primary)
    case TypeCast:
        Act(u, n.Cast)
    case TypeExpr:
        u.printOut("(")
        Act(u, n.Expr)
        u.printOut(")")
    case TypeType:
        Act(u, n.Typ)
    default:
        Act(u, unimpl)
    }
    return
}
func (u *Unparser) ActAllocSize(n *AllocSizeNode) (v Value) {
    u.printOut("[")
    Act(u, n.From)
    if n.Length != nil {
        u.printOut(" len ")
        Act(u, n.Length)
    }
    if n.To != nil {
        u.printOut(":")
        Act(u, n.To)
    }
    u.printOut("]")
    return
}
func (u *Unparser) ActPrimaryList(l *PrimaryList) (v Value) {
    if len(l.List) > 1 {
        u.printOut("{")
    }
    for i, n := range l.List {
        Act(u, n)
        if i+1 < len(l.List) {
            u.printOut(", ")
        }
    }
    if len(l.List) > 1 {
        u.printOut("}")
    }
    return
}
func (u *Unparser) ActPrimary(n *PrimaryNode) (v Value) {
    Act(u, &TokenNode{Val: n.Val})
    return
}
func (u *Unparser) ActToken(n *TokenNode) (v Value) {
    u.printOut(n.Val)
    return
}
func (u *Unparser) ActNodeType(t NodeType) (v Value) {
    return
}
func (u *Unparser) ActBool(b bool) (v Value) {
    return
}

func (u *Unparser) ActFallbackStmt(n *FallbackStmtNode) (v Value) {
    u.printOut("()")
    return
}
