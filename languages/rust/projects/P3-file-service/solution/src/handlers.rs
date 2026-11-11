//! Route handlers: upload (streamed to disk), download (streamed back), delete.

use std::io;
use std::path::Path as FsPath;

use axum::Json;
use axum::extract::multipart::{Field, MultipartRejection};
use axum::extract::{Multipart, Path, State};
use axum::http::{HeaderMap, HeaderName, HeaderValue, StatusCode, header};
use axum::response::{IntoResponse, Response};
use sha2::{Digest, Sha256};
use tokio::fs::{self, File};
use tokio::io::AsyncWriteExt;

use crate::AppState;
use crate::error::ApiError;
use crate::rules::{content_disposition, etag_matches, is_valid_id, sanitize_filename};
use crate::store::{Store, StoredFile, stream_file};

const DEFAULT_CONTENT_TYPE: &str = "application/octet-stream";

pub async fn upload(
    State(state): State<AppState>,
    multipart: Result<Multipart, MultipartRejection>,
) -> Result<Response, ApiError> {
    let mut multipart = multipart
        .map_err(|_| ApiError::BadRequest("expected a multipart/form-data body".to_owned()))?;

    while let Some(mut field) = multipart.next_field().await? {
        if field.name() != Some("file") {
            continue;
        }
        let id = Store::new_id();
        let name = sanitize_filename(field.file_name().unwrap_or_default());
        let content_type = field
            .content_type()
            .filter(|ct| HeaderValue::from_str(ct).is_ok())
            .unwrap_or(DEFAULT_CONTENT_TYPE)
            .to_owned();

        let part = state.store.part_path(&id);
        let stored = receive(&mut field, &part, state.max_size).await;
        let meta = match stored {
            Ok((size, sha256)) => StoredFile {
                id,
                name,
                size,
                sha256,
                content_type,
            },
            Err(err) => {
                let _ = fs::remove_file(&part).await;
                return Err(err);
            }
        };
        if let Err(err) = commit(&state.store, &part, &meta).await {
            let _ = fs::remove_file(&part).await;
            return Err(err.into());
        }
        let location = [(header::LOCATION, format!("/files/{}", meta.id))];
        return Ok((StatusCode::CREATED, location, Json(meta)).into_response());
    }
    Err(ApiError::BadRequest(
        "multipart field `file` is required".to_owned(),
    ))
}

/// Streams the field to `path`, hashing as it goes. Only one chunk is in memory
/// at a time, and the size limit is enforced while streaming.
async fn receive(
    field: &mut Field<'_>,
    path: &FsPath,
    max_size: u64,
) -> Result<(u64, String), ApiError> {
    let mut file = File::create(path).await?;
    let mut hasher = Sha256::new();
    let mut size = 0u64;
    while let Some(chunk) = field.chunk().await? {
        size += chunk.len() as u64;
        if size > max_size {
            return Err(ApiError::PayloadTooLarge);
        }
        hasher.update(&chunk);
        file.write_all(&chunk).await?;
    }
    file.flush().await?;
    Ok((size, hex::encode(hasher.finalize())))
}

async fn commit(store: &Store, part: &FsPath, meta: &StoredFile) -> io::Result<()> {
    fs::rename(part, store.data_path(&meta.id)).await?;
    store.write_meta(meta).await
}

pub async fn download(
    State(state): State<AppState>,
    Path(id): Path<String>,
    request_headers: HeaderMap,
) -> Result<Response, ApiError> {
    if !is_valid_id(&id) {
        return Err(ApiError::NotFound);
    }
    let meta = state
        .store
        .read_meta(&id)
        .await?
        .ok_or(ApiError::NotFound)?;
    let etag = format!("\"{}\"", meta.sha256);

    let cached = request_headers
        .get(header::IF_NONE_MATCH)
        .and_then(|value| value.to_str().ok())
        .is_some_and(|value| etag_matches(value, &meta.sha256));
    if cached {
        return Ok((StatusCode::NOT_MODIFIED, [(header::ETAG, etag)]).into_response());
    }

    let file = match File::open(state.store.data_path(&id)).await {
        Ok(file) => file,
        Err(err) if err.kind() == io::ErrorKind::NotFound => return Err(ApiError::NotFound),
        Err(err) => return Err(err.into()),
    };
    let headers: [(HeaderName, String); 4] = [
        (header::CONTENT_TYPE, meta.content_type),
        (header::CONTENT_LENGTH, meta.size.to_string()),
        (header::CONTENT_DISPOSITION, content_disposition(&meta.name)),
        (header::ETAG, etag),
    ];
    Ok((headers, stream_file(file)).into_response())
}

pub async fn delete(
    State(state): State<AppState>,
    Path(id): Path<String>,
) -> Result<StatusCode, ApiError> {
    if is_valid_id(&id) && state.store.remove(&id).await? {
        Ok(StatusCode::NO_CONTENT)
    } else {
        Err(ApiError::NotFound)
    }
}
