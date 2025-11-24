#!/usr/bin/env python

from pathlib import Path
import glob
import os
import sys

import pandas as pd

df = None
df_diff = None


def find_diff(file1, file2):
    global df, df_diff

    usecols = "block,tx,md5,result".split(",")
    df1 = pd.read_csv(file1, usecols=usecols)
    df1["ok"] = df1["result"].isin(["OK", "PASSED"])
    df2 = pd.read_csv(file2, usecols=usecols)
    df2["ok"] = df2["result"].isin(["OK", "PASSED"])
    df = pd.merge(df1, df2, how="outer", on=[
                  "block", "tx", "md5"], suffixes=["1", "2"])

    rows = df[df["ok1"] != df["ok2"]]
    md5s = rows["md5"].unique()
    rows = df[df["md5"].isin(md5s)]

    return rows


contracts1 = contractsOK1 = pd.Series(dtype="int64")
contracts2 = contractsOK2 = pd.Series(dtype="int64")
emi1 = set()
emi2 = set()


def run_emi(rows):
    global contracts1, contractsOK1
    global contracts2, contractsOK2
    global emi1
    global emi2

    def aggCount(s1, s2):
        return pd.concat([s1, s2]).groupby(level=0).sum()

    contracts1 = aggCount(contracts1, rows.value_counts(["md5"]))
    contractsOK1 = aggCount(
        contractsOK1, rows[rows["ok1"]].value_counts(["md5"]))
    for key, value in contracts1.items():
        pvalue = contractsOK1.get(key, 0)
        if pvalue == value:
            emi1.add("_".join(key))

    contracts2 = aggCount(contracts2, rows.value_counts(["md5"]))
    contractsOK2 = aggCount(
        contractsOK2, rows[rows["ok2"]].value_counts(["md5"]))
    for key, value in contracts2.items():
        pvalue = contractsOK2.get(key, 0)
        if pvalue == value:
            emi2.add("_".join(key))


if __name__ == "__main__":
    if len(sys.argv) != 3:
        raise RuntimeError("Usage: ./diff_pandas.py output1.csv output2.csv")

    arg1 = Path(sys.argv[1]).expanduser()
    if arg1.is_dir():
        ps1 = [x for x in arg1.glob("*.csv") if x.is_file()]
        ps1.sort()
    else:
        ps1 = [arg1]

    arg2 = Path(sys.argv[2]).expanduser()
    if arg2.is_dir():
        ps2 = [x for x in arg2.glob("*.csv") if x.is_file()]
        ps2.sort()
    else:
        ps2 = [arg2]

    for p1, p2 in zip(ps1, ps2):
        print(p1, p2)
        rows = find_diff(p1, p2)
        print("#tx:", len(rows))
        run_emi(rows)

    n1 = arg1.stem
    n2 = arg2.stem
    name = f"diff_emi_{n1}_{n2}.txt"
    print()

    inter = set.intersection(emi1, emi2)
    print(f"EMI in {n1}, EMI in {n2}, {len(inter)} contracts")
    print()

    d1 = list(emi1 - emi2)
    d1.sort()
    print(f"EMI in {n1}, not EMI in {n2}, {len(d1)} contracts")
    print("\n".join(d1))
    print()

    d2 = list(emi2 - emi1)
    d2.sort()
    print()
    print(f"not EMI in {n1}, EMI in {n2}, {len(d2)} contracts")
    print("\n".join(sorted(d2)))
    print()
