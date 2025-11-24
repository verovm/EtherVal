package ast

import (
	"encoding/hex"
	"sync/atomic"

	"golang.org/x/crypto/sha3"
)

type Pos struct {
	line   int
	column int
}

func NewPos(l int, c int) Pos {
	return Pos{line: l, column: c}
}

type AstNode interface {
	Pos() Pos
	GetPos() (int, int)
}

type BaseNode struct {
	pos       Pos
	isCovered atomic.Value
}

func (n *BaseNode) Pos() Pos {
	return n.pos
}
func (n *BaseNode) GetPos() (int, int) {
	return n.pos.line, n.pos.column
}
func (n *BaseNode) SetCovered(f bool) {
	n.isCovered.Store(f)
}
func (n *BaseNode) GetCovered() bool {
	if n.isCovered.Load() == nil {
		return false
	} else {
		return n.isCovered.Load().(bool)
	}
}

var unimpl *BaseNode

type SubstrateNode struct {
	BaseNode
	SList  *StorageDeclList
	StList *StructDefList
	EList  *EventDeclList
	FList  *FunctionDefList
	Date   string
}

func BuildAST(s *StorageDeclList, st *StructDefList, e *EventDeclList, f *FunctionDefList) *SubstrateNode {
	return &SubstrateNode{SList: s, StList: st, EList: e, FList: f}
}
func (n *SubstrateNode) GetFuncPos() map[string]int {
	if n == nil {
		return nil
	}
	if n.FList == nil {
		return nil
	}

	funcList := make(map[string]int)
	for _, f := range n.FList.List {
		funcList[f.Name.Val], _ = f.Name.GetPos()
	}
	return funcList
}

type StorageDeclList struct {
	BaseNode
	List []*StorageDeclNode
}

func NewStorageDeclList(l *StorageDeclList, n *StorageDeclNode) *StorageDeclList {
	if l == nil {
		l = new(StorageDeclList)
		l.List = make([]*StorageDeclNode, 0, 4)
	}
	l.List = append(l.List, n)
	return l
}

type StorageDeclNode struct {
	BaseNode
	Typ     *TypeNameNode
	Id      *PrimaryNode
	Comment CommentNode
}

func NewStorageDeclNode(t *TypeNameNode, p *PrimaryNode, c string) *StorageDeclNode {
	if p.Typ != TypeID {
		panic("StorageDecl ID type is wrong")
	}
	return &StorageDeclNode{BaseNode: BaseNode{pos: t.pos}, Typ: t, Id: p, Comment: CommentNode(c)}
}

type StructFieldList struct {
	BaseNode
	List []*StructFieldNode
}

func NewStructFieldList(l *StructFieldList, n *StructFieldNode) *StructFieldList {
	if l == nil {
		l = new(StructFieldList)
		l.List = make([]*StructFieldNode, 0, 4)
	}
	l.List = append(l.List, n)
	return l
}

type StructFieldNode struct {
	BaseNode
	Typ *TypeNameNode
	Id  *PrimaryNode
}

func NewStructFieldNode(t *TypeNameNode, p *PrimaryNode) *StructFieldNode {
	return &StructFieldNode{BaseNode: BaseNode{pos: t.pos}, Typ: t, Id: p}
}

type StructDefList struct {
	BaseNode
	List []*StructDefNode
}

func NewStructDefList(l *StructDefList, n *StructDefNode) *StructDefList {
	if l == nil {
		l = new(StructDefList)
		l.List = make([]*StructDefNode, 0, 4)
	}
	l.List = append(l.List, n)
	return l
}

type StructDefNode struct {
	BaseNode
	Typ    *TypeNameNode
	Fields *StructFieldList
}

func NewStructDefNode(t *TypeNameNode, p *StructFieldList) *StructDefNode {
	return &StructDefNode{BaseNode: BaseNode{pos: t.pos}, Typ: t, Fields: p}
}

type EventDeclList struct {
	BaseNode
	List []*EventDeclNode
}

func NewEventDeclList(l *EventDeclList, n *EventDeclNode) *EventDeclList {
	if l == nil {
		l = new(EventDeclList)
		l.List = make([]*EventDeclNode, 0, 4)
	}
	l.List = append(l.List, n)
	return l
}

type EventDeclNode struct {
	BaseNode
	Id      *PrimaryNode
	TypList *TypeNameList
}

func NewEventDeclNode(p *PrimaryNode, l *TypeNameList) *EventDeclNode {
	if l == nil {
		if p.Typ != TypeHEX {
			panic("Event HEX type is wrong")
		}
	} else if p.Typ != TypeID {
		panic("Event ID type is wrong")
	}
	return &EventDeclNode{BaseNode: BaseNode{pos: p.pos}, Id: p, TypList: l}
}

type FunctionDefList struct {
	BaseNode
	List []*FunctionDefNode
}

func NewFunctionDefList(l *FunctionDefList, n *FunctionDefNode) *FunctionDefList {
	if l == nil {
		l = new(FunctionDefList)
		l.List = make([]*FunctionDefNode, 0, 4)
	}
	l.List = append(l.List, n)
	return l
}

type FunctionDefNode struct {
	BaseNode
	Name      *PrimaryNode
	Args      *FormalArgList
	Acc, Pay  *TokenNode
	Sb        *StmtBlockNode
	signature string
}

func NewFunctionDefNode(pos Pos, s *PrimaryNode, f *FormalArgList, a *TokenNode, p *TokenNode, b *StmtBlockNode) *FunctionDefNode {
	return &FunctionDefNode{BaseNode: BaseNode{pos: pos}, Name: s, Args: f, Acc: a, Pay: p, Sb: b}
}
func (n *FunctionDefNode) Signature() string {
	if n.signature == "" {
		if n.Name.Val[0:2] == "0x" {
			if len(n.Name.Val) == 9 {
				n.signature = n.Name.Val[0:2] + "0" + n.Name.Val[2:len(n.Name.Val)]
			} else {
				n.signature = n.Name.Val
			}
		} else {
			hasher := sha3.NewLegacyKeccak256()
			funcSig := n.Name.Val + "("
			for i, a := range n.Args.List {
				if a.Typ != nil {
					funcSig = funcSig + a.Typ.String()
				}
				//TODO: need to check how gigahorse deal with empty typename. Does it make signature contain ,?
				if i+1 < len(n.Args.List) {
					funcSig = funcSig + ","
				}
			}
			funcSig = funcSig + ")"
			hasher.Write([]byte(funcSig))
			n.signature = "0x" + hex.EncodeToString(hasher.Sum(nil)[0:4])
		}
	}
	return n.signature
}

type FormalArgList struct {
	BaseNode
	List []*FormalArgNode
}

func NewFormalArgList(l *FormalArgList, n *FormalArgNode) *FormalArgList {
	if l == nil {
		l = new(FormalArgList)
		l.List = make([]*FormalArgNode, 0, 4)
	}
	if n != nil {
		l.List = append(l.List, n)
	}
	return l
}

type FormalArgNode struct {
	BaseNode
	Typ *TypeNameNode
	Id  *PrimaryNode
}

func NewFormalArgNode(t *TypeNameNode, i *PrimaryNode) *FormalArgNode {
	var pos Pos
	if t != nil {
		pos = t.pos
	} else {
		pos = i.pos
	}
	return &FormalArgNode{BaseNode: BaseNode{pos: pos}, Typ: t, Id: i}
}

// Type
type (
	TypeNameInterface interface {
		_typeName()
	}
	TypeNameList struct {
		BaseNode
		List []*TypeNameNode
	}
	TypeNameNode struct {
		BaseNode
		Internal interface{}
	}
	BasicTypeNode struct {
		BaseNode
		Typ NodeType
		Val string
	}
	MapTypeNode struct {
		BaseNode
		From *BasicTypeNode
		To   *TypeNameNode
	}
	StructArgTypeNode struct {
		BaseNode
		List *TypeNameList
	}
	ArrayTypeNode struct {
		BaseNode
		Typ       *TypeNameNode
		Alloc     *AllocSizeNode
		IsStorage bool
	}
	LibraryTypeNode struct {
		BaseNode
		Lvalue    *LvalueNode
		IsArray   bool
		IsStorage bool
	}
)

func (*TypeNameList) _typeName()      {}
func (*TypeNameNode) _typeName()      {}
func (*BasicTypeNode) _typeName()     {}
func (*MapTypeNode) _typeName()       {}
func (*StructArgTypeNode) _typeName() {}
func (*ArrayTypeNode) _typeName()     {}
func (*LibraryTypeNode) _typeName()   {}

func (n *TypeNameNode) String() string {
	switch n.Internal.(type) {
	case *BasicTypeNode:
		return n.Internal.(*BasicTypeNode).String()
	case *MapTypeNode:
		return n.Internal.(*MapTypeNode).String()
	case *StructArgTypeNode:
		return n.Internal.(*StructArgTypeNode).String()
	case *ArrayTypeNode:
		return n.Internal.(*ArrayTypeNode).String()
	case *LibraryTypeNode:
		return n.Internal.(*LibraryTypeNode).String()
	}
	return "EMPTY TYPENAME"
}
func (n *BasicTypeNode) String() string {
	return n.Val
}
func (n *MapTypeNode) String() string {
	u := Unparser{}
	u.ActMapType(n)
	return u.unparsed
}
func (n *StructArgTypeNode) String() string {
	u := Unparser{}
	u.ActStructArgType(n)
	return u.unparsed
}
func (n *ArrayTypeNode) String() string {
	u := Unparser{}
	u.ActArrayType(n)
	return u.unparsed
}
func (n *LibraryTypeNode) String() string {
	u := Unparser{}
	u.ActLibraryType(n)
	return u.unparsed
}

func NewTypeNameList(l *TypeNameList, n *TypeNameNode) *TypeNameList {
	if l == nil {
		l = new(TypeNameList)
		l.List = make([]*TypeNameNode, 0, 1)
	}
	if n != nil {
		l.List = append(l.List, n)
	}
	return l
}
func NewTypeNameNode(t interface{}) *TypeNameNode {
	return &TypeNameNode{BaseNode: BaseNode{pos: t.(AstNode).Pos()}, Internal: t}
}
func NewBasicTypeNode(pos Pos, v string, t NodeType) *BasicTypeNode {
	return &BasicTypeNode{BaseNode: BaseNode{pos: pos}, Val: v, Typ: t}
}
func NewMapTypeNode(pos Pos, f *BasicTypeNode, t *TypeNameNode) *MapTypeNode {
	return &MapTypeNode{BaseNode: BaseNode{pos: pos}, From: f, To: t}
}
func NewStructArgTypeNode(pos Pos, l *TypeNameList) *StructArgTypeNode {
	return &StructArgTypeNode{BaseNode: BaseNode{pos: pos}, List: l}
}
func NewArrayTypeNode(t *TypeNameNode, a *AllocSizeNode, s bool) *ArrayTypeNode {
	return &ArrayTypeNode{BaseNode: BaseNode{pos: t.pos}, Typ: t, Alloc: a, IsStorage: s}
}
func NewLibraryTypeNode(l *LvalueNode, a bool, s bool) *LibraryTypeNode {
	return &LibraryTypeNode{BaseNode: BaseNode{pos: l.pos}, Lvalue: l, IsArray: a, IsStorage: s}
}

// Statement
type StmtBlockNode struct {
	BaseNode
	List *StmtList
}

func NewStmtBlockNode(l *StmtList) *StmtBlockNode {
	return &StmtBlockNode{BaseNode: BaseNode{pos: l.pos}, List: l}
}

type (
	StmtInterface interface {
		_statement()
		GetPos() (int, int)
	}
	StmtList struct {
		BaseNode
		List []*StmtNode
	}
	StmtNode struct {
		BaseNode
		Internal interface{}
	}
	IfStmtNode struct {
		BaseNode
		IfN  *IfNode
		Elif *ElseIfList
		Els  *ElseNode
	}
	IfNode struct {
		BaseNode
		Expr          *ExpressionNode
		Sb            *StmtBlockNode
		isElseCovered atomic.Value //flag for IfNode w/o ElseNode
	}
	ElseNode struct {
		BaseNode
		Sb *StmtBlockNode
	}
	ElseIfList struct {
		BaseNode
		List []*ElseIfNode
	}
	ElseIfNode struct {
		BaseNode
		Expr *ExpressionNode
		Sb   *StmtBlockNode
	}
	WhileNode struct {
		BaseNode
		Expr         *ExpressionNode
		Stmt         *StmtNode
		isOutCovered atomic.Value
	}
	DoWhileNode struct {
		BaseNode
		Stmt         *StmtNode
		Expr         *ExpressionNode
		isOutCovered atomic.Value
	}
	ContinueStmtNode struct {
		BaseNode
	}
	BreakStmtNode struct {
		BaseNode
	}
	ReturnStmtNode struct {
		BaseNode
		Exprs *ExpressionList
	}
	RequireStmtNode struct {
		BaseNode
		Exprs *ExpressionList
	}
	ThrowStmtNode struct {
		BaseNode
		Exprs *ExpressionList
	}
	EmitStmtNode struct {
		BaseNode
		Expr *SubexprNode
	}
	GotoStmtNode struct {
		BaseNode
		Addrs *PrimaryList
	}
	CalcAssignStmtNode struct {
		BaseNode
		Lvalue *LvalueNode
		Op     string
		Expr   *ExpressionNode
	}
	ExprStmtNode struct {
		BaseNode
		Expr *ExpressionNode
	}
	FallbackStmtNode struct {
		BaseNode
	}
)

func (*StmtList) _statement()           {}
func (*StmtNode) _statement()           {}
func (*IfStmtNode) _statement()         {}
func (*IfNode) _statement()             {}
func (*ElseNode) _statement()           {}
func (*ElseIfList) _statement()         {}
func (*ElseIfNode) _statement()         {}
func (*WhileNode) _statement()          {}
func (*DoWhileNode) _statement()        {}
func (*ContinueStmtNode) _statement()   {}
func (*BreakStmtNode) _statement()      {}
func (*ReturnStmtNode) _statement()     {}
func (*RequireStmtNode) _statement()    {}
func (*ThrowStmtNode) _statement()      {}
func (*EmitStmtNode) _statement()       {}
func (*GotoStmtNode) _statement()       {}
func (*CalcAssignStmtNode) _statement() {}
func (*ExprStmtNode) _statement()       {}
func (*FallbackStmtNode) _statement()   {}

func NewStmtList(l *StmtList, n *StmtNode) *StmtList {
	if l == nil {
		l = new(StmtList)
		l.List = make([]*StmtNode, 0, 1)
		if n != nil {
			l.pos = n.pos
		}
	}
	if n != nil {
		l.List = append(l.List, n)
	}
	return l
}
func NewStmtNode(s interface{}) *StmtNode {
	return &StmtNode{BaseNode: BaseNode{pos: s.(AstNode).Pos()}, Internal: s}
}
func NewIfStmtNode(i *IfNode, ei *ElseIfList, e *ElseNode) *IfStmtNode {
	return &IfStmtNode{BaseNode: BaseNode{pos: i.pos}, IfN: i, Elif: ei, Els: e}
}
func NewIfNode(pos Pos, e *ExpressionNode, s *StmtBlockNode) *IfNode {
	return &IfNode{BaseNode: BaseNode{pos: pos}, Expr: e, Sb: s}
}
func (n *IfNode) SetElseCovered(f bool) { n.isElseCovered.Store(f) }
func (n *IfNode) GetElseCovered() bool {
	if n.isElseCovered.Load() == nil {
		return false
	} else {
		return n.isElseCovered.Load().(bool)
	}
}
func NewElseNode(pos Pos, s *StmtBlockNode) *ElseNode {
	return &ElseNode{BaseNode: BaseNode{pos: pos}, Sb: s}
}
func NewElseIfList(l *ElseIfList, n *ElseIfNode) *ElseIfList {
	if l == nil {
		l = new(ElseIfList)
		l.List = make([]*ElseIfNode, 0, 1)
	}
	l.List = append(l.List, n)
	return l
}
func NewElseIfNode(pos Pos, e *ExpressionNode, s *StmtBlockNode) *ElseIfNode {
	return &ElseIfNode{BaseNode: BaseNode{pos: pos}, Expr: e, Sb: s}
}
func NewWhileNode(pos Pos, e *ExpressionNode, s *StmtNode) *WhileNode {
	return &WhileNode{BaseNode: BaseNode{pos: pos}, Expr: e, Stmt: s}
}
func (n *WhileNode) SetOutCovered(f bool) { n.isOutCovered.Store(f) }
func (n *WhileNode) GetOutCovered() bool {
	if n.isOutCovered.Load() == nil {
		return false
	} else {
		return n.isOutCovered.Load().(bool)
	}
}
func NewDoWhileNode(pos Pos, s *StmtNode, e *ExpressionNode) *DoWhileNode {
	return &DoWhileNode{BaseNode: BaseNode{pos: pos}, Stmt: s, Expr: e}
}
func (n *DoWhileNode) SetOutCovered(f bool) { n.isOutCovered.Store(f) }
func (n *DoWhileNode) GetOutCovered() bool {
	if n.isOutCovered.Load() == nil {
		return false
	} else {
		return n.isOutCovered.Load().(bool)
	}
}
func NewContinueStmtNode(pos Pos) *ContinueStmtNode {
	return &ContinueStmtNode{BaseNode: BaseNode{pos: pos}}
}
func NewBreakStmtNode(pos Pos) *BreakStmtNode {
	return &BreakStmtNode{BaseNode: BaseNode{pos: pos}}
}
func NewReturnStmtNode(pos Pos, e *ExpressionList) *ReturnStmtNode {
	return &ReturnStmtNode{BaseNode: BaseNode{pos: pos}, Exprs: e}
}
func NewRequireStmtNode(pos Pos, e *ExpressionList) *RequireStmtNode {
	return &RequireStmtNode{BaseNode: BaseNode{pos: pos}, Exprs: e}
}
func NewThrowStmtNode(pos Pos, e *ExpressionList) *ThrowStmtNode {
	return &ThrowStmtNode{BaseNode: BaseNode{pos: pos}, Exprs: e}
}
func NewEmitStmtNode(pos Pos, e *SubexprNode) *EmitStmtNode {
	return &EmitStmtNode{BaseNode: BaseNode{pos: pos}, Expr: e}
}
func NewGotoStmtNode(pos Pos, a *PrimaryList) *GotoStmtNode {
	return &GotoStmtNode{BaseNode: BaseNode{pos: a.pos}, Addrs: a}
}
func NewCalcAssignStmtNode(l *LvalueNode, o string, e *ExpressionNode) *CalcAssignStmtNode {
	return &CalcAssignStmtNode{BaseNode: BaseNode{pos: l.pos}, Lvalue: l, Op: o, Expr: e}
}
func NewExprStmtNode(e *ExpressionNode) *ExprStmtNode {
	return &ExprStmtNode{BaseNode: BaseNode{pos: e.pos}, Expr: e}
}
func NewFallbackStmtNode(pos Pos) *FallbackStmtNode {
	return &FallbackStmtNode{BaseNode: BaseNode{pos: pos}}
}

type CastExpressionNode struct {
	BaseNode
	Typ  *TypeNameNode
	Expr *ExpressionNode
}

func NewCastExpressionNode(t *TypeNameNode, e *ExpressionNode) *CastExpressionNode {
	return &CastExpressionNode{BaseNode: BaseNode{pos: t.pos}, Typ: t, Expr: e}
}

type ExternalCallExprNode struct {
	BaseNode
	Lvalue   *LvalueNode
	CallId   string
	ExprList *ExpressionList
}

func NewExternalCallExprNode(l *LvalueNode, c string, e *ExpressionList) *ExternalCallExprNode {
	return &ExternalCallExprNode{BaseNode: BaseNode{pos: l.pos}, Lvalue: l, CallId: c, ExprList: e}
}

type ExtSpecifierList struct {
	BaseNode
	List []*ExtSpecifierNode
}

func NewExtSpecifierList(a *ExtSpecifierNode, b *ExtSpecifierNode) *ExtSpecifierList {
	spec := new(ExtSpecifierList)
	spec.List = make([]*ExtSpecifierNode, 0, 2)
	if a != nil {
		spec.pos = a.pos
		spec.List = append(spec.List, a)
	}
	if b != nil {
		spec.List = append(spec.List, b)
	}
	return spec
}

type ExtSpecifierNode struct {
	BaseNode
	Id   string
	Expr *ExpressionNode
}

func NewExtSpecifierNode(i string, e *ExpressionNode) *ExtSpecifierNode {
	return &ExtSpecifierNode{BaseNode: BaseNode{pos: e.pos}, Id: i, Expr: e}
}

type (
	ExpressionInterface interface {
		_expression()
	}
	ExpressionList struct {
		BaseNode
		List []*ExpressionNode
	}
	ExpressionNode struct {
		BaseNode
		Subexpr *SubexprNode
	}
	SubexprNode struct {
		BaseNode
		Exprs       *ExpressionList //assignExpr
		Left, Right *SubexprNode
		Op          string
		IsUnary     bool
		Rvalue      *RvalueNode
	}
	RvalueNode struct {
		BaseNode
		Cast    *CastExpressionNode
		ExtCall *ExternalCallExprNode
		ExtSpec *ExtSpecifierList
		Lvalue  *LvalueNode
	}
	LvalueNode struct {
		BaseNode
		AllocLoc   string
		Lval       *LvalueNode
		AllocSize  *AllocSizeNode
		ExprList   *ExpressionList
		Member     string
		Primary    *PrimaryNode
		Cast       *CastExpressionNode
		Expr       *ExpressionNode
		Typ        *TypeNameNode
		LvalueType NodeType
	}
)

func (*ExpressionList) _expression() {}
func (*ExpressionNode) _expression() {}
func (*SubexprNode) _expression()    {}
func (*RvalueNode) _expression()     {}
func (*LvalueNode) _expression()     {}

func (n *LvalueNode) String() string {
	u := Unparser{}
	u.ActLvalue(n)
	return u.unparsed
}
func (n *LvalueNode) Change(str string) {
	if n.Lval != nil {
		n.Lval.Primary.Val = str
	} else {
		n.Primary.Val = str
	}
}
func NewExpressionList(l *ExpressionList, n *ExpressionNode) *ExpressionList {
	if l == nil {
		l = new(ExpressionList)
		l.List = make([]*ExpressionNode, 0, 1)
		if n != nil {
			l.pos = n.pos
		}
	}
	if n != nil {
		l.List = append(l.List, n)
	}
	return l
}
func NewExpressionNode(s *SubexprNode) *ExpressionNode {
	return &ExpressionNode{BaseNode: BaseNode{pos: s.pos}, Subexpr: s}
}
func NewAssignExprNode(el *ExpressionList, e *SubexprNode) *SubexprNode {
	return &SubexprNode{BaseNode: BaseNode{pos: el.pos}, Exprs: el, Op: "=", Right: e}
}
func NewOpExprNode(l *SubexprNode, o string, r *SubexprNode) *SubexprNode {
	return &SubexprNode{BaseNode: BaseNode{pos: l.pos}, Left: l, Op: o, Right: r}
}
func NewUnaryExprNode(o string, r *RvalueNode) *SubexprNode {
	return &SubexprNode{BaseNode: BaseNode{pos: r.pos}, IsUnary: true, Op: o, Rvalue: r}
}
func NewAllocRvalueNode(c *CastExpressionNode) *RvalueNode {
	return &RvalueNode{BaseNode: BaseNode{pos: c.pos}, Cast: c}
}
func NewCallRvalueNode(c *ExternalCallExprNode, s *ExtSpecifierList) *RvalueNode {
	return &RvalueNode{BaseNode: BaseNode{pos: c.pos}, ExtCall: c, ExtSpec: s}
}
func NewRvalueNode(l *LvalueNode) *RvalueNode {
	return &RvalueNode{BaseNode: BaseNode{pos: l.pos}, Lvalue: l}
}
func NewMemAllocNode(a *AllocSizeNode) *LvalueNode {
	return &LvalueNode{BaseNode: BaseNode{pos: a.pos}, AllocLoc: "MEM", AllocSize: a, LvalueType: TypeAlloc}
}
func NewStorageAllocNode(a *AllocSizeNode) *LvalueNode {
	return &LvalueNode{BaseNode: BaseNode{pos: a.pos}, AllocLoc: "STORAGE", AllocSize: a, LvalueType: TypeAlloc}
}
func NewLvalueAllocNode(l *LvalueNode, a *AllocSizeNode) *LvalueNode {
	return &LvalueNode{BaseNode: BaseNode{pos: l.pos}, Lval: l, AllocSize: a, LvalueType: TypeAlloc}
}
func NewFuntionCallNode(l *LvalueNode, e *ExpressionList) *LvalueNode {
	return &LvalueNode{BaseNode: BaseNode{pos: l.pos}, Lval: l, ExprList: e, LvalueType: TypeCall}
}
func NewMemberAccessNode(l *LvalueNode, s string) *LvalueNode {
	return &LvalueNode{BaseNode: BaseNode{pos: l.pos}, Lval: l, Member: s, LvalueType: TypeAccess}
}
func NewPrimaryLvalueNode(p *PrimaryNode) *LvalueNode {
	return &LvalueNode{BaseNode: BaseNode{pos: p.pos}, Primary: p, LvalueType: TypePrimary}
}
func NewCastLvalueNode(c *CastExpressionNode) *LvalueNode {
	return &LvalueNode{BaseNode: BaseNode{pos: c.pos}, Cast: c, LvalueType: TypeCast}
}
func NewExprLvalueNode(e *ExpressionNode) *LvalueNode {
	return &LvalueNode{BaseNode: BaseNode{pos: e.pos}, Expr: e, LvalueType: TypeExpr}
}
func NewTypeLvalueNode(t *TypeNameNode) *LvalueNode {
	return &LvalueNode{BaseNode: BaseNode{pos: t.pos}, Typ: t, LvalueType: TypeType}
}

type AllocSizeNode struct {
	BaseNode
	From, Length, To *ExpressionNode
}

func NewAllocSizeNode(f *ExpressionNode, l *ExpressionNode, t *ExpressionNode) *AllocSizeNode {
	return &AllocSizeNode{BaseNode: BaseNode{pos: f.pos}, From: f, Length: l, To: t}
}

type PrimaryList struct {
	BaseNode
	List []*PrimaryNode
}

func NewPrimaryList(l *PrimaryList, n *PrimaryNode) *PrimaryList {
	if l == nil {
		l = new(PrimaryList)
		l.List = make([]*PrimaryNode, 0, 1)
		l.pos = n.pos
	}
	l.List = append(l.List, n)
	return l
}
func (l *PrimaryList) Get(i int) *PrimaryNode {
	return l.List[i]
}

type PrimaryNode struct {
	BaseNode
	Typ NodeType
	Val string
}

func NewPrimaryNode(pos Pos, v string, t NodeType) *PrimaryNode {
	return &PrimaryNode{BaseNode: BaseNode{pos: pos}, Val: v, Typ: t}
}

type TokenNode struct {
	BaseNode
	Val string
}

func NewTokenNode(pos Pos, v string) *TokenNode {
	return &TokenNode{BaseNode: BaseNode{pos: pos}, Val: v}
}

type CommentNode string
