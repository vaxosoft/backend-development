//! Stream a large file through SHA-256 with a fixed buffer, synchronously and with tokio.
use std::fs::File;
use std::io::{BufWriter, Read, Write};
use std::path::Path;

use sha2::{Digest, Sha256};
use tokio::io::AsyncReadExt;

const SIZE: usize = 8 * 1024 * 1024;

fn main() -> anyhow::Result<()> {
    let dir = tempfile::tempdir()?;
    let path = dir.path().join("big.bin");

    // The writer hashes as it writes: the data never exists in memory as one 8 MiB value.
    let written_hash = write_pattern(&path)?;
    println!(
        "wrote {} bytes, sha256 {written_hash}",
        std::fs::metadata(&path)?.len()
    );

    let sync_hash = hash_sync(&path)?;
    println!("sync read:  {sync_hash}");

    let runtime = tokio::runtime::Builder::new_current_thread().build()?;
    let async_hash = runtime.block_on(hash_async(&path))?;
    println!("async read: {async_hash}");

    println!(
        "all equal: {}",
        written_hash == sync_hash && sync_hash == async_hash
    );
    Ok(())
}

fn write_pattern(path: &Path) -> anyhow::Result<String> {
    let mut out = BufWriter::new(File::create(path)?);
    let mut hasher = Sha256::new();
    let chunk: Vec<u8> = (0..64 * 1024).map(|i| (i % 251) as u8).collect();
    for _ in 0..SIZE / chunk.len() {
        out.write_all(&chunk)?;
        hasher.update(&chunk);
    }
    out.flush()?;
    Ok(hex::encode(hasher.finalize()))
}

fn hash_sync(path: &Path) -> anyhow::Result<String> {
    let mut file = File::open(path)?;
    let mut hasher = Sha256::new();
    let mut buffer = [0u8; 8 * 1024];
    loop {
        // `read` may return fewer bytes than requested; 0 means end of file.
        let n = file.read(&mut buffer)?;
        if n == 0 {
            break;
        }
        hasher.update(&buffer[..n]);
    }
    Ok(hex::encode(hasher.finalize()))
}

async fn hash_async(path: &Path) -> anyhow::Result<String> {
    let mut file = tokio::fs::File::open(path).await?;
    let mut hasher = Sha256::new();
    let mut buffer = vec![0u8; 8 * 1024];
    loop {
        let n = file.read(&mut buffer).await?;
        if n == 0 {
            break;
        }
        hasher.update(&buffer[..n]);
    }
    Ok(hex::encode(hasher.finalize()))
}
