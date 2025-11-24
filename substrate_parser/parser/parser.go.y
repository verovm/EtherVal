%{
   package parser
   import . "github.com/ethereum/go-ethereum/substrate_parser/ast"
%}
%token COMMENT
%token MESSAGE
%token GOTOADDRESS
%token BOOLLITERAL
%token NUM
%token HEX
%token ID
%token MAPPING
%token STORAGET
%token BOOL UINT INT BYTE ADDRESS STRING ARRAY STRUCT
%token FUNCTION
%token MEM STORAGE //LEN
%token IF ELSEIF ELSE WHILE DO BREAK CONTINUE RETURN THIS
%token NEW EMIT THROW REQUIRE GOTO
%token PUBLIC PRIVATE PAYABLE NONPAYABLE
%token MSGGAS MSGVAL MSGDATA MSGSENDER
%token ADD SUB NEG NOT EXP MUL DIV MOD SL SR AND XOR OR LT GT LE GE EQ NEQ
%token LOGICALAND LOGICALOR ASSIGN ASSIGNADD ASSIGNSUB ASSIGNMUL ASSIGNDIV ARROW

%right ASSIGN
%left LOGICALOR
%left LOGICALAND
%left EQ NEQ
%left LT GT LE GE
%left OR
%left XOR
%left AND
%left SL SR
%left ADD SUB
%left MUL DIV MOD
%right EXP

%start substrate
%union{
   typ int
   val string
   pos position
   node AstNode
   list AstNode
}

%type <val> COMMENT
%type <val> SEMICOLON
%type <val> ID
%type <pos> EMPTY
%type <pos> mappingToken functionToken ifToken elseToken elseifToken whileToken doToken
%type <pos> continueToken breakToken returnToken requireToken throwToken emitToken gotoToken    
%type <list> storageDecls structFields structDefs eventDecls functionDefs
%type <node> storageDecl structField structDef eventDecl functionDef
%type <list> formalArgs
%type <node> formalArg
%type <list> typeNames
%type <node> typeName basicType structType mapType structArgType
%type <node> primary signature
%type <node> accessModifier payability
%type <list> stmts
%type <node> stmtBlock stmt ifStmt whileStmt dowhileStmt continueStmt breakStmt 
%type <node> calcAssignStmt exprStmt returnStmt requireStmt throwStmt emitStmt gotoStmt fallbackStmt
%type <list> elseifs
%type <node> if elseif else
%type <list> jumpAddrs
%type <node> jumpAddr
%type <list> expressionList expressions
%type <val> assignOps equal compare shift opL opM opH
%type <list> extSpecifiers
%type <node> castExpression externalCallExpr extSpecifier
%type <node> expression assignExpr logicalOrExpr logicalAndExpr equalityExpr compareExpr
%type <node> bitwiseOrExpr bitwiseXorExpr bitwiseAndExpr shiftExpr addExpr multExpr expExpr unaryExpr
%type <node> rvalue lvalue allocSize
%%
functionToken : FUNCTION {$$ = buffer.pos} 
mappingToken  : MAPPING {$$ = buffer.pos}
ifToken       : IF {$$ = buffer.pos}
elseToken     : ELSE {$$ = buffer.pos}
elseifToken   : ELSEIF {$$ = buffer.pos}
whileToken    : WHILE {$$ = buffer.pos}
doToken       : DO {$$ = buffer.pos}
continueToken : CONTINUE {$$ = buffer.pos}
breakToken    : BREAK {$$ = buffer.pos}
returnToken   : RETURN {$$ = buffer.pos}
requireToken  : REQUIRE {$$ = buffer.pos}
throwToken    : THROW {$$ = buffer.pos}
emitToken     : EMIT {$$ = buffer.pos}
gotoToken     : GOTO {$$ = buffer.pos}

SEMICOLON
   : ';' COMMENT {
      $$ = buffer.val
   }
   | ';' {
      $$ = ""
   }

EMPTY : '(' ')' { $$ = buffer.pos }

substrate
   : storageDecls structDefs eventDecls functionDefs {
      Itemlex.(*Lexer).Root = BuildAST($1.(*StorageDeclList), $2.(*StructDefList), $3.(*EventDeclList), $4.(*FunctionDefList))
   }
   | storageDecls structDefs functionDefs {
      Itemlex.(*Lexer).Root = BuildAST($1.(*StorageDeclList), $2.(*StructDefList), nil, $3.(*FunctionDefList))
   }
   | storageDecls eventDecls functionDefs {
      Itemlex.(*Lexer).Root = BuildAST($1.(*StorageDeclList), nil, $2.(*EventDeclList), $3.(*FunctionDefList))
   }
   | structDefs eventDecls functionDefs {
      Itemlex.(*Lexer).Root = BuildAST(nil, $1.(*StructDefList), $2.(*EventDeclList), $3.(*FunctionDefList))
   }
   | storageDecls functionDefs {
      Itemlex.(*Lexer).Root = BuildAST($1.(*StorageDeclList), nil, nil, $2.(*FunctionDefList))
   }
   | structDefs functionDefs {
      Itemlex.(*Lexer).Root = BuildAST(nil, $1.(*StructDefList), nil, $2.(*FunctionDefList))
   }
   | eventDecls functionDefs {
      Itemlex.(*Lexer).Root = BuildAST(nil, nil, $1.(*EventDeclList), $2.(*FunctionDefList))
   }
   | functionDefs {
      Itemlex.(*Lexer).Root = BuildAST(nil, nil, nil, $1.(*FunctionDefList))
   }

storageDecls
   : storageDecls storageDecl {
      $$ = NewStorageDeclList($1.(*StorageDeclList), $2.(*StorageDeclNode))
   }
   | storageDecl {
      $$ = NewStorageDeclList(nil, $1.(*StorageDeclNode))
   }
storageDecl
   : typeName primary SEMICOLON {
      $$ = NewStorageDeclNode($1.(*TypeNameNode), $2.(*PrimaryNode), $3)
   }

structFields
   : structFields structField {
      $$ = NewStructFieldList($1.(*StructFieldList), $2.(*StructFieldNode))
   }
   | structField {
      $$ = NewStructFieldList(nil, $1.(*StructFieldNode))
   }
structField
   : typeName primary SEMICOLON {
      $$ = NewStructFieldNode($1.(*TypeNameNode), $2.(*PrimaryNode))
   }

structDefs
   : structDefs structDef {
      $$ = NewStructDefList($1.(*StructDefList), $2.(*StructDefNode))
   }
   | structDef {
      $$ = NewStructDefList(nil, $1.(*StructDefNode))
   }
structDef
   : structType typeName '{' structFields '}' SEMICOLON {
      $$ = NewStructDefNode($2.(*TypeNameNode), $4.(*StructFieldList))
   }


eventDecls
   : eventDecls eventDecl {
      $$ = NewEventDeclList($1.(*EventDeclList), $2.(*EventDeclNode))
   }
   | eventDecl {
      $$ = NewEventDeclList(nil, $1.(*EventDeclNode))
   }
eventDecl
   : primary '(' typeNames ')' SEMICOLON {
      t := $1.(*PrimaryNode).Typ //primary must be ID
      if t != TypeID {
         Itemlex.Error("Primary is not ID in EventDecl")
         return 1
      }
      $$ = NewEventDeclNode($1.(*PrimaryNode), $3.(*TypeNameList))
   }
   | primary SEMICOLON {
      t := $1.(*PrimaryNode).Typ //primary must be HEX
      if t != TypeHEX {
         Itemlex.Error("Primary is not HEX in EventDecl")
         return 1
      }
      $$ = NewEventDeclNode($1.(*PrimaryNode), nil)
   }

functionDefs
   : functionDefs functionDef {
      $$ = NewFunctionDefList($1.(*FunctionDefList), $2.(*FunctionDefNode))
   }
   | functionDef {
      $$ = NewFunctionDefList(nil, $1.(*FunctionDefNode))
   }
functionDef
   : functionToken signature '(' formalArgs ')' accessModifier payability stmtBlock {
      $$ = NewFunctionDefNode(setPos($1), $2.(*PrimaryNode), $4.(*FormalArgList),
            $6.(*TokenNode), $7.(*TokenNode), $8.(*StmtBlockNode))
   }
   | functionToken EMPTY accessModifier payability stmtBlock {
      fallback := NewPrimaryNode(setPos($2), "()", TypeID)
      tmp := NewFormalArgList(nil, nil)
      $$ = NewFunctionDefNode(setPos($1), fallback, tmp,
            $3.(*TokenNode), $4.(*TokenNode), $5.(*StmtBlockNode))
   }

signature : ID  {$$ = NewPrimaryNode(setPos(buffer.pos), buffer.val, TypeID)}
          | HEX {$$ = NewPrimaryNode(setPos(buffer.pos), buffer.val, TypeHEX)}
formalArgs
   : formalArgs ',' formalArg {
      $$ = NewFormalArgList($1.(*FormalArgList), $3.(*FormalArgNode))
   }
   | formalArg {
      $$ = NewFormalArgList(nil, $1.(*FormalArgNode))
   }
   | {
      $$ = NewFormalArgList(nil, nil)
   }

formalArg
   : typeName ID {
      tmp := NewPrimaryNode(setPos(buffer.pos), buffer.val, TypeID)
      $$ = NewFormalArgNode($1.(*TypeNameNode), tmp)
   }
   | lvalue ID {
      tmp := NewPrimaryNode(setPos(buffer.pos), buffer.val, TypeID)
      if $1.(*LvalueNode).LvalueType == TypePrimary {    //LA(1) makes trouble when two IDs occurs in a row
         $1.(*LvalueNode).Change(prevBuffer.val)
      }
      $$ = NewFormalArgNode(NewTypeNameNode(NewLibraryTypeNode($1.(*LvalueNode), false, false)), tmp)
   }
   |  ID {
      tmp := NewPrimaryNode(setPos(buffer.pos), buffer.val, TypeID)
      $$ = NewFormalArgNode(nil, tmp)
   }

basicType
   : UINT       {$$ = NewBasicTypeNode(setPos(buffer.pos), buffer.val, TypeUINT)}
   | INT        {$$ = NewBasicTypeNode(setPos(buffer.pos), buffer.val, TypeINT)}
   | BYTE       {$$ = NewBasicTypeNode(setPos(buffer.pos), buffer.val, TypeBYTE)}
   | ADDRESS    {$$ = NewBasicTypeNode(setPos(buffer.pos), buffer.val, TypeADDRESS)}
   | STRING     {$$ = NewBasicTypeNode(setPos(buffer.pos), buffer.val, TypeSTRING)}
   | ARRAY      {$$ = NewBasicTypeNode(setPos(buffer.pos), buffer.val, TypeARRAY)}
   | BOOL       {$$ = NewBasicTypeNode(setPos(buffer.pos), buffer.val, TypeBOOL)}
   | FUNCTION   {$$ = NewBasicTypeNode(setPos(buffer.pos), buffer.val, TypeFUNCTION)}
structType
   : STRUCT     {$$ = NewBasicTypeNode(setPos(buffer.pos), buffer.val, TypeSTRUCT)}
mapType
   : mappingToken '(' basicType ARROW typeName ')' {
      $$ = NewMapTypeNode(setPos($1), $3.(*BasicTypeNode), $5.(*TypeNameNode))
   }
   | mappingToken '(' basicType ARROW '[' typeName ']' ')' {
      $$ = NewMapTypeNode(setPos($1), $3.(*BasicTypeNode), $6.(*TypeNameNode))
   }
structArgType  //used in function selector
   : '(' typeNames ')' {
      $$ = NewStructArgTypeNode(setPos(buffer.pos), $2.(*TypeNameList))
   }

typeName
   : basicType {
      $$ = NewTypeNameNode($1)
   }
   | mapType {
      $$ = NewTypeNameNode($1)
   }
   | structType {
      $$ = NewTypeNameNode($1)
   }
   | structArgType {
      $$ = NewTypeNameNode($1)
   }
   | typeName '[' ']' {
      $$ = NewTypeNameNode(NewArrayTypeNode($1.(*TypeNameNode), nil, false))
   }
   | typeName '[' ']' STORAGET {
      $$ = NewTypeNameNode(NewArrayTypeNode($1.(*TypeNameNode), nil, true))
   }
   | typeName allocSize {
      $$ = NewTypeNameNode(NewArrayTypeNode($1.(*TypeNameNode), $2.(*AllocSizeNode), false))
   }
   | lvalue STORAGET {
      $$ = NewTypeNameNode(NewLibraryTypeNode($1.(*LvalueNode), false, true))
   }
   | lvalue '[' ']' STORAGET {
      $$ = NewTypeNameNode(NewLibraryTypeNode($1.(*LvalueNode), true, true))
   }
typeNames
   : typeNames ',' typeName  {
      $$ = NewTypeNameList($1.(*TypeNameList), $3.(*TypeNameNode))
   }
   | typeName  {
      $$ = NewTypeNameList(nil, $1.(*TypeNameNode))
   }
   | {
      $$ = NewTypeNameList(nil, nil)
   }

stmtBlock : '{' stmts '}' {$$ = NewStmtBlockNode($2.(*StmtList))}
stmts
   : stmts stmt {
      $$ = NewStmtList($1.(*StmtList), $2.(*StmtNode))
   }
   | stmt {
      $$ = NewStmtList(nil, $1.(*StmtNode))
   }
   | {
      $$ = NewStmtList(nil, nil)
   }
stmt
   : ifStmt          {$$ = NewStmtNode($1)}
   | whileStmt       {$$ = NewStmtNode($1)}
   | dowhileStmt     {$$ = NewStmtNode($1)}
   | stmtBlock       {$$ = NewStmtNode($1)}
   | continueStmt    {$$ = NewStmtNode($1)}
   | breakStmt       {$$ = NewStmtNode($1)}
   | calcAssignStmt  {$$ = NewStmtNode($1)}
   | exprStmt        {$$ = NewStmtNode($1)}
   | returnStmt      {$$ = NewStmtNode($1)}
   | requireStmt     {$$ = NewStmtNode($1)}
   | throwStmt       {$$ = NewStmtNode($1)}
   | emitStmt        {$$ = NewStmtNode($1)}
   | gotoStmt        {$$ = NewStmtNode($1)}
   | fallbackStmt    {$$ = NewStmtNode($1)}
   | COMMENT         {$$ = NewStmtNode(NewTokenNode(setPos(buffer.pos), buffer.val))}

ifStmt
   : if elseifs else {
      $$ = NewIfStmtNode($1.(*IfNode), $2.(*ElseIfList), $3.(*ElseNode))
   }
   | if elseifs {
      $$ = NewIfStmtNode($1.(*IfNode), $2.(*ElseIfList), nil)
   }
   | if else {
      $$ = NewIfStmtNode($1.(*IfNode), nil, $2.(*ElseNode))
   }
   | if {
      $$ = NewIfStmtNode($1.(*IfNode), nil, nil) 
   }
if
   : ifToken '(' expression ')' stmtBlock {
      $$ = NewIfNode(setPos($1), $3.(*ExpressionNode), $5.(*StmtBlockNode))
   }
else
   : elseToken stmtBlock {
      $$ = NewElseNode(setPos($1), $2.(*StmtBlockNode))
   }
elseifs
   : elseifs elseif {
      $$ = NewElseIfList($1.(*ElseIfList), $2.(*ElseIfNode))
   }
   | elseif {
      $$ = NewElseIfList(nil, $1.(*ElseIfNode))
   }
elseif
   : elseifToken '(' expression ')' stmtBlock {
      $$ = NewElseIfNode(setPos($1), $3.(*ExpressionNode), $5.(*StmtBlockNode))
   }
   
whileStmt
   : whileToken '(' expression ')' stmt {
      $$ = NewWhileNode(setPos($1), $3.(*ExpressionNode), $5.(*StmtNode))
   }
dowhileStmt
   : doToken stmt WHILE '(' expression ')' SEMICOLON {
      $$ = NewDoWhileNode(setPos($1), $2.(*StmtNode), $5.(*ExpressionNode))
   }
continueStmt : continueToken SEMICOLON {
      $$ = NewContinueStmtNode(setPos($1))
   }
breakStmt : breakToken SEMICOLON {
      $$ = NewBreakStmtNode(setPos($1))
   }
returnStmt
   : returnToken '(' expressionList ')' SEMICOLON {
      $$ = NewReturnStmtNode(setPos($1), $3.(*ExpressionList))
   }
   | returnToken expressionList SEMICOLON  {
      $$ = NewReturnStmtNode(setPos($1), $2.(*ExpressionList))
   }
requireStmt
   : requireToken '(' expressionList ')' SEMICOLON {
         //requireArgs : expression ',' ID | expression ',' MESSAGE | expression
      $$ = NewRequireStmtNode(setPos($1), $3.(*ExpressionList))
   }
throwStmt
   : throwToken '(' expressionList ')' SEMICOLON {
      $$ = NewThrowStmtNode(setPos($1), $3.(*ExpressionList))
   }
emitStmt
   : emitToken logicalOrExpr SEMICOLON {
      $$ = NewEmitStmtNode(setPos($1), $2.(*SubexprNode))
   }
gotoStmt
   : gotoToken jumpAddr SEMICOLON {
      $$ = NewGotoStmtNode(setPos($1), $2.(*PrimaryList))
   }
   | gotoToken '{'  jumpAddrs '}' SEMICOLON {
      $$ = NewGotoStmtNode(setPos($1), $3.(*PrimaryList))
   }
jumpAddrs
   : jumpAddrs ',' jumpAddr {
      $$ = NewPrimaryList($1.(*PrimaryList), $3.(*PrimaryList).Get(0))
   }
   | jumpAddr {
      $$ = $1.(*PrimaryList)
   }
jumpAddr
   : HEX {
      tmp := NewPrimaryNode(setPos(buffer.pos), buffer.val, TypeHEX)
      $$ = NewPrimaryList(nil, tmp)
   }
   | GOTOADDRESS {
      tmp := NewPrimaryNode(setPos(buffer.pos), buffer.val, TypeGOTOADDRESS)
      $$ = NewPrimaryList(nil, tmp)
   }
   | MESSAGE {//MESSAGE must be '0x...'
      tmp := NewPrimaryNode(setPos(buffer.pos), buffer.val, TypeMESSAGE)
      $$ = NewPrimaryList(nil, tmp)
   }
calcAssignStmt : lvalue assignOps expression SEMICOLON {
      $$ = NewCalcAssignStmtNode($1.(*LvalueNode), $2, $3.(*ExpressionNode))
   }
exprStmt : expression SEMICOLON {
      $$ = NewExprStmtNode($1.(*ExpressionNode))
   }
fallbackStmt : EMPTY SEMICOLON {
      $$ = NewFallbackStmtNode(setPos($1))
}

castExpression
   : typeName '(' expression ')' {
      $$ = NewCastExpressionNode($1.(*TypeNameNode), $3.(*ExpressionNode))
   }
externalCallExpr
   : lvalue '.' ID {$3 = buffer.val} '(' expressionList ')' {       //distinguish call & staticcall from ID
                   //lvalue => castExpression | ID | HEX | MSGSENDER
      $$ = NewExternalCallExprNode($1.(*LvalueNode), $3, $6.(*ExpressionList))
   }
extSpecifiers
   : extSpecifier extSpecifier {
      $$ = NewExtSpecifierList($1.(*ExtSpecifierNode), $2.(*ExtSpecifierNode))
   }
   | extSpecifier {
      $$ = NewExtSpecifierList($1.(*ExtSpecifierNode), nil)
   }
   | {
      $$ = NewExtSpecifierList(nil, nil)
   }
extSpecifier
   : '.' ID {$2 = buffer.val} '(' expression ')' {
      $$ = NewExtSpecifierNode($2, $5.(*ExpressionNode))
   }

expressionList
   : expressions { $$ = $1 }
   | {
      $$ = NewExpressionList(nil, nil)
   }
expressions
   : expressions ',' expression {
      $$ = NewExpressionList($1.(*ExpressionList), $3.(*ExpressionNode))
   }
   | expression {
      $$ = NewExpressionList(nil, $1.(*ExpressionNode))
   }

assignOps : ASSIGNADD {$$ = "+="} | ASSIGNSUB {$$ = "-="} | ASSIGNMUL {$$ = "*="} | ASSIGNDIV {$$ = "/="}
equal : EQ {$$ = "=="} | NEQ {$$ = "!="}
compare : LT {$$ = "<"} | GT {$$ = ">"} | LE {$$ = "<="} | GE {$$ = ">="}
shift : SL {$$ = "<<"} | SR {$$ = ">>"}
opL : ADD {$$ = "+"} | SUB {$$ = "-"}
opM : MUL {$$ = "*"} | DIV {$$ = "/"} | MOD {$$ = "%"}
opH : EXP {$$ = "**"}

expression : assignExpr {$$ = NewExpressionNode($1.(*SubexprNode))}
assignExpr
   : logicalOrExpr {$$ = $1}
   | expressions ASSIGN assignExpr {
      $$ = NewAssignExprNode($1.(*ExpressionList), $3.(*SubexprNode))
   }
logicalOrExpr
   : logicalAndExpr {$$ = $1}
   | logicalOrExpr LOGICALOR logicalAndExpr {
      $$ = NewOpExprNode($1.(*SubexprNode), "||", $3.(*SubexprNode))
   }
logicalAndExpr
   : equalityExpr {$$ = $1}
   | logicalAndExpr LOGICALAND equalityExpr {
      $$ = NewOpExprNode($1.(*SubexprNode), "&&", $3.(*SubexprNode))
   }
equalityExpr
   : compareExpr {$$ = $1}
   | equalityExpr equal compareExpr {
      $$ = NewOpExprNode($1.(*SubexprNode), $2, $3.(*SubexprNode))
   }
compareExpr
   : bitwiseOrExpr {$$ = $1}
   | compareExpr compare bitwiseOrExpr {
      $$ = NewOpExprNode($1.(*SubexprNode), $2, $3.(*SubexprNode))
   }
bitwiseOrExpr
   : bitwiseXorExpr {$$ = $1}
   | bitwiseOrExpr OR bitwiseXorExpr {
      $$ = NewOpExprNode($1.(*SubexprNode), "|", $3.(*SubexprNode))
   }
bitwiseXorExpr
   : bitwiseAndExpr {$$ = $1}
   | bitwiseXorExpr XOR bitwiseAndExpr {
      $$ = NewOpExprNode($1.(*SubexprNode), "^", $3.(*SubexprNode))
   }
bitwiseAndExpr
   : shiftExpr {$$ = $1}
   | bitwiseAndExpr AND shiftExpr {
      $$ = NewOpExprNode($1.(*SubexprNode), "&", $3.(*SubexprNode))
   }
shiftExpr
   : addExpr {$$ = $1}
   | shiftExpr shift addExpr {
      $$ = NewOpExprNode($1.(*SubexprNode), $2, $3.(*SubexprNode))
   }
addExpr
   : multExpr {$$ = $1}
   | addExpr opL multExpr {
      $$ = NewOpExprNode($1.(*SubexprNode), $2, $3.(*SubexprNode))
   }
multExpr
   : expExpr {$$ = $1}
   | multExpr opM expExpr {
      $$ = NewOpExprNode($1.(*SubexprNode), $2, $3.(*SubexprNode))
   }
expExpr
   : unaryExpr {$$ = $1}
   | unaryExpr opH expExpr {
      $$ = NewOpExprNode($1.(*SubexprNode), $2, $3.(*SubexprNode))
   }
unaryExpr
   : ADD rvalue %prec '*' {
      $$ = NewUnaryExprNode("+", $2.(*RvalueNode))
   }
   | SUB rvalue %prec '*' {
      $$ = NewUnaryExprNode("-", $2.(*RvalueNode))
   }
   | NOT rvalue {
      $$ = NewUnaryExprNode("!", $2.(*RvalueNode))
   }
   | NEG rvalue {
      $$ = NewUnaryExprNode("~", $2.(*RvalueNode))
   }
   | rvalue {
      $$ = NewUnaryExprNode("", $1.(*RvalueNode))
   }

rvalue
   : NEW castExpression {
      $$ = NewAllocRvalueNode($2.(*CastExpressionNode))
   }
   | externalCallExpr extSpecifiers {
      $$ = NewCallRvalueNode($1.(*ExternalCallExprNode), $2.(*ExtSpecifierList))
   }
   | lvalue {
      $$ = NewRvalueNode($1.(*LvalueNode))
   }

lvalue
   : MEM allocSize {
      $$ = NewMemAllocNode($2.(*AllocSizeNode))
   }
   | STORAGE allocSize {
      $$ = NewStorageAllocNode($2.(*AllocSizeNode))
   }
   | lvalue allocSize {
      $$ = NewLvalueAllocNode($1.(*LvalueNode), $2.(*AllocSizeNode))
   }
   | lvalue '(' expressionList ')' {     //all or none for typeName
      $$ = NewFuntionCallNode($1.(*LvalueNode), $3.(*ExpressionList))
   }
   | lvalue '.' ID {
      $$ = NewMemberAccessNode($1.(*LvalueNode), buffer.val)
   }
   | lvalue '.' THIS {
      $$ = NewMemberAccessNode($1.(*LvalueNode), "this")
   }
   | primary {
      $$ = NewPrimaryLvalueNode($1.(*PrimaryNode))
   }
   | castExpression {
      $$ = NewCastLvalueNode($1.(*CastExpressionNode))
   }
   | '(' expression ')' {
      $$ = NewExprLvalueNode($2.(*ExpressionNode))
   }
   | typeName {
      $$ = NewTypeLvalueNode($1.(*TypeNameNode))
   }

allocSize
   : '[' expression ']' {
      $$ = NewAllocSizeNode($2.(*ExpressionNode), nil, nil)
   }
   | '[' expression ID expression ']' { //ID must be 'len'
      $$ = NewAllocSizeNode($2.(*ExpressionNode), $4.(*ExpressionNode), nil)
   }
   | '[' expression ':' expression ']' {
      $$ = NewAllocSizeNode($2.(*ExpressionNode), nil, $4.(*ExpressionNode))
   }

primary
   : BOOLLITERAL{$$ = NewPrimaryNode(setPos(buffer.pos), buffer.val, TypeBOOLLITERAL)}
   | NUM        {$$ = NewPrimaryNode(setPos(buffer.pos), buffer.val, TypeNUM)}
   | HEX        {$$ = NewPrimaryNode(setPos(buffer.pos), buffer.val, TypeHEX)}
   | MESSAGE    {$$ = NewPrimaryNode(setPos(buffer.pos), buffer.val, TypeMESSAGE)}
   | MSGVAL     {$$ = NewPrimaryNode(setPos(buffer.pos), buffer.val, TypeMSGVAL)}
   | MSGDATA    {$$ = NewPrimaryNode(setPos(buffer.pos), buffer.val, TypeMSGDATA)}
   | MSGGAS     {$$ = NewPrimaryNode(setPos(buffer.pos), buffer.val, TypeMSGGAS)}
   | MSGSENDER  {$$ = NewPrimaryNode(setPos(buffer.pos), buffer.val, TypeMSGSENDER)}
   | THIS       {$$ = NewPrimaryNode(setPos(buffer.pos), buffer.val, TypeTHIS)}
   | ID         {$$ = NewPrimaryNode(setPos(buffer.pos), buffer.val, TypeID)}
   | '?'        {$$ = NewPrimaryNode(setPos(buffer.pos), buffer.val, TypeNA)}

accessModifier
   : PUBLIC  {$$ = NewTokenNode(setPos(buffer.pos), buffer.val)}
   | PRIVATE {$$ = NewTokenNode(setPos(buffer.pos), buffer.val)}
payability
   : PAYABLE    {$$ = NewTokenNode(setPos(buffer.pos), buffer.val)}
   | NONPAYABLE {$$ = NewTokenNode(setPos(buffer.pos), buffer.val)}
   |            {$$ = NewTokenNode(NewPos(0,0), "")}

%%
type Token struct {
   val string
   pos position
}
func setPos(p position) Pos {
   return NewPos(p.line, p.column)
}
