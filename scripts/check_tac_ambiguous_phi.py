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

    var2block = dict()
    phivars = dict()
    func = block = var = ""
    for line in p.read_text().splitlines():
        s = line.strip()
        if s == "}":
            func = block = var = ""
        if s.startswith("function ") and s.endswith("{"):
            func = s
            block = var = ""
        elif len(func) > 0 and s.startswith("Begin block "):
            block = s
        elif len(block) > 0 and s[:2] == "0x" and " = " in s:
            vars = s.split(": ")[1].split(" = ")[0].split(", ")
            vars = [x.split("(0x")[0] for x in vars]
            for var in vars:
                var2block[var] = block
                if "= PHI " in s:
                    phivars[var] = s.split("= PHI ")[1].split(", ")
                    phivars[var] = [x for x in phivars[var] if "arg" not in x]
                    phivars[var] = [x.split("(0x")[0] for x in phivars[var]]

    for phi in phivars:
        blist = [var2block[v] for v in phivars[phi] if v in var2block]
        bset = set(blist)
        if len(bset) < len(blist):
            res += (phi + "\n")

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
        "contracts have ambiguous PHI nodes",
    )


if __name__ == "__main__":
    main()
