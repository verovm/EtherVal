package research

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/urfave/cli/v2"
)

func OpenSubstateDB() {
	fmt.Println("record-replay: OpenSubstateDB")
	backend, err := rawdb.NewLevelDBDatabase(substateDir, 1024, 100, "substatedir", false)
	if err != nil {
		panic(fmt.Errorf("error opening substate leveldb %s: %v", substateDir, err))
	}
	staticSubstateDB = NewSubstateDB(backend)
}

func OpenSubstateDBReadOnly() {
	fmt.Println("record-replay: OpenSubstateDB")
	backend, err := rawdb.NewLevelDBDatabase(substateDir, 1024, 100, "substatedir", true)
	if err != nil {
		panic(fmt.Errorf("error opening substate leveldb %s: %v", substateDir, err))
	}
	staticSubstateDB = NewSubstateDB(backend)
}

func CloseSubstateDB() {
	defer fmt.Println("record-replay: CloseSubstateDB")

	err := staticSubstateDB.Close()
	if err != nil {
		panic(fmt.Errorf("error closing substate leveldb %s: %v", substateDir, err))
	}
}

func CompactSubstateDB() {
	fmt.Println("record-replay: CompactSubstateDB")

	// compact entire DB
	err := staticSubstateDB.Compact(nil, nil)
	if err != nil {
		panic(fmt.Errorf("error compacting substate leveldb %s: %v", substateDir, err))
	}
}

func OpenFakeSubstateDB() {
	backend := rawdb.NewMemoryDatabase()
	staticSubstateDB = NewSubstateDB(backend)
}

func CloseFakeSubstateDB() {
	staticSubstateDB.Close()
}

func SetSubstateFlags(ctx *cli.Context) {
	substateDir = ctx.Path(SubstateDirFlag.Name)
	fmt.Printf("record-replay: --substatedir=%s\n", substateDir)

	substrateDir = ctx.String(SubstrateDirFlag.Name)
	contractIso = ctx.Bool(ContractIsolationFlag.Name)
	debugSubstrate = ctx.Bool(DebugSubstrateFlag.Name)
	slowTxCalls = ctx.Int(SlowTxCallsFlag.Name)
	skipSlowTxs = ctx.Bool(SkipSlowTxsFlag.Name)
	fmt.Printf("substrate-interpreter: --substratedir=%s, --iso-emi=%v, --debug=%v\n", substrateDir, contractIso, debugSubstrate)
	fmt.Printf("substrate-interpreter: --slow-tx-calls=%v --skip-slow-txs=%v\n", slowTxCalls, skipSlowTxs)

	//fuzzNum = ctx.Int(FuzzNFlag.Name)
	//noFuzz = ctx.Bool(NoFuzzFlag.Name)
	//fmt.Printf("substrate-interpreter: --no-fuzz=%v\n", noFuzz)
}

func HasCode(codeHash common.Hash) bool {
	return staticSubstateDB.HasCode(codeHash)
}

func GetCode(codeHash common.Hash) []byte {
	return staticSubstateDB.GetCode(codeHash)
}

func PutCode(code []byte) {
	staticSubstateDB.PutCode(code)
}

func HasSubstate(block uint64, tx int) bool {
	return staticSubstateDB.HasSubstate(block, tx)
}

func GetSubstate(block uint64, tx int) *Substate {
	return staticSubstateDB.GetSubstate(block, tx)
}

func GetBlockSubstates(block uint64) map[int]*Substate {
	return staticSubstateDB.GetBlockSubstates(block)
}

func PutSubstate(block uint64, tx int, substate *Substate) {
	staticSubstateDB.PutSubstate(block, tx, substate)
}

func DeleteSubstate(block uint64, tx int) {
	staticSubstateDB.DeleteSubstate(block, tx)
}
