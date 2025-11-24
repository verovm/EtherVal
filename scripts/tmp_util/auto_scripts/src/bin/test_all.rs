use std::{
    fs::{canonicalize, File},
    io::Write,
    process::Command,
};

use auto_scripts::AnyResult;
use clap::Parser;

#[derive(Parser, Debug)]
#[command(about = "optionally rebuild, execute both versions and generate diff")]
struct Args {
    #[arg(short = 'b', long = "rebuild")]
    rebuild_new: bool,
    #[arg(short = 'B', long = "REBUILD")]
    rebuild_both: bool,
    #[arg(short = 'o', long = "out")]
    output_path: String,
    #[arg(short = 'N', long = "new")]
    new_proj_path: String,
    #[arg(short = 'O', long = "old")]
    old_proj_path: String,
    #[arg(short = 'i', long = "init")]
    init_block: usize,
    #[arg(short = 'f', long = "fin")]
    fin_block: usize,
    #[arg(short = 'w', long = "worker", value_parser = clap::value_parser!(u64).range(1..), default_value_t = 1)]
    worker: u64,
    #[arg(short = 'p', long = "old_substate_dir")]
    old_substate_dir: Option<String>,
    #[arg(short = 'd', long = "new_substate_dir")]
    new_substate_dir: Option<String>,
    #[arg(short = 'D', long = "substrate_dir")]
    substrate_dir: Option<String>,
    #[arg(short = 's', long = "sort")]
    sort: bool,
    #[arg(short = 'u', long = "unique_func_select")]
    unique_func_select: bool,
    #[arg(short = 'e', long = "exclude_false_positive")]
    exclude_false_positive: bool,
    #[arg(short = 'k', long = "ok_only")]
    ok_only: bool,
}

const OLD_OUT_PATH: &str = "__tmp_old_out.txt";
const OLD_SORT_PATH: &str = "__tmp_old_sort.txt";
const NEW_OUT_PATH: &str = "__tmp_new_out.txt";
const NEW_SORT_PATH: &str = "__tmp_new_sort.txt";

fn main() -> AnyResult<()> {
    let args = Args::parse();
    let new_proj_path = canonicalize(args.new_proj_path)?;
    let old_proj_path = canonicalize(args.old_proj_path)?;

    let old_substate_dir = match args.old_substate_dir {
        Some(ref path) => path,
        None => "/zpool0/call-substate.ethereum.0-10M.putcode.record-trace",
    };

    let new_substate_dir = match args.new_substate_dir {
        Some(ref path) => path,
        None => {
            "/zpool2/backup-zpool1/substrate-interpreter/rr0.4.substate.ethereum.0-10M.record-trace"
        }
    };

    let substrate_dir = match args.substrate_dir {
        Some(ref path) => path,
        None => "/zpool2/dedaub/2024-11-21-data/substrates",
    };

    if args.rebuild_both {
        println!("rebuilding new version...");
        let p = Command::new("make")
            .arg("all")
            .current_dir(&new_proj_path)
            .spawn()?
            .wait()?;
        if !p.success() {
            panic!("new version make all failed");
        }
        println!("rebuilding old version...");
        let p = Command::new("make")
            .arg("all")
            .current_dir(&old_proj_path)
            .spawn()?
            .wait()?;
        if !p.success() {
            panic!("old version make all failed");
        }
    } else if args.rebuild_new {
        println!("rebuilding new version...");
        let p = Command::new("make")
            .arg("all")
            .current_dir(&new_proj_path)
            .spawn()?
            .wait()?;
        if !p.success() {
            panic!("new version make all failed");
        }
    }

    println!("running old version...");
    let old_bin = old_proj_path.join("build").join("bin").join("evm");
    let old_args = [
        "t8n-substate",
        &args.init_block.to_string(),
        &args.fin_block.to_string(),
        "--workers",
        &args.worker.to_string(),
        "--skip-transfer-txs",
        "--skip-create-txs",
        "--substatedir",
        old_substate_dir,
        "--substratedir",
        substrate_dir,
    ];
    println!("{:?}", old_args);
    let old_out = Command::new(&old_bin).args(old_args).output()?;
    let stdout = String::from_utf8_lossy(&old_out.stdout).to_string();
    let mut old_out_file = File::create(OLD_OUT_PATH)?;
    old_out_file.write_all(stdout.as_bytes())?;

    println!("running new version...");
    let new_bin = new_proj_path.join("build").join("bin").join("substate-cli");
    let new_args = [
        "replay",
        "--block-segment",
        &format!("{}-{}", args.init_block, args.fin_block),
        "--workers",
        &args.worker.to_string(),
        "--skip-transfer-txs",
        "--skip-create-txs",
        "--substatedir",
        new_substate_dir,
        "--substratedir",
        substrate_dir,
    ];
    println!("{:?}", new_args);
    let new_out = Command::new(&new_bin).args(new_args).output()?;
    let stdout = String::from_utf8_lossy(&new_out.stdout).to_string();
    let mut new_out_file = File::create(NEW_OUT_PATH)?;
    new_out_file.write_all(stdout.as_bytes())?;

    if args.sort {
        println!("sorting new version...");
        let p = Command::new("sort_line")
            .args(["-i", NEW_OUT_PATH, "-o", NEW_SORT_PATH])
            .spawn()?
            .wait()?;
        if !p.success() {
            panic!("new version sort failed");
        }
        println!("sorting old version...");
        let p = Command::new("sort_line")
            .args(["-i", OLD_OUT_PATH, "-o", OLD_SORT_PATH])
            .spawn()?
            .wait()?;
        if !p.success() {
            panic!("old version sort failed");
        }
    }
    println!(
        "comparing {}...",
        if args.sort { "sorted" } else { "unsorted" }
    );
    let mut cmp_args = vec![
        "-N",
        if args.sort {
            NEW_SORT_PATH
        } else {
            NEW_OUT_PATH
        },
        "-O",
        if args.sort {
            OLD_SORT_PATH
        } else {
            OLD_OUT_PATH
        },
        "-o",
        &args.output_path,
    ];
    if args.sort {
        cmp_args.push("-s");
    }
    if args.unique_func_select {
        cmp_args.push("-u");
        println!("unique func select enabled");
    }
    if args.exclude_false_positive {
        cmp_args.push("-e");
        println!("exclude all cases where new has OK");
    }
    if args.ok_only {
        cmp_args.push("-k");
        println!("ok only mode")
    }
    let p = Command::new("compare_migration")
        .args(&cmp_args)
        .spawn()?
        .wait()?;
    if !p.success() {
        panic!("compare failed");
    }
    println!("jobs complete");
    Ok(())
}
