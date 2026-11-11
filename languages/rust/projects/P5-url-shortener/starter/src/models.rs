//! Request and response bodies (provided).

use serde::{Deserialize, Serialize};

#[derive(Debug, Deserialize)]
pub struct Credentials {
    pub email: String,
    pub password: String,
}

#[derive(Debug, Serialize)]
pub struct UserView {
    pub id: i64,
    pub email: String,
}

#[derive(Debug, Serialize)]
pub struct TokenResponse {
    pub token: String,
}

#[derive(Debug, Deserialize)]
pub struct NewLink {
    pub url: String,
    pub slug: Option<String>,
}

#[derive(Debug, Serialize)]
pub struct LinkView {
    pub slug: String,
    pub url: String,
}

#[derive(Debug, Serialize, sqlx::FromRow)]
pub struct LinkStats {
    pub slug: String,
    pub url: String,
    pub hits: i64,
}

#[derive(Debug, Serialize)]
pub struct LinkList {
    pub items: Vec<LinkStats>,
}

#[derive(Debug, sqlx::FromRow)]
pub struct User {
    pub id: i64,
    pub email: String,
    pub password_hash: String,
}
