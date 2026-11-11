//! Exercise 3: a typed client for the Task Tracker API.
use std::time::Duration;

use reqwest::{Client, Url, header};
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

impl From<reqwest::Error> for ApiError {
    fn from(error: reqwest::Error) -> Self {
        if error.is_timeout() {
            ApiError::Timeout
        } else if error.is_decode() {
            ApiError::Decode(error.to_string())
        } else {
            ApiError::Transport(error.to_string())
        }
    }
}

#[derive(Serialize)]
struct NewTask<'a> {
    title: &'a str,
    tags: &'a [String],
}

#[derive(Deserialize)]
struct Page {
    items: Vec<Task>,
    total: usize,
}

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
    pub fn new(base_url: &str, token: &str, timeout: Duration) -> Result<Self, ApiError> {
        let base = Url::parse(base_url).map_err(|e| ApiError::Transport(e.to_string()))?;
        let http = Client::builder()
            .timeout(timeout)
            .user_agent("task-tracker-client/1.0")
            .build()?;
        Ok(Self {
            http,
            base,
            token: token.to_owned(),
            page_size: 20,
        })
    }

    /// Number of items requested per page by [`TaskApiClient::list_all`] (default 20).
    pub fn with_page_size(mut self, page_size: usize) -> Self {
        self.page_size = page_size;
        self
    }

    /// `POST /tasks` with the JSON body `{"title": ..., "tags": [...]}` and
    /// `Authorization: Bearer <token>`. Expects `201` and the created task.
    pub async fn create_task(&self, title: &str, tags: &[String]) -> Result<Task, ApiError> {
        let url = self
            .base
            .join("/tasks")
            .map_err(|e| ApiError::Transport(e.to_string()))?;
        let request = self
            .http
            .post(url)
            .bearer_auth(&self.token)
            .header(header::ACCEPT, "application/json");
        Self::decode(request.json(&NewTask { title, tags }).send().await?).await
    }

    /// Fetch every task by following `GET /tasks?limit=<page_size>&offset=<n>` (bearer token on
    /// each call) until `offset >= total`. Items keep server order.
    pub async fn list_all(&self) -> Result<Vec<Task>, ApiError> {
        let mut tasks = Vec::new();
        let mut offset = 0;
        loop {
            let mut url = self
                .base
                .join("/tasks")
                .map_err(|e| ApiError::Transport(e.to_string()))?;
            url.query_pairs_mut()
                .append_pair("limit", &self.page_size.to_string())
                .append_pair("offset", &offset.to_string());
            let request = self
                .http
                .get(url)
                .bearer_auth(&self.token)
                .header(header::ACCEPT, "application/json");
            let page: Page = Self::decode(request.send().await?).await?;
            let fetched = page.items.len();
            tasks.extend(page.items);
            offset += fetched;
            // An empty page also ends the loop, so a misbehaving server cannot make us spin forever.
            if fetched == 0 || offset >= page.total {
                return Ok(tasks);
            }
        }
    }

    async fn decode<T: serde::de::DeserializeOwned>(
        response: reqwest::Response,
    ) -> Result<T, ApiError> {
        let status = response.status();
        if status.is_success() {
            return Ok(response.json().await?);
        }
        let body: serde_json::Value = response.json().await.unwrap_or(serde_json::Value::Null);
        Err(ApiError::Api {
            status: status.as_u16(),
            code: body["error"]["code"]
                .as_str()
                .unwrap_or("unknown")
                .to_owned(),
            message: body["error"]["message"]
                .as_str()
                .unwrap_or_default()
                .to_owned(),
        })
    }
}
