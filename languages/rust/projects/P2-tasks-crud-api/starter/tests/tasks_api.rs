//! Black-box tests: a real server on an ephemeral port, a fresh in-memory
//! database per test, requests over HTTP.

use p2_tasks_crud_api::{Config, build_app};
use reqwest::{Client, Response, StatusCode};
use serde_json::{Value, json};
use tokio::net::TcpListener;

struct Api {
    base: String,
    client: Client,
}

impl Api {
    async fn start() -> Self {
        let app = build_app(&Config::default()).await.unwrap();
        let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let base = format!("http://{}", listener.local_addr().unwrap());
        tokio::spawn(async move { axum::serve(listener, app).await.unwrap() });
        Self {
            base,
            client: Client::new(),
        }
    }

    fn url(&self, path: &str) -> String {
        format!("{}{path}", self.base)
    }

    async fn get(&self, path: &str) -> Response {
        self.client.get(self.url(path)).send().await.unwrap()
    }

    async fn post(&self, body: Value) -> Response {
        self.client
            .post(self.url("/tasks"))
            .json(&body)
            .send()
            .await
            .unwrap()
    }

    async fn put(&self, path: &str, body: Value) -> Response {
        self.client
            .put(self.url(path))
            .json(&body)
            .send()
            .await
            .unwrap()
    }

    async fn patch(&self, path: &str, body: Value) -> Response {
        self.client
            .patch(self.url(path))
            .json(&body)
            .send()
            .await
            .unwrap()
    }

    async fn delete(&self, path: &str) -> Response {
        self.client.delete(self.url(path)).send().await.unwrap()
    }

    /// Creates a task, asserts 201 and returns the body.
    async fn create(&self, body: Value) -> Value {
        let resp = self.post(body).await;
        assert_eq!(resp.status(), StatusCode::CREATED, "create failed");
        resp.json().await.unwrap()
    }

    async fn titles(&self, query: &str) -> Vec<String> {
        let resp = self.get(&format!("/tasks{query}")).await;
        assert_eq!(resp.status(), StatusCode::OK, "list {query} failed");
        let body: Value = resp.json().await.unwrap();
        body["items"]
            .as_array()
            .expect("items must be an array")
            .iter()
            .map(|t| t["title"].as_str().unwrap().to_owned())
            .collect()
    }
}

async fn assert_error(resp: Response, status: StatusCode, code: &str) -> Value {
    assert_eq!(resp.status(), status);
    let content_type = resp.headers()["content-type"].to_str().unwrap().to_owned();
    assert!(
        content_type.starts_with("application/json"),
        "{content_type}"
    );
    let body: Value = resp.json().await.unwrap();
    assert_eq!(body["error"]["code"], code, "{body}");
    assert!(body["error"]["message"].is_string(), "{body}");
    body
}

async fn assert_invalid_field(resp: Response, field: &str) {
    let body = assert_error(resp, StatusCode::UNPROCESSABLE_ENTITY, "validation_failed").await;
    assert!(
        body["error"]["fields"][field].is_string(),
        "no message for {field}: {body}"
    );
}

// ---- create -------------------------------------------------------------

#[tokio::test]
async fn create_returns_201_location_and_the_stored_task() {
    let api = Api::start().await;
    let resp = api.post(json!({"title": "Write docs"})).await;
    assert_eq!(resp.status(), StatusCode::CREATED);
    let location = resp.headers()["location"].to_str().unwrap().to_owned();
    let task: Value = resp.json().await.unwrap();

    assert_eq!(location, format!("/tasks/{}", task["id"]));
    assert_eq!(task["title"], "Write docs");
    assert_eq!(task["status"], "todo");
    assert_eq!(task["due_date"], Value::Null);
    assert_eq!(task["tags"], json!([]));
    assert_eq!(task["owner_id"], Value::Null);
    let created_at = task["created_at"].as_str().unwrap();
    assert!(
        created_at.ends_with('Z') && created_at.contains('T'),
        "{created_at}"
    );
}

#[tokio::test]
async fn create_trims_the_title() {
    let api = Api::start().await;
    let task = api.create(json!({"title": "  padded  "})).await;
    assert_eq!(task["title"], "padded");
}

#[tokio::test]
async fn create_keeps_all_given_fields() {
    let api = Api::start().await;
    let task = api
        .create(json!({
            "title": "Ship it", "status": "in_progress", "due_date": "2030-01-31",
            "tags": ["docs", "p1"], "owner_id": 7
        }))
        .await;
    assert_eq!(task["status"], "in_progress");
    assert_eq!(task["due_date"], "2030-01-31");
    assert_eq!(task["tags"], json!(["docs", "p1"]));
    assert_eq!(task["owner_id"], 7);
}

#[tokio::test]
async fn create_without_title_is_422_with_a_title_field_error() {
    let api = Api::start().await;
    assert_invalid_field(api.post(json!({"status": "done"})).await, "title").await;
}

#[tokio::test]
async fn create_enforces_title_length_between_1_and_200() {
    let api = Api::start().await;
    assert_invalid_field(api.post(json!({"title": "   "})).await, "title").await;
    assert_invalid_field(api.post(json!({"title": "x".repeat(201)})).await, "title").await;
    let ok = api.create(json!({"title": "x".repeat(200)})).await;
    assert_eq!(ok["title"].as_str().unwrap().len(), 200);
}

#[tokio::test]
async fn create_with_unknown_status_is_422() {
    let api = Api::start().await;
    assert_invalid_field(
        api.post(json!({"title": "a", "status": "finished"})).await,
        "status",
    )
    .await;
}

#[tokio::test]
async fn create_with_malformed_due_date_is_422() {
    let api = Api::start().await;
    for bad in ["tomorrow", "2030-02-31", "2030-1-5", "31/01/2030"] {
        assert_invalid_field(
            api.post(json!({"title": "a", "due_date": bad})).await,
            "due_date",
        )
        .await;
    }
}

#[tokio::test]
async fn create_rejects_invalid_tag_lists_with_422() {
    let api = Api::start().await;
    let too_many: Vec<String> = (0..11).map(|i| format!("t{i}")).collect();
    let cases = [
        json!(["Docs"]),
        json!(["a", "a"]),
        json!([""]),
        json!(["x".repeat(31)]),
        json!(too_many),
    ];
    for tags in cases {
        assert_invalid_field(api.post(json!({"title": "a", "tags": tags})).await, "tags").await;
    }
}

#[tokio::test]
async fn create_reports_every_invalid_field_at_once() {
    let api = Api::start().await;
    let resp = api
        .post(json!({"title": "", "status": "nope", "due_date": "x", "tags": ["A"]}))
        .await;
    let body = assert_error(resp, StatusCode::UNPROCESSABLE_ENTITY, "validation_failed").await;
    let fields = body["error"]["fields"].as_object().unwrap();
    for name in ["title", "status", "due_date", "tags"] {
        assert!(fields.contains_key(name), "missing {name} in {fields:?}");
    }
}

#[tokio::test]
async fn malformed_json_is_400_bad_request() {
    let api = Api::start().await;
    let resp = api
        .client
        .post(api.url("/tasks"))
        .header("content-type", "application/json")
        .body("{\"title\": ")
        .send()
        .await
        .unwrap();
    assert_error(resp, StatusCode::BAD_REQUEST, "bad_request").await;
}

// ---- read ---------------------------------------------------------------

#[tokio::test]
async fn get_returns_the_created_task() {
    let api = Api::start().await;
    let created = api.create(json!({"title": "Find me", "tags": ["x"]})).await;
    let resp = api.get(&format!("/tasks/{}", created["id"])).await;
    assert_eq!(resp.status(), StatusCode::OK);
    assert_eq!(resp.json::<Value>().await.unwrap(), created);
}

#[tokio::test]
async fn get_unknown_or_non_numeric_id_is_404() {
    let api = Api::start().await;
    assert_error(
        api.get("/tasks/999").await,
        StatusCode::NOT_FOUND,
        "not_found",
    )
    .await;
    assert_error(
        api.get("/tasks/abc").await,
        StatusCode::NOT_FOUND,
        "not_found",
    )
    .await;
}

// ---- replace and patch --------------------------------------------------

#[tokio::test]
async fn put_replaces_everything_but_id_and_created_at() {
    let api = Api::start().await;
    let old = api
        .create(json!({"title": "Old", "status": "done", "due_date": "2030-01-01", "tags": ["a"], "owner_id": 3}))
        .await;
    let resp = api
        .put(&format!("/tasks/{}", old["id"]), json!({"title": "New"}))
        .await;
    assert_eq!(resp.status(), StatusCode::OK);
    let new: Value = resp.json().await.unwrap();

    assert_eq!(new["id"], old["id"]);
    assert_eq!(new["created_at"], old["created_at"]);
    assert_eq!(new["title"], "New");
    assert_eq!(new["status"], "todo");
    assert_eq!(new["due_date"], Value::Null);
    assert_eq!(new["tags"], json!([]));
    assert_eq!(new["owner_id"], Value::Null);
    let again: Value = api
        .get(&format!("/tasks/{}", old["id"]))
        .await
        .json()
        .await
        .unwrap();
    assert_eq!(again, new);
}

#[tokio::test]
async fn put_with_invalid_body_is_422_and_changes_nothing() {
    let api = Api::start().await;
    let task = api.create(json!({"title": "Keep"})).await;
    let path = format!("/tasks/{}", task["id"]);
    assert_invalid_field(api.put(&path, json!({"title": ""})).await, "title").await;
    assert_eq!(api.get(&path).await.json::<Value>().await.unwrap(), task);
}

#[tokio::test]
async fn put_and_patch_on_unknown_id_are_404() {
    let api = Api::start().await;
    assert_error(
        api.put("/tasks/42", json!({"title": "x"})).await,
        StatusCode::NOT_FOUND,
        "not_found",
    )
    .await;
    assert_error(
        api.patch("/tasks/42", json!({"title": "x"})).await,
        StatusCode::NOT_FOUND,
        "not_found",
    )
    .await;
}

#[tokio::test]
async fn patch_changes_only_the_fields_that_are_present() {
    let api = Api::start().await;
    let task = api
        .create(json!({"title": "Before", "due_date": "2030-05-05", "tags": ["a"], "owner_id": 9}))
        .await;
    let resp = api
        .patch(
            &format!("/tasks/{}", task["id"]),
            json!({"status": "done", "tags": ["b", "c"]}),
        )
        .await;
    assert_eq!(resp.status(), StatusCode::OK);
    let patched: Value = resp.json().await.unwrap();

    assert_eq!(patched["status"], "done");
    assert_eq!(patched["tags"], json!(["b", "c"]));
    assert_eq!(patched["title"], "Before");
    assert_eq!(patched["due_date"], "2030-05-05");
    assert_eq!(patched["owner_id"], 9);
    assert_eq!(patched["created_at"], task["created_at"]);
}

#[tokio::test]
async fn patch_with_null_due_date_clears_it() {
    let api = Api::start().await;
    let task = api
        .create(json!({"title": "Dated", "due_date": "2030-05-05"}))
        .await;
    let path = format!("/tasks/{}", task["id"]);
    let patched: Value = api
        .patch(&path, json!({"due_date": null}))
        .await
        .json()
        .await
        .unwrap();
    assert_eq!(patched["due_date"], Value::Null);
    assert_eq!(patched["title"], "Dated");
    let empty: Value = api.patch(&path, json!({})).await.json().await.unwrap();
    assert_eq!(empty, patched, "an empty patch must change nothing");
}

#[tokio::test]
async fn patch_with_invalid_value_is_422_and_changes_nothing() {
    let api = Api::start().await;
    let task = api.create(json!({"title": "Stable"})).await;
    let path = format!("/tasks/{}", task["id"]);
    assert_invalid_field(
        api.patch(&path, json!({"title": "ok", "status": "nope"}))
            .await,
        "status",
    )
    .await;
    assert_eq!(api.get(&path).await.json::<Value>().await.unwrap(), task);
}

// ---- delete -------------------------------------------------------------

#[tokio::test]
async fn delete_returns_204_then_the_task_is_gone() {
    let api = Api::start().await;
    let task = api.create(json!({"title": "Doomed"})).await;
    let path = format!("/tasks/{}", task["id"]);
    assert_eq!(api.delete(&path).await.status(), StatusCode::NO_CONTENT);
    assert_error(api.get(&path).await, StatusCode::NOT_FOUND, "not_found").await;
    assert_error(api.delete(&path).await, StatusCode::NOT_FOUND, "not_found").await;
}

// ---- list ---------------------------------------------------------------

#[tokio::test]
async fn list_uses_the_envelope_and_defaults_to_newest_first() {
    let api = Api::start().await;
    for title in ["first", "second", "third"] {
        api.create(json!({"title": title})).await;
    }
    let body: Value = api.get("/tasks").await.json().await.unwrap();
    assert_eq!(body["total"], 3);
    assert_eq!(body["limit"], 20);
    assert_eq!(body["offset"], 0);
    assert_eq!(api.titles("").await, ["third", "second", "first"]);
    assert_eq!(
        api.titles("?sort=created_at").await,
        ["first", "second", "third"]
    );
}

#[tokio::test]
async fn list_filters_by_status_and_by_tag() {
    let api = Api::start().await;
    api.create(json!({"title": "a", "status": "done", "tags": ["x"]}))
        .await;
    api.create(json!({"title": "b", "status": "todo", "tags": ["x", "y"]}))
        .await;
    api.create(json!({"title": "c", "status": "done", "tags": ["y"]}))
        .await;

    assert_eq!(api.titles("?status=done&sort=created_at").await, ["a", "c"]);
    assert_eq!(api.titles("?tag=y&sort=created_at").await, ["b", "c"]);
    assert_eq!(api.titles("?status=done&tag=y").await, ["c"]);
    assert!(api.titles("?tag=nothing").await.is_empty());
    let body: Value = api.get("/tasks?status=done").await.json().await.unwrap();
    assert_eq!(body["total"], 2);
}

#[tokio::test]
async fn list_paginates_and_reports_the_total_of_all_matches() {
    let api = Api::start().await;
    for n in 1..=5 {
        api.create(json!({"title": format!("t{n}")})).await;
    }
    let body: Value = api
        .get("/tasks?sort=created_at&limit=2&offset=2")
        .await
        .json()
        .await
        .unwrap();
    assert_eq!(body["total"], 5);
    assert_eq!(body["limit"], 2);
    assert_eq!(body["offset"], 2);
    assert_eq!(
        api.titles("?sort=created_at&limit=2&offset=2").await,
        ["t3", "t4"]
    );
    assert!(api.titles("?offset=5").await.is_empty());
}

#[tokio::test]
async fn list_clamps_limit_to_100() {
    let api = Api::start().await;
    let body: Value = api.get("/tasks?limit=1000").await.json().await.unwrap();
    assert_eq!(body["limit"], 100);
}

#[tokio::test]
async fn list_sorts_by_due_date_with_undated_tasks_last() {
    let api = Api::start().await;
    api.create(json!({"title": "none"})).await;
    api.create(json!({"title": "late", "due_date": "2031-01-01"}))
        .await;
    api.create(json!({"title": "soon", "due_date": "2030-01-01"}))
        .await;

    assert_eq!(api.titles("?sort=due_date").await, ["soon", "late", "none"]);
    assert_eq!(
        api.titles("?sort=-due_date").await,
        ["late", "soon", "none"]
    );
}

#[tokio::test]
async fn list_with_bad_query_values_is_400() {
    let api = Api::start().await;
    for query in [
        "status=bogus",
        "limit=abc",
        "limit=0",
        "offset=-1",
        "sort=title",
    ] {
        let resp = api.get(&format!("/tasks?{query}")).await;
        assert_error(resp, StatusCode::BAD_REQUEST, "bad_request").await;
    }
}
