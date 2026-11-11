//! Exercise 2: request validation, including PATCH semantics.
//!
//! A **user** has a `name` (trimmed, 1-100 characters) and an `email` (trimmed;
//! `local@domain` with both parts non-empty and exactly one `@`).
//! Every violated rule is reported at once, keyed by field name.

use std::collections::BTreeMap;

use serde::{Deserialize, Deserializer};

pub type FieldErrors = BTreeMap<String, String>;

#[derive(Debug, Deserialize)]
pub struct UserInput {
    pub name: Option<String>,
    pub email: Option<String>,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct User {
    pub name: String,
    pub email: String,
}

/// Body of `PATCH /users/{id}`. For each field the three JSON states map to:
/// absent -> `None`, `null` -> `Some(None)`, `"text"` -> `Some(Some("text"))`.
#[derive(Debug, Deserialize)]
pub struct UserPatchInput {
    #[serde(default, deserialize_with = "present")]
    pub name: Option<Option<String>>,
    #[serde(default, deserialize_with = "present")]
    pub email: Option<Option<String>>,
}

fn present<'de, D>(deserializer: D) -> Result<Option<Option<String>>, D::Error>
where
    D: Deserializer<'de>,
{
    Option::<String>::deserialize(deserializer).map(Some)
}

/// A validated patch: `None` means "leave unchanged".
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct UserPatch {
    pub name: Option<String>,
    pub email: Option<String>,
}

/// Create/replace validation.
///
/// Messages: a missing field -> `is required`; a bad name -> `must be 1-100
/// characters`; a bad email -> `must be an email address`.
pub fn validate_user(_input: UserInput) -> Result<User, FieldErrors> {
    todo!("check name and email, collect problems in a FieldErrors map, return the trimmed User")
}

/// PATCH validation: only fields that are present are checked.
///
/// * An explicit `null` is an error for both fields: `must not be null`.
/// * Present values follow the same rules (and messages) as `validate_user`.
/// * A patch that changes nothing (both absent) is an error under the key
///   `body`: `at least one field is required`.
pub fn validate_user_patch(_input: UserPatchInput) -> Result<UserPatch, FieldErrors> {
    todo!("treat None / Some(None) / Some(Some(v)) differently")
}

/// Applies a validated patch, leaving absent fields untouched.
pub fn apply_patch(_user: &User, _patch: &UserPatch) -> User {
    todo!("take the patched value or keep the old one")
}
