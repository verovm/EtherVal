use rayon::prelude::*;
use std::{
    collections::VecDeque,
    error::Error,
    fs::{File, OpenOptions},
    io,
    path::Path,
};

pub type AnyResult<T> = Result<T, Box<dyn Error>>;

pub fn open_n_files<P, R, W>(read: R, overwrite: W) -> io::Result<(Vec<File>, Vec<File>)>
where
    P: AsRef<Path>,
    R: IntoIterator<Item = P>,
    W: IntoIterator<Item = P>,
{
    let read_files: io::Result<Vec<File>> = read
        .into_iter()
        .map(|path| {
            OpenOptions::new()
                .read(true)
                .write(false)
                .create(false)
                .open(path)
        })
        .collect();
    let write_files: io::Result<Vec<File>> = overwrite
        .into_iter()
        .map(|path| {
            OpenOptions::new()
                .write(true)
                .create(true)
                .truncate(true)
                .open(path)
        })
        .collect();
    Ok((read_files?, write_files?))
}

pub fn par_tokenize_filter<S, I>(
    lines: I,
    delimiter: S,
    min_count: usize,
) -> VecDeque<(Vec<String>, String)>
where
    S: AsRef<str> + Sync,
    I: IntoParallelIterator<Item: AsRef<str>>,
{
    lines
        .into_par_iter()
        .filter_map(|line| {
            let tokens: Vec<String> = line
                .as_ref()
                .split(delimiter.as_ref())
                .map(str::to_string)
                .collect();
            if tokens.len() < min_count {
                None
            } else {
                Some((tokens, line.as_ref().to_string()))
            }
        })
        .collect()
}
