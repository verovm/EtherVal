package tac_parser

import (
	"fmt"
)

// RendarVar renders variable name with register number
func (f *TACFunction) RenderVar(id int) string {
	if id < 0 {
		return "void"
	}
	for name, idx := range f.VariableEncoding {
		if idx == id {
			return fmt.Sprintf("%s(r%d)", name, idx)
		}
	}
	panic("Cannot find encoding for variable")
}

// RenderAddress renders jump address with register number
func (f *TACFunction) RenderAddress(id int) string {
	if name, ok := f.BlockByIndex[id]; ok {
		return fmt.Sprintf("%s(r@%d)", name, id)
	}
	return "INVALID_DEST"
}

func (f *TACFunction) RenderConst(id int) string {
	for name, idx := range f.VariableEncoding {
		if idx == id {
			for _, sym := range f.ConstSym {
				if idx := sym.Index; idx == id {
					return fmt.Sprintf("%s(%s)", name, sym.Value.Hex())
				}
			}
		}
	}
	panic("Cannot find encoding for constant")
}

func (p *TACProgram) RenderFunction(id int) string {
	for name, idx := range p.FunctionEncoding {
		if idx == id {
			return name
		}
	}
	panic("Cannot find encoding for function")
}
