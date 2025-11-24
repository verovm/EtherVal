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


def check_tac_invalid_static_jump(p: os.PathLike):
    if type(p) is not Path:
        p = Path(p).expanduser()

    res = ""

    addrfunc: dict[str, str] = dict()
    funcargc: dict[str, int] = dict()
    func = block = ""
    for line in p.read_text().splitlines():
        s = line.strip()
        if s == "}":
            func = block = ""
        if s.startswith("function ") and s.endswith("private {"):
            func = s
            block = ""
            # save argc
            f = func[len("function "):func.find("(")]
            c = len(func[func.find("(")+1:func.find(")")].replace(",", " ").split())
            funcargc[f] = c
        elif func != "" and s.startswith("Begin block "):
            block = s
            addr = base_addr(block[len("Begin block "):])
            if f not in addrfunc.values():
                addrfunc[addr] = f

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
            addr = base_addr(jump[jump.find("(")+1:jump.find(")")])
            if addr in addrfunc and funcargc[addrfunc[addr]] > 0:
                f = addrfunc[addr]
                c = funcargc[addrfunc[addr]]
                res += "\n".join([func, block, succ, jump, ""])
                res += "\n".join([addr, f, str(c), ""])
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
                # print(res)
            else:
                print(n, "/", len(ps), p.stem, "PASS")
    print(
        fail, "out of", len(ps),
        "contracts have invalid static jump destinations to private functions with parameters",
    )


if __name__ == "__main__":
    main()
