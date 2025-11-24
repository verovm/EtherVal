package ast

//import "fmt"
//import "encoding/json"

type Input struct {
   Name string `json:"name,omitempty"`
   Type string `json:"type"`
   Out  []interface{} `json:"out,omitempty"`
}

type Function struct {
   Inputs []Input `json:"inputs,omitempty"`
   Name string `json:"name,omitempty"`
   Type string `json:"type"`
   Payable bool `json:"payable"`
   Outputs []Input `json:"outputs,omitempty"`
   Constant string `json:"constant,omitempty"`
}

type SubstrateAbi struct {
   funcs []*Function
}

func getFunctionAbi(fNode *FunctionDefNode) *Function {
   abi := new(Function)
   abi.Name = fNode.Name.Val
   abi.Type = "function"
   if fNode.Pay.Val == "payable" {
      abi.Payable = true
   } else {
      abi.Payable = false
   }
   for _, args := range fNode.Args.List {
      abi.Inputs = append(abi.Inputs, Input{Type:args.Typ.String()})
   }
   return abi
}

func (a *SubstrateAbi) SetAbis(root *SubstrateNode) {
   for _, fNode := range root.FList.List {
      if fNode.Name.Val == "__function_selector__" || fNode.Name.Val == "" {
         continue
      }
      
      fAbi := getFunctionAbi(fNode)
      a.funcs = append(a.funcs, fAbi)
      //fmt.Println(fAbi)
   }
}

func (a *SubstrateAbi) GetAbis() []*Function {
   return a.funcs
}

func (a *SubstrateAbi) GetAbi(root *SubstrateNode, sig string) *Function {
   for _, fNode := range root.FList.List {
      if fNode.Signature()[2:] == sig {
         return getFunctionAbi(fNode)
      }
   }
   return nil
}
