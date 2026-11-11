//! Task Tracker with register/login, JWT bearer middleware, roles and a per-user rate limit.
//!
//! ADDR=127.0.0.1:20700 cargo run -p m11-auth-and-security-basics --example tracker_auth

use std::collections::HashMap;
use std::sync::{Arc, Mutex};
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

use argon2::Argon2;
use argon2::password_hash::phc::PasswordHash;
use argon2::password_hash::{PasswordHasher, PasswordVerifier};
use axum::extract::{Request, State};
use axum::http::{HeaderValue, StatusCode, header};
use axum::middleware::{self, Next};
use axum::response::{IntoResponse, Response};
use axum::routing::{get, post};
use axum::{Extension, Json, Router};
use jsonwebtoken::{Algorithm, DecodingKey, EncodingKey, Header, Validation, decode, encode};
use m11_auth_and_security_basics::install_jwt_crypto;
use serde::{Deserialize, Serialize};
use serde_json::json;

#[derive(Debug, thiserror::Error)]
enum ApiError {
    #[error("invalid email or password")]
    BadCredentials,
    #[error("missing or invalid bearer token")]
    Unauthenticated,
    #[error("requires the {0} role")]
    Forbidden(&'static str),
    #[error("too many requests")]
    RateLimited(Duration),
    #[error("{0}")]
    Invalid(&'static str),
    #[error("email already registered")]
    Conflict,
    #[error("internal error")]
    Internal,
}

impl IntoResponse for ApiError {
    fn into_response(self) -> Response {
        let (status, code) = match &self {
            Self::BadCredentials | Self::Unauthenticated => {
                (StatusCode::UNAUTHORIZED, "unauthorized")
            }
            Self::Forbidden(_) => (StatusCode::FORBIDDEN, "forbidden"),
            Self::RateLimited(_) => (StatusCode::TOO_MANY_REQUESTS, "rate_limited"),
            Self::Invalid(_) => (StatusCode::UNPROCESSABLE_ENTITY, "validation_failed"),
            Self::Conflict => (StatusCode::CONFLICT, "conflict"),
            Self::Internal => (StatusCode::INTERNAL_SERVER_ERROR, "internal"),
        };
        let body = Json(json!({"error": {"code": code, "message": self.to_string()}}));
        let mut resp = (status, body).into_response();
        match self {
            Self::Unauthenticated => {
                resp.headers_mut()
                    .insert(header::WWW_AUTHENTICATE, HeaderValue::from_static("Bearer"));
            }
            Self::RateLimited(wait) => {
                let secs = wait.as_secs_f64().ceil().max(1.0) as u64;
                resp.headers_mut()
                    .insert(header::RETRY_AFTER, HeaderValue::from(secs));
            }
            _ => {}
        }
        resp
    }
}

#[derive(Clone, Copy, Debug, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
enum Role {
    Admin,
    Member,
}

#[derive(Clone, Debug, Serialize, Deserialize)]
struct Claims {
    sub: String,
    role: Role,
    exp: u64,
}

struct User {
    id: u64,
    email: String,
    password_hash: String,
    role: Role,
}

struct Bucket {
    tokens: f64,
    last: Instant,
}

#[derive(Default)]
struct Db {
    users: HashMap<String, User>,
    tasks: Vec<(u64, String)>,
    buckets: HashMap<u64, Bucket>,
}

struct AppState {
    db: Mutex<Db>,
    jwt_secret: Vec<u8>,
    // Verified against when the email is unknown, so both failures cost the same time
    dummy_hash: String,
}

type Shared = Arc<AppState>;

const BURST: f64 = 2.0;
const REFILL_PER_SEC: f64 = 0.2;

fn now_secs() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|d| d.as_secs())
        .unwrap_or_default()
}

fn hash(password: &str) -> Result<String, ApiError> {
    Argon2::default()
        .hash_password(password.as_bytes())
        .map(|h| h.to_string())
        .map_err(|_| ApiError::Internal)
}

fn verify(password: &str, stored: &str) -> bool {
    PasswordHash::new(stored)
        .map(|parsed| {
            Argon2::default()
                .verify_password(password.as_bytes(), &parsed)
                .is_ok()
        })
        .unwrap_or(false)
}

#[derive(Deserialize)]
struct Credentials {
    email: String,
    password: String,
}

async fn register(
    State(app): State<Shared>,
    Json(c): Json<Credentials>,
) -> Result<Response, ApiError> {
    if !c.email.contains('@') {
        return Err(ApiError::Invalid("email must contain @"));
    }
    if c.password.chars().count() < 8 {
        return Err(ApiError::Invalid("password must be at least 8 characters"));
    }
    let password = c.password;
    let password_hash = tokio::task::spawn_blocking(move || hash(&password))
        .await
        .map_err(|_| ApiError::Internal)??;

    let mut db = app.db.lock().map_err(|_| ApiError::Internal)?;
    if db.users.contains_key(&c.email) {
        return Err(ApiError::Conflict);
    }
    // Demo rule: the first account becomes the admin
    let role = if db.users.is_empty() {
        Role::Admin
    } else {
        Role::Member
    };
    let id = db.users.len() as u64 + 1;
    db.users.insert(
        c.email.clone(),
        User {
            id,
            email: c.email.clone(),
            password_hash,
            role,
        },
    );
    Ok((
        StatusCode::CREATED,
        Json(json!({"id": id, "email": c.email, "role": role})),
    )
        .into_response())
}

async fn login(
    State(app): State<Shared>,
    Json(c): Json<Credentials>,
) -> Result<Json<serde_json::Value>, ApiError> {
    let found = {
        let db = app.db.lock().map_err(|_| ApiError::Internal)?;
        db.users
            .get(&c.email)
            .map(|u| (u.id, u.role, u.password_hash.clone()))
    };
    let stored = found
        .as_ref()
        .map_or_else(|| app.dummy_hash.clone(), |(_, _, h)| h.clone());
    let password = c.password;
    let matches = tokio::task::spawn_blocking(move || verify(&password, &stored))
        .await
        .map_err(|_| ApiError::Internal)?;

    // One error for "no such user" and "wrong password"
    let (id, role) = match found {
        Some((id, role, _)) if matches => (id, role),
        _ => return Err(ApiError::BadCredentials),
    };
    let claims = Claims {
        sub: id.to_string(),
        role,
        exp: now_secs() + 3600,
    };
    let token = encode(
        &Header::new(Algorithm::HS256),
        &claims,
        &EncodingKey::from_secret(&app.jwt_secret),
    )
    .map_err(|_| ApiError::Internal)?;
    Ok(Json(json!({"token": token})))
}

async fn require_auth(
    State(app): State<Shared>,
    mut req: Request,
    next: Next,
) -> Result<Response, ApiError> {
    let token = req
        .headers()
        .get(header::AUTHORIZATION)
        .and_then(|v| v.to_str().ok())
        .and_then(|v| v.strip_prefix("Bearer "))
        .ok_or(ApiError::Unauthenticated)?;

    let mut validation = Validation::new(Algorithm::HS256);
    validation.leeway = 0;
    validation.set_required_spec_claims(&["exp", "sub"]);
    let data = decode::<Claims>(
        token,
        &DecodingKey::from_secret(&app.jwt_secret),
        &validation,
    )
    .map_err(|_| ApiError::Unauthenticated)?;

    req.extensions_mut().insert(data.claims);
    Ok(next.run(req).await)
}

async fn require_admin(
    Extension(claims): Extension<Claims>,
    req: Request,
    next: Next,
) -> Result<Response, ApiError> {
    if claims.role != Role::Admin {
        return Err(ApiError::Forbidden("admin"));
    }
    Ok(next.run(req).await)
}

#[derive(Deserialize)]
struct NewTask {
    title: String,
}

async fn create_task(
    State(app): State<Shared>,
    Extension(claims): Extension<Claims>,
    Json(t): Json<NewTask>,
) -> Result<Response, ApiError> {
    let user_id: u64 = claims.sub.parse().map_err(|_| ApiError::Unauthenticated)?;
    let title = t.title.trim().to_string();
    if title.is_empty() || title.chars().count() > 200 {
        return Err(ApiError::Invalid("title must be 1-200 characters"));
    }

    let mut db = app.db.lock().map_err(|_| ApiError::Internal)?;
    let now = Instant::now();
    let bucket = db.buckets.entry(user_id).or_insert(Bucket {
        tokens: BURST,
        last: now,
    });
    bucket.tokens =
        (bucket.tokens + now.duration_since(bucket.last).as_secs_f64() * REFILL_PER_SEC).min(BURST);
    bucket.last = now;
    if bucket.tokens < 1.0 {
        return Err(ApiError::RateLimited(Duration::from_secs_f64(
            (1.0 - bucket.tokens) / REFILL_PER_SEC,
        )));
    }
    bucket.tokens -= 1.0;

    let id = db.tasks.len() as u64 + 1;
    db.tasks.push((user_id, title.clone()));
    Ok((
        StatusCode::CREATED,
        Json(json!({"id": id, "title": title, "owner_id": user_id})),
    )
        .into_response())
}

async fn my_tasks(
    State(app): State<Shared>,
    Extension(claims): Extension<Claims>,
) -> Result<Json<serde_json::Value>, ApiError> {
    let db = app.db.lock().map_err(|_| ApiError::Internal)?;
    let items: Vec<_> = db
        .tasks
        .iter()
        .enumerate()
        .filter(|(_, (owner, _))| owner.to_string() == claims.sub)
        .map(|(i, (owner, title))| json!({"id": i + 1, "title": title, "owner_id": owner}))
        .collect();
    Ok(Json(json!({"items": items})))
}

async fn list_users(State(app): State<Shared>) -> Result<Json<serde_json::Value>, ApiError> {
    let db = app.db.lock().map_err(|_| ApiError::Internal)?;
    let mut users: Vec<_> = db
        .users
        .values()
        .map(|u| (u.id, u.email.clone(), u.role))
        .collect();
    users.sort_by_key(|(id, ..)| *id);
    let items: Vec<_> = users
        .into_iter()
        .map(|(id, email, role)| json!({"id": id, "email": email, "role": role}))
        .collect();
    Ok(Json(json!({"items": items})))
}

fn build_app(state: Shared) -> Router {
    let admin = Router::new()
        .route("/admin/users", get(list_users))
        // layers run outside-in: require_auth first, then require_admin
        .route_layer(middleware::from_fn(require_admin));

    let protected = Router::new()
        .route("/tasks", get(my_tasks).post(create_task))
        .merge(admin)
        .route_layer(middleware::from_fn_with_state(state.clone(), require_auth));

    Router::new()
        .route("/auth/register", post(register))
        .route("/auth/login", post(login))
        .merge(protected)
        .with_state(state)
}

async fn shutdown_signal() {
    let ctrl_c = async {
        let _ = tokio::signal::ctrl_c().await;
    };
    #[cfg(unix)]
    let terminate = async {
        match tokio::signal::unix::signal(tokio::signal::unix::SignalKind::terminate()) {
            Ok(mut s) => {
                s.recv().await;
            }
            Err(_) => std::future::pending::<()>().await,
        }
    };
    #[cfg(not(unix))]
    let terminate = std::future::pending::<()>();
    tokio::select! { _ = ctrl_c => {}, _ = terminate => {} }
}

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    install_jwt_crypto();
    let addr = std::env::var("ADDR").unwrap_or_else(|_| "127.0.0.1:8080".into());
    // A real service must fail to start without a strong secret; the default is for this demo only.
    let jwt_secret = std::env::var("JWT_SECRET")
        .unwrap_or_else(|_| "dev-only-secret-change-me-0123456789".into())
        .into_bytes();
    let dummy_hash = hash("dummy-password").map_err(|e| anyhow::anyhow!("{e}"))?;
    let state = Arc::new(AppState {
        db: Mutex::new(Db::default()),
        jwt_secret,
        dummy_hash,
    });

    let listener = tokio::net::TcpListener::bind(&addr).await?;
    println!("listening on {}", listener.local_addr()?);
    axum::serve(listener, build_app(state))
        .with_graceful_shutdown(shutdown_signal())
        .await?;
    println!("shut down cleanly");
    Ok(())
}
