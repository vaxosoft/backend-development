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

fn check_name(raw: &str) -> Result<String, &'static str> {
    let name = raw.trim();
    if (1..=100).contains(&name.chars().count()) {
        Ok(name.to_string())
    } else {
        Err("must be 1-100 characters")
    }
}

fn check_email(raw: &str) -> Result<String, &'static str> {
    let email = raw.trim();
    match email.split_once('@') {
        Some((local, domain))
            if !local.is_empty() && !domain.is_empty() && !domain.contains('@') =>
        {
            Ok(email.to_string())
        }
        _ => Err("must be an email address"),
    }
}

fn record<T>(fields: &mut FieldErrors, field: &str, result: Result<T, &'static str>) -> Option<T> {
    result
        .map_err(|problem| {
            fields.insert(field.to_string(), problem.to_string());
        })
        .ok()
}

/// Create/replace validation.
///
/// Messages: a missing field -> `is required`; a bad name -> `must be 1-100
/// characters`; a bad email -> `must be an email address`.
pub fn validate_user(input: UserInput) -> Result<User, FieldErrors> {
    let mut fields = FieldErrors::new();
    let name = record(
        &mut fields,
        "name",
        input
            .name
            .as_deref()
            .ok_or("is required")
            .and_then(check_name),
    );
    let email = record(
        &mut fields,
        "email",
        input
            .email
            .as_deref()
            .ok_or("is required")
            .and_then(check_email),
    );
    match (name, email) {
        (Some(name), Some(email)) => Ok(User { name, email }),
        _ => Err(fields),
    }
}

/// PATCH validation: only fields that are present are checked.
///
/// * An explicit `null` is an error for both fields: `must not be null`.
/// * Present values follow the same rules (and messages) as `validate_user`.
/// * A patch that changes nothing (both absent) is an error under the key
///   `body`: `at least one field is required`.
pub fn validate_user_patch(input: UserPatchInput) -> Result<UserPatch, FieldErrors> {
    let mut fields = FieldErrors::new();
    if input.name.is_none() && input.email.is_none() {
        fields.insert(
            "body".to_string(),
            "at least one field is required".to_string(),
        );
        return Err(fields);
    }
    let name = input.name.and_then(|v| {
        record(
            &mut fields,
            "name",
            v.as_deref().ok_or("must not be null").and_then(check_name),
        )
    });
    let email = input.email.and_then(|v| {
        record(
            &mut fields,
            "email",
            v.as_deref().ok_or("must not be null").and_then(check_email),
        )
    });
    if fields.is_empty() {
        Ok(UserPatch { name, email })
    } else {
        Err(fields)
    }
}

/// Applies a validated patch, leaving absent fields untouched.
pub fn apply_patch(user: &User, patch: &UserPatch) -> User {
    User {
        name: patch.name.clone().unwrap_or_else(|| user.name.clone()),
        email: patch.email.clone().unwrap_or_else(|| user.email.clone()),
    }
}
