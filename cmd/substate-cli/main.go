package main

import (
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/cmd/substate-cli/db"
	"github.com/ethereum/go-ethereum/cmd/substate-cli/replay"
	rr03_db "github.com/ethereum/go-ethereum/cmd/substate-cli/rr03/db"
	"github.com/ethereum/go-ethereum/cmd/substate-cli/validate"
	"github.com/ethereum/go-ethereum/internal/flags"
	cli "github.com/urfave/cli/v2"
)

var (
	app = flags.NewApp("Ethereum substate command line interface")
)

func init() {
	app.Flags = []cli.Flag{}
	app.Commands = []*cli.Command{
		replay.ReplayCommand,
		replay.ReplayTxCommand,
		replay.ReplayForkCommand,
		db.DbCloneCommand,
		db.DbCompactCommand,
		db.DbDumpCodeCommand,
		db.DbExportCommand,
		db.DbRr03ToRr04Command,
		rr03_db.UpgradeCommand,
		rr03_db.CloneCommand,
		rr03_db.CompactCommand,
		validate.RecordTraceCommand,
		validate.ValidateSubstrateCommand,
		validate.ValidateSubstrateTxCommand,
		validate.ValidateTacCommand,
		validate.ValidateTacTxCommand,
		validate.DbCloneTraceCommand,
		validate.DetectSubstrateDeviationCommand,
		validate.DetectSubstrateDeviationTxCommand,
		validate.DetectDeviationTacCommand,
		validate.DetectDeviationTacTxCommand,
		validate.StatTxErrCommand,
	}
	for _, command := range app.Commands {
		if command.Category == "" {
			command.Category = "~uncategorized"
		}
	}
}

func main() {
	if err := app.Run(os.Args); err != nil {
		code := 1
		fmt.Fprintln(os.Stderr, err)
		os.Exit(code)
	}
}
