package validate

import (
	"fmt"

	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/research"
	cli "github.com/urfave/cli/v2"
)

var DbCloneTraceCommand = &cli.Command{
	Action: dbCloneTrace,
	Name:   "db-clone-trace",
	Usage:  "Create a clone of substate DB with traces of a given block segment",
	Flags: []cli.Flag{
		research.WorkersFlag,
		research.BlockSegmentFlag,
		&cli.PathFlag{
			Name:     "src-path",
			Usage:    "Source DB path",
			Required: true,
		},
		&cli.PathFlag{
			Name:     "dst-path",
			Usage:    "Destination DB path",
			Required: true,
		},
		&cli.BoolFlag{
			Name:  "out-of-gas",
			Usage: "Clone only substates with OOG error",
		},
		research.TxListFlag,
	},
	Description: `
substate-cli db-clone-trace creates a clone of substate DB with traces.
This skips cloning substates without traces.
`,
	Category: "validate",
}

func dbCloneTrace(ctx *cli.Context) error {
	var err error

	srcPath := ctx.Path("src-path")
	srcBackend, err := rawdb.NewLevelDBDatabase(srcPath, 1024, 100, "srcDB", true)
	if err != nil {
		return fmt.Errorf("substate-cli db-clone: error opening %s: %w", srcPath, err)
	}
	srcDB := research.NewSubstateDB(srcBackend)
	defer srcDB.Close()

	// Create dst DB
	dstPath := ctx.Path("dst-path")
	dstBackend, err := rawdb.NewLevelDBDatabase(dstPath, 1024, 100, "srcDB", false)
	if err != nil {
		return fmt.Errorf("substate-cli db-clone: error creating %s: %w", dstPath, err)
	}
	dstDB := research.NewSubstateDB(dstBackend)
	defer dstDB.Close()

	onlyOOG := ctx.Bool("out-of-gas")

	type txIndex struct {
		block uint64
		tx    int
	}

	cloneTask := func(block uint64, tx int, substate *research.Substate, taskPool *research.SubstateTaskPool) error {
		if !srcDB.HasEVMTrace(block, tx) {
			return nil
		}

		trace := srcDB.GetEVMTrace(block, tx)
		if onlyOOG {
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

		dstDB.PutSubstate(block, tx, substate)
		dstDB.PutEVMTrace(block, tx, trace)

		return nil
	}

	taskPool := &research.SubstateTaskPool{
		Name:     "substate-cli db-clone-trace",
		TaskFunc: cloneTask,
		Config:   research.NewSubstateTaskConfigCli(ctx),

		DB: srcDB,
	}

	segment, err := research.ParseBlockSegment(ctx.String(research.BlockSegmentFlag.Name))
	if err != nil {
		return fmt.Errorf("substate-cli db-clone-trace: error parsing block segment: %s", err)
	}

	err = taskPool.ExecuteSegment(segment)

	return err
}
