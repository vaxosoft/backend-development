//! Exercise 2: store an uploaded stream under a generated id, safely.
use std::io::{self, Read, Write};
use std::path::{Path, PathBuf};

use sha2::{Digest, Sha256};

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
pub fn sanitize_filename(client_name: &str) -> Option<String> {
    let last = client_name.rsplit(['/', '\\']).next().unwrap_or_default();
    let cleaned: String = last.chars().filter(|c| !c.is_control()).collect();
    let cleaned = cleaned.trim();
    match cleaned {
        "" | "." | ".." => None,
        name => Some(name.to_owned()),
    }
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
    root: &Path,
    id: &str,
    client_name: &str,
    body: impl Read,
    max_bytes: u64,
) -> Result<Stored, StoreError> {
    if id.is_empty()
        || !id
            .chars()
            .all(|c| c.is_ascii_alphanumeric() || c == '_' || c == '-')
    {
        return Err(StoreError::InvalidId);
    }

    // Reading one byte past the limit is enough to detect an oversized body.
    let mut body = body.take(max_bytes.saturating_add(1));
    let mut temp = tempfile::NamedTempFile::new_in(root)?;
    let mut hasher = Sha256::new();
    let mut buffer = [0u8; 8 * 1024];
    let mut size = 0u64;
    loop {
        let n = body.read(&mut buffer)?;
        if n == 0 {
            break;
        }
        size += n as u64;
        if size > max_bytes {
            return Err(StoreError::TooLarge { limit: max_bytes });
        }
        hasher.update(&buffer[..n]);
        temp.write_all(&buffer[..n])?;
    }
    temp.flush()?;

    let path = root.join(id);
    temp.persist(&path).map_err(|e| e.error)?;
    Ok(Stored {
        name: sanitize_filename(client_name).unwrap_or_else(|| "upload".to_owned()),
        path,
        size,
        sha256: hex::encode(hasher.finalize()),
    })
}
