//! Passwords, tokens and the `AuthUser` extractor.

use std::time::{SystemTime, UNIX_EPOCH};

use argon2::password_hash::phc::PasswordHash;
use argon2::password_hash::{PasswordHasher, PasswordVerifier};
use argon2::{Algorithm, Argon2, Params, Version};
use axum::extract::FromRequestParts;
use axum::http::request::Parts;
use axum::http::{HeaderMap, header};
use jsonwebtoken::{DecodingKey, EncodingKey, Header, Validation, decode, encode};
use serde::{Deserialize, Serialize};

use crate::AppState;
use crate::error::ApiError;

/// The authenticated caller; add it as a handler argument to require a valid token.
#[derive(Debug, Clone, Copy)]
pub struct AuthUser {
    pub id: i64,
}

#[derive(Debug, Serialize, Deserialize)]
struct Claims {
    sub: String,
    exp: u64,
}

/// Argon2id (t=2, p=1) with a fresh random salt; returns the PHC string.
pub fn hash_password(password: &str, memory_kib: u32) -> Result<String, ApiError> {
    let internal = |e: &dyn std::fmt::Display| ApiError::Internal(e.to_string());
    let params = Params::new(memory_kib, 2, 1, None).map_err(|e| internal(&e))?;
    let hasher = Argon2::new(Algorithm::Argon2id, Version::V0x13, params);
    let hash = hasher
        .hash_password(password.as_bytes())
        .map_err(|e| internal(&e))?;
    Ok(hash.to_string())
}

/// The cost parameters are read from the stored hash, so old hashes keep working
/// after the configured cost changes.
pub fn verify_password(password: &str, stored: &str) -> bool {
    let Ok(parsed) = PasswordHash::new(stored) else {
        return false;
    };
    Argon2::default()
        .verify_password(password.as_bytes(), &parsed)
        .is_ok()
}

pub fn issue_token(secret: &str, user_id: i64, ttl_secs: i64) -> Result<String, ApiError> {
    let now = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map_or(0, |d| d.as_secs());
    let claims = Claims {
        sub: user_id.to_string(),
        exp: now.saturating_add_signed(ttl_secs),
    };
    encode(
        &Header::new(jsonwebtoken::Algorithm::HS256),
        &claims,
        &EncodingKey::from_secret(secret.as_bytes()),
    )
    .map_err(|e| ApiError::Internal(e.to_string()))
}

pub fn verify_token(secret: &str, token: &str) -> Option<i64> {
    // Pin the algorithm: never trust the one named in the token header.
    let mut validation = Validation::new(jsonwebtoken::Algorithm::HS256);
    validation.leeway = 0;
    validation.set_required_spec_claims(&["exp", "sub"]);
    let data = decode::<Claims>(
        token,
        &DecodingKey::from_secret(secret.as_bytes()),
        &validation,
    )
    .ok()?;
    data.claims.sub.parse().ok()
}

pub fn bearer_user(headers: &HeaderMap, secret: &str) -> Option<i64> {
    let value = headers.get(header::AUTHORIZATION)?.to_str().ok()?;
    verify_token(secret, value.strip_prefix("Bearer ")?)
}

impl FromRequestParts<AppState> for AuthUser {
    type Rejection = ApiError;

    async fn from_request_parts(
        parts: &mut Parts,
        state: &AppState,
    ) -> Result<Self, Self::Rejection> {
        bearer_user(&parts.headers, &state.config.jwt_secret)
            .map(|id| AuthUser { id })
            .ok_or_else(|| ApiError::Unauthorized("missing or invalid token".into()))
    }
}
