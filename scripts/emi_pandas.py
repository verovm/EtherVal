#!/usr/bin/env python

from datetime import datetime
from multiprocessing import Pool
from pathlib import Path
import sys

import pandas as pd

time_begin = datetime.now()


def countEMI(total, passed):
    count, countEMI, countEMIsingle = 0, 0, 0
    emiRate = [0, 0, 0, 0, 0]  # 100%, 90%, 80%, 70%, 60%
    for key, value in total.items():
        count += 1
        pvalue = passed.get(key, 0)
        if pvalue == value:
            countEMI += 1
            if pvalue == 1:
                countEMIsingle += 1
        for i, _ in enumerate(emiRate):
            if pvalue/value >= 1-i/10:
                emiRate[i] += 1
    # print("avg: ", sum(total.values())/len(total))
    count_div = 1 if count < 1 else count
    print(f'{countEMI:>6}/{count:>6}, {countEMI/count_div*100:>0.4f}%', end='\t')
    result = [f'{n/count_div*100:>0.1f}%' for n in emiRate]
    for r in result:
        print(r, end=', ')
    print()


def printEMI(filename):
    print(filename)
    print('contracts: ', end='')
    countEMI(contracts, contractsOK)
    print('functions: ', end='')
    countEMI(functions, functionsOK)
    print("total tx:", contracts.sum())
    print(end="", flush=True)


def aggCount(s1, s2):
    return pd.concat([s1, s2]).groupby(level=0).sum()


def runEMI(args):
    local_time_begin = datetime.now()
    fid, filename = args

    print(f"fid: {fid}, file: {filename}")

    contracts = functions = pd.Series(dtype="int64")
    contractsOK = functionsOK = pd.Series(dtype="int64")

    usecols = "md5,func,result".split(",")

    chunksize = 10_000_000

    chunkreader = pd.read_csv(filename, usecols=usecols, dtype="category", chunksize=chunksize)
    ptx = ntx = 0
    for chunk in chunkreader:
        df = chunk
        df_ok = df[df["result"].isin(["OK", "PASSED"])]
        s2s = [
            df.value_counts("md5"),
            df.value_counts(["md5", "func"]),
            df_ok.value_counts("md5"),
            df_ok.value_counts(["md5", "func"]),
        ]
        contracts = aggCount(contracts, s2s[0])
        functions = aggCount(functions, s2s[1])
        contractsOK = aggCount(contractsOK, s2s[2])
        functionsOK = aggCount(functionsOK, s2s[3])

        ntx += s2s[0].sum()
        if ntx >= ptx + 10_000_000:
            ptx = ntx
            nct = len(contracts)
            td = datetime.now() - local_time_begin
            speed = ntx/td.total_seconds()
            print(f"fid: {fid}, {td}, #tx: {ntx/1e6:.02f}M, #contracts: {nct/1e3:.02f}K, speed: {speed/1e3:.02f}K tx/s", flush=True)

    if ntx > ptx:
        ptx = ntx
        nct = len(contracts)
        td = datetime.now() - local_time_begin
        speed = ntx/td.total_seconds()
        print(f"fid: {fid}, {td}, #tx: {ntx/1e6:.02f}M, #contracts: {nct/1e3:.02f}K, speed: {speed/1e3:.02f}K tx/s", flush=True)

    chunkreader.close()

    return fid, filename, ntx, contracts, functions, contractsOK, functionsOK


if __name__ == "__main__":
    if len(sys.argv) < 2:
        raise RuntimeError("no command line argument")

    filelist = []
    for arg in sys.argv[1:]:
        p = Path(arg).expanduser()
        if p.is_dir():
            ps = [x for x in p.glob("*.csv") if x.is_file()]
            filelist += ps
        else:
            filelist += [p]
    filelist.sort()

    with Pool() as pool:
        ntx_all = 0
        contracts = functions = pd.Series(dtype="int64")
        contractsOK = functionsOK = pd.Series(dtype="int64")

        for fid, filename, ntx, ct, fn, ctok, fnok in pool.imap(runEMI, enumerate(filelist, start=1)):
            ntx_all += ntx
            contracts = aggCount(contracts, ct)
            functions = aggCount(functions, fn)
            contractsOK = aggCount(contractsOK, ctok)
            functionsOK = aggCount(functionsOK, fnok)

            td = datetime.now() - time_begin
            speed = ntx_all/td.total_seconds()
            nct_all = len(contracts)
            print(f"after fid: {fid}, {td}, #tx: {ntx_all/1e6:.02f}M, #contracts: {nct_all/1e3:.02f}K, speed: {speed/1e3:.02f}K tx/s", flush=True)

            printEMI(filename)

    Path("contracts.txt").write_text("\n".join(sorted(contracts.index))+"\n")
    Path("contractsOK.txt").write_text("\n".join(sorted(contractsOK.index))+"\n")
