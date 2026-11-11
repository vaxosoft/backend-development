//! Exercise 3: streaming gzip with checksums and atomic output files.
use std::fs::File;
use std::io::{self, Read, Write};
use std::path::Path;

use flate2::{Compression, read::GzDecoder, write::GzEncoder};
use sha2::{Digest, Sha256};

#[derive(Debug, thiserror::Error)]
pub enum PackError {
    #[error(transparent)]
    Io(#[from] io::Error),
    #[error("checksum mismatch: expected {expected}, got {actual}")]
    ChecksumMismatch { expected: String, actual: String },
}

#[derive(Debug, PartialEq, Eq)]
pub struct PackInfo {
    /// Uncompressed length in bytes.
    pub raw_len: u64,
    /// Lowercase hex SHA-256 of the UNCOMPRESSED data.
    pub sha256: String,
    /// Size of the file on disk.
    pub packed_len: u64,
}

/// Gzip everything from `src` into the file `dest`.
///
/// - stream: never hold the whole input (or output) in memory
/// - compute the SHA-256 of the raw bytes while reading
/// - the file must appear atomically: write to a temp file in `dest`'s directory and rename it
///   to `dest` only after the gzip trailer has been written and flushed. If anything fails,
///   `dest` keeps its previous content (or stays absent) and no temp file is left behind.
pub fn pack(src: impl Read, dest: &Path) -> Result<PackInfo, PackError> {
    let mut temp = tempfile::NamedTempFile::new_in(dir_of(dest))?;
    let mut encoder = GzEncoder::new(&mut temp, Compression::default());
    let (raw_len, sha256) = copy_hashing(src, &mut encoder)?;
    encoder.finish()?;
    temp.flush()?;
    let packed_len = temp.as_file().metadata()?.len();
    temp.persist(dest).map_err(|e| e.error)?;
    Ok(PackInfo {
        raw_len,
        sha256,
        packed_len,
    })
}

/// Decompress the gzip file `packed` into the file `dest`, but only if the SHA-256 of the
/// decompressed data equals `expected_sha256`.
///
/// - stream the data; on a checksum mismatch return [`PackError::ChecksumMismatch`] and make sure
///   `dest` was NOT created or modified (again: temp file + rename) and nothing is left behind
/// - corrupt or truncated gzip input is an [`PackError::Io`]
/// - returns the number of decompressed bytes
pub fn unpack_verified(
    packed: &Path,
    dest: &Path,
    expected_sha256: &str,
) -> Result<u64, PackError> {
    let mut temp = tempfile::NamedTempFile::new_in(dir_of(dest))?;
    let (len, actual) = copy_hashing(GzDecoder::new(File::open(packed)?), &mut temp)?;
    if !actual.eq_ignore_ascii_case(expected_sha256) {
        return Err(PackError::ChecksumMismatch {
            expected: expected_sha256.to_owned(),
            actual,
        });
    }
    temp.flush()?;
    temp.persist(dest).map_err(|e| e.error)?;
    Ok(len)
}

fn dir_of(path: &Path) -> &Path {
    path.parent()
        .filter(|p| !p.as_os_str().is_empty())
        .unwrap_or(Path::new("."))
}

fn copy_hashing(mut reader: impl Read, mut writer: impl Write) -> io::Result<(u64, String)> {
    let mut hasher = Sha256::new();
    let mut buffer = [0u8; 16 * 1024];
    let mut total = 0u64;
    loop {
        let n = reader.read(&mut buffer)?;
        if n == 0 {
            return Ok((total, hex::encode(hasher.finalize())));
        }
        hasher.update(&buffer[..n]);
        writer.write_all(&buffer[..n])?;
        total += n as u64;
    }
}
