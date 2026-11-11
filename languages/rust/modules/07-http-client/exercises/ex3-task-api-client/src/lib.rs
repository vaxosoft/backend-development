//! Exercise 3: a typed client for the Task Tracker API.
use std::time::Duration;

use reqwest::{Client, Url};
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum Status {
    Todo,
    InProgress,
    Done,
}

#[derive(Debug, Clone, PartialEq, Eq, Deserialize)]
pub struct Task {
    pub id: u64,
    pub title: String,
    pub status: Status,
    pub tags: Vec<String>,
}

#[derive(Debug, thiserror::Error)]
pub enum ApiError {
    #[error("the request timed out")]
    Timeout,
    #[error("transport error: {0}")]
    Transport(String),
    /// A non-2xx answer. `code` and `message` come from the JSON error envelope
    /// `{"error":{"code":"...","message":"..."}}`; use `"unknown"` / `""` if the body is not one.
    #[error("api error {status} {code}: {message}")]
    Api {
        status: u16,
        code: String,
        message: String,
    },
    #[error("invalid response body: {0}")]
    Decode(String),
}

// The private helpers below are for your implementation; they are unused in the starter.
#[allow(dead_code)]
#[derive(Serialize)]
struct NewTask<'a> {
    title: &'a str,
    tags: &'a [String],
}

#[allow(dead_code)]
#[derive(Deserialize)]
struct Page {
    items: Vec<Task>,
    total: usize,
}

#[allow(dead_code)]
pub struct TaskApiClient {
    http: Client,
    base: Url,
    token: String,
    page_size: usize,
}

impl TaskApiClient {
    /// `base_url` is the server root (for example `http://127.0.0.1:8080`).
    ///
    /// Build ONE `reqwest::Client` here with the given overall `timeout` and the
    /// user agent `task-tracker-client/1.0`; reuse it for every call.
    pub fn new(_base_url: &str, _token: &str, _timeout: Duration) -> Result<Self, ApiError> {
        todo!("build the client")
    }

    /// Number of items requested per page by [`TaskApiClient::list_all`] (default 20).
    pub fn with_page_size(mut self, page_size: usize) -> Self {
        self.page_size = page_size;
        self
    }

    /// `POST /tasks` with the JSON body `{"title": ..., "tags": [...]}` and
    /// `Authorization: Bearer <token>`. Expects `201` and the created task.
    pub async fn create_task(&self, _title: &str, _tags: &[String]) -> Result<Task, ApiError> {
        todo!("POST the task and decode the answer")
    }

    /// Fetch every task by following `GET /tasks?limit=<page_size>&offset=<n>` (bearer token on
    /// each call) until `offset >= total`. Items keep server order.
    pub async fn list_all(&self) -> Result<Vec<Task>, ApiError> {
        todo!("page through the collection")
    }
}
