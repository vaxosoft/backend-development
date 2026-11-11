//! Exercise 3: bearer-token authentication and role checks as axum middleware.

use std::time::{SystemTime, UNIX_EPOCH};

use axum::extract::{Request, State};
use axum::http::{HeaderValue, StatusCode, header};
use axum::middleware::{self, Next};
use axum::response::{IntoResponse, Response};
use axum::routing::get;
use axum::{Extension, Json, Router};
use jsonwebtoken::{Algorithm, EncodingKey, Header, encode};
use serde::{Deserialize, Serialize};
use serde_json::json;

#[derive(Clone, Copy, Debug, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum Role {
    Member,
    Admin,
}

#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct Claims {
    pub sub: String,
    pub role: Role,
    pub exp: u64,
}

#[derive(Debug)]
pub enum ApiError {
    Unauthenticated,
    Forbidden,
}

impl IntoResponse for ApiError {
    fn into_response(self) -> Response {
        let (status, code, message) = match self {
            Self::Unauthenticated => (
                StatusCode::UNAUTHORIZED,
                "unauthorized",
                "missing or invalid bearer token",
            ),
            Self::Forbidden => (StatusCode::FORBIDDEN, "forbidden", "insufficient role"),
        };
        let mut resp = (
            status,
            Json(json!({"error": {"code": code, "message": message}})),
        )
            .into_response();
        if status == StatusCode::UNAUTHORIZED {
            resp.headers_mut()
                .insert(header::WWW_AUTHENTICATE, HeaderValue::from_static("Bearer"));
        }
        resp
    }
}

#[derive(Clone)]
pub struct Auth {
    secret: Vec<u8>,
}

fn now_secs() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|d| d.as_secs())
        .unwrap_or_default()
}

impl Auth {
    pub fn new(secret: &[u8]) -> Self {
        m11_auth_and_security_basics::install_jwt_crypto();
        Self {
            secret: secret.to_vec(),
        }
    }

    /// Signs an HS256 token that expires `ttl_secs` from now (negative = already expired).
    pub fn issue(&self, sub: &str, role: Role, ttl_secs: i64) -> String {
        let claims = Claims {
            sub: sub.to_string(),
            role,
            exp: now_secs().saturating_add_signed(ttl_secs),
        };
        encode(
            &Header::new(Algorithm::HS256),
            &claims,
            &EncodingKey::from_secret(&self.secret),
        )
        .expect("HS256 signing with an in-memory key cannot fail")
    }

    /// Validates signature, algorithm (HS256 only) and expiry (no leeway); `sub` and `exp` are required.
    pub fn verify(&self, _token: &str) -> Option<Claims> {
        todo!("decode with a pinned algorithm and strict expiry; None on any failure")
    }
}

/// Requires `Authorization: Bearer <token>` (scheme is case-insensitive) and stores the
/// verified `Claims` in the request extensions. Every failure is the same 401.
pub async fn require_auth(
    State(_auth): State<Auth>,
    _req: Request,
    _next: Next,
) -> Result<Response, ApiError> {
    todo!("read the header, split scheme and token, verify, insert Claims, call next")
}

/// Allows the request only when the caller's role is `required` (admins also pass `Member` checks).
pub async fn require_role(
    State(_required): State<Role>,
    Extension(_claims): Extension<Claims>,
    _req: Request,
    _next: Next,
) -> Result<Response, ApiError> {
    todo!("403 unless the caller's role is sufficient")
}

async fn me(Extension(claims): Extension<Claims>) -> Json<serde_json::Value> {
    Json(json!({"sub": claims.sub, "role": claims.role}))
}

async fn stats() -> Json<serde_json::Value> {
    Json(json!({"tasks": 0}))
}

pub fn app(auth: Auth) -> Router {
    let admin_only = Router::new()
        .route("/admin/stats", get(stats))
        .route_layer(middleware::from_fn_with_state(Role::Admin, require_role));

    Router::new()
        .route("/me", get(me))
        .merge(admin_only)
        .route_layer(middleware::from_fn_with_state(auth, require_auth))
        .route("/healthz", get(|| async { "ok" }))
}
