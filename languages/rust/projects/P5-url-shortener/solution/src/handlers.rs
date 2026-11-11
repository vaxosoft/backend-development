//! Route handlers.

use std::collections::BTreeMap;

use axum::Json;
use axum::extract::{Path, State};
use axum::http::{StatusCode, header};
use axum::response::{IntoResponse, Response};
use rand::RngExt;

use crate::AppState;
use crate::auth::{self, AuthUser};
use crate::error::{ApiError, ApiJson};
use crate::models::{Credentials, LinkList, LinkView, NewLink, TokenResponse, UserView};
use crate::store;

const BAD_LOGIN: &str = "invalid email or password";
const ALPHANUMERIC: &[u8] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789";

fn normalize_email(raw: &str) -> String {
    raw.trim().to_lowercase()
}

/// Argon2 is deliberately slow: run it off the async worker threads.
async fn blocking<T: Send + 'static>(
    job: impl FnOnce() -> T + Send + 'static,
) -> Result<T, ApiError> {
    tokio::task::spawn_blocking(job)
        .await
        .map_err(|e| ApiError::Internal(e.to_string()))
}

pub async fn register(
    State(state): State<AppState>,
    ApiJson(body): ApiJson<Credentials>,
) -> Result<(StatusCode, Json<UserView>), ApiError> {
    let email = normalize_email(&body.email);
    let mut fields = BTreeMap::new();
    if !email.contains('@') {
        fields.insert("email", "must contain @".to_string());
    }
    if body.password.chars().count() < 8 {
        fields.insert("password", "must be at least 8 characters".to_string());
    }
    if !fields.is_empty() {
        return Err(ApiError::Validation(fields));
    }
    let memory_kib = state.config.hash_memory_kib;
    let hash = blocking(move || auth::hash_password(&body.password, memory_kib)).await??;
    let id = store::create_user(&state.pool, &email, &hash).await?;
    Ok((StatusCode::CREATED, Json(UserView { id, email })))
}

pub async fn login(
    State(state): State<AppState>,
    ApiJson(body): ApiJson<Credentials>,
) -> Result<Json<TokenResponse>, ApiError> {
    let rejected = || ApiError::Unauthorized(BAD_LOGIN.into());
    let user = store::find_user(&state.pool, &normalize_email(&body.email))
        .await?
        .ok_or_else(rejected)?;
    let user_id = user.id;
    let matches =
        blocking(move || auth::verify_password(&body.password, &user.password_hash)).await?;
    if !matches {
        return Err(rejected());
    }
    let config = &state.config;
    let token = auth::issue_token(&config.jwt_secret, user_id, config.token_ttl_secs)?;
    Ok(Json(TokenResponse { token }))
}

/// Returns the URL in its normalised form (which is also safe to put in a header).
fn validate_url(raw: &str) -> Result<String, ApiError> {
    reqwest::Url::parse(raw.trim())
        .ok()
        .filter(|url| matches!(url.scheme(), "http" | "https") && url.host_str().is_some())
        .filter(|url| url.as_str().len() <= 2048)
        .map(String::from)
        .ok_or_else(|| ApiError::invalid("url", "must be an absolute http or https URL"))
}

fn validate_slug(slug: &str) -> Result<(), ApiError> {
    let chars_ok = slug
        .bytes()
        .all(|b| b.is_ascii_alphanumeric() || b == b'-' || b == b'_');
    // "links" would be shadowed by the GET /links route.
    if (3..=32).contains(&slug.len()) && chars_ok && slug != "links" {
        Ok(())
    } else {
        Err(ApiError::invalid(
            "slug",
            "must be 3-32 characters of A-Z a-z 0-9 _ - and not `links`",
        ))
    }
}

fn generate_slug() -> String {
    let mut rng = rand::rng();
    (0..7)
        .map(|_| char::from(ALPHANUMERIC[rng.random_range(0..ALPHANUMERIC.len())]))
        .collect()
}

pub async fn create_link(
    State(state): State<AppState>,
    user: AuthUser,
    ApiJson(body): ApiJson<NewLink>,
) -> Result<(StatusCode, Json<LinkView>), ApiError> {
    if let Err(wait) = state.limiter.check(user.id) {
        let retry_after_secs = wait.as_secs_f64().ceil().max(1.0) as u64;
        return Err(ApiError::RateLimited { retry_after_secs });
    }
    let url = validate_url(&body.url)?;
    let slug = match body.slug {
        Some(slug) => {
            validate_slug(&slug)?;
            store::insert_link(&state.pool, &slug, &url, user.id).await?;
            slug
        }
        None => insert_with_generated_slug(&state, &url, user.id).await?,
    };
    Ok((StatusCode::CREATED, Json(LinkView { slug, url })))
}

async fn insert_with_generated_slug(
    state: &AppState,
    url: &str,
    owner_id: i64,
) -> Result<String, ApiError> {
    for _ in 0..5 {
        let slug = generate_slug();
        match store::insert_link(&state.pool, &slug, url, owner_id).await {
            Ok(()) => return Ok(slug),
            Err(ApiError::Conflict(_)) => continue,
            Err(other) => return Err(other),
        }
    }
    Err(ApiError::Internal("could not generate a free slug".into()))
}

pub async fn redirect(
    State(state): State<AppState>,
    Path(slug): Path<String>,
) -> Result<Response, ApiError> {
    let url = store::record_hit(&state.pool, &slug)
        .await?
        .ok_or(ApiError::NotFound)?;
    // `Redirect::temporary` would send 307; the spec asks for a classic 302.
    Ok((StatusCode::FOUND, [(header::LOCATION, url)]).into_response())
}

pub async fn list_links(
    State(state): State<AppState>,
    user: AuthUser,
) -> Result<Json<LinkList>, ApiError> {
    let items = store::links_of(&state.pool, user.id).await?;
    Ok(Json(LinkList { items }))
}

pub async fn delete_link(
    State(state): State<AppState>,
    user: AuthUser,
    Path(slug): Path<String>,
) -> Result<StatusCode, ApiError> {
    match store::link_owner(&state.pool, &slug).await? {
        None => Err(ApiError::NotFound),
        Some(owner) if owner != user.id => Err(ApiError::Forbidden("not your link".into())),
        Some(_) => {
            store::delete_link(&state.pool, &slug).await?;
            Ok(StatusCode::NO_CONTENT)
        }
    }
}
