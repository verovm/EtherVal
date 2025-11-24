#!/usr/bin/env python

from datetime import datetime
from multiprocessing import Pool
from pathlib import Path
import csv
import sys


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


def runEMI(args):
    local_time_begin = datetime.now()
    fid, filename = args

    contracts = {}  # contract MD5. same bytecodes with different address are considered same
    functions = {}
    contractsOK = {}
    functionsOK = {}

    ntx = 0
    with open(filename, 'r') as f:
        print(f"fid: {fid}, file: {filename}")
        reader = csv.DictReader(f)
        for line in reader:
            md5, func, OK= line['md5'], line['func'], line['result']

            key = md5+','+func
            contracts[md5] = contracts.get(md5, 0) + 1
            functions[key] = functions.get(key, 0) + 1
            if OK in ("OK", "PASSED"):
                contractsOK[md5] = contractsOK.get(md5, 0) + 1
                functionsOK[key] = functionsOK.get(key, 0) + 1

            ntx += 1
            if ntx % 10_000_000 == 0:
                nct = len(contracts)
                td = datetime.now() - local_time_begin
                speed = ntx/td.total_seconds()
                print(f"fid: {fid}, {td}, #tx: {ntx/1e6:.02f}M, #contracts: {nct/1e3:.02f}K, speed: {speed/1e3:.02f}K tx/s")

        if ntx % 10_000_000 != 0:
            nct = len(contracts)
            td = datetime.now() - local_time_begin
            speed = ntx/td.total_seconds()
            print(f"fid: {fid}, {td}, #tx: {ntx/1e6:.02f}M, #contracts: {nct/1e3:.02f}K, speed: {speed/1e3:.02f}K tx/s")

        return fid, filename, ntx, contracts, functions, contractsOK, functionsOK


def sumCount(d1, d2):
    d3 = {}
    for k, v in d1.items():
        d3[k] = v
    for k, v in d2.items():
        d3[k] = d3.get(k, 0) + v
    return d3


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

    with Pool() as pool:
        ntx_all = 0
        contracts = {}  # contract MD5. same bytecodes with different address are considered same
        functions = {}
        contractsOK = {}
        functionsOK = {}

        for fid, filename, ntx, ct, fn, ctok, fnok, in pool.imap(runEMI, enumerate(filelist, start=1)):
            ntx_all += ntx
            contracts = sumCount(contracts, ct)
            functions = sumCount(functions, fn)
            contractsOK = sumCount(contractsOK, ctok)
            functionsOK = sumCount(functionsOK, fnok)

            td = datetime.now() - time_begin
            speed = ntx_all/td.total_seconds()
            nct_all = len(contracts)
            print(f"after fid: {fid}, {td}, #tx: {ntx_all/1e6:.02f}M, #contracts: {nct_all/1e3:.02f}K, speed: {speed/1e3:.02f}K tx/s")

            print(filename)
            print('contracts: ', end='')
            countEMI(contracts, contractsOK)
            print('functions: ', end='')
            countEMI(functions, functionsOK)
            print("total tx:", sum(contracts.values()))
            print(end="", flush=True)

    Path("contracts.txt").write_text("\n".join(sorted(contracts))+"\n")
    Path("contractsOK.txt").write_text("\n".join(sorted(contractsOK))+"\n")
