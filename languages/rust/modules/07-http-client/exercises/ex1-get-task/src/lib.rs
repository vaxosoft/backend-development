//! Exercise 1: fetch one task and turn every outcome into a typed result.
use reqwest::Client;
use serde::Deserialize;

#[derive(Debug, Clone, PartialEq, Deserialize)]
pub struct Task {
    pub id: u64,
    pub title: String,
    pub status: String,
}

#[derive(Debug, thiserror::Error)]
pub enum FetchError {
    #[error("task not found")]
    NotFound,
    #[error("unexpected status {0}")]
    Status(u16),
    #[error("transport error: {0}")]
    Transport(reqwest::Error),
    #[error("invalid response body: {0}")]
    Decode(reqwest::Error),
}

/// `GET {base}/tasks/{id}` with `Accept: application/json`.
///
/// - 2xx: decode the JSON body into a [`Task`]
/// - 404: [`FetchError::NotFound`]
/// - any other non-2xx status: [`FetchError::Status`]
/// - the request could not be sent / no response: [`FetchError::Transport`]
/// - 2xx but the body is not a valid task: [`FetchError::Decode`]
///
/// `base` may or may not end with `/`.
pub async fn get_task(_client: &Client, _base: &str, _id: u64) -> Result<Task, FetchError> {
    todo!("send the request and map each outcome to a FetchError")
}
