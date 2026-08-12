package validate

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/ethereum/go-ethereum/cmd/substate-cli/replay"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/research"
	cli "github.com/urfave/cli/v2"
)

var StatTxErrCommand = &cli.Command{
	Action: statTxErrAction,
	Name:   "stat-tx-err",
	Usage:  "Report statistics for OOG transactions",
	Flags: []cli.Flag{
		research.WorkersFlag,
		research.BlockSegmentFlag,
		research.SubstateDirFlag,
		&cli.StringFlag{
			Name:  "output-csv",
			Usage: "Specify the path of output CSV file",
			Value: "stat-tx-err.csv",
		},
		research.SlowTxCallsFlag,
		research.SkipSlowTxsFlag,
	},
	Description: `
substate-cli stat-tx-err reports error messages of isolated EVM external/internal transactions.
`,
	Category: "validate",
}

func statTxErrAction(ctx *cli.Context) error {
	var err error

	research.SetSubstateFlags(ctx)
	research.OpenSubstateDBReadOnly()
	defer research.CloseSubstateDB()

	csvPath := ctx.String("output-csv")
	csvFile, err := os.OpenFile(csvPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer csvFile.Close()

	csvWriter := csv.NewWriter(csvFile)
	defer csvWriter.Flush()

	var totalTx int64
	statChan := make(chan [][]string, 1_000_000)
	statWg := sync.WaitGroup{}

	statWg.Add(1)
	go func() {
		defer statWg.Done()
		defer csvWriter.Flush()
		csvWriter.Write([]string{
			"block",
			"tx",
			"evmIdx",
			"OOG",
			"errMsg",
		})
		csvWriter.Flush()
		for tuples := range statChan {
			prevTotalTx := totalTx
			for _, tuple := range tuples {
				totalTx++
				csvWriter.Write(tuple)

				if totalTx-prevTotalTx >= 10 {
					csvWriter.Flush()
					prevTotalTx = totalTx
				}
			}
			csvWriter.Flush()
		}
	}()

	statTask := func(block uint64, tx int, substate *research.Substate, taskPool *research.SubstateTaskPool) error {
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

		var recordedTrace *research.EVMTrace
		if research.HasEVMTrace(block, tx) {
			recordedTrace = research.GetEVMTrace(block, tx)
		} else {
			statedb, _, _, _, err := replay.ReplayEVM(block, tx, substate)
			if err != nil {
				return fmt.Errorf("error replaying transaction to get trace: %w", err)
			}
			recordedTrace = statedb.ResearchEVMTrace.Copy()
		}

		repeat := len(recordedTrace.CallTraces)
		isSlowTx := repeat >= research.GetSlowTxCalls()
		if isSlowTx {
			if research.GetSkipSlowTxs() {
				fmt.Printf("skip slow tx: block %v, tx %v, calls: %v\n", block, tx, repeat)
				return nil
			}
			fmt.Printf("slow tx begin: block %v, tx %v, calls: %v\n", block, tx, repeat)
		}

		tuples := make([][]string, 0, len(recordedTrace.CallTraces))
		for idx, ct := range recordedTrace.CallTraces {
			if ct.Err == nil && ct.ErrMsg != "" {
				ct.Err = errors.New(ct.ErrMsg)
			}
			if ct.Err != nil && ct.ErrMsg == "" {
				ct.ErrMsg = ct.Err.Error()
			}
			tuple := []string{
				fmt.Sprintf("%v", block),
				fmt.Sprintf("%v", tx),
				fmt.Sprintf("%v", idx),
				fmt.Sprintf("%v", ct.ErrMsg == vm.ErrOutOfGas.Error()),
				ct.ErrMsg,
			}
			tuples = append(tuples, tuple)
		}
		if len(tuples) > 0 {
			statChan <- tuples
		}

		return nil
	}

	taskPool := research.NewSubstateTaskPoolCli("substate-cli tx-stat-errmsg", statTask, ctx)

	segment, err := research.ParseBlockSegment(ctx.String(research.BlockSegmentFlag.Name))
	if err != nil {
		return fmt.Errorf("substate-cli tx-stat-errmsg: error parsing block segment: %s", err)
	}

	err = taskPool.ExecuteSegment(segment)

	close(statChan)
	statWg.Wait()

	return err
}
