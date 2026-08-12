package research

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	sync "sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/urfave/cli/v2"
)

const (
	stage2SubstratePrefix = "2s" // stage2SubstratePrefix + codeHash (256-bit) -> code
	stage2EVMTracePrefix  = "2e" // stage2EvmTracePrefix + block (64-bit) + tx (64-bit) -> evmTraceJson
)

var (
	SubstrateDirFlag = &cli.PathFlag{
		Name:  "substratedir",
		Usage: "Directory for substrate codes",
		Value: "",
	}
	substrateDir = SubstrateDirFlag.Value

	ContractIsolationFlag = &cli.BoolFlag{
		Name:  "iso-emi",
		Usage: "Enable per-contract validation",
	}
	contractIso = false

	IsolationIndexFlag = &cli.IntFlag{
		Name:  "iso-index",
		Usage: "The index of transaction for isolated execution, only effective with --iso-emi=true",
		Value: -1,
	}
	isoIndex = -1

	DebugSubstrateFlag = &cli.BoolFlag{
		Name:  "debug",
		Usage: "Output detailed information for debugging EMI",
	}
	debugSubstrate = false

	slowTxCalls     = int(5000)
	SlowTxCallsFlag = &cli.IntFlag{
		Name:  "slow-tx-calls",
		Usage: "The threshold of internal calls for slow transactions",
		Value: slowTxCalls,
	}

	skipSlowTxs     = false
	SkipSlowTxsFlag = &cli.BoolFlag{
		Name:  "skip-slow-txs",
		Usage: "Skip slow txs with more calls than the slow-tx-calls threshold",
		Value: skipSlowTxs,
	}

	SkipInternalTxsFlag = &cli.BoolFlag{
		Name:  "skip-internal-txs",
		Usage: "Skip per-contract validation of internal txs, only effective with --iso-emi=true",
	}
	skipInternalTxs = false

	RunOutOfGasFlag = &cli.BoolFlag{
		Name:  "out-of-gas",
		Usage: "Execute transaction containing out-of-gas error",
	}
	runOutOfGas = false

	FuzzNFlag = &cli.IntFlag{
		Name:  "fuzz-num",
		Usage: "Number of fuzz execution",
		Value: 1,
	}
	fuzzNum int

	NoFuzzFlag = &cli.BoolFlag{
		Name:  "no-fuzz",
		Usage: "Disable fuzz and use substate input",
	}
	noFuzz = false

	funcInfo sync.Map
)

// Substate_Result
func (x *Substate_Result) EqualLog(y *Substate_Result) bool {
	equal := len(x.Logs) == len(y.Logs)
	if !equal {
		return false
	}

	for i, xl := range x.Logs {
		yl := y.Logs[i]

		equal := (bytes.Equal(xl.Address, yl.Address) &&
			len(xl.Topics) == len(yl.Topics) &&
			bytes.Equal(xl.Data, yl.Data))
		if !equal {
			return false
		}

		for i, xt := range xl.Topics {
			yt := yl.Topics[i]
			if !bytes.Equal(xt, yt) {
				return false
			}
		}
	}

	return true
}

func GetSubstrateDir() string {
	return substrateDir
}
func GetContractIso() bool {
	return contractIso
}
func GetIsoIndex() int {
	return isoIndex
}
func GetDebugFlag() bool {
	return debugSubstrate
}

func GetSlowTxCalls() int {
	return slowTxCalls
}

func GetSkipSlowTxs() bool {
	return skipSlowTxs
}
func GetSkipInternalTxs() bool {
	return skipInternalTxs
}
func GetRunOutOfGas() bool {
	return runOutOfGas
}

func GetFuzzNum() int {
	return fuzzNum
}
func GetNoFuzz() bool {
	return noFuzz
}
func PutFuncInfo(addr common.Address, fi map[string]int) {
	funcInfo.LoadOrStore(addr, fi)
}
func PrintFuncInfo() {
	fmt.Println("**************************************")
	//for contract, list := range funcInfo {
	funcInfo.Range(func(k, v interface{}) bool {
		contract := k.(common.Address)
		list := v.(map[string]int)
		str := contract.String() + ","
		for sig, line := range list {
			str += fmt.Sprintf("%s-%d,", sig, line)
		}
		fmt.Println(str)
		return true
	})
	fmt.Println("**************************************")
}

func (db *SubstateDB) HasSubstrate(codeHash common.Hash) bool {
	if codeHash == EmptyCodeHash {
		return false
	}
	key := Stage2SubstrateKey(codeHash)
	has, err := db.backend.Has(key)
	if err != nil {
		panic(fmt.Errorf("substrate-interpreter: error checking substrate for codeHash %s: %v", codeHash.Hex(), err))
	}
	return has
}

// temporary solution for missing *SubstateDB handle in substate_interpreter.go
func HasSubstrateByCode(code []byte) bool {
	return staticSubstateDB.HasSubstrateByCode(code)
}

func (db *SubstateDB) HasSubstrateByCode(code []byte) bool {
	codeHash := CodeHash(code)
	return db.HasSubstrate(codeHash)
}

func (db *SubstateDB) GetSubstrate(codeHash common.Hash) []byte {
	if codeHash == EmptyCodeHash {
		return nil
	}
	key := Stage2SubstrateKey(codeHash)
	substrate, err := db.backend.Get(key)
	if err != nil {
		panic(fmt.Errorf("substrate-interpreter: error getting substrate for codeHash %s: %v", codeHash.Hex(), err))
	}
	return substrate
}

func (db *SubstateDB) GetSubstrateByCode(code []byte) []byte {
	codeHash := CodeHash(code)
	if db.HasSubstrate(codeHash) {
		return db.GetSubstrate(codeHash)
	}
	return nil
}

func (db *SubstateDB) PutSubstrate(codeHash common.Hash, substrate []byte) {
	if codeHash == EmptyCodeHash {
		return
	}
	key := Stage2SubstrateKey(codeHash)
	err := db.backend.Put(key, substrate)
	if err != nil {
		panic(fmt.Errorf("substrate-interpreter: error putting substrate for codeHash %s: %v", codeHash.Hex(), err))
	}
}

func Stage2SubstrateKey(codeHash common.Hash) []byte {
	prefix := []byte(stage2SubstratePrefix)
	return append(prefix, codeHash.Bytes()...)
}

func DecodeStage2SubstrateKey(key []byte) (codeHash common.Hash, err error) {
	prefix := stage2SubstratePrefix
	if len(key) != len(prefix)+32 {
		err = fmt.Errorf("invalid length of stage2 substrate key: %v", len(key))
		return
	}
	if p := string(key[:2]); p != prefix {
		err = fmt.Errorf("invalid prefix of stage2 substrate key: %#x", p)
		return
	}
	codeHash = common.BytesToHash(key[len(prefix):])
	return
}

func Stage2EVMTraceKey(block uint64, tx int) []byte {
	prefix := []byte(stage2EVMTracePrefix)

	blockTx := make([]byte, 16)
	binary.BigEndian.PutUint64(blockTx[0:8], block)
	binary.BigEndian.PutUint64(blockTx[8:16], uint64(tx))

	return append(prefix, blockTx...)
}

func DecodeStage2EVMTraceKey(key []byte) (block uint64, tx int, err error) {
	prefix := stage2EVMTracePrefix
	if len(key) != len(prefix)+8+8 {
		err = fmt.Errorf("invalid length of stage2 substrate key: %v", len(key))
		return
	}
	if p := string(key[:len(prefix)]); p != prefix {
		err = fmt.Errorf("invalid prefix of stage2 substrate key: %v", p)
		return
	}
	blockTx := key[len(prefix):]
	block = binary.BigEndian.Uint64(blockTx[0:8])
	tx = int(binary.BigEndian.Uint64(blockTx[8:16]))
	return
}

func Md5Hash(code []byte) [md5.Size]byte {
	return md5.Sum(code)
}

func Md5HashHex(code []byte) string {
	h := Md5Hash(code)
	return hex.EncodeToString(h[:])
}
