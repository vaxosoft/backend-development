//! Exercise 3: secrets from env vars or `*_FILE` files, never printed.
use std::io;

use secrecy::SecretString;

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
    _lookup: impl Fn(&str) -> Option<String>,
    _name: &str,
) -> Result<Option<SecretString>, SecretError> {
    todo!("env value, or file contents, or nothing")
}

#[derive(Debug)]
pub struct DbConfig {
    pub host: String,
    pub name: String,
    pub user: String,
    #[allow(dead_code)] // read by your implementation
    password: SecretString,
}

impl DbConfig {
    /// `DB_HOST` (default `localhost`), `DB_NAME` (required), `DB_USER` (required),
    /// `DB_PASSWORD` via [`load_secret`] (required, at least 8 characters).
    /// Errors must not contain the password.
    pub fn from_lookup(_lookup: impl Fn(&str) -> Option<String>) -> Result<Self, DbConfigError> {
        todo!("read the plain values, load the secret, validate it")
    }

    /// `postgres://user:password@host/name`. The result is a secret itself, because it contains one.
    pub fn connection_string(&self) -> SecretString {
        todo!("expose the password only inside this function")
    }

    /// The same URL for logs: `postgres://user:***@host/name`.
    pub fn redacted_url(&self) -> String {
        todo!("never touch the password")
    }
}
