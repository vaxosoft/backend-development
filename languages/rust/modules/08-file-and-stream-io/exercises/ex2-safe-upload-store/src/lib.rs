//! Exercise 2: store an uploaded stream under a generated id, safely.
use std::io::{self, Read};
use std::path::{Path, PathBuf};

#[derive(Debug, thiserror::Error)]
pub enum StoreError {
    /// The id is empty or contains anything other than `[A-Za-z0-9_-]`.
    #[error("invalid id")]
    InvalidId,
    /// The body is longer than the allowed number of bytes.
    #[error("upload exceeds {limit} bytes")]
    TooLarge { limit: u64 },
    #[error(transparent)]
    Io(#[from] io::Error),
}

#[derive(Debug, PartialEq, Eq)]
pub struct Stored {
    /// The sanitized client file name, for display only. It is never used as a path.
    pub name: String,
    /// `root/<id>`
    pub path: PathBuf,
    pub size: u64,
    /// Lowercase hex SHA-256 of the content.
    pub sha256: String,
}

/// Turn a client-supplied file name into something safe to show and put in a header.
///
/// - keep only the part after the last `/` or `\`
/// - drop control characters (including NUL), trim surrounding whitespace
/// - return `None` if nothing usable is left, or the result is `.` or `..`
///
/// `"../../etc/passwd"` -> `Some("passwd")`, `"C:\\docs\\a.txt"` -> `Some("a.txt")`.
pub fn sanitize_filename(_client_name: &str) -> Option<String> {
    todo!("keep the last path component, strip control characters")
}

/// Stream `body` into `root/<id>`, never holding the whole body in memory.
///
/// - validate `id` first ([`StoreError::InvalidId`]); the client name is only sanitized for the
///   returned [`Stored::name`] (fall back to `"upload"` when [`sanitize_filename`] gives `None`)
/// - read at most `max_bytes`: a body of exactly `max_bytes` is fine, one byte more is
///   [`StoreError::TooLarge`]
/// - hash while copying
/// - on ANY failure leave nothing behind in `root` (write to a temp file in `root` and rename it
///   to the final name only once everything succeeded)
pub fn store_upload(
    _root: &Path,
    _id: &str,
    _client_name: &str,
    _body: impl Read,
    _max_bytes: u64,
) -> Result<Stored, StoreError> {
    todo!("validate, stream to a temp file, hash, enforce the limit, persist")
}
