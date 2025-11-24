package ast

type NodeType int

const (
   NOTUSED NodeType = iota
//TypeNameNode
   TypeUINT
   TypeINT
   TypeBYTE
   TypeADDRESS
   TypeSTRING
   TypeARRAY
   TypeBOOL
   TypeFUNCTION
   TypeSTRUCT
//PrimaryNode
   TypeMESSAGE
   TypeGOTOADDRESS
   TypeBOOLLITERAL
   TypeNUM
   TypeHEX
   TypeMSGVAL
   TypeMSGDATA
   TypeMSGGAS
   TypeMSGSENDER
   TypeTHIS
   TypeID
   TypeNA
//LvalueNode
   TypeAlloc
   TypeCall
   TypeAccess
   TypePrimary
   TypeCast
   TypeExpr
   TypeType
)

type Value interface {
   _Value()
}
