package validate

import (
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/research"
	cli "github.com/urfave/cli/v2"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// substrate-intpereter: record-trace command
var RecordTraceCommand = &cli.Command{
	Action: RecordTrace,
	Name:   "record-trace",
	Usage:  "replay EVM bytecodes and store EVM traces in DB",
	Flags: []cli.Flag{
		research.WorkersFlag,
		research.SkipTransferTxsFlag,
		research.SkipCallTxsFlag,
		research.SkipCreateTxsFlag,
		research.SubstateDirFlag,
		research.BlockSegmentFlag,
	},
	Description: `
The record-trace command requires two arguments:
<blockNumFirst> <blockNumLast>

<blockNumFirst> and <blockNumLast> are the first and last block
of the inclusive range of blocks to record traces of EVM bytecodes.`,
	Category: "validate",
}

// substrate-interpreter: func recordTraceSubstateTask
func recordTraceSubstateTask(block uint64, tx int, substate *research.Substate, taskPool *research.SubstateTaskPool) error {
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

	statedb.SetTxContext(common.Hash{}, tx)

	evm := vm.NewEVM(blockContext, vm.TxContext{}, statedb, chainConfig, vmConfig)

	txContext := core.NewEVMTxContext(txMessage)
	evm.Reset(txContext, statedb)

	gaspool := new(core.GasPool).AddGas(blockContext.GasLimit)

	result, err := core.ApplyMessage(evm, txMessage, gaspool)
	if err != nil {
		return err
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

	eqAlloc := proto.Equal(substate.OutputAlloc, replaySubstate.OutputAlloc)
	eqResult := proto.Equal(substate.Result, replaySubstate.Result)

	if !(eqAlloc && eqResult) {
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

		return fmt.Errorf("not faithful replay - inconsistent output")
	}

	// substrate-interpreter: store EVM trace
	evmTrace := statedb.ResearchEVMTrace
	research.PutEVMTrace(block, tx, evmTrace)

	return nil
}

// substrate-interpreter: func RecordTrace for record-trace command
func RecordTrace(ctx *cli.Context) error {
	var err error

	research.SetSubstateFlags(ctx)
	research.OpenSubstateDB()
	defer research.CloseSubstateDB()

	taskPool := research.NewSubstateTaskPoolCli("substate-cli record-trace", recordTraceSubstateTask, ctx)

	segment, err := research.ParseBlockSegment(ctx.String(research.BlockSegmentFlag.Name))
	if err != nil {
		return fmt.Errorf("substate-cli replay: error parsing block segment: %w", err)
	}

	err = taskPool.ExecuteSegment(segment)

	return err
}
