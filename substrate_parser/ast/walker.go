package ast

type Walker struct{}

func (w *Walker) ActBaseNode(n *BaseNode) (v Value) {
   panic("Act on BaseNode")
   return
}
func (w *Walker) ActSubstrate(n *SubstrateNode) (v Value) {
   if n.SList != nil {
      Act(w, n.SList)
   }
   if n.EList != nil {
      Act(w, n.EList)
   }
   if n.FList != nil {
      Act(w, n.FList)
   }
   return
}
func (w *Walker) ActStorageDeclList(l *StorageDeclList) (v Value) {
   for _, n := range l.List {
      Act(w, n)
   }
   return
}
func (w *Walker) ActStorageDecl(n *StorageDeclNode) (v Value) {
   Act(w, n.Typ)
   Act(w, n.Id)
   return
}
func (w *Walker) ActStructFieldList(l *StructFieldList) (v Value) {
   for _, n := range l.List {
      Act(w, n)
   }
   return
}
func (w *Walker) ActStructFieldNode(n *StructFieldNode) (v Value) {
   Act(w, n.Typ)
   Act(w, n.Id)
   return
}
func (w *Walker) ActStructDefList(l *StructDefList) (v Value) {
   for _, n := range l.List {
      Act(w, n)
   }
   return
}
func (w *Walker) ActStructDefNode(n *StructDefNode) (v Value) {
   Act(w, n.Typ)
   Act(w, n.Fields)
   return
}
func (w *Walker) ActEventDeclList(l *EventDeclList) (v Value) {
   for _, n := range l.List {
      Act(w, n)
   }
   return
}
func (w *Walker) ActEventDecl(n *EventDeclNode) (v Value) {
   Act(w, n.Id)
   if n.TypList != nil {
      Act(w, n.TypList)
   }
   return
}
func (w *Walker) ActFunctionDefList(l *FunctionDefList) (v Value) {
   for _, n := range l.List {
      Act(w, n)
   }
   return
}
func (w *Walker) ActFunctionDef(n *FunctionDefNode) (v Value) {
   Act(w, n.Name)
   Act(w, n.Args)
   Act(w, n.Acc)
   if n.Pay != nil {
      Act(w, n.Pay)
   }
   Act(w, n.Sb)
   return
}
func (w *Walker) ActFormalArgList(l *FormalArgList) (v Value) {
   for _, n := range l.List {
      Act(w, n)
   }
   return
}
func (w *Walker) ActFormalArgNode(n *FormalArgNode) (v Value) {
   if n.Typ != nil {
      Act(w, n.Typ)
   }
   Act(w, n.Id)
   return
}
func (w *Walker) ActTypeNameList(l *TypeNameList) (v Value) {
   for _, n := range l.List {
      Act(w, n)
   }
   return
}
func (w *Walker) ActTypeName(n *TypeNameNode) (v Value) {
   switch n.Internal.(type) {
   case *BasicTypeNode:
      Act(w, n.Internal.(*BasicTypeNode))
   case *MapTypeNode:
      Act(w, n.Internal.(*MapTypeNode))
   case *StructArgTypeNode:
      Act(w, n.Internal.(*StructArgTypeNode))
   case *ArrayTypeNode:
      Act(w, n.Internal.(*ArrayTypeNode))
   case *LibraryTypeNode:
      Act(w, n.Internal.(*LibraryTypeNode))
   default:
      Act(w, unimpl)
   }
   return
}
func (w *Walker) ActBasicType(n *BasicTypeNode) (v Value) {
   w.ActNodeType(n.Typ)
   Act(w, &TokenNode{Val: n.Val})
   return
}
func (w *Walker) ActMapType(n *MapTypeNode) (v Value) {
   Act(w, n.From)
   Act(w, n.To)
   return
}
func (w *Walker) ActStructArgType(n *StructArgTypeNode) (v Value) {
   Act(w, n.List)
   return
}
func (w *Walker) ActArrayType(n *ArrayTypeNode) (v Value) {
   Act(w, n.Typ)
   if n.Alloc != nil {
      Act(w, n.Alloc)
   }
   w.ActBool(n.IsStorage)
   return
}
func (w *Walker) ActLibraryType(n *LibraryTypeNode) (v Value) {
   if n.Lvalue.Lval != nil {
      Act(w, n.Lvalue.Lval)
      Act(w, &TokenNode{Val: n.Lvalue.Member})
   } else {
      Act(w, n.Lvalue.Primary)
   }
   return
}
func (w *Walker) ActStmtBlock(n *StmtBlockNode) (v Value) {
   Act(w, n.List)
   return
}
func (w *Walker) ActStmtList(l *StmtList) (v Value) {
   for _, n := range l.List {
      Act(w, n)
   }
   return
}
func (w *Walker) ActStmt(n *StmtNode) (v Value) {
   switch n.Internal.(type) {
   case *IfStmtNode:
      Act(w, n.Internal.(*IfStmtNode))
   case *WhileNode:
      Act(w, n.Internal.(*WhileNode))
   case *DoWhileNode:
      Act(w, n.Internal.(*DoWhileNode))
   case *StmtBlockNode:
      Act(w, n.Internal.(*StmtBlockNode))
   case *ContinueStmtNode:
      Act(w, n.Internal.(*ContinueStmtNode))
   case *BreakStmtNode:
      Act(w, n.Internal.(*BreakStmtNode))
   case *ReturnStmtNode:
      Act(w, n.Internal.(*ReturnStmtNode))
   case *RequireStmtNode:
      Act(w, n.Internal.(*RequireStmtNode))
   case *ThrowStmtNode:
      Act(w, n.Internal.(*ThrowStmtNode))
   case *EmitStmtNode:
      Act(w, n.Internal.(*EmitStmtNode))
   case *GotoStmtNode:
      Act(w, n.Internal.(*GotoStmtNode))
   case *CalcAssignStmtNode:
      Act(w, n.Internal.(*CalcAssignStmtNode))
   case *ExprStmtNode:
      Act(w, n.Internal.(*ExprStmtNode))
   default:
      Act(w, unimpl)
   }
   return
}
func (w *Walker) ActIfStmt(n *IfStmtNode) (v Value) {
   Act(w, n.IfN)
   if n.Elif != nil {
      Act(w, n.Elif)
   }
   if n.Els != nil {
      Act(w, n.Els)
   }
   return
}
func (w *Walker) ActIf(n *IfNode) (v Value) {
   Act(w, n.Expr)
   Act(w, n.Sb)
   return
}
func (w *Walker) ActElse(n *ElseNode) (v Value) {
   Act(w, n.Sb)
   return
}
func (w *Walker) ActElseIfList(l *ElseIfList) (v Value) {
   for _, n := range l.List {
      Act(w, n)
   }
   return
}
func (w *Walker) ActElseIf(n *ElseIfNode) (v Value) {
   Act(w, n.Expr)
   Act(w, n.Sb)
   return
}
func (w *Walker) ActWhile(n *WhileNode) (v Value) {
   Act(w, n.Expr)
   Act(w, n.Stmt)
   return
}
func (w *Walker) ActDoWhile(n *DoWhileNode) (v Value) {
   Act(w, n.Stmt)
   Act(w, n.Expr)
   return
}
func (w *Walker) ActContinueStmt(n *ContinueStmtNode) (v Value) {
   return
}
func (w *Walker) ActBreakStmt(n *BreakStmtNode) (v Value) {
   return
}
func (w *Walker) ActReturnStmt(n *ReturnStmtNode) (v Value) {
   Act(w, n.Exprs)
   return
}
func (w *Walker) ActRequireStmt(n *RequireStmtNode) (v Value) {
   Act(w, n.Exprs)
   return
}
func (w *Walker) ActThrowStmt(n *ThrowStmtNode) (v Value) {
   Act(w, n.Exprs)
   return
}
func (w *Walker) ActEmitStmt(n *EmitStmtNode) (v Value) {
   Act(w, n.Expr)
   return
}
func (w *Walker) ActGotoStmt(n *GotoStmtNode) (v Value) {
   Act(w, n.Addrs)
   return
}
func (w *Walker) ActCalcAssignStmt(n *CalcAssignStmtNode) (v Value) {
   Act(w, n.Lvalue)
   Act(w, &TokenNode{Val: n.Op})
   Act(w, n.Expr)
   return
}
func (w *Walker) ActExprStmt(n *ExprStmtNode) (v Value) {
   Act(w, n.Expr)
   return
}
func (w *Walker) ActCastExpression(n *CastExpressionNode) (v Value) {
   Act(w, n.Typ)
   Act(w, n.Expr)
   return
}
func (w *Walker) ActExternalCallExpr(n *ExternalCallExprNode) (v Value) {
   Act(w, n.Lvalue)
   Act(w, &TokenNode{Val: n.CallId})
   Act(w, n.ExprList)
   return
}
func (w *Walker) ActExtSpecifierList(l *ExtSpecifierList) (v Value) {
   for _, n := range l.List {
      Act(w, n)
   }
   return
}
func (w *Walker) ActExtSpecifier(n *ExtSpecifierNode) (v Value) {
   Act(w, &TokenNode{Val: n.Id})
   Act(w, n.Expr)
   return
}
func (w *Walker) ActExpressionList(l *ExpressionList) (v Value) {
   for _, n := range l.List {
      Act(w, n)
   }
   return
}
func (w *Walker) ActExpression(n *ExpressionNode) (v Value) {
   Act(w, n.Subexpr)
   return
}
func (w *Walker) ActSubexpr(n *SubexprNode) (v Value) {
   if n.Exprs != nil { //assignExpr
      Act(w, n.Exprs)
      Act(w, &TokenNode{Val: n.Op})
      Act(w, n.Right)
   } else if !n.IsUnary {
      Act(w, n.Left)
      Act(w, &TokenNode{Val: n.Op})
      Act(w, n.Right)
   } else {
      Act(w, &TokenNode{Val: n.Op})
      Act(w, n.Rvalue)
   }
   return
}
func (w *Walker) ActRvalue(n *RvalueNode) (v Value) {
   if n.Cast != nil {
      Act(w, n.Cast)
   } else if n.ExtCall != nil {
      Act(w, n.ExtCall)
      Act(w, n.ExtSpec)
   } else if n.Lvalue != nil {
      Act(w, n.Lvalue)
   } else {
      Act(w, unimpl)
   }
   return
}
func (w *Walker) ActLvalue(n *LvalueNode) (v Value) {
   switch n.LvalueType {
   case TypeAlloc:
      if n.Lval != nil {
         Act(w, n.Lval)
      } else {
         Act(w, &TokenNode{Val: n.AllocLoc})
      }
      Act(w, n.AllocSize)
   case TypeCall:
      Act(w, n.Lval)
      Act(w, n.ExprList)
   case TypeAccess:
      Act(w, n.Lval)
      Act(w, &TokenNode{Val: n.Member})
   case TypePrimary:
      Act(w, n.Primary)
   case TypeCast:
      Act(w, n.Cast)
   case TypeExpr:
      Act(w, n.Expr)
   case TypeType:
      Act(w, n.Typ)
   default:
      Act(w, unimpl)
   }
   return
}
func (w *Walker) ActAllocSize(n *AllocSizeNode) (v Value) {
   Act(w, n.From)
   if n.Length != nil {
      Act(w, n.Length)
   }
   if n.To != nil {
      Act(w, n.To)
   }
   return
}
func (w *Walker) ActPrimaryList(l *PrimaryList) (v Value) {
   for _, n := range l.List {
      Act(w, n)
   }
   return
}
func (w *Walker) ActPrimary(n *PrimaryNode) (v Value) {
   Act(w, &TokenNode{Val: n.Val})
   return
}
func (w *Walker) ActToken(n *TokenNode) (v Value) {
   return
}
func (w *Walker) ActNodeType(t NodeType) (v Value) {
   return
}
func (w *Walker) ActBool(b bool) (v Value) {
   return
}

func (w *Walker) ActFallbackStmt(n *FallbackStmtNode) (v Value) {
   return
}