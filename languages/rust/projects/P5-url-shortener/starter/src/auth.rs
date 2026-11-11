//! Passwords, tokens and the `AuthUser` extractor.

use axum::extract::FromRequestParts;
use axum::http::HeaderMap;
use axum::http::request::Parts;

use crate::AppState;
use crate::error::ApiError;

/// The authenticated caller; add it as a handler argument to require a valid token.
#[derive(Debug, Clone, Copy)]
pub struct AuthUser {
    pub id: i64,
}

/// TODO: Argon2id (t=2, p=1, `memory_kib` memory) with a fresh random salt; return the PHC string.
pub fn hash_password(_password: &str, _memory_kib: u32) -> Result<String, ApiError> {
    todo!("hash with argon2id")
}

/// TODO: verify against a stored PHC string. A malformed hash is simply `false`.
pub fn verify_password(_password: &str, _stored: &str) -> bool {
    todo!("verify with argon2")
}

/// TODO: an HS256 JWT with claims `sub` (the user id as a string) and `exp` (now + `ttl_secs`).
pub fn issue_token(_secret: &str, _user_id: i64, _ttl_secs: i64) -> Result<String, ApiError> {
    todo!("sign a jwt")
}

/// TODO: the user id inside a valid token. Accept only HS256, require `exp`, allow no leeway.
pub fn verify_token(_secret: &str, _token: &str) -> Option<i64> {
    todo!("validate a jwt")
}

/// TODO: the user id of the request's `Authorization: Bearer <token>` header, or `None` when
/// the header is missing, not a bearer token, or the token does not verify.
pub fn bearer_user(_headers: &HeaderMap, _secret: &str) -> Option<i64> {
    todo!("read and verify the bearer token")
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
