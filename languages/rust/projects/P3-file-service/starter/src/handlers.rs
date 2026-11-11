//! Route handlers. All TODO stubs answer 501.

use axum::http::StatusCode;

// TODO: `State<AppState>` + `Result<Multipart, MultipartRejection>`.
// Find the field named `file`, stream it to `<id>.part` while counting bytes and
// hashing, enforce `max_size` WHILE streaming, then move it to its final name,
// write the metadata and answer 201 + `Location: /files/{id}` + `StoredFile`.
pub async fn upload() -> StatusCode {
    StatusCode::NOT_IMPLEMENTED
}

// TODO: validate the id, read the metadata, answer 304 on a matching
// `If-None-Match`, else 200 with Content-Length / Content-Type /
// Content-Disposition / ETag and `stream_file(..)` as the body.
pub async fn download() -> StatusCode {
    StatusCode::NOT_IMPLEMENTED
}

// TODO: validate the id; 204 when removed, 404 when unknown.
pub async fn delete() -> StatusCode {
    StatusCode::NOT_IMPLEMENTED
}
