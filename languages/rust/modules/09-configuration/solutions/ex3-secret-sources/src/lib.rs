//! Exercise 3: secrets from env vars or `*_FILE` files, never printed.
use std::io;

use secrecy::{ExposeSecret, SecretString};

#[derive(Debug, PartialEq, Eq, thiserror::Error)]
pub enum SecretError {
    #[error("{name} and {name}_FILE are both set; use only one")]
    Conflict { name: String },
    #[error("{name}_FILE: cannot read {path}: {kind}")]
    Unreadable {
        name: String,
        path: String,
        kind: io::ErrorKind,
    },
    #[error("{name} is empty")]
    Empty { name: String },
}

#[derive(Debug, PartialEq, Eq, thiserror::Error)]
pub enum DbConfigError {
    #[error(transparent)]
    Secret(#[from] SecretError),
    #[error("{0} is required")]
    Missing(&'static str),
    #[error("DB_PASSWORD must have at least {min} characters")]
    WeakPassword { min: usize },
}

/// Docker/Kubernetes style secret loading.
///
/// - `NAME` set: its value is the secret (unchanged)
/// - `NAME_FILE` set: read that file; remove ONE trailing `\n` or `\r\n` (editors and
///   `echo` add one) but keep every other character
/// - both set: [`SecretError::Conflict`]; neither set: `Ok(None)`
/// - the resulting secret is empty: [`SecretError::Empty`]
/// - the file cannot be read: [`SecretError::Unreadable`] (never include secret content in errors)
pub fn load_secret(
    lookup: impl Fn(&str) -> Option<String>,
    name: &str,
) -> Result<Option<SecretString>, SecretError> {
    let file_key = format!("{name}_FILE");
    let value = match (lookup(name), lookup(&file_key)) {
        (None, None) => return Ok(None),
        (Some(_), Some(_)) => {
            return Err(SecretError::Conflict {
                name: name.to_owned(),
            });
        }
        (Some(value), None) => value,
        (None, Some(path)) => {
            let content = std::fs::read_to_string(&path).map_err(|e| SecretError::Unreadable {
                name: name.to_owned(),
                path: path.clone(),
                kind: e.kind(),
            })?;
            let content = content.strip_suffix('\n').unwrap_or(&content);
            content.strip_suffix('\r').unwrap_or(content).to_owned()
        }
    };
    if value.is_empty() {
        return Err(SecretError::Empty {
            name: name.to_owned(),
        });
    }
    Ok(Some(SecretString::from(value)))
}

#[derive(Debug)]
pub struct DbConfig {
    pub host: String,
    pub name: String,
    pub user: String,
    password: SecretString,
}

impl DbConfig {
    /// `DB_HOST` (default `localhost`), `DB_NAME` (required), `DB_USER` (required),
    /// `DB_PASSWORD` via [`load_secret`] (required, at least 8 characters).
    /// Errors must not contain the password.
    pub fn from_lookup(lookup: impl Fn(&str) -> Option<String>) -> Result<Self, DbConfigError> {
        let name = lookup("DB_NAME").ok_or(DbConfigError::Missing("DB_NAME"))?;
        let user = lookup("DB_USER").ok_or(DbConfigError::Missing("DB_USER"))?;
        let host = lookup("DB_HOST").unwrap_or_else(|| "localhost".to_owned());
        let password =
            load_secret(&lookup, "DB_PASSWORD")?.ok_or(DbConfigError::Missing("DB_PASSWORD"))?;
        let min = 8;
        if password.expose_secret().chars().count() < min {
            return Err(DbConfigError::WeakPassword { min });
        }
        Ok(Self {
            host,
            name,
            user,
            password,
        })
    }

    /// `postgres://user:password@host/name`. The result is a secret itself, because it contains one.
    pub fn connection_string(&self) -> SecretString {
        SecretString::from(format!(
            "postgres://{}:{}@{}/{}",
            self.user,
            self.password.expose_secret(),
            self.host,
            self.name
        ))
    }

    /// The same URL for logs: `postgres://user:***@host/name`.
    pub fn redacted_url(&self) -> String {
        format!("postgres://{}:***@{}/{}", self.user, self.host, self.name)
    }
}
