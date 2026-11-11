//! Paths, whole-file helpers, buffered line I/O and directory listing in a temp directory.
use std::fs::{self, File, OpenOptions};
use std::io::{BufRead, BufReader, BufWriter, Write};
use std::path::Path;

fn main() -> anyhow::Result<()> {
    // The directory and everything in it is deleted when `dir` is dropped.
    let dir = tempfile::tempdir()?;
    let root = dir.path();

    let notes = root.join("notes").join("today.txt");
    println!(
        "file_name={:?} extension={:?}",
        notes.file_name().unwrap(),
        notes.extension().unwrap()
    );
    println!("relative: {}", notes.strip_prefix(root)?.display());

    // Whole-file helpers are fine for small files.
    fs::create_dir_all(notes.parent().unwrap())?;
    fs::write(&notes, "write docs\nreview PR\n")?;
    println!("size: {} bytes", fs::metadata(&notes)?.len());
    print!("content: {}", fs::read_to_string(&notes)?);

    // Append instead of truncating.
    let mut file = OpenOptions::new().append(true).open(&notes)?;
    writeln!(file, "ship it")?;
    drop(file);

    // Buffered, line-at-a-time reading never holds the whole file.
    let reader = BufReader::new(File::open(&notes)?);
    for (number, line) in reader.lines().enumerate() {
        println!("{:>2}: {}", number + 1, line?);
    }

    write_report(&root.join("report.txt"))?;
    list_sorted(root)?;

    match fs::read_to_string(root.join("missing.txt")) {
        Ok(_) => println!("unexpected"),
        Err(error) => println!("missing.txt -> {:?}", error.kind()),
    }
    Ok(())
}

fn write_report(path: &Path) -> anyhow::Result<()> {
    let mut out = BufWriter::new(File::create(path)?);
    for n in 1..=3 {
        writeln!(out, "line {n}")?;
    }
    // Dropping a BufWriter flushes too, but silently ignores errors: flush explicitly.
    out.flush()?;
    Ok(())
}

fn list_sorted(dir: &Path) -> anyhow::Result<()> {
    // `read_dir` order is platform dependent: sort before printing.
    let mut entries: Vec<_> = fs::read_dir(dir)?.collect::<Result<_, _>>()?;
    entries.sort_by_key(|entry| entry.file_name());
    for entry in entries {
        let kind = if entry.file_type()?.is_dir() {
            "dir "
        } else {
            "file"
        };
        println!("{kind} {}", entry.file_name().to_string_lossy());
    }
    Ok(())
}
