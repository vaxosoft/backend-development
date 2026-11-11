//! Exercise 3: streaming gzip with checksums and atomic output files.
use std::io::{self, Read};
use std::path::Path;

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
pub fn pack(_src: impl Read, _dest: &Path) -> Result<PackInfo, PackError> {
    todo!("hash while reading, gzip into a temp file, persist")
}

/// Decompress the gzip file `packed` into the file `dest`, but only if the SHA-256 of the
/// decompressed data equals `expected_sha256`.
///
/// - stream the data; on a checksum mismatch return [`PackError::ChecksumMismatch`] and make sure
///   `dest` was NOT created or modified (again: temp file + rename) and nothing is left behind
/// - corrupt or truncated gzip input is an [`PackError::Io`]
/// - returns the number of decompressed bytes
pub fn unpack_verified(
    _packed: &Path,
    _dest: &Path,
    _expected_sha256: &str,
) -> Result<u64, PackError> {
    todo!("decompress into a temp file while hashing, verify, persist")
}
