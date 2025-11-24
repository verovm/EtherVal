package validate

import (
	"fmt"

	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/urfave/cli/v2"
)

var (
	TACInconsistentResult = fmt.Errorf("Inconsistent Result")
)

var (
	tacResultCategoryMap = map[error]string{
		nil:                   "PASSED",
		TACInconsistentResult: "FAILED",

		vm.ErrTACParseError: "TACParseError",
		vm.ErrTACPanic:      "TACPanic",
		vm.ErrTACTimeout:    "TACTimeout",

		vm.ErrTACIllJump:    "IllJumpTx",
		vm.ErrTACIllPhiExec: "IllPhiTx",
		vm.ErrTACThrow:      "TACThrowErr",

		vm.ErrTACNoTac:       "NoTacTx",
		vm.ErrTACNoGasTrace:  "GasTraceErr",
		vm.ErrTACNoCallTrace: "CallTraceErr",
	}
)

func TACResultCategory(err error) string {
	cat, exist := tacResultCategoryMap[err]
	if !exist {
		panic(err)
	}
	return cat
}

var (
	ValidateOutputCsvFlag = &cli.StringFlag{
		Name:    "validate-output-csv",
		Aliases: []string{"val-out-csv"},
		Usage:   "Specify the path of output CSV file",
		Value:   "validate-output.csv",
	}
)
