//! On-disk layout: `<dir>/<id>.part` while uploading, then `<id>.bin` (content)
//! and `<id>.json` (metadata). Provided; nothing to implement here.

use std::io;
use std::path::PathBuf;

use axum::body::Body;
use futures::stream;
use serde::{Deserialize, Serialize};
use tokio::fs::{self, File};
use tokio::io::AsyncReadExt;
use uuid::Uuid;

/// Metadata of a stored file; also the JSON returned by `POST /files`.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct StoredFile {
    pub id: String,
    pub name: String,
    pub size: u64,
    pub sha256: String,
    pub content_type: String,
}

/// Every method taking an `id` requires `rules::is_valid_id(id)`: the id
/// becomes part of a path.
#[derive(Debug, Clone)]
pub struct Store {
    dir: PathBuf,
}

impl Store {
    pub fn open(dir: impl Into<PathBuf>) -> io::Result<Self> {
        let dir = dir.into();
        std::fs::create_dir_all(&dir)?;
        Ok(Self { dir })
    }

    /// 32 lowercase hex characters.
    pub fn new_id() -> String {
        Uuid::new_v4().simple().to_string()
    }

    pub fn part_path(&self, id: &str) -> PathBuf {
        self.dir.join(format!("{id}.part"))
    }

    pub fn data_path(&self, id: &str) -> PathBuf {
        self.dir.join(format!("{id}.bin"))
    }

    fn meta_path(&self, id: &str) -> PathBuf {
        self.dir.join(format!("{id}.json"))
    }

    pub async fn write_meta(&self, meta: &StoredFile) -> io::Result<()> {
        let json = serde_json::to_vec(meta).map_err(io::Error::other)?;
        fs::write(self.meta_path(&meta.id), json).await
    }

    /// `Ok(None)` when there is no such file.
    pub async fn read_meta(&self, id: &str) -> io::Result<Option<StoredFile>> {
        match fs::read(self.meta_path(id)).await {
            Ok(bytes) => Ok(Some(
                serde_json::from_slice(&bytes).map_err(io::Error::other)?,
            )),
            Err(err) if err.kind() == io::ErrorKind::NotFound => Ok(None),
            Err(err) => Err(err),
        }
    }

    /// Deletes content and metadata; `false` when there was nothing to delete.
    pub async fn remove(&self, id: &str) -> io::Result<bool> {
        let meta = remove_if_exists(self.meta_path(id)).await?;
        let data = remove_if_exists(self.data_path(id)).await?;
        Ok(meta || data)
    }
}

async fn remove_if_exists(path: PathBuf) -> io::Result<bool> {
    match fs::remove_file(path).await {
        Ok(()) => Ok(true),
        Err(err) if err.kind() == io::ErrorKind::NotFound => Ok(false),
        Err(err) => Err(err),
    }
}

/// A response body that reads `file` in 64 KiB chunks instead of loading it.
pub fn stream_file(file: File) -> Body {
    const CHUNK: usize = 64 * 1024;
    let chunks = stream::try_unfold(file, |mut file| async move {
        let mut buf = vec![0u8; CHUNK];
        let n = file.read(&mut buf).await?;
        if n == 0 {
            return Ok::<_, io::Error>(None);
        }
        buf.truncate(n);
        Ok(Some((buf, file)))
    });
    Body::from_stream(chunks)
}
