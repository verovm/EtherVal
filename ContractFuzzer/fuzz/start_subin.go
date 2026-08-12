package fuzz

import (
	//   "log"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	//   abi_gen "github.com/seonghojj/ContractFuzzer/abi"
	"github.com/seonghojj/substrate_parser/ast"
)

var doFuzz = false
var checkCoverage = false
type Coverage struct {
   BranchList *[]ast.StmtInterface
   TxCount uint32
   ErrorCount uint32
   m *sync.Mutex
}

func (c *Coverage) AddErrorCount() {
   atomic.AddUint32(&c.ErrorCount, 1)
}
type Addr [20]byte
//var CoverageMap map[Addr]*Coverage
var CoverageMap *sync.Map
var mutex *sync.Mutex

type txKey struct {
	Block, TxIndex uint64
}
func TxKey(b, t uint64) txKey { return txKey{b, t} }

var TxMap map[uint64][]uint64  //block -> []txIndex
func NewTxMap() {
	TxMap = make(map[uint64][]uint64)
}

var SubstateMap map[txKey]interface{}
func NewSubstateMap() { 
	SubstateMap = make(map[txKey]interface{})
}

func Setup(f bool) {
	doFuzz = f
   checkCoverage = true
   CoverageMap = new(sync.Map)
   mutex = &sync.Mutex{}
	_, b, _, _ := runtime.Caller(0)
	basepath := filepath.Join(filepath.Dir(b), "../config")
	Global_addrSeed = filepath.Join(basepath, "addressSeed.json")
	Global_intSeed = filepath.Join(basepath, "intSeed.json")
	Global_uintSeed = filepath.Join(basepath, "uintSeed.json")
	Global_stringSeed = filepath.Join(basepath, "stringSeed.json")
	Global_byteSeed = filepath.Join(basepath, "byteSeed.json")
	Global_bytesSeed = filepath.Join(basepath, "bytesSeed.json")
	Global_addr_map = filepath.Join(basepath, "addrmap.csv")
}
func GetFlag() bool { return doFuzz }
func CheckCoverage() bool { return checkCoverage }

/*func NewCoverage(a Addr) *Coverage {
   mutex.Lock()
   CoverageMap[a] = new(Coverage)
   CoverageMap[a].BranchList = new([]ast.StmtInterface)
   CoverageMap[a].m = &sync.Mutex{}
   mutex.Unlock()
   return CoverageMap[a]
}*/
/*func NewCoverage(a Addr) *Coverage {
   c := new(Coverage)
   c.BranchList = new([]ast.StmtInterface)
   c.m = &sync.Mutex{}

   ret, loaded := CoverageMap.LoadOrStore(a, c)
   if !loaded && c == ret {
      println("Saved")
   }
   return ret.(*Coverage)
}*/
func GetCoverageMap(a Addr) *Coverage {
   var c *Coverage
   value, loaded := CoverageMap.Load(a)
   if !loaded {
      c = new(Coverage)
      c.BranchList = new([]ast.StmtInterface)
      c.m = &sync.Mutex{}

      ret, loaded := CoverageMap.LoadOrStore(a, c)
      if loaded {
         c = ret.(*Coverage)
      }
   } else {
      c = value.(*Coverage)
   }
   atomic.AddUint32(&c.TxCount, 1)
   return c
}

func GetFuzzInput(abiData *ast.Function) []byte {
	var ioput = new(IOput)
	inputData, _ := json.Marshal(abiData.Inputs)
	if err := json.Unmarshal(inputData, ioput); err != nil {
		fmt.Println(err)
	}
	//buf, _ := json.Marshal(ioput)
	//println(string(buf))
	//println(ioput.String())
	var fuzzedInput []byte
	if len(abiData.Inputs) > 0 {
		if ret, err := ioput.fuzz(); err == nil {
			//fmt.Printf("%s:[%s]\n", "Fuzzed", ret.(string))
			inputs := strings.Split(ret.(string), ",")
			for _, i := range inputs {
				str := strings.ReplaceAll(i, "\"", "")
				str = strings.ReplaceAll(str, "0x", "")
				bstr, _ := hex.DecodeString(str)
				/*if err != nil {
					fmt.Println(err)
				}*/
				b := make([]byte, 32)
				copy(b[32-len(bstr):], bstr)
				fuzzedInput = append(fuzzedInput, b[:]...)
			}
		} else {
			fmt.Println(err)
		}
	} else {
		//fmt.Println("ABI INPUT 0")
	}
	return fuzzedInput
}

func PutBranch(currNode ast.StmtInterface, cov *Coverage) {
   i := 0
   cov.m.Lock()
   defer cov.m.Unlock()
   for _, l := range(*cov.BranchList) {
      a, b := l.GetPos()
      c, d := currNode.GetPos()
      if a == c && b == d {
         break
      }
      i++
   }
   if i == len(*cov.BranchList) {
      *cov.BranchList = append(*cov.BranchList, currNode)
   }
}

var debug = false
//run only in single thread
func (c *Coverage) GetCoverage(node ast.StmtInterface) (int, int, int) {
   p := ast.Unparser{}
   BranchN, CoverN, Details := 2, 0, 0
   bit := 1
   switch node.(type) {
   case *ast.IfStmtNode:
      n := node.(*ast.IfStmtNode)
      if n.Elif != nil {
         BranchN = BranchN + len(n.Elif.List)
      }
      if n.IfN.GetCovered() { 
         if debug {
            ast.Act(&p, n.IfN.Expr)
            println("IfN " + p.GetUnparsed())
         }
         CoverN++
         Details += bit
      }
      bit *= 2
      if n.Elif != nil {
         for _, eli := range n.Elif.List {
            if eli.GetCovered() {
               if debug {
                  ast.Act(&p, n.IfN.Expr)
                  println("Elif " + p.GetUnparsed())
               }
               CoverN++
               Details += bit
            }
            bit *= 2
         }
      }
      if n.Els != nil {
         if n.Els.GetCovered() {
            if debug {
               ast.Act(&p, n.IfN.Expr)
               println("Els " + p.GetUnparsed())
            }
            CoverN++
            Details += bit
         }
      } else if n.Elif == nil {
         if n.IfN.GetElseCovered() {
            if debug {
               ast.Act(&p, n.IfN.Expr)
               println("No IfN " + p.GetUnparsed())
            }
            CoverN++
            Details += bit
         }
      }
   case *ast.WhileNode:
      n := node.(*ast.WhileNode)
      if n.GetCovered() {
         if debug {
            ast.Act(&p, n.Expr)
            println("While in" + p.GetUnparsed())
         }
         CoverN++
         Details += bit
      }
      bit *= 2
      if n.GetOutCovered() {
         if debug {
            ast.Act(&p, n.Expr)
            println("While out " + p.GetUnparsed())
         }
         CoverN++
         Details += bit
      }
   case *ast.DoWhileNode:
      n := node.(*ast.DoWhileNode)
      if n.GetCovered() {
         if debug {
            ast.Act(&p, n.Expr)
            println("DoWhile " + p.GetUnparsed())
         }
         CoverN = 1
         Details += bit
      }
      bit *= 2
      if n.GetOutCovered() {
         if debug {
            ast.Act(&p, n.Expr)
            println("DoWhile out " + p.GetUnparsed())
         }
         CoverN++
         Details += bit
      }
   default:
      println("DEFAULT")
   }
   return BranchN, CoverN, Details
}

func PrintBranch(detail bool, ca []byte) {
	//contract := common.BytesToAddress(ca)
	fullyC, partialC, totalC, totalB := 0, 0, 0, 0

	fmt.Println("ContractAddr,TxCount,ErrorCount,CoveredBranch,TotalBranch,details")
	CoverageMap.Range(func(a, c interface{}) bool {
		contractC, contractB := 0, 0
		addr := a.(Addr)
		if ca != nil && !bytes.EqualFold(addr[:], ca[:]) {
			return true
		}
		cov := c.(*Coverage)
		str := ""
		branchInfo := ""
		if len(*cov.BranchList) > 0 {
			str = fmt.Sprintf("0x%s,%d,%d", hex.EncodeToString(addr[:]), cov.TxCount, cov.ErrorCount)
		}
		for _, n := range(*cov.BranchList) {
         typStr := strings.Split(fmt.Sprintf("%T", n), ".")[1]
			branchN, coverN, details := cov.GetCoverage(n)
			l, k := n.GetPos()
			branchInfo += fmt.Sprintf("%d/%d_%b-%s-%d_%d,", coverN, branchN, details, typStr[:len(typStr)-4], l, k)
			if coverN == branchN {
				fullyC++
			} else {
				partialC++
			}
			contractC += coverN
			contractB += branchN
		}
		totalC += contractC
		totalB += contractB
		if detail && str != "" {
			fmt.Printf("%s,%d,%d,%s\n", str, contractC, contractB, branchInfo)
		} //else { fmt.Printf("%s,%d,nil\n", common.Address(addr).String(), cov.TxCount) }
		return true
	})

	fmt.Printf("Covered %d branches in %d (%.2f%%)\n", totalC, totalB, float64(totalC)/float64(totalB)*100)
	fmt.Printf("Fully covered: %d, partially covered: %d\n", fullyC, partialC)
	fmt.Println("(deep: ", ca==nil, ")")
}