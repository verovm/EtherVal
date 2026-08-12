package validate

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/cmd/substate-cli/replay"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/research"
	cli "github.com/urfave/cli/v2"
	"google.golang.org/protobuf/proto"
)

var ValidateSubstrateCommand = &cli.Command{
	Action:  validateSubstrateAction,
	Name:    "validate-substrate",
	Aliases: []string{"val-sbt"},
	Usage:   "validate substrates with transactions and report output equivalence",
	Flags: []cli.Flag{
		research.WorkersFlag,
		research.SkipTransferTxsFlag,
		research.SkipCallTxsFlag,
		research.SkipCreateTxsFlag,
		research.SubstateDirFlag,
		research.BlockSegmentFlag,
		research.SubstrateDirFlag,
		research.ContractIsolationFlag,
		research.DebugSubstrateFlag,
		research.SlowTxCallsFlag,
		research.SkipSlowTxsFlag,
		research.SkipInternalTxsFlag,
		research.RunOutOfGasFlag,
	},
	Description: `
substate-cli validate-substrate executes transactions in the given block segment
with substrate interpreter and report output equivalence.`,
	Category: "validate",
}

var valSbtCh chan string

func passSubstrateResult(subsResult string, passed bool, misc string) bool {
	if subsResult == "" {
		return false
	}

	if passed {
		subsResult += ",OK,"
	} else {
		subsResult += ",,"
	}
	subsResult += misc

	if valSbtCh == nil {
		fmt.Println(subsResult)
	} else {
		valSbtCh <- subsResult
	}
	return true
}

func validateSubstrateTask(block uint64, tx int, substate *research.Substate, taskPool *research.SubstateTaskPool) error {
	DEBUG = research.GetDebugFlag()
	trace := research.GetEVMTrace(block, tx)
	mode := vm.InterpreterMode_Substrate
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
		if !research.GetSkipInternalTxs() {
			repeat = len(trace.CallTraces)
		}
		mode = vm.InterpreterMode_EVM
	}
	isSlowTx := repeat >= research.GetSlowTxCalls()
	if isSlowTx {
		if research.GetSkipSlowTxs() {
			fmt.Printf("skip slow tx: block %v, tx %v, calls: %v\n", block, tx, repeat)
			return nil
		}
		fmt.Printf("slow tx begin: block %v, tx %v, calls: %v\n", block, tx, repeat)
	}
	for i := 0; i < repeat; i++ {
		if research.GetIsoIndex() != -1 && i != research.GetIsoIndex() {
			continue
		}

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
		vmConfig.IsolationMode = vm.InterpreterMode_Substrate
		vmConfig.IsolationIndex = i

		statedb.SetTxContext(common.Hash{}, tx)

		evm := vm.NewEVM(blockContext, vm.TxContext{}, statedb, chainConfig, vmConfig)

		evm.Delta = vm.NewDeltaSemantics(evm, substate, trace)

		txContext := core.NewEVMTxContext(txMessage)
		evm.Reset(txContext, statedb)

		gaspool := new(core.GasPool).AddGas(blockContext.GasLimit)

		var result *core.ExecutionResult
		var err error
		var errStr string
		func() {
			defer func() {
				if r := recover(); r != nil {
					errStr = strings.Replace(fmt.Sprint(r), ",", "", -1)
					err = fmt.Errorf("panic outside interpreter: %v", errStr)
				}
			}()
			result, err = core.ApplyMessage(evm, txMessage, gaspool)
		}()
		if err != nil {
			columns := strings.Split(evm.Delta.SubstrateResult, ",")
			columns[6] = err.Error()
			result := strings.Join(columns, ",")
			misc := fmt.Sprintf("%t,%t,%t,%s", research.GetContractIso(), false, false, txMessage.To.Hex())
			passSubstrateResult(result, false, misc)
			if research.GetContractIso() {
				continue
			} else {
				return err
			}
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
		if DEBUG {
			debugSubstateResult(substate, replaySubstate)
		}

		misc := fmt.Sprintf("%t,%t,%t,%s", research.GetContractIso(), matchedSA, substate.Result.EqualLog(replaySubstate.Result), txMessage.To.Hex())
		passSubstrateResult(evm.Delta.SubstrateResult, eqResult && eqAlloc, misc)
	}

	return nil
}

func validateSubstrateAction(ctx *cli.Context) error {
	var err error

	research.SetSubstateFlags(ctx)
	research.OpenSubstateDBReadOnly()
	defer research.CloseSubstateDB()

	taskPool := research.NewSubstateTaskPoolCli("substate-cli validate-substrate", validateSubstrateTask, ctx)

	segment, err := research.ParseBlockSegment(ctx.String(research.BlockSegmentFlag.Name))
	if err != nil {
		return fmt.Errorf("substate-cli validate-substrate: error parsing block segment: %w", err)
	}

	valSbtCh = make(chan string, 1_000_000)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		csvFile, err := os.Create(fmt.Sprintf("%d-%dM.csv", segment.First/1000000, segment.Last/1000000))
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		defer csvFile.Close()

		csvWriter := bufio.NewWriter(csvFile)
		defer csvWriter.Flush()

		//OoGInt&errInt = 10, OoGEVM&errEVM = 01
		header := "block,tx,intIdx,md5,date,func,errmsg,callInt,callEVM,sstoreInt,sstoreEVM,OoGstate,errstate,gas,codesize,result,isoEMI,matchedSA,log,addr"
		fmt.Fprintln(csvWriter, header)
		count := 0
		for result := range valSbtCh {
			count++
			fmt.Fprintln(csvWriter, result)

			if count%10 == 0 {
				csvWriter.Flush()
			}
		}
	}()

	err = taskPool.ExecuteSegment(segment)

	close(valSbtCh)
	wg.Wait()

	return err
}

var ValidateSubstrateTxCommand = &cli.Command{
	Action:  ValidateSubstrateTxAction,
	Name:    "validate-substrate-tx",
	Aliases: []string{"val-sbt-tx"},
	Usage:   "validate substrate with one transaction and report output equivalence",
	Flags: []cli.Flag{
		research.WorkersFlag,
		research.SkipTransferTxsFlag,
		research.SkipCallTxsFlag,
		research.SkipCreateTxsFlag,
		research.SubstateDirFlag,
		research.BlockSegmentFlag,
		research.TxIndexFlag,
		research.SubstrateDirFlag,
		research.ContractIsolationFlag,
		research.IsolationIndexFlag,
		research.DebugSubstrateFlag,
		research.SlowTxCallsFlag,
		research.SkipSlowTxsFlag,
		research.SkipInternalTxsFlag,
		research.RunOutOfGasFlag,
	},
	Description: `
substate-cli validate-substrate-tx executes one transaction
with substrate interpreter and report output equivalence.`,
	Category: "validate",
}

func ValidateSubstrateTxAction(ctx *cli.Context) error {
	var err error

	research.SetSubstateFlags(ctx)
	research.OpenSubstateDBReadOnly()
	defer research.CloseSubstateDB()

	taskPool := research.NewSubstateTaskPoolCli("substate-cli validate-substrate-tx", validateSubstrateTask, ctx)

	block := ctx.Uint64(research.BlockSegmentFlag.Name)
	tx := ctx.Int(research.TxIndexFlag.Name)

	err = taskPool.TaskFunc(block, tx, taskPool.DB.GetBlockSubstates(block)[tx], taskPool)

	return err
}

var DetectSubstrateDeviationCommand = &cli.Command{
	Action:  DetectSubstrateDeviationAction,
	Name:    "detect-deviation-substrate",
	Aliases: []string{"dev-sbt"},
	Usage:   "execute substrate with transactions until it deviates from bytecode",
	Flags: []cli.Flag{
		research.WorkersFlag,
		research.SkipTransferTxsFlag,
		research.SkipCallTxsFlag,
		research.SkipCreateTxsFlag,
		research.SubstateDirFlag,
		research.BlockSegmentFlag,
		research.SubstrateDirFlag,
		research.ContractIsolationFlag,
		research.DebugSubstrateFlag,
		research.SlowTxCallsFlag,
		research.SkipSlowTxsFlag,
		research.SkipInternalTxsFlag,
		research.RunOutOfGasFlag,
	},
	Description: `
substate-cli detect-substrate-deviation executes transactions in the given block segment
with substrate interpreter, and report the deviation point in the substrate based on SSTORE, LOG, calls, and SHA3.`,
	Category: "validate",
}

func DetectSubstrateDeviationAction(ctx *cli.Context) error {
	var err error

	research.SetSubstateFlags(ctx)
	research.OpenSubstateDBReadOnly()
	defer research.CloseSubstateDB()

	taskPool := research.NewSubstateTaskPoolCli("substate-cli detect-deviation", detectDeviationTask, ctx)

	segment, err := research.ParseBlockSegment(ctx.String(research.BlockSegmentFlag.Name))
	if err != nil {
		return fmt.Errorf("substate-cli detect-deviation: error parsing block segment: %w", err)
	}

	valSbtCh = make(chan string, 1_000_000)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		csvFile, err := os.Create(fmt.Sprintf("%d-%dM-deviation.csv", segment.First/1000000, segment.Last/1000000))
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		defer csvFile.Close()

		csvWriter := bufio.NewWriter(csvFile)
		defer csvWriter.Flush()

		//OoGInt&errInt = 10, OoGEVM&errEVM = 01
		header := "block,tx,intIdx,md5,date,func,errmsg,callInt,callEVM,sstoreInt,sstoreEVM,OoGstate,errstate,gas,codesize,result,isoEMI,matchedSA,log,addr"
		fmt.Fprintln(csvWriter, header)
		count := 0
		for result := range valSbtCh {
			count++
			fmt.Fprintln(csvWriter, result)

			if count%10 == 0 {
				csvWriter.Flush()
			}
		}
	}()

	err = taskPool.ExecuteSegment(segment)

	close(valSbtCh)
	wg.Wait()

	return err
}

var DetectSubstrateDeviationTxCommand = &cli.Command{
	Action:  DetectSubstrateDeviationTxAction,
	Name:    "detect-deviation-substrate-tx",
	Aliases: []string{"dev-sbt-tx"},
	Usage:   "execute substrate with a specific transaction until it deviates from bytecode",
	Flags: []cli.Flag{
		research.WorkersFlag,
		research.SkipTransferTxsFlag,
		research.SkipCallTxsFlag,
		research.SkipCreateTxsFlag,
		research.SubstateDirFlag,
		research.BlockSegmentFlag,
		research.TxIndexFlag,
		research.SubstrateDirFlag,
		research.ContractIsolationFlag,
		research.IsolationIndexFlag,
		research.DebugSubstrateFlag,
		research.SlowTxCallsFlag,
		research.SkipSlowTxsFlag,
		research.SkipInternalTxsFlag,
		research.RunOutOfGasFlag,
	},
	Description: `
substate-cli detect-substrate-deviation executes a transaction
with substrate interpreter, and report the deviation point in the substrate based on SSTORE, LOG, calls, and SHA3.`,
	Category: "validate",
}

func DetectSubstrateDeviationTxAction(ctx *cli.Context) error {
	var err error

	research.SetSubstateFlags(ctx)
	research.OpenSubstateDBReadOnly()
	defer research.CloseSubstateDB()

	taskPool := research.NewSubstateTaskPoolCli("substate-cli detect-deviation-tx", detectDeviationTask, ctx)

	block := ctx.Uint64(research.BlockSegmentFlag.Name)
	tx := ctx.Int(research.TxIndexFlag.Name)

	err = taskPool.TaskFunc(block, tx, taskPool.DB.GetBlockSubstates(block)[tx], taskPool)

	return err
}

func detectDeviationTask(block uint64, tx int, substate *research.Substate, taskPool *research.SubstateTaskPool) error {
	var err error

	statedb, _, _, _, err := replay.ReplayEVM(block, tx, substate)
	if err != nil {
		return fmt.Errorf("error replaying transaction to get trace: %w", err)
	}
	recordedTrace := statedb.ResearchEVMTrace.Copy()
	if research.GetDebugFlag() {
		fmt.Println("EVM finished. Start interpreter for deviation analysis.")
	}

	// Run substrate interpreter with the trace, and check deviation based on SSTORE, LOG, calls, and SHA3.
	// recordedTrace is passed to substrate interpreter via vmConfig.EVMTrace.
	err = validateSubstrateWithTrace(block, tx, substate, recordedTrace)
	if err != nil {
		return fmt.Errorf("error running interpreter for deviation analysis: %w", err)
	}

	return nil
}

func validateSubstrateWithTrace(block uint64, tx int, substate *research.Substate, recordedTrace *research.EVMTrace) error {
	DEBUG = research.GetDebugFlag()
	trace := research.GetEVMTrace(block, tx)
	mode := vm.InterpreterMode_Substrate
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
		if !research.GetSkipInternalTxs() {
			repeat = len(trace.CallTraces)
		}
		mode = vm.InterpreterMode_EVM
	}
	isSlowTx := repeat >= research.GetSlowTxCalls()
	if isSlowTx {
		if research.GetSkipSlowTxs() {
			fmt.Printf("skip slow tx: block %v, tx %v, calls: %v\n", block, tx, repeat)
			return nil
		}
		fmt.Printf("slow tx begin: block %v, tx %v, calls: %v\n", block, tx, repeat)
	}
	for i := 0; i < repeat; i++ {
		if research.GetIsoIndex() != -1 && i != research.GetIsoIndex() {
			continue
		}

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
		vmConfig.IsolationMode = vm.InterpreterMode_Substrate
		vmConfig.IsolationIndex = i
		vmConfig.EVMTrace = recordedTrace

		statedb.SetTxContext(common.Hash{}, tx)

		evm := vm.NewEVM(blockContext, vm.TxContext{}, statedb, chainConfig, vmConfig)

		evm.Delta = vm.NewDeltaSemantics(evm, substate, trace)

		txContext := core.NewEVMTxContext(txMessage)
		evm.Reset(txContext, statedb)

		gaspool := new(core.GasPool).AddGas(blockContext.GasLimit)

		var result *core.ExecutionResult
		var err error
		var errStr string
		func() {
			defer func() {
				if r := recover(); r != nil {
					errStr = strings.Replace(fmt.Sprint(r), ",", "", -1)
					err = fmt.Errorf("panic outside interpreter: %v", errStr)
				}
			}()
			result, err = core.ApplyMessage(evm, txMessage, gaspool)
		}()
		if err != nil {
			columns := strings.Split(evm.Delta.SubstrateResult, ",")
			columns[6] = err.Error()
			result := strings.Join(columns, ",")
			misc := fmt.Sprintf("%t,%t,%t,%s", research.GetContractIso(), false, false, txMessage.To.Hex())
			passSubstrateResult(result, false, misc)
			if research.GetContractIso() {
				continue
			} else {
				return err
			}
		}

		if chainConfig.IsByzantium(blockNumber) {
			statedb.Finalise(true)
		} else {
			// No need for root hash, call  Finalise instead of IntermediateRoot
			statedb.Finalise(chainConfig.IsEIP158(blockNumber))
		}

		// compare the traces and check remaining deviation for calls, sstore, log, and sha3 operations
		if evm.Delta.SubstrateResult != "" && err == nil {
			if msg, exist := evm.CheckRemainingDeviation(i); exist {
				tmp := strings.Split(evm.Delta.SubstrateResult, ",")
				if tmp[6] == "" {
					tmp[6] = "Deviated by remaining " + msg
					evm.Delta.SubstrateResult = strings.Join(tmp, ",")
				}
			}
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
		if DEBUG {
			debugSubstateResult(substate, replaySubstate)
		}

		misc := fmt.Sprintf("%t,%t,%t,%s", research.GetContractIso(), matchedSA, substate.Result.EqualLog(replaySubstate.Result), txMessage.To.Hex())
		passSubstrateResult(evm.Delta.SubstrateResult, eqResult && eqAlloc, misc)
	}

	return nil
}
