package validate

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/cmd/substate-cli/replay"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/research"
	"github.com/urfave/cli/v2"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// Ignore empty TAC result
const IgnoreEmptyTACResult = true

var ValidateTacCommand = &cli.Command{
	Action:  validateTacAction,
	Name:    "validate-tac",
	Aliases: []string{"val-tac"},
	Usage:   "validate TAC with transactions and report output equivalence",
	Flags: []cli.Flag{
		research.WorkersFlag,
		research.SkipTransferTxsFlag,
		research.SkipCallTxsFlag,
		research.SkipCreateTxsFlag,
		research.SubstateDirFlag,
		research.BlockSegmentFlag,
		research.SubstrateDirFlag,
		ValidateOutputCsvFlag,
		vm.TacTimeoutFlag,
		AllowlistFlag,
		research.ContractIsolationFlag,
		vm.TacMaxInstCountFlag,
		vm.TacGasInstCountFlag,
		research.DebugSubstrateFlag,
		research.SlowTxCallsFlag,
		research.SkipSlowTxsFlag,
		research.SkipInternalTxsFlag,
		research.RunOutOfGasFlag,
		research.TxListFlag,
	},
	Description: `
substate-cli validate-tac executes transactions in the given block segment
with TAC interpreter and report output equivalence.`,
	Category: "validate",
}

type valTacResult struct {
	idx int
	row string
	err error
}

func validateTacSubstate(block uint64, tx int, substate *research.Substate, allowlist *Allowlist) (valResults []*valTacResult) {
	return validateTacSubstateWithTrace(block, tx, substate, allowlist, nil)
}

// validateTacSubstateWithTrace returns a slice of tacIdx+result+stats+isoEMI+matchedSA values
// If recordedTrace is not nil, this function runs deviation point check and appends eqAlloc+eqResult+eqBoth.
func validateTacSubstateWithTrace(block uint64, tx int, substate *research.Substate, allowlist *Allowlist, recordedTrace *research.EVMTrace) (valResults []*valTacResult) {
	appendValResult := func(idx int, row string, err error) {
		valResults = append(valResults, &valTacResult{
			idx: idx,
			row: row,
			err: err,
		})
	}

	deviationCheck := (recordedTrace != nil)

	mode := vm.InterpreterMode_TAC
	trace := research.GetEVMTrace(block, tx)
	repeat := 1

	if research.GetRunOutOfGas() {
		found := false
		for i := 0; i < len(trace.CallTraces); i++ {
			err := trace.CallTraces[i].Err
			if err != nil && err.Error() == vm.ErrOutOfGas.Error() {
				found = true
				break
			}
		}
		if !found {
			return nil
		}
	}
	if research.GetContractIso() {
		if research.GetSkipInternalTxs() {
			repeat = 1
		} else {
			repeat = len(trace.CallTraces)
		}
		mode = vm.InterpreterMode_EVM
	}

	isAllowedAddr := make(map[common.Address]bool)
	emptyCodeAddr := make(map[common.Address]bool)
	bothAlloc := append(substate.InputAlloc.Alloc, substate.OutputAlloc.Alloc...)
	for _, entry := range bothAlloc {
		addr := *research.BytesToAddress(entry.Address)
		code := entry.Account.GetCode()
		isAllowedAddr[addr] = allowlist.IsAllowed(addr, code)
		emptyCodeAddr[addr] = (len(code) == 0)
	}

	txStart := time.Now()
	isSlowTx := repeat >= research.GetSlowTxCalls()
	if isSlowTx {
		if research.GetSkipSlowTxs() {
			fmt.Printf("skip slow tx: block %v, tx %v, calls: %v\n", block, tx, repeat)
			return nil
		}
		fmt.Printf("slow tx begin: block %v, tx %v, calls: %v\n", block, tx, repeat)
	}

	for idx := 0; idx < repeat; idx++ {
		if research.GetIsoIndex() != -1 && idx != research.GetIsoIndex() {
			continue
		}

		if isSlowTx && idx > 0 && idx%1000 == 0 {
			fmt.Printf("slow tx: time: %v, block %v, tx %v, idx: %v\n", time.Since(txStart).Round(1*time.Millisecond), block, tx, idx)
		}
		idxStart := time.Now()
		callee := trace.CallTraces[idx].Callee

		if research.GetDebugFlag() {
			fmt.Printf("init begin: %v, block %v, tx %v, tacIdx %v, callee: %s, gasUsed: %v\n", time.Since(idxStart).Round(1*time.Millisecond), block, tx, idx, callee.Hex(), substate.Result.GetGasUsed())
		}

		if !isAllowedAddr[callee] {
			if research.GetDebugFlag() {
				fmt.Printf("skip unallowed account: %v, block %v, tx %v, tacIdx %v, callee: %s\n", time.Since(idxStart).Round(1*time.Millisecond), block, tx, idx, callee.Hex())
			}
			continue
		}

		if idx > 0 {
			// Skip internal transactions calling empty bytecode
			if emptyCodeAddr[callee] {
				if research.GetDebugFlag() {
					fmt.Printf("skip empty code: %v, block %v, tx %v, tacIdx %v, callee: %s\n", time.Since(idxStart).Round(1*time.Millisecond), block, tx, idx, callee.Hex())
				}
				continue
			}
		}

		var (
			err       error
			matchedSA bool
			resRow    string
		)

		// InputAlloc
		statedb, _ := state.New(types.EmptyRootHash, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
		statedb.LoadSubstate(substate)

		// BlockEnv
		blockContext := vm.BlockContext{
			CanTransfer: core.CanTransfer,
			Transfer:    core.Transfer,
		}
		blockContext.LoadSubstate(substate)
		blockNumber := blockContext.BlockNumber

		// TxMessage
		txMessage := &core.Message{}
		txMessage.LoadSubstate(substate)

		chainConfig := &params.ChainConfig{}
		*chainConfig = *params.MainnetChainConfig
		// disable DAOForkSupport, otherwise account states will be overwritten
		chainConfig.DAOForkSupport = false

		vmConfig := vm.Config{}
		vmConfig.InterpreterMode = mode
		vmConfig.IsolationMode = vm.InterpreterMode_TAC
		vmConfig.IsolationIndex = idx
		vmConfig.EVMTrace = recordedTrace

		statedb.SetTxContext(common.Hash{}, tx)

		evm := vm.NewEVM(blockContext, vm.TxContext{}, statedb, chainConfig, vmConfig)

		evm.Delta = vm.NewDeltaSemantics(evm, substate, trace)

		txContext := core.NewEVMTxContext(txMessage)
		evm.Reset(txContext, statedb)

		gaspool := new(core.GasPool).AddGas(blockContext.GasLimit)

		if research.GetDebugFlag() {
			fmt.Printf("init done: %v, block %v, tx %v, tacIdx %v, callee: %s, maxInst: %v\n", time.Since(idxStart).Round(1*time.Millisecond), block, tx, idx, callee.Hex(), evm.Delta.TACMaxInstCount)
		}

		if research.GetDebugFlag() {
			fmt.Printf("exec begin: %v, block %v, tx %v, tacIdx %v, callee: %s, maxInst: %v\n", time.Since(idxStart).Round(1*time.Millisecond), block, tx, idx, callee.Hex(), evm.Delta.TACMaxInstCount)
		}

		var result *core.ExecutionResult
		func() {
			defer func() {
				if r := recover(); r != nil {
					err = vm.ErrTACPanic
				}
			}()
			result, err = core.ApplyMessage(evm, txMessage, gaspool)
		}()

		if IgnoreEmptyTACResult && evm.Delta.SubstrateResult == "" {
			if research.GetDebugFlag() {
				fmt.Printf("skip empty result: %v, block %v, tx %v, tacIdx %v, callee: %s\n", time.Since(idxStart).Round(1*time.Millisecond), block, tx, idx, callee.Hex())
			}
			continue
		}

		if research.GetDebugFlag() {
			delta := evm.Delta
			fmt.Printf("exec done: %v, block %v, tx %v, tacIdx %v, callee %s, tacInst: %v, tacJump: %v, tacAmbJump: %v, tacPhi: %v\n", time.Since(idxStart).Round(1*time.Millisecond), block, tx, idx, callee.Hex(), delta.TACInstCount, delta.TACJumpCount, delta.TACAmbiguousJumpCount, delta.TACPhiCount)
		}

		if vm.IsTACGlobalError(evm.Delta.SubstrateErr) {
			err = evm.Delta.SubstrateErr
		}

		if deviationCheck && result == nil {
			result = &core.ExecutionResult{
				Err: err,
			}
		}

		// skip early error return without deviation check
		if err != nil && !deviationCheck {
			matchedSA = false
			resRow = fmt.Sprintf("%s,%t", evm.Delta.SubstrateResult, matchedSA)
			appendValResult(idx, resRow, err)
			continue
		}

		if chainConfig.IsByzantium(blockNumber) {
			statedb.Finalise(true)
		} else {
			// No need for root hash, call  Finalise instead of IntermediateRoot
			statedb.Finalise(chainConfig.IsEIP158(blockNumber))
		}

		rr := &research.ResearchReceipt{}
		if result.Failed() {
			rr.Status = types.ReceiptStatusFailed
		} else {
			rr.Status = types.ReceiptStatusSuccessful
		}
		rr.Logs = statedb.GetLogs(common.Hash{}, blockContext.BlockNumber.Uint64(), common.Hash{})
		rr.Bloom = types.CreateBloom(types.Receipts{&types.Receipt{Logs: rr.Logs}})

		rr.GasUsed = result.UsedGas

		replaySubstate := &research.Substate{}
		statedb.SaveSubstate(replaySubstate)
		blockContext.SaveSubstate(replaySubstate)
		txMessage.SaveSubstate(replaySubstate)
		rr.SaveSubstate(replaySubstate)

		eqAlloc, matchedSA := EquivalentAlloc(substate, replaySubstate)
		eqResult := proto.Equal(substate.Result, replaySubstate.Result)

		resRow = fmt.Sprintf("%s,%t", evm.Delta.SubstrateResult, matchedSA)
		if deviationCheck {
			eqBoth := eqAlloc && eqResult
			resRow += fmt.Sprintf(",%t,%t,%t", eqAlloc, eqResult, eqBoth)
		}

		// Return any TAC related error
		if vm.IsTACGlobalError(result.Err) {
			appendValResult(idx, resRow, result.Err)
			continue
		}

		if !(eqResult && eqAlloc) {
			if vm.TraceTAC {
				fmt.Printf("block %v, tx %v, inconsistent output\n", block, tx)
				jm := protojson.MarshalOptions{
					Indent: "  ",
				}

				var b []byte

				b, _ = jm.Marshal(substate)
				os.WriteFile(fmt.Sprintf("record_substate_%v_%v.json", block, tx), b, 0644)
				b, _ = jm.Marshal(substate.HashedCopy())
				os.WriteFile(fmt.Sprintf("record_substate_%v_%v_hashed.json", block, tx), b, 0644)

				b, _ = jm.Marshal(replaySubstate)
				os.WriteFile(fmt.Sprintf("replay_substate_%v_%v.json", block, tx), b, 0644)
				b, _ = jm.Marshal(replaySubstate.HashedCopy())
				os.WriteFile(fmt.Sprintf("replay_substate_%v_%v_hashed.json", block, tx), b, 0644)

				fmt.Printf("Saved record/replay_substate_*.json files (bytes in base64)\n")

				recordTrace := research.GetEVMTrace(block, tx)
				replayTrace := evm.StateDB.(*state.StateDB).ResearchEVMTrace

				b, _ = json.MarshalIndent(recordTrace, "", "  ")
				os.WriteFile(fmt.Sprintf("record_trace_%v_%v.json", block, tx), b, 0o644)
				b, _ = json.MarshalIndent(replayTrace, "", "  ")
				os.WriteFile(fmt.Sprintf("replay_trace_%v_%v.json", block, tx), b, 0o644)
			}
			appendValResult(idx, resRow, TACInconsistentResult)
			continue
		}

		appendValResult(idx, resRow, nil)
		continue
	}
	if isSlowTx {
		fmt.Printf("slow tx done: time: %v, block %v, tx %v, calls: %v\n", time.Since(txStart).Round(1*time.Millisecond), block, tx, repeat)
	}

	return valResults
}

func validateTacAction(ctx *cli.Context) error {
	var err error

	research.SetSubstateFlags(ctx)
	research.OpenSubstateDBReadOnly()
	defer research.CloseSubstateDB()

	vm.SetTacFlags(ctx)

	var allowlist *Allowlist = nil
	allowlistPath := ctx.String(AllowlistFlag.Name)
	if allowlistPath != "" {
		if research.GetContractIso() && !research.GetSkipInternalTxs() {
			panic(fmt.Errorf("--%s does not work correctly with --%s without --%s yet", AllowlistFlag.Name, research.ContractIsolationFlag.Name, research.SkipInternalTxsFlag.Name))
		}
		allowlist = ReadAllowlist(allowlistPath)
	}

	csvPath := ctx.String(ValidateOutputCsvFlag.Name)
	csvFile, err := os.OpenFile(csvPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer csvFile.Close()

	csvWriter := bufio.NewWriter(csvFile)
	defer csvWriter.Flush()

	type statTuple struct {
		csvRow    string
		tacResult string
	}
	var totalTx int64
	statCount := make(map[string]int64)
	statChan := make(chan []*statTuple, 1_000_000)
	statWg := sync.WaitGroup{}

	statWg.Add(1)
	go func() {
		defer statWg.Done()
		defer csvWriter.Flush()
		fmt.Fprintf(csvWriter, "block,tx,tacIdx,result,%s,matchedSA\n", vm.TACResultHeader)
		csvWriter.Flush()
		for tuples := range statChan {
			prevTotalTx := totalTx
			for _, tuple := range tuples {
				totalTx++
				csvWriter.WriteString(tuple.csvRow)
				statCount[tuple.tacResult]++

				if totalTx-prevTotalTx >= 10 {
					csvWriter.Flush()
					prevTotalTx = totalTx
				}
			}
			csvWriter.Flush()
		}
	}()

	validateTacStatsTask := func(block uint64, tx int, substate *research.Substate, taskPool *research.SubstateTaskPool) error {
		to := substate.GetTxMessage().GetTo().GetValue()
		if to == nil {
			return nil
		}

		exist := false
		for _, entry := range substate.GetInputAlloc().GetAlloc() {
			if bytes.Equal(to, entry.GetAddress()) {
				exist = true
				break
			}
		}
		if !exist {
			return nil
		}

		valResults := validateTacSubstate(block, tx, substate, allowlist)
		tuples := make([]*statTuple, 0, len(valResults))
		for _, valRes := range valResults {
			idx := valRes.idx
			resRow := valRes.row
			err := valRes.err

			tacResult := TACResultCategory(err)
			csvRow := fmt.Sprintf("%v,%v,%v,%s,%s\n", block, tx, idx, tacResult, resRow)
			tuples = append(tuples, &statTuple{
				csvRow:    csvRow,
				tacResult: tacResult,
			})
		}
		if len(tuples) > 0 {
			statChan <- tuples
		}

		return nil
	}

	taskPool := research.NewSubstateTaskPoolCli("substate-cli validate-tac", validateTacStatsTask, ctx)

	segment, err := research.ParseBlockSegment(ctx.String(research.BlockSegmentFlag.Name))
	if err != nil {
		return fmt.Errorf("substate-cli validate-tac: error parsing block segment: %w", err)
	}

	err = taskPool.ExecuteSegment(segment)

	close(statChan)
	statWg.Wait()

	fmt.Printf(`

Total %d PASSED %d FAILED %d
TACParseError %d TACPanic %d TACTimeout %d
IllJumpTx %d IllPhiTx %d TACThrowErr %d
NoTacTx %d CallTraceErr %d GasTraceErr %d
MissingGasSem %d AmbiJumpTx %d

`,
		totalTx, statCount["PASSED"], statCount["FAILED"],
		statCount["TACParseError"], statCount["TACPanic"], statCount["TACTimeout"],
		statCount["IllJumpTx"], statCount["IllPhiTx"], statCount["TACThrowErr"],
		statCount["NoTacTx"], statCount["CallTraceErr"], statCount["GasTraceErr"],
		statCount["MissingGasSem"], statCount["AmbiJumpTx"],
	)
	if totalTx > 0 {
		fmt.Printf("PASSED ratio: %f%%\n", float64(statCount["PASSED"])/float64(totalTx)*100)
	}

	return err
}

var ValidateTacTxCommand = &cli.Command{
	Action:  validateTacTxAction,
	Name:    "validate-tac-tx",
	Aliases: []string{"val-tac-tx"},
	Usage:   "validate TAC with one transaction and report output equivalence",
	Flags: []cli.Flag{
		research.WorkersFlag,
		research.SkipTransferTxsFlag,
		research.SkipCallTxsFlag,
		research.SkipCreateTxsFlag,
		research.SubstateDirFlag,
		research.BlockSegmentFlag,
		research.TxIndexFlag,
		research.SubstrateDirFlag,
		vm.TacTimeoutFlag,
		research.ContractIsolationFlag,
		vm.TacMaxInstCountFlag,
		vm.TacGasInstCountFlag,
		research.DebugSubstrateFlag,
		research.SlowTxCallsFlag,
		research.SkipSlowTxsFlag,
		research.SkipInternalTxsFlag,
		research.RunOutOfGasFlag,
	},
	Description: `
substate-cli validate-tac-tx executes one transaction
with substrate interpreter and report output equivalence.`,
	Category: "validate",
}

func validateTacTxAction(ctx *cli.Context) error {
	var err error

	research.SetSubstateFlags(ctx)
	research.OpenSubstateDBReadOnly()
	defer research.CloseSubstateDB()

	vm.SetTacFlags(ctx)

	validateTacTxTask := func(block uint64, tx int, substate *research.Substate, taskpool *research.SubstateTaskPool) error {
		valResults := validateTacSubstate(block, tx, substate, nil)

		for _, valRes := range valResults {
			idx := valRes.idx
			resRow := valRes.row
			err := valRes.err
			fmt.Printf("block,tx,tacIdx,result,tacIdx,%s,matchedSA\n", vm.TACResultHeader)
			fmt.Printf("%v,%v,%v,%s,%s\n", block, tx, idx, TACResultCategory(err), resRow)
		}

		return nil
	}

	taskPool := research.NewSubstateTaskPoolCli("substate-cli validate-tac-tx", validateTacTxTask, ctx)

	block := ctx.Uint64(research.BlockSegmentFlag.Name)
	tx := ctx.Int(research.TxIndexFlag.Name)

	err = taskPool.TaskFunc(block, tx, taskPool.DB.GetBlockSubstates(block)[tx], taskPool)

	return err
}

var DetectDeviationTacCommand = &cli.Command{
	Action:  detectDeviationTacAction,
	Name:    "detect-deviation-tac",
	Aliases: []string{"dev-tac"},
	Usage:   "execute TAC with transactions until it deviates from bytecode",
	Flags: []cli.Flag{
		research.WorkersFlag,
		research.SkipTransferTxsFlag,
		research.SkipCallTxsFlag,
		research.SkipCreateTxsFlag,
		research.SubstateDirFlag,
		research.BlockSegmentFlag,
		research.SubstrateDirFlag,
		ValidateOutputCsvFlag,
		vm.TacTimeoutFlag,
		AllowlistFlag,
		research.ContractIsolationFlag,
		vm.TacMaxInstCountFlag,
		vm.TacGasInstCountFlag,
		research.DebugSubstrateFlag,
		research.SlowTxCallsFlag,
		research.SkipSlowTxsFlag,
		research.SkipInternalTxsFlag,
		research.RunOutOfGasFlag,
		research.TxListFlag,
		research.IsolationIndexFlag,
	},
	Description: `
substate-cli detect-deviation-tac executes transactions in the given block
segment with TAC interpreter, and report the deviation point in the substrate
based on SSTORE, LOG, calls, and SHA3.`,
	Category: "validate",
}

func detectDeviationTacAction(ctx *cli.Context) error {
	var err error

	research.SetSubstateFlags(ctx)
	research.OpenSubstateDBReadOnly()
	defer research.CloseSubstateDB()

	vm.SetTacFlags(ctx)

	var allowlist *Allowlist = nil
	allowlistPath := ctx.String(AllowlistFlag.Name)
	if allowlistPath != "" {
		if research.GetContractIso() && !research.GetSkipInternalTxs() {
			panic(fmt.Errorf("--%s does not work correctly with --%s without --%s yet", AllowlistFlag.Name, research.ContractIsolationFlag.Name, research.SkipInternalTxsFlag.Name))
		}
		allowlist = ReadAllowlist(allowlistPath)
	}

	csvPath := ctx.String(ValidateOutputCsvFlag.Name)
	csvFile, err := os.OpenFile(csvPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer csvFile.Close()

	csvWriter := bufio.NewWriter(csvFile)
	defer csvWriter.Flush()

	type statTuple struct {
		csvRow    string
		tacResult string
	}
	var totalTx int64
	statCount := make(map[string]int64)
	statChan := make(chan []*statTuple, 1_000_000)
	statWg := sync.WaitGroup{}

	statWg.Add(1)
	go func() {
		defer statWg.Done()
		defer csvWriter.Flush()
		fmt.Fprintf(csvWriter, "block,tx,tacIdx,result,%s,matchedSA,eqAlloc,eqResult,eqBoth\n", vm.TACDeviationHeader)
		csvWriter.Flush()
		for tuples := range statChan {
			prevTotalTx := totalTx
			for _, tuple := range tuples {
				totalTx++
				csvWriter.WriteString(tuple.csvRow)
				statCount[tuple.tacResult]++

				if totalTx-prevTotalTx >= 10 {
					csvWriter.Flush()
					prevTotalTx = totalTx
				}
			}
			csvWriter.Flush()
		}
	}()

	detectDeviationTacStatsTask := func(block uint64, tx int, substate *research.Substate, taskPool *research.SubstateTaskPool) error {
		to := substate.GetTxMessage().GetTo().GetValue()
		if to == nil {
			return nil
		}

		exist := false
		for _, entry := range substate.GetInputAlloc().GetAlloc() {
			if bytes.Equal(to, entry.GetAddress()) {
				exist = true
				break
			}
		}
		if !exist {
			return nil
		}

		statedb, _, _, _, err := replay.ReplayEVM(block, tx, substate)
		if err != nil {
			return fmt.Errorf("error replaying transaction to get trace: %w", err)
		}

		recordedTrace := statedb.ResearchEVMTrace.Copy()
		if research.GetDebugFlag() {
			fmt.Println("EVM finished. Start interpreter for deviation analysis.")
		}

		// Run substrate interpreter with the trace, and check deviation points
		valResults := validateTacSubstateWithTrace(block, tx, substate, allowlist, recordedTrace)
		tuples := make([]*statTuple, 0, len(valResults))
		for _, valRes := range valResults {
			idx := valRes.idx
			resRow := valRes.row
			err := valRes.err

			tacResult := TACResultCategory(err)
			csvRow := fmt.Sprintf("%v,%v,%v,%s,%s\n", block, tx, idx, tacResult, resRow)
			tuples = append(tuples, &statTuple{
				csvRow:    csvRow,
				tacResult: tacResult,
			})
		}
		if len(tuples) > 0 {
			statChan <- tuples
		}

		return nil
	}

	taskPool := research.NewSubstateTaskPoolCli("substate-cli dev-tac", detectDeviationTacStatsTask, ctx)

	segment, err := research.ParseBlockSegment(ctx.String(research.BlockSegmentFlag.Name))
	if err != nil {
		return fmt.Errorf("substate-cli dev-tac: error parsing block segment: %w", err)
	}

	err = taskPool.ExecuteSegment(segment)

	close(statChan)
	statWg.Wait()

	fmt.Printf(`

Total %d PASSED %d FAILED %d
TACParseError %d TACPanic %d TACTimeout %d
IllJumpTx %d IllPhiTx %d TACThrowErr %d
NoTacTx %d CallTraceErr %d GasTraceErr %d
MissingGasSem %d AmbiJumpTx %d

`,
		totalTx, statCount["PASSED"], statCount["FAILED"],
		statCount["TACParseError"], statCount["TACPanic"], statCount["TACTimeout"],
		statCount["IllJumpTx"], statCount["IllPhiTx"], statCount["TACThrowErr"],
		statCount["NoTacTx"], statCount["CallTraceErr"], statCount["GasTraceErr"],
		statCount["MissingGasSem"], statCount["AmbiJumpTx"],
	)
	if totalTx > 0 {
		fmt.Printf("PASSED ratio: %f%%\n", float64(statCount["PASSED"])/float64(totalTx)*100)
	}

	return err
}

var DetectDeviationTacTxCommand = &cli.Command{
	Action:  detectDeviationTacTxAction,
	Name:    "detect-deviation-tac-tx",
	Aliases: []string{"dev-tac-tx"},
	Usage:   "execute TAC with a specific transaction until it deviates from bytecode",
	Flags: []cli.Flag{
		research.WorkersFlag,
		research.SkipTransferTxsFlag,
		research.SkipCallTxsFlag,
		research.SkipCreateTxsFlag,
		research.SubstateDirFlag,
		research.BlockSegmentFlag,
		research.TxIndexFlag,
		research.SubstrateDirFlag,
		vm.TacTimeoutFlag,
		research.ContractIsolationFlag,
		vm.TacMaxInstCountFlag,
		vm.TacGasInstCountFlag,
		research.DebugSubstrateFlag,
		research.SlowTxCallsFlag,
		research.SkipSlowTxsFlag,
		research.SkipInternalTxsFlag,
		research.RunOutOfGasFlag,
		research.IsolationIndexFlag,
	},
	Description: `
substate-cli dev-tac-tx executes a transaction with TAC interpreter,
and report the deviation point in the substrate based on SSTORE, LOG,
calls, and SHA3.`,
	Category: "validate",
}

func detectDeviationTacTxAction(ctx *cli.Context) error {
	var err error

	research.SetSubstateFlags(ctx)
	research.OpenSubstateDBReadOnly()
	defer research.CloseSubstateDB()

	vm.SetTacFlags(ctx)

	detectDeviationTacTxTask := func(block uint64, tx int, substate *research.Substate, taskpool *research.SubstateTaskPool) error {
		statedb, _, _, _, err := replay.ReplayEVM(block, tx, substate)
		if err != nil {
			return fmt.Errorf("error replaying transaction to get trace: %w", err)
		}
		recordedTrace := statedb.ResearchEVMTrace.Copy()

		valResults := validateTacSubstateWithTrace(block, tx, substate, nil, recordedTrace)

		for _, valRes := range valResults {
			idx := valRes.idx
			resRow := valRes.row
			err := valRes.err
			fmt.Printf("block,tx,tacIdx,result,%s,matchedSA,eqAlloc,eqResult,eqBoth\n", vm.TACDeviationHeader)
			fmt.Printf("%v,%v,%v,%s,%s\n", block, tx, idx, TACResultCategory(err), resRow)
		}

		return nil
	}

	taskPool := research.NewSubstateTaskPoolCli("substate-cli validate-tac-tx", detectDeviationTacTxTask, ctx)

	block := ctx.Uint64(research.BlockSegmentFlag.Name)
	tx := ctx.Int(research.TxIndexFlag.Name)

	err = taskPool.TaskFunc(block, tx, taskPool.DB.GetBlockSubstates(block)[tx], taskPool)

	return err
}
