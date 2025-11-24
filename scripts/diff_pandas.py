#!/usr/bin/env python

from pathlib import Path
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

    return rows


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
        if df_diff is None:
            df_diff = rows
        else:
            df_diff = pd.concat([df_diff, rows])

    if len(df_diff) > 0:
        print("NOT EMPTY")
        print("Total #tx:", len(df_diff))
        n1 = arg1.stem
        n2 = arg2.stem
        name = f"diff_{n1}_{n2}.csv"
        print(name)
        df_diff = df_diff.set_index(["block", "tx", "md5"])
        df_diff.to_csv(name)
    else:
        print("EMPTY")
