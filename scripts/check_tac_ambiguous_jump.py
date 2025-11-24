#!/usr/bin/env python

from multiprocessing import Pool
from pathlib import Path
import os
import sys


def base_addr(name: str):
    if name[:2] != "0x":
        raise RuntimeError("base_addr assumes address name string starts with 0x")

    name = name[2:]
    name = name.split("B")[0]
    name = name.split("0x")[0]
    name = name.split("_")[0]
    name = "0x"+name

    return name


def check_tac_ambiguous_jump(p: os.PathLike):
    if type(p) is not Path:
        p = Path(p).expanduser()

    res = ""

    # collect block names
    func = block = ""
    fdest: dict[str, list] = {}
    for line in p.read_text().splitlines():
        s = line.strip()
        if s == "}":
            func = block = jump = ""
        if s.startswith("function ") and s.endswith("{"):
            func = s
            fdest[func] = list()
        elif len(func) > 0 and s.startswith("Begin block "):
            block = s
            fdest[func].append(block[len("Begin block "):])

    # check if jump destination is ambiguous
    func = block = jump = ""
    dest = []
    for line in p.read_text().splitlines():
        s = line.strip()
        if s == "}":
            func = block = jump = ""
        if s.startswith("function ") and s.endswith("{"):
            func = s
            block = jump = ""
            dest = list(map(base_addr, fdest[func]))
        elif len(func) > 0 and s.startswith("Begin block "):
            block = s
            jump = ""
        elif len(block) > 0 and ": JUMP v" in s and "(0x" in s:
            jump = s[s.find("(0x")+1:-1]
            if dest.count(jump) > 1:
                res += "\n".join([func, block, jump, ""])
                func = block = jump = ""

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
        for i, res in enumerate(pool.imap(check_tac_ambiguous_jump, ps, 10)):
            p = ps[i]
            n = i + 1
            if len(res) > 0:
                fail += 1
                print(n, "/", len(ps), p.stem, "FAIL")
            else:
                print(n, "/", len(ps), p.stem, "PASS")
    print(
        fail, "out of", len(ps),
        "contracts have ambiguous jump destinations",
    )


if __name__ == "__main__":
    main()
