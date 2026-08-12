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

		vm.ErrTACThrow: "TACThrowErr",

		vm.ErrTACNoTac:      "NoTacTx",
		vm.ErrTACIllPhiExec: "IllPhiTx",
		vm.ErrTACIllJump:    "IllJumpTx",
		vm.ErrTACAmbiJump:   "AmbiJumpTx",

		vm.ErrTACNoCallTrace:   "CallTraceErr",
		vm.ErrTACNoGasTrace:    "GasTraceErr",
		vm.ErrTACMissingGasSem: "MissingGasSem",

		vm.ErrTACTimeout:    "TACTimeout",
		vm.ErrTACPanic:      "TACPanic",
		vm.ErrTACParseError: "TACParseError",
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
