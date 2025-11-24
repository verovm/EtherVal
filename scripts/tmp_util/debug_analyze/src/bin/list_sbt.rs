use std::{
    collections::HashMap,
    error::Error,
    fmt,
    fmt::Display,
    fs::OpenOptions,
    io::{BufWriter, Write},
    sync::{Arc, OnceLock},
};

use clap::Parser;
use serde::{Deserialize, Deserializer};
use tokio::sync::mpsc::UnboundedSender;

#[derive(Parser, Debug)]
#[command(about = "filter substrate md5 and function")]
struct Args {
    in_path: String,
    out_path: String,
    #[arg(short = 'c', long = "max_children")]
    max_children: Option<usize>,
    #[arg(short = 's', long = "substate_dir")]
    substate_dir: Option<String>,
    #[arg(short = 't', long = "substrate_dir")]
    substrate_dir: Option<String>,
}

static SUBSTATE_DIR: OnceLock<String> = OnceLock::new();
static SUBSTRATE_DIR: OnceLock<String> = OnceLock::new();

fn main() -> Result<(), Box<dyn Error>> {
    let args = Args::parse();
    let max_children = args.max_children.unwrap_or(5);
    let default_substate_dir =
        "/zpool2/backup-zpool1/substrate-interpreter/rr0.4.substate.ethereum.0-10M.record-trace"
            .into();
    let default_substrate_dir = "/zpool2/ntamv29/dedaub-web+api/".into();
    SUBSTATE_DIR.set(args.substate_dir.unwrap_or(default_substate_dir))?;
    SUBSTRATE_DIR.set(args.substrate_dir.unwrap_or(default_substrate_dir))?;

    // process output channel
    let (sender, mut receiver) = tokio::sync::mpsc::unbounded_channel::<(usize, usize, Reason)>();

    // query result map channel
    let (map_sender, map_receiver) =
        tokio::sync::oneshot::channel::<HashMap<(usize, usize), Reason>>();

    let tasks = collect_task_list(&args.in_path)?;

    let block_tx_queries: Vec<(usize, usize)> = tasks
        .iter()
        .map(|(_, v)| v)
        .flatten()
        .map(|x| (x.block, x.tx))
        .collect();

    // wouldn't use all cores on server
    let runtime = tokio::runtime::Builder::new_multi_thread()
        .worker_threads(3)
        .enable_all()
        .build()?;

    // I don't want async context except for process launching
    runtime.block_on(async {
        let semaphore = Arc::new(tokio::sync::Semaphore::new(max_children));
        let mut futures = Vec::new();

        // collect output into map
        tokio::spawn(async move {
            let mut query_result = HashMap::new();
            while let Some((block, tx, reason)) = receiver.recv().await {
                println!("reason received {block}-{tx}");
                query_result.insert((block, tx), reason);
            }
            let _ = map_sender.send(query_result);
        });

        // launch process
        for (block, tx) in block_tx_queries {
            let semaphore = Arc::clone(&semaphore);
            let sender = sender.clone();
            let future = tokio::spawn(async move {
                let _permit = semaphore.acquire().await.expect("semaphore acquire");
                query_tx(block, tx, sender).await;
            });
            futures.push(future);
        }
        for future in futures {
            let _ = future.await;
        }

        // close channel
        drop(sender);
    });

    let query_result = map_receiver.blocking_recv().expect("one shot error");

    // write
    let out_file = OpenOptions::new()
        .write(true)
        .append(false)
        .create(true)
        .truncate(true)
        .open(&args.out_path)?;
    let mut writer = BufWriter::new(out_file);
    for (k, v) in tasks {
        writeln!(writer, "{k}")?;
        for e in v {
            writeln!(
                writer,
                "    {}, matchSA: {}, log: {}, block: {}, tx: {}",
                e.func, e.matched_sa, e.log, e.block, e.tx
            )?;
            match query_result.get(&(e.block, e.tx)) {
                Some(reason) => writeln!(writer, "    {reason}")?,
                None => writeln!(writer, "reason missing, query failed?")?,
            }
            writeln!(writer)?;
        }
        writeln!(writer)?;
    }
    writer.flush()?;

    Ok(())
}

async fn query_tx(block: usize, tx: usize, sender: UnboundedSender<(usize, usize, Reason)>) {
    let cmd = [
        "./substate-cli",
        "val-sbt-tx",
        "--block-segment",
        &block.to_string(),
        "-tx",
        &tx.to_string(),
        "--workers",
        "1",
        "--skip-transfer-txs",
        "--skip-create-txs",
        "--substatedir",
        SUBSTATE_DIR.get().unwrap(),
        "--substratedir",
        SUBSTRATE_DIR.get().unwrap(),
    ];
    println!("query {block}-{tx}...");
    let output = execute_cmd(&cmd).await;
    let mut reverted = false;
    let mut storage = None;
    let mut nonce = None;
    let mut balance = None;
    let mut code = None;
    let mut log = String::new();
    output.lines().for_each(|line| {
        if line.contains("execution reverted") {
            reverted = true;
        }
        if line.contains("STORAGE") && line.contains("NOT EQUAL") {
            let token_it = line.split(' ').map(str::to_string);
            let mut val_it = token_it
                .enumerate()
                // 1: STORAGE 3,4: NOT EQUAL
                .filter(|(idx, _)| ![1, 3, 4].contains(idx))
                .map(|(_, val)| val);
            storage = Some((
                val_it.next().expect("no account address"),
                val_it.next().expect("no storage key"),
                val_it.next().expect("no storage value ground truth"),
                val_it.next().expect("no storage value"),
            ));
        }

        if line.contains("NONCE NOT EQUAL") {
            let token_it = line.split(' ').map(str::to_string);
            let mut val_it = token_it
                .enumerate()
                // 1,2,3: NONCE NOT EQUAL
                .filter(|(idx, _)| ![1, 2, 3].contains(idx))
                .map(|(_, val)| val);
            nonce = Some((
                val_it.next().expect("no nonce addr"),
                val_it.next().expect("no truth"),
                val_it.next().expect("no nonce value"),
            ));
        }

        if line.contains("BALANCE NOT EQUAL") {
            let token_it = line.split(' ').map(str::to_string);
            let mut val_it = token_it
                .enumerate()
                // 1,2,3: BALANCE NOT EQUAL
                .filter(|(idx, _)| ![1, 2, 3].contains(idx))
                .map(|(_, val)| val);
            balance = Some((
                val_it.next().expect("no balance addr"),
                val_it.next().expect("no truth"),
                val_it.next().expect("no balance value"),
            ));
        }

        if line.contains("CODE NOT EQUAL") {
            let token_it = line.split(' ').map(str::to_string);
            let mut val_it = token_it
                .enumerate()
                // 1,2,3,4: CODE NOT EQUAL MD5
                .filter(|(idx, _)| ![1, 2, 3, 4].contains(idx))
                .map(|(_, val)| val);
            code = Some((
                val_it.next().expect("no code addr"),
                val_it.next().expect("no truth"),
                val_it.next().expect("no md5 value"),
            ));
        }
        if line.contains("not eq")
            || line.contains("is nil")
            || line.contains("differ")
            || line.contains("substate:")
            || line.contains("replay  :")
        {
            log.push_str("    ");
            log.push_str(line);
            log.push_str("\n");
        }
    });

    let _ = sender.send((
        block,
        tx,
        Reason {
            reverted,
            storage,
            nonce,
            balance,
            code,
            log,
        },
    ));
}

async fn execute_cmd(args: &[&str]) -> String {
    let mut cmd = tokio::process::Command::new(args[0]);
    cmd.args(&args[1..]);

    let Ok(output) = cmd.output().await else {
        return "SPAWN CHILD FAIL".into();
    };

    // idk why, some msg in stderr some in stdout despite success
    let stdout = String::from_utf8_lossy(&output.stdout).to_string();
    let stderr = String::from_utf8_lossy(&output.stderr).to_string();
    return format!("{stdout}\n{stderr}");
}

fn collect_task_list(in_path: &str) -> Result<HashMap<String, Vec<CallStatus>>, Box<dyn Error>> {
    let mut reader = csv::ReaderBuilder::new()
        .has_headers(true)
        .delimiter(b',')
        .flexible(true)
        .from_path(in_path)?;

    // to fix trailing comma of header
    let headers = reader.headers()?;
    let headers = headers.iter().filter(|&x| !x.trim().is_empty()).collect();

    // exclude not useful executions
    reader.set_headers(headers);
    let tasks: HashMap<String, Vec<CallStatus>> = reader
        .deserialize::<Record>()
        .filter_map(|x| Some(x.unwrap()))
        .filter(not_ok)
        .filter(not_broken_func)
        .filter(no_err_msg)
        .map(slice_status)
        .fold(HashMap::new(), |mut acc, (key, val)| {
            let entry = acc.entry(key).or_default();
            if !entry.contains(&val) {
                entry.push(val);
            }
            acc
        });
    Ok(tasks)
}

fn not_ok(x: &Record) -> bool {
    !x.result
}

fn no_err_msg(x: &Record) -> bool {
    x.errmsg.is_none()
}

fn not_broken_func(x: &Record) -> bool {
    x.func != "0x"
}

fn slice_status(x: Record) -> (String, CallStatus) {
    let call_status = CallStatus {
        func: x.func,
        matched_sa: x.matched_sa,
        log: x.log,
        block: x.block,
        tx: x.tx,
    };
    (x.md5, call_status)
}

#[derive(Deserialize, Debug)]
#[allow(unused)]
struct Record {
    block: usize,
    tx: usize,
    md5: String,
    func: String,
    errmsg: Option<String>,
    #[serde(rename = "usedEVM")]
    used_evm: bool,
    #[serde(rename = "callInt")]
    call_int: usize,
    #[serde(rename = "callEVM")]
    call_evm: usize,
    #[serde(rename = "sstoreInt")]
    sstore_int: usize,
    #[serde(rename = "sstoreEVM")]
    sstore_evm: usize,
    gas: usize,
    codesize: usize,
    #[serde(deserialize_with = "deserialize_ok")]
    result: bool,
    #[serde(rename = "matchedSA")]
    matched_sa: bool,
    log: bool,
    addr: String,
}

#[derive(Debug, Clone)]
struct CallStatus {
    func: String,
    matched_sa: bool,
    log: bool,
    block: usize,
    tx: usize,
}

#[derive(Debug, Clone)]
struct Reason {
    storage: Option<(String, String, String, String)>,
    nonce: Option<(String, String, String)>,
    balance: Option<(String, String, String)>,
    code: Option<(String, String, String)>,
    log: String,
    reverted: bool,
}

impl Display for Reason {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        let indent = "\n        ";
        let reverted = if self.reverted { "reverted!" } else { "" };
        let storage = if let Some((addr, key, truth, val)) = &self.storage {
            format!("STORAGE ERR{indent}addr: {addr}{indent}storage key: {key}{indent}truth: {truth}{indent}val: {val}")
        } else {
            "".into()
        };
        let nonce = if let Some((addr, truth, val)) = &self.nonce {
            format!("NONCE ERR{indent}addr: {addr}{indent}true nonce: {truth}{indent}val: {val}")
        } else {
            "".into()
        };
        let balance = if let Some((addr, truth, val)) = &self.balance {
            format!(
                "BALANCE ERR{indent}addr: {addr}{indent}true balance: {truth}{indent}val: {val}"
            )
        } else {
            "".into()
        };
        let code = if let Some((addr, truth, val)) = &self.code {
            format!("CODE ERR{indent}addr: {addr}{indent}true md5: {truth}{indent}val: {val}")
        } else {
            "".into()
        };
        write!(
            f,
            "Reason: {reverted}{storage}{nonce}{balance}{code}\n{}",
            self.log
        )
    }
}

impl PartialEq for CallStatus {
    fn eq(&self, other: &Self) -> bool {
        self.func == other.func && self.matched_sa == other.matched_sa && self.log == other.log
    }
}

fn deserialize_ok<'de, D>(d: D) -> Result<bool, D::Error>
where
    D: Deserializer<'de>,
{
    let s = String::deserialize(d)?;
    match s.as_str() {
        "OK" => Ok(true),
        _ => Ok(false),
    }
}
