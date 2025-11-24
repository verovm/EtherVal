#!/usr/bin/env python

from datetime import datetime
from multiprocessing import Pool
from statistics import median
import csv
import sys
import os

time_begin = datetime.now()

def filterTimeout(args):
    local_time_begin = datetime.now()
    fid, filename = args
    timeoutOnly = set()
    otherErrors = set()
    ntx = 0
    with open(filename, "r") as f:
        print(f"fid: {fid}, file: {filename}")
        reader = csv.DictReader(f)
        for line in reader:
            ntx += 1
            if ntx % 10_000_000 == 0:
                nct = len(timeoutOnly) + len(otherErrors)
                td = datetime.now() - local_time_begin
                speed = ntx/td.total_seconds()
                print(f"fid: {fid}, {td} #tx: {ntx/1e6:.02f}M, #contractsWithErrors: {nct/1e3:.02f}K, speed: {speed/1e3:.02f}K tx/s")

            md5, func, OK, usedEVM = line['md5'], line['func'], line['result'], line['usedEVM']

            if OK in ("OK", "PASSED"):
                continue
            elif md5 in otherErrors:
                continue
            elif OK == "TACTimeout":
                timeoutOnly.add(md5)
            else:
                otherErrors.add(md5)
                timeoutOnly.discard(md5)

        if ntx % 10_000_000 != 0:
            nct = len(timeoutOnly) + len(otherErrors)
            td = datetime.now() - local_time_begin
            speed = ntx/td.total_seconds()
            print(f"fid: {fid}, {td} #tx: {ntx/1e6:.02f}M, #contractsWithErrors: {nct/1e3:.02f}K, speed: {speed/1e3:.02f}K tx/s")

        return fid, filename, ntx, timeoutOnly, otherErrors


if __name__ == "__main__":
    if len(sys.argv) < 2:
        raise RuntimeError("no command line argument")

    filelist = []
    for arg in sys.argv[1:]:
        if os.path.isdir(arg):
            for filename in os.listdir(arg):
                f = os.path.join(arg, filename)
                if f.endswith(".csv"):
                    filelist.append(f)
        elif os.path.isfile(arg):
            filelist.append(arg)
    filelist.sort()

    with Pool() as pool:
        ntx_all = 0
        timeoutOnly = set()
        otherErrors = set()
        for fid, filename, ntx, to, oe in pool.imap(filterTimeout, enumerate(filelist, start=1)):
            ntx_all += ntx
            timeoutOnly = timeoutOnly.union(to)
            otherErrors = otherErrors.union(oe)
            timeoutOnly = timeoutOnly.difference(otherErrors)
            td = datetime.now() - time_begin
            speed = ntx_all/td.total_seconds()
            nct_all = len(timeoutOnly) + len(otherErrors)
            print(f"after fid: {fid}, {td}, #tx: {ntx_all/1e6:.02f}M, #contractsWithErrors: {nct_all}, #timeoutOnly: {len(timeoutOnly)}, #otherErrors: {len(otherErrors)}, speed: {speed/1e3:.02f}K tx/s")

    outname = "allowlist-timeout-only.txt"
    with open(outname, "w") as f:
        print(f"saving {len(timeoutOnly)} contracts in {outname}...")
        t = sorted(timeoutOnly)
        for i in range(len(t)):
            print(t[i], file=f)
    td = datetime.now() - time_begin
    print(f"finished in {td} (total {td.total_seconds()} seconds)")

