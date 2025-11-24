#!/usr/bin/env bash

if [ "$#" -ne 2 ];  then
    echo "$0 input_path output_path"
    exit 1
fi

INPUT_PATH="$1"
OUTPUT_PATH="$2"

head -n 1 "$INPUT_PATH" > "$OUTPUT_PATH"
tail -n +2 "$INPUT_PATH" | sort -t, -k1,1 -k2,2 >> "$OUTPUT_PATH"
