#!/usr/bin/env python3

from multiprocessing import Pool
from pathlib import Path
import os
import re
import sys


UNARY_OPS = """
ISZERO BALANCE CALLDATALOAD EXTCODESIZE EXTCODEHASH
BLOCKHASH MLOAD SLOAD JUMP SELFDESTRUCT
NOT
""".split()

BINARY_OPS = """
ADD MUL SUB DIV SDIV MOD
SMOD ADDMOD MULMOD EXP SIGNEXTEND
LT GT SLT SGT EQ
AND OR XOR BYTE SHL
SHR SAR SHA3 MSTORE MSTORE8
SSTORE JUMPI REVERT RETURN LOG0
""".split()

TRINARY_OPS = """
CALLDATACOPY CODECOPY RETURNDATACOPY LOG1 CREATE
""".split()


def check_tac_missing_operand(p: os.PathLike):
    if type(p) is not Path:
        p = Path(p).expanduser()

    taccode = p.read_text()
    missing = set()
    for op in UNARY_OPS + BINARY_OPS + TRINARY_OPS:
        matches = re.findall(r'{}\s*\n'.format(op), taccode)
        missing = missing.union([m.strip() for m in matches])

    res = " ".join(sorted(missing))

    return res


def main():
    if len(sys.argv) != 2:
        print("Usage: python {} tac-file-or-dir-name".format(sys.argv[1]))
        sys.exit(1)
    p = Path(sys.argv[1]).expanduser()
    if p.is_dir():
        ps = [x for x in p.glob("*.tac") if x.is_file()]
        ps.sort()
    else:
        ps = [p]

    fail = 0
    missing = set()
    with Pool(4) as pool:
        for i, res in enumerate(pool.imap(check_tac_missing_operand, ps, 10)):
            p = ps[i]
            n = i + 1

            if res != "":
                fail += 1
                missing = missing.union(res.split())
                print(n, "/", len(ps), p.stem, "FAIL")
            else:
                print(n, "/", len(ps), p.stem, "PASS")
    print(
        fail, "out of", len(ps),
        "contracts have instructions with missing operands",
    )
    print("Instructions with missing operands:", " ".join(sorted(missing)))


if __name__ == "__main__":
    main()
