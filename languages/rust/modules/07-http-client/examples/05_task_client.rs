//! A small typed client for the Task Tracker API: one `reqwest::Client`, a base URL,
//! a bearer token, query parameters and the shared error envelope.
use std::sync::{Arc, Mutex};

use axum::{
    Json, Router,
    extract::{Query, State},
    http::{HeaderMap, StatusCode},
    routing::get,
};
use reqwest::{Client, Url};
use serde::{Deserialize, Serialize};
use serde_json::{Value, json};

#[derive(Debug, Clone, Copy, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
enum Status {
    Todo,
    InProgress,
    Done,
}

#[derive(Debug, Deserialize)]
struct Task {
    id: u64,
    title: String,
    status: Status,
    tags: Vec<String>,
}

#[derive(Debug, Deserialize)]
struct Page {
    items: Vec<Task>,
    total: usize,
    limit: usize,
    offset: usize,
}

#[derive(Serialize)]
struct NewTask<'a> {
    title: &'a str,
    tags: &'a [&'a str],
}

#[derive(Debug, thiserror::Error)]
enum ClientError {
    #[error("transport: {0}")]
    Transport(#[from] reqwest::Error),
    #[error("{status} {code}: {message}")]
    Api {
        status: u16,
        code: String,
        message: String,
    },
}

struct TaskClient {
    http: Client,
    base: Url,
    token: String,
}

impl TaskClient {
    fn new(base: &str, token: &str) -> Result<Self, reqwest::Error> {
        let http = Client::builder()
            .user_agent("task-tracker-client/1.0")
            .build()?;
        Ok(Self {
            http,
            base: Url::parse(base).expect("valid base url"),
            token: token.to_owned(),
        })
    }

    async fn create(&self, title: &str, tags: &[&str]) -> Result<Task, ClientError> {
        let url = self.base.join("tasks").expect("valid path");
        let response = self
            .http
            .post(url)
            .bearer_auth(&self.token)
            .json(&NewTask { title, tags })
            .send()
            .await?;
        Self::decode(response).await
    }

    async fn list(
        &self,
        status: Option<Status>,
        limit: usize,
        offset: usize,
    ) -> Result<Page, ClientError> {
        // reqwest's `.query()` needs a cargo feature this workspace does not enable;
        // `Url` builds (and percent-encodes) the query string itself.
        let mut url = self.base.join("tasks").expect("valid path");
        {
            let mut query = url.query_pairs_mut();
            if let Some(status) = status {
                query.append_pair(
                    "status",
                    serde_json::to_value(status)
                        .expect("enum")
                        .as_str()
                        .unwrap_or_default(),
                );
            }
            query
                .append_pair("limit", &limit.to_string())
                .append_pair("offset", &offset.to_string());
        }
        let response = self.http.get(url).bearer_auth(&self.token).send().await?;
        Self::decode(response).await
    }

    async fn decode<T: serde::de::DeserializeOwned>(
        response: reqwest::Response,
    ) -> Result<T, ClientError> {
        let status = response.status();
        if status.is_success() {
            return Ok(response.json().await?);
        }
        let body: Value = response.json().await.unwrap_or(Value::Null);
        Err(ClientError::Api {
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

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    let base = spawn_upstream().await;
    let client = TaskClient::new(&base, "s3cret")?;

    let created = client.create("Ship it", &["release"]).await?;
    println!(
        "created #{} {:?} [{}]",
        created.id,
        created.title,
        created.tags.join(",")
    );

    let page = client.list(None, 2, 0).await?;
    println!(
        "page 1: {} of {} (limit {}, offset {})",
        page.items.len(),
        page.total,
        page.limit,
        page.offset
    );
    let page = client.list(None, 2, 2).await?;
    println!(
        "page 2: {} of {} (limit {}, offset {})",
        page.items.len(),
        page.total,
        page.limit,
        page.offset
    );
    let done = client.list(Some(Status::Done), 20, 0).await?;
    println!(
        "done: {:?}",
        done.items
            .iter()
            .map(|t| (t.id, t.status))
            .collect::<Vec<_>>()
    );

    let anonymous = TaskClient::new(&base, "wrong")?;
    match anonymous.list(None, 20, 0).await {
        Err(error) => println!("bad token -> {error}"),
        Ok(_) => println!("unexpected success"),
    }
    Ok(())
}

type Db = Arc<Mutex<Vec<Value>>>;

async fn spawn_upstream() -> String {
    let seed = vec![
        json!({"id": 1, "title": "Write docs", "status": "todo", "tags": ["docs"]}),
        json!({"id": 2, "title": "Review PR", "status": "done", "tags": []}),
    ];
    let app = Router::new()
        .route("/tasks", get(list_tasks).post(create_task))
        .with_state(Arc::new(Mutex::new(seed)));
    let listener = tokio::net::TcpListener::bind("127.0.0.1:0")
        .await
        .expect("bind loopback");
    let addr = listener.local_addr().expect("local addr");
    tokio::spawn(async move { axum::serve(listener, app).await });
    format!("http://{addr}/")
}

type ApiResult = Result<(StatusCode, Json<Value>), (StatusCode, Json<Value>)>;

fn authorize(headers: &HeaderMap) -> Result<(), (StatusCode, Json<Value>)> {
    match headers.get("authorization") {
        Some(value) if value == "Bearer s3cret" => Ok(()),
        _ => Err((
            StatusCode::UNAUTHORIZED,
            Json(
                json!({"error": {"code": "unauthorized", "message": "missing or invalid credentials"}}),
            ),
        )),
    }
}

async fn create_task(
    State(db): State<Db>,
    headers: HeaderMap,
    Json(mut body): Json<Value>,
) -> ApiResult {
    authorize(&headers)?;
    let mut db = db.lock().expect("lock");
    body["id"] = json!(db.len() + 1);
    body["status"] = json!("todo");
    db.push(body.clone());
    Ok((StatusCode::CREATED, Json(body)))
}

async fn list_tasks(
    State(db): State<Db>,
    headers: HeaderMap,
    Query(q): Query<std::collections::HashMap<String, String>>,
) -> ApiResult {
    authorize(&headers)?;
    let db = db.lock().expect("lock");
    let matching: Vec<&Value> = db
        .iter()
        .filter(|t| q.get("status").is_none_or(|s| t["status"] == s.as_str()))
        .collect();
    let limit: usize = q.get("limit").and_then(|v| v.parse().ok()).unwrap_or(20);
    let offset: usize = q.get("offset").and_then(|v| v.parse().ok()).unwrap_or(0);
    let items: Vec<&Value> = matching.iter().skip(offset).take(limit).copied().collect();
    Ok((
        StatusCode::OK,
        Json(json!({"items": items, "total": matching.len(), "limit": limit, "offset": offset})),
    ))
}
