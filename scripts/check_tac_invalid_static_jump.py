#!/usr/bin/env python

from multiprocessing import Pool
from pathlib import Path
import os
import sys


def check_tac_invalid_static_jump(p: os.PathLike):
    if type(p) is not Path:
        p = Path(p).expanduser()

    res = ""

    func = block = succ = jump = ""
    for line in p.read_text().splitlines():
        s = line.strip()
        if s == "}":
            func = block = succ = jump = ""
        if s.startswith("function ") and s.endswith("{"):
            func = s
            block = succ = jump = ""
        elif func != "" and s.startswith("Begin block "):
            block = s
            succ = jump = ""
        elif block != "" and ", succ=[]" in s:
            succ = s
            jump = ""
        elif succ != "" and ": JUMP v" in s and "(0x" in s:
            jump = s

        if jump != "":
            res += "\n".join([func, block, succ, jump, ""])
            func = block = succ = jump = ""

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
    with Pool(4) as pool:
        for i, res in enumerate(pool.imap(check_tac_invalid_static_jump, ps, 10)):
            p = ps[i]
            n = i + 1
            if res != "":
                fail += 1
                print(n, "/", len(ps), p.stem, "FAIL")
            else:
                print(n, "/", len(ps), p.stem, "PASS")
    print(
        fail, "out of", len(ps),
        "contracts have invalid static jump destinations",
    )


if __name__ == "__main__":
    main()
