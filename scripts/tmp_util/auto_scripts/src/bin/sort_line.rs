use auto_scripts::{open_n_files, par_tokenize_filter, AnyResult};
use clap::Parser;
use std::io::{BufRead, BufReader, BufWriter, Write};
use rayon::prelude::*;

#[derive(Parser, Debug)]
#[command(about = "sort substate interpret debug output by block_idx")]
struct Args {
    #[arg(short = 'i', long = "in")]
    input_path: String,
    #[arg(short = 'o', long = "out")]
    output_path: String,
}

fn main() -> AnyResult<()> {
    let args = Args::parse();
    let (mut read, mut write) = open_n_files([args.input_path], [args.output_path])?;
    let (input, output) = (read.remove(0), write.remove(0));
    let reader = BufReader::new(input);
    let mut writer = BufWriter::new(output);

    let lines: Vec<String> = reader.lines().map_while(Result::ok).collect();

    let token_map = par_tokenize_filter(lines, ",", 13);
    let mut block_idx_line: Vec<(usize, usize, String)> = token_map
        .into_par_iter()
        .filter_map(|(tokens, line)| {
            let split_idx = tokens[10].find('_')?;
            let (block, tx) = (&tokens[10][0..split_idx], &tokens[10][split_idx + 1..]);
            let (block, tx) = (block.parse().ok()?, tx.parse().ok()?);
            Some((block, tx, line))
        })
        .collect();

    block_idx_line.par_sort_unstable_by(|(block1, tx1, _), (block2, tx2, _)| {
        if let std::cmp::Ordering::Equal = block1.cmp(block2) {
            return tx1.cmp(tx2);
        }
        block1.cmp(block2)
    });
    for (_, _, line) in block_idx_line {
        writeln!(writer, "{}", line)?;
    }
    Ok(())
}
