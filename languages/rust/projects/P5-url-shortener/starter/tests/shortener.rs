//! Black-box tests over real HTTP against an in-process server on a random loopback port.

use std::sync::{Arc, Mutex};
use std::time::Duration;

use base64::Engine;
use base64::engine::general_purpose::URL_SAFE_NO_PAD;
use p5_url_shortener::{Config, build_app};
use reqwest::header::HeaderMap;
use reqwest::{Client, Method, redirect::Policy};
use serde_json::{Value, json};
use tokio::net::TcpListener;

const PASSWORD: &str = "correct horse battery";

struct Reply {
    status: u16,
    headers: HeaderMap,
    body: Value,
}

struct App {
    base: String,
    http: Client,
    logs: Arc<Mutex<Vec<String>>>,
}

async fn start_with(tune: impl FnOnce(&mut Config)) -> App {
    let logs = Arc::new(Mutex::new(Vec::new()));
    let sink = logs.clone();
    let mut config = Config::new("test-secret-that-is-long-enough");
    config.hash_memory_kib = 64;
    config.rate_burst = 100;
    config.log_sink = Arc::new(move |line| sink.lock().unwrap().push(line));
    tune(&mut config);
    let app = build_app(config).await.expect("app starts");
    let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
    let addr = listener.local_addr().unwrap();
    tokio::spawn(async move { axum::serve(listener, app).await.unwrap() });
    App {
        base: format!("http://{addr}"),
        // The redirect itself is under test, so the client must not follow it.
        http: Client::builder().redirect(Policy::none()).build().unwrap(),
        logs,
    }
}

async fn start() -> App {
    start_with(|_| {}).await
}

impl App {
    async fn send(
        &self,
        method: Method,
        path: &str,
        token: Option<&str>,
        body: Option<Value>,
    ) -> Reply {
        let mut request = self.http.request(method, format!("{}{path}", self.base));
        if let Some(token) = token {
            request = request.bearer_auth(token);
        }
        if let Some(body) = body {
            request = request.json(&body);
        }
        let response = request.send().await.unwrap();
        let status = response.status().as_u16();
        let headers = response.headers().clone();
        let text = response.text().await.unwrap();
        let body = serde_json::from_str(&text).unwrap_or(Value::Null);
        Reply {
            status,
            headers,
            body,
        }
    }

    async fn get(&self, path: &str, token: Option<&str>) -> Reply {
        self.send(Method::GET, path, token, None).await
    }

    async fn post(&self, path: &str, token: Option<&str>, body: Value) -> Reply {
        self.send(Method::POST, path, token, Some(body)).await
    }

    async fn delete(&self, path: &str, token: Option<&str>) -> Reply {
        self.send(Method::DELETE, path, token, None).await
    }

    async fn register(&self, email: &str) -> Reply {
        let body = json!({"email": email, "password": PASSWORD});
        self.post("/auth/register", None, body).await
    }

    async fn login(&self, email: &str, password: &str) -> Reply {
        let body = json!({"email": email, "password": password});
        self.post("/auth/login", None, body).await
    }

    /// Registers a user and returns its JWT.
    async fn user(&self, email: &str) -> String {
        assert_eq!(self.register(email).await.status, 201);
        let reply = self.login(email, PASSWORD).await;
        assert_eq!(reply.status, 200);
        reply.body["token"].as_str().unwrap().to_string()
    }

    async fn shorten(&self, token: &str, url: &str, slug: Option<&str>) -> Reply {
        let mut body = json!({"url": url});
        if let Some(slug) = slug {
            body["slug"] = json!(slug);
        }
        self.post("/links", Some(token), body).await
    }

    fn log_lines(&self) -> Vec<Value> {
        let lines = self.logs.lock().unwrap();
        lines
            .iter()
            .map(|l| serde_json::from_str(l).expect("log line is JSON"))
            .collect()
    }
}

fn error_code(reply: &Reply) -> &str {
    reply.body["error"]["code"]
        .as_str()
        .unwrap_or("<no error envelope>")
}

// ---- registration and login ----

#[tokio::test]
async fn register_creates_a_user_and_never_echoes_the_password() {
    let app = start().await;
    let reply = app.register("Ada@Example.com").await;
    assert_eq!(reply.status, 201);
    assert_eq!(reply.body["email"], "ada@example.com");
    assert!(reply.body["id"].is_i64());
    assert!(!reply.body.to_string().contains(PASSWORD));
}

#[tokio::test]
async fn register_rejects_bad_email_and_short_password_with_field_errors() {
    let app = start().await;
    let reply = app
        .post(
            "/auth/register",
            None,
            json!({"email": "nope", "password": "short"}),
        )
        .await;
    assert_eq!(reply.status, 422);
    assert_eq!(error_code(&reply), "validation_failed");
    assert!(reply.body["error"]["fields"]["email"].is_string());
    assert!(reply.body["error"]["fields"]["password"].is_string());
}

#[tokio::test]
async fn registering_the_same_email_twice_is_a_conflict() {
    let app = start().await;
    assert_eq!(app.register("a@example.com").await.status, 201);
    let again = app.register("A@example.com").await;
    assert_eq!(again.status, 409);
    assert_eq!(error_code(&again), "conflict");
}

#[tokio::test]
async fn malformed_json_is_a_400_with_the_error_envelope() {
    let app = start().await;
    let response = app
        .http
        .post(format!("{}/auth/register", app.base))
        .header("content-type", "application/json")
        .body("{not json")
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), 400);
    let body: Value = response.json().await.unwrap();
    assert_eq!(body["error"]["code"], "bad_request");
}

#[tokio::test]
async fn passwords_are_stored_as_argon2_hashes() {
    let dir = tempfile::tempdir().unwrap();
    let url = format!("sqlite://{}", dir.path().join("test.db").display());
    let app = start_with(|c| c.database_url = url.clone()).await;
    assert_eq!(app.register("hash@example.com").await.status, 201);

    let pool = sqlx::SqlitePool::connect(&url).await.unwrap();
    let stored: String = sqlx::query_scalar("SELECT password_hash FROM users")
        .fetch_one(&pool)
        .await
        .unwrap();
    assert!(stored.starts_with("$argon2id$"), "stored: {stored}");
    assert!(!stored.contains(PASSWORD));
}

#[tokio::test]
async fn login_returns_an_hs256_jwt_that_expires_in_one_hour() {
    let app = start().await;
    app.register("jwt@example.com").await;
    let reply = app.login("jwt@example.com", PASSWORD).await;
    assert_eq!(reply.status, 200);
    let token = reply.body["token"].as_str().unwrap();
    let parts: Vec<&str> = token.split('.').collect();
    assert_eq!(parts.len(), 3);
    let header: Value = serde_json::from_slice(&URL_SAFE_NO_PAD.decode(parts[0]).unwrap()).unwrap();
    let claims: Value = serde_json::from_slice(&URL_SAFE_NO_PAD.decode(parts[1]).unwrap()).unwrap();
    assert_eq!(header["alg"], "HS256");
    let now = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap()
        .as_secs();
    let exp = claims["exp"].as_u64().unwrap();
    assert!(
        (now + 3500..=now + 3700).contains(&exp),
        "exp {exp}, now {now}"
    );
}

#[tokio::test]
async fn unknown_user_and_wrong_password_are_indistinguishable() {
    let app = start().await;
    app.register("real@example.com").await;
    let wrong = app.login("real@example.com", "not the password").await;
    let unknown = app.login("ghost@example.com", PASSWORD).await;
    assert_eq!(wrong.status, 401);
    assert_eq!(unknown.status, 401);
    assert_eq!(error_code(&wrong), "unauthorized");
    assert_eq!(wrong.body, unknown.body);
}

#[tokio::test]
async fn protected_routes_reject_missing_garbage_and_expired_tokens() {
    let app = start_with(|c| c.token_ttl_secs = -10).await;
    let expired = app.user("old@example.com").await;
    for token in [None, Some("garbage"), Some(expired.as_str())] {
        let reply = app.get("/links", token).await;
        assert_eq!(reply.status, 401, "token {token:?}");
        assert_eq!(error_code(&reply), "unauthorized");
    }
    let reply = app
        .post("/links", None, json!({"url": "https://example.com/"}))
        .await;
    assert_eq!(reply.status, 401);
}

// ---- links ----

#[tokio::test]
async fn create_link_with_a_custom_slug() {
    let app = start().await;
    let token = app.user("a@example.com").await;
    let reply = app
        .shorten(&token, "https://example.com/docs?page=1", Some("My_Slug-1"))
        .await;
    assert_eq!(reply.status, 201);
    assert_eq!(
        reply.body,
        json!({"slug": "My_Slug-1", "url": "https://example.com/docs?page=1"})
    );
}

#[tokio::test]
async fn create_link_without_a_slug_generates_seven_alphanumeric_characters() {
    let app = start().await;
    let token = app.user("a@example.com").await;
    let reply = app.shorten(&token, "http://example.com/x", None).await;
    assert_eq!(reply.status, 201);
    let slug = reply.body["slug"].as_str().unwrap();
    assert_eq!(slug.len(), 7);
    assert!(
        slug.chars().all(|c| c.is_ascii_alphanumeric()),
        "slug {slug}"
    );
    assert_eq!(app.get(&format!("/{slug}"), None).await.status, 302);
}

#[tokio::test]
async fn only_absolute_http_and_https_urls_are_accepted() {
    let app = start().await;
    let token = app.user("a@example.com").await;
    for bad in [
        "ftp://example.com/f",
        "not a url",
        "/relative",
        "javascript:alert(1)",
        "",
        "example.com",
    ] {
        let reply = app.shorten(&token, bad, None).await;
        assert_eq!(reply.status, 422, "url {bad:?}");
        assert!(reply.body["error"]["fields"]["url"].is_string());
    }
}

#[tokio::test]
async fn slugs_must_be_3_to_32_url_safe_characters_and_not_reserved() {
    let app = start().await;
    let token = app.user("a@example.com").await;
    let too_long = "a".repeat(33);
    for bad in [
        "ab",
        too_long.as_str(),
        "has space",
        "a/b",
        "dot.dot",
        "links",
    ] {
        let reply = app.shorten(&token, "https://example.com/", Some(bad)).await;
        assert_eq!(reply.status, 422, "slug {bad:?}");
        assert!(reply.body["error"]["fields"]["slug"].is_string());
    }
    let longest = "a".repeat(32);
    assert_eq!(
        app.shorten(&token, "https://example.com/", Some(&longest))
            .await
            .status,
        201
    );
    assert_eq!(
        app.shorten(&token, "https://example.com/", Some("abc"))
            .await
            .status,
        201
    );
}

#[tokio::test]
async fn a_taken_slug_is_a_409_even_for_another_user() {
    let app = start().await;
    let alice = app.user("alice@example.com").await;
    let bob = app.user("bob@example.com").await;
    assert_eq!(
        app.shorten(&alice, "https://example.com/1", Some("shared"))
            .await
            .status,
        201
    );
    let reply = app
        .shorten(&bob, "https://example.com/2", Some("shared"))
        .await;
    assert_eq!(reply.status, 409);
    assert_eq!(error_code(&reply), "conflict");
}

#[tokio::test]
async fn visiting_a_slug_redirects_with_302_and_needs_no_login() {
    let app = start().await;
    let token = app.user("a@example.com").await;
    app.shorten(&token, "https://example.com/target?x=1", Some("go-here"))
        .await;
    let reply = app.get("/go-here", None).await;
    assert_eq!(reply.status, 302);
    assert_eq!(reply.headers["location"], "https://example.com/target?x=1");
}

#[tokio::test]
async fn unknown_slug_is_a_404_with_the_error_envelope() {
    let app = start().await;
    let reply = app.get("/nothing-here", None).await;
    assert_eq!(reply.status, 404);
    assert_eq!(error_code(&reply), "not_found");
}

#[tokio::test]
async fn every_redirect_counts_a_hit_and_list_shows_only_own_links() {
    let app = start().await;
    let alice = app.user("alice@example.com").await;
    let bob = app.user("bob@example.com").await;
    app.shorten(&alice, "https://example.com/a", Some("alice-a"))
        .await;
    app.shorten(&alice, "https://example.com/b", Some("alice-b"))
        .await;
    app.shorten(&bob, "https://example.com/c", Some("bob-c"))
        .await;
    for _ in 0..3 {
        app.get("/alice-a", None).await;
    }

    let reply = app.get("/links", Some(&alice)).await;
    assert_eq!(reply.status, 200);
    assert_eq!(
        reply.body["items"],
        json!([
            {"slug": "alice-a", "url": "https://example.com/a", "hits": 3},
            {"slug": "alice-b", "url": "https://example.com/b", "hits": 0},
        ])
    );
    let bobs = app.get("/links", Some(&bob)).await;
    assert_eq!(bobs.body["items"].as_array().unwrap().len(), 1);
}

#[tokio::test]
async fn owner_can_delete_a_link_and_it_stops_redirecting() {
    let app = start().await;
    let token = app.user("a@example.com").await;
    app.shorten(&token, "https://example.com/", Some("temp-link"))
        .await;
    assert_eq!(
        app.delete("/links/temp-link", Some(&token)).await.status,
        204
    );
    assert_eq!(app.get("/temp-link", None).await.status, 404);
}

#[tokio::test]
async fn deleting_needs_login_and_ownership_and_an_existing_slug() {
    let app = start().await;
    let alice = app.user("alice@example.com").await;
    let bob = app.user("bob@example.com").await;
    app.shorten(&alice, "https://example.com/", Some("alices"))
        .await;

    assert_eq!(app.delete("/links/alices", None).await.status, 401);
    let forbidden = app.delete("/links/alices", Some(&bob)).await;
    assert_eq!(forbidden.status, 403);
    assert_eq!(error_code(&forbidden), "forbidden");
    assert_eq!(
        app.get("/alices", None).await.status,
        302,
        "link must survive"
    );
    assert_eq!(app.delete("/links/missing", Some(&bob)).await.status, 404);
}

// ---- rate limiting ----

#[tokio::test]
async fn creating_links_beyond_the_burst_is_a_429_with_retry_after() {
    let app = start_with(|c| {
        c.rate_burst = 3;
        c.rate_refill_per_sec = 0.01;
    })
    .await;
    let token = app.user("a@example.com").await;
    for i in 0..3 {
        let reply = app
            .shorten(&token, &format!("https://example.com/{i}"), None)
            .await;
        assert_eq!(reply.status, 201, "request {i}");
    }
    let limited = app.shorten(&token, "https://example.com/4", None).await;
    assert_eq!(limited.status, 429);
    assert_eq!(error_code(&limited), "rate_limited");
    let retry_after: u64 = limited.headers["retry-after"]
        .to_str()
        .unwrap()
        .parse()
        .unwrap();
    assert!(
        (1..=100).contains(&retry_after),
        "retry-after {retry_after}"
    );
    assert_eq!(
        app.get("/links", Some(&token)).await.status,
        200,
        "only POST /links is limited"
    );
}

#[tokio::test]
async fn the_rate_limit_is_per_user() {
    let app = start_with(|c| {
        c.rate_burst = 1;
        c.rate_refill_per_sec = 0.01;
    })
    .await;
    let alice = app.user("alice@example.com").await;
    let bob = app.user("bob@example.com").await;
    assert_eq!(
        app.shorten(&alice, "https://example.com/1", None)
            .await
            .status,
        201
    );
    assert_eq!(
        app.shorten(&alice, "https://example.com/2", None)
            .await
            .status,
        429
    );
    assert_eq!(
        app.shorten(&bob, "https://example.com/3", None)
            .await
            .status,
        201
    );
}

#[tokio::test]
async fn tokens_refill_over_time() {
    let app = start_with(|c| {
        c.rate_burst = 1;
        c.rate_refill_per_sec = 20.0;
    })
    .await;
    let token = app.user("a@example.com").await;
    assert_eq!(
        app.shorten(&token, "https://example.com/1", None)
            .await
            .status,
        201
    );
    assert_eq!(
        app.shorten(&token, "https://example.com/2", None)
            .await
            .status,
        429
    );
    tokio::time::sleep(Duration::from_millis(120)).await;
    assert_eq!(
        app.shorten(&token, "https://example.com/3", None)
            .await
            .status,
        201
    );
}

// ---- structured logging ----

#[tokio::test]
async fn every_request_writes_one_json_line_with_the_standard_fields() {
    let app = start().await;
    let before = app.log_lines().len();
    app.get("/nothing-here", None).await;
    let lines = app.log_lines();
    assert_eq!(lines.len(), before + 1);
    let line = lines.last().unwrap();
    assert_eq!(line["method"], "GET");
    assert_eq!(line["path"], "/nothing-here");
    assert_eq!(line["status"], 404);
    assert!(line["request_id"].as_str().is_some_and(|id| !id.is_empty()));
    assert!(line["duration_ms"].is_u64());
    assert!(
        line.get("user_id").is_none(),
        "anonymous requests have no user_id"
    );
}

#[tokio::test]
async fn authenticated_requests_log_the_user_id() {
    let app = start().await;
    let registered = app.register("a@example.com").await;
    let token = app.login("a@example.com", PASSWORD).await.body["token"]
        .as_str()
        .unwrap()
        .to_string();
    app.get("/links", Some(&token)).await;
    let lines = app.log_lines();
    let line = lines.last().unwrap();
    assert_eq!(line["path"], "/links");
    assert_eq!(line["user_id"], registered.body["id"]);
}

#[tokio::test]
async fn request_id_is_echoed_or_generated_and_matches_the_log() {
    let app = start().await;
    let echoed = app
        .http
        .get(format!("{}/x-nothing", app.base))
        .header("X-Request-ID", "trace-me-42")
        .send()
        .await
        .unwrap();
    assert_eq!(echoed.headers()["x-request-id"], "trace-me-42");

    let generated = app.get("/y-nothing", None).await;
    let id = generated.headers["x-request-id"]
        .to_str()
        .unwrap()
        .to_string();
    assert!(id.len() >= 16);

    let lines = app.log_lines();
    assert_eq!(lines[0]["request_id"], "trace-me-42");
    assert_eq!(lines[1]["request_id"], id.as_str());
}

#[tokio::test]
async fn passwords_tokens_and_query_strings_never_appear_in_logs() {
    let app = start().await;
    let token = app.user("secret@example.com").await;
    app.get("/links?api_key=hunter2hunter2", Some(&token)).await;
    app.login("secret@example.com", "a wrong password").await;
    let everything = app.logs.lock().unwrap().join("\n");
    for secret in [
        PASSWORD,
        "a wrong password",
        token.as_str(),
        "hunter2hunter2",
    ] {
        assert!(!everything.contains(secret), "log leaked {secret:?}");
    }
    assert!(!app.log_lines().is_empty());
}
