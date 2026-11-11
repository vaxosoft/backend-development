//! Task Tracker export: embedded seed data -> gzipped JSON Lines, written atomically,
//! then streamed back one record at a time.
use std::collections::BTreeMap;
use std::fs::File;
use std::io::{BufRead, BufReader, BufWriter, Write};
use std::path::Path;

use flate2::{Compression, read::GzDecoder, write::GzEncoder};
use serde::{Deserialize, Serialize};

// Compiled into the binary: no runtime path to get wrong.
const SEED: &str = include_str!("assets/seed_tasks.jsonl");

#[derive(Debug, Serialize, Deserialize)]
struct Task {
    id: u64,
    title: String,
    status: String,
    due_date: Option<String>,
    tags: Vec<String>,
    owner_id: Option<u64>,
    created_at: String,
}

fn main() -> anyhow::Result<()> {
    let tasks: Vec<Task> = SEED
        .lines()
        .map(serde_json::from_str)
        .collect::<Result<_, _>>()?;
    println!("embedded seed: {} tasks, {} bytes", tasks.len(), SEED.len());

    let dir = tempfile::tempdir()?;
    let target = dir.path().join("tasks.jsonl.gz");
    export(&tasks, &target)?;
    println!(
        "exported to {}",
        target.file_name().unwrap().to_string_lossy()
    );

    let mut by_status: BTreeMap<String, usize> = BTreeMap::new();
    for task in read_tasks(&target)? {
        *by_status.entry(task?.status).or_default() += 1;
    }
    println!("by status: {by_status:?}");

    // Only the final file exists: the temp file was renamed over the target.
    let names: Vec<_> = std::fs::read_dir(dir.path())?
        .map(|e| e.map(|e| e.file_name()))
        .collect::<Result<_, _>>()?;
    println!("directory: {names:?}");
    Ok(())
}

/// Write to a temp file in the SAME directory, then rename: readers never see a half-written file.
fn export(tasks: &[Task], target: &Path) -> anyhow::Result<()> {
    let dir = target.parent().expect("target has a parent");
    let temp = tempfile::NamedTempFile::new_in(dir)?;
    let mut gzip = GzEncoder::new(BufWriter::new(temp.reopen()?), Compression::default());
    for task in tasks {
        serde_json::to_writer(&mut gzip, task)?;
        gzip.write_all(b"\n")?;
    }
    // Unwrap the layers in order so every flush error is reported, not swallowed by `Drop`.
    gzip.finish()?
        .into_inner()
        .map_err(|e| e.into_error())?
        .sync_all()?;
    temp.persist(target)?;
    Ok(())
}

fn read_tasks(path: &Path) -> anyhow::Result<impl Iterator<Item = anyhow::Result<Task>>> {
    let reader = BufReader::new(GzDecoder::new(File::open(path)?));
    Ok(reader.lines().map(|line| Ok(serde_json::from_str(&line?)?)))
}
