use std::{
    collections::{BTreeMap, HashMap, HashSet, VecDeque},
    io::{BufRead, BufReader, BufWriter, Write},
    thread::{self, available_parallelism},
};

use auto_scripts::{open_n_files, par_tokenize_filter, AnyResult};
use clap::Parser;
use rayon::iter::{
    FromParallelIterator, IndexedParallelIterator, IntoParallelIterator, IntoParallelRefIterator,
    ParallelIterator,
};
use strum::VariantArray;
use strum_macros::{Display, VariantArray};

#[derive(Parser, Debug)]
#[command(about = "compare output from old version and new version, analyze what are different")]
struct Args {
    #[arg(short = 'N', long = "new")]
    new_path: String,
    #[arg(short = 'O', long = "old")]
    old_path: String,
    #[arg(short = 'o', long = "out")]
    out_path: String,
    #[arg(short = 'u', long = "unique_func_select")]
    unique_func_select: bool,
    #[arg(short = 's', long = "sorted")]
    sorted: bool,
    #[arg(short = 'e', long = "exclude_false_positive")]
    exclude_false_positive: bool,
    #[arg(short = 'k', long = "ok_only")]
    ok_only: bool,
}

#[derive(Display, VariantArray, Hash, Eq, PartialEq, Clone, Copy, PartialOrd, Ord)]
enum Element {
    ContextBlock,       // 0
    ContractMD5,        // 1
    PanicMsg,           // 2
    HexInput,           // 3
    EvmDepth,           // 4
    SubStrateSstoreCnt, // 5
    RecordedSstoreCnt,  // 6
    LeftOverGas,        // 7
    ContractCodeLen,    // 8
    EqResultAlloc,      // 9
    NoMatchingBlockTx,  // 10 bcz BlockTx is used like primary key
    TxMsgHex,           // 11
    ReplayEqLog,        // 12
}

static TABLE: &[Element] = Element::VARIANTS;

use std::cmp::Ordering::*;
use Element::*;

#[derive(Eq, PartialEq, Hash)]
struct DiffKey(Vec<Element>);

impl PartialOrd for DiffKey {
    fn partial_cmp(&self, other: &Self) -> Option<std::cmp::Ordering> {
        Some(self.cmp(other))
    }
}

impl Ord for DiffKey {
    fn cmp(&self, other: &Self) -> std::cmp::Ordering {
        let (a, b) = (&self.0, &other.0);
        if a.len() != b.len() {
            return a.len().cmp(&b.len());
        }
        a.cmp(b)
    }
}

fn main() -> AnyResult<()> {
    let args = Args::parse();
    let (mut read, mut write) = open_n_files([args.old_path, args.new_path], [args.out_path])?;
    let (old, new, out) = (read.remove(0), read.remove(0), write.remove(0));
    let old_reader = BufReader::new(old);
    let new_reader = BufReader::new(new);
    let mut writer = BufWriter::new(out);

    let old_lines: Vec<String> = old_reader.lines().map_while(Result::ok).collect();
    let new_lines: Vec<String> = new_reader.lines().map_while(Result::ok).collect();
    let old_t = par_tokenize_filter(old_lines, ",", 13);
    let new_t = par_tokenize_filter(new_lines, ",", 13);

    // diff_list -> (new, old)
    let mut diff_map: HashMap<DiffKey, Vec<(String, String)>> = if !args.sorted {
        // unsorted -> brute force map reduce
        new_t
            .into_par_iter()
            .filter_map(|(tokens, line)| {
                let new_has_ok = tokens.iter().find(|&x| x == "OK").is_some();
                let skip_new_ok = args.ok_only || args.exclude_false_positive;
                if skip_new_ok && new_has_ok {
                    return None;
                }
                let Some((ref_tokens, ref_line)) = old_t.iter().find(|(t, _)| {
                    let Some(block_tx_ref) = t
                        .iter()
                        .skip(10)
                        .find(|token| token.find(|ch| ch == '_').is_some())
                    else {
                        return false;
                    };
                    let Some(block_tx) = tokens
                        .iter()
                        .skip(10)
                        .find(|token| token.find(|ch| ch == '_').is_some())
                    else {
                        return false;
                    };
                    block_tx_ref == block_tx
                }) else {
                    return Some(HashMap::from([(
                        DiffKey(vec![NoMatchingBlockTx]),
                        vec![(line, "NOT EXIST".into())],
                    )]));
                };
                if args.ok_only {
                    if ref_tokens.iter().find(|&s| s == "OK").is_none() {
                        return None;
                    }
                }
                let mut diff_list = Vec::new();
                for (idx, val) in TABLE.iter().enumerate() {
                    if ref_tokens[idx] != tokens[idx] {
                        diff_list.push(*val);
                    }
                }
                match diff_list.is_empty() {
                    true => None,
                    false => Some(HashMap::from([(
                        DiffKey(diff_list),
                        vec![(line, ref_line.to_string())],
                    )])),
                }
            })
            .reduce(HashMap::new, |mut a, b| {
                for (k, v) in b {
                    a.entry(k).or_default().extend(v);
                }
                a
            })
    } else {
        // sorted -> partition and pop
        let cores = available_parallelism()?.get();
        println!("num cores: {cores}");
        let base = old_t.len() / cores;
        let remain = old_t.len() % cores;
        let mut workload_left = new_t;
        let mut ref_left = old_t;

        let mut partitions = Vec::with_capacity(cores);

        let mut diff_map_split_phase: HashMap<Vec<Element>, Vec<(String, String)>> = HashMap::new();

        for i in 0..cores {
            let size = if i < remain { base + 1 } else { base };
            let cur_new_t = if workload_left.len() > size {
                let mut right_part = workload_left.split_off(size);
                (right_part, workload_left) = (workload_left, right_part);
                right_part
            } else {
                partitions.push((workload_left, ref_left));
                break;
            };
            let mut ref_end = None;
            for (tokens, _) in cur_new_t.iter().rev() {
                ref_end = ref_left.par_iter().position_first(|(t, _)| {
                    let Some(block_tx_ref) = t
                        .iter()
                        .skip(10)
                        .find(|token| token.find(|ch| ch == '_').is_some())
                    else {
                        return false;
                    };
                    let Some(block_tx) = tokens
                        .iter()
                        .skip(10)
                        .find(|token| token.find(|ch| ch == '_').is_some())
                    else {
                        return false;
                    };
                    block_tx_ref == block_tx
                });
                if ref_end.is_some() {
                    break;
                }
            }
            let Some(ref_end) = ref_end else {
                let diff_not_exists: Vec<(String, String)> = cur_new_t
                    .into_par_iter()
                    .map(|(_, line)| (line, "NOT EXIST".into()))
                    .collect();
                diff_map_split_phase
                    .entry(vec![NoMatchingBlockTx])
                    .or_default()
                    .extend(diff_not_exists);
                continue;
            };
            if ref_end + 1 >= ref_left.len() {
                partitions.push((cur_new_t, ref_left));
                break;
            } else {
                let mut right_part = ref_left.split_off(ref_end + 1);
                (right_part, ref_left) = (ref_left, right_part);
                partitions.push((cur_new_t, right_part));
            }
        }

        let mut handles = Vec::with_capacity(cores);
        for (new_t, old_t) in partitions {
            let handle = thread::spawn(|| sorted_task(new_t, old_t));
            handles.push(handle);
        }
        let mut diff_map: HashMap<DiffKey, Vec<(String, String)>> = HashMap::new();
        for h in handles {
            let result = h.join().unwrap();
            for (k, v) in result {
                diff_map.entry(DiffKey(k)).or_default().extend(v);
            }
        }
        diff_map
    };

    if args.unique_func_select {
        diff_map = diff_map
            .into_par_iter()
            .map(|(k, v)| {
                let mut check_set: HashSet<(String, String)> = HashSet::new();
                let mut filtered_list = Vec::new();
                for (new, old) in &v {
                    let new_sel_fun = extract_input_hex(new);
                    let old_sel_fun = extract_input_hex(old);
                    let entry = (new_sel_fun, old_sel_fun);
                    if check_set.contains(&entry) {
                        continue;
                    }
                    check_set.insert(entry);
                    filtered_list.push((new.into(), old.into()));
                }
                (k, filtered_list)
            })
            .collect();
    }

    let sorted_map: BTreeMap<DiffKey, Vec<(String, String)>> = BTreeMap::from_par_iter(diff_map);

    for (k, v) in sorted_map {
        let diff_types: Vec<String> =
            k.0.iter()
                .map(|e| {
                    let idx = TABLE.iter().position(|t| t == e).unwrap();
                    format!("{e}({idx})")
                })
                .collect();
        let diff_types = diff_types.join(", ");
        writeln!(writer, "{}", diff_types)?;
        for (new, old) in v {
            writeln!(writer, "  new: {}", new)?;
            writeln!(writer, "  old: {}", old)?;
            writeln!(writer)?;
        }
    }
    writer.flush()?;
    Ok(())
}

#[inline]
fn extract_input_hex(line: &str) -> String {
    let tokens: Vec<&str> = line.split(",").into_iter().skip(3).collect();
    let Some(select_func) = tokens.into_par_iter().find_first(|&s| s.starts_with("0x")) else {
        return String::new();
    };
    select_func.into()
}

#[inline]
fn sorted_cmp(new: &[String], old: &[String]) -> Option<Vec<Element>> {
    match new[10].cmp(&old[10]) {
        Less => None,
        Equal => {
            let mut diff_list = Vec::new();
            for (idx, val) in TABLE.iter().enumerate() {
                if new[idx] != old[idx] {
                    diff_list.push(*val);
                }
            }
            Some(diff_list)
        }
        Greater => Some(vec![NoMatchingBlockTx]),
    }
}

fn sorted_task(
    new_t: VecDeque<(Vec<String>, String)>,
    mut old_t: VecDeque<(Vec<String>, String)>,
) -> HashMap<Vec<Element>, Vec<(String, String)>> {
    let mut diff_map: HashMap<Vec<Element>, Vec<(String, String)>> = HashMap::new();
    for (tokens, line) in new_t {
        while let Some((ref_tokens, ref_line)) = old_t.pop_front() {
            match sorted_cmp(&tokens, &ref_tokens) {
                None => continue, // not yet reached to target, pop more
                Some(diff_list) => {
                    if diff_list.is_empty() {
                        break;
                    }
                    if diff_list[0] == NoMatchingBlockTx {
                        // restore
                        old_t.push_front((ref_tokens, ref_line));
                        diff_map
                            .entry(diff_list)
                            .or_default()
                            .push((line, "NOT EXIST".into()));
                    } else {
                        diff_map
                            .entry(diff_list)
                            .or_default()
                            .push((line, ref_line));
                    }
                    break;
                }
            }
        }
    }
    diff_map
}
