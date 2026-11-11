//! Dates, enums, custom (de)serializers and validation while decoding.

use chrono::{DateTime, NaiveDate, Utc};
use serde::{Deserialize, Deserializer, Serialize};

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
enum Status {
    Todo,
    #[serde(alias = "doing")]
    InProgress,
    Done,
}

/// A string newtype that cannot exist unless it is valid: `try_from` runs during decoding.
#[derive(Debug, Serialize, Deserialize)]
#[serde(try_from = "String")]
struct Email(String);

impl TryFrom<String> for Email {
    type Error = String;

    fn try_from(raw: String) -> Result<Self, Self::Error> {
        if raw.contains('@') {
            Ok(Email(raw.to_lowercase()))
        } else {
            Err(format!("`{raw}` is not an email address"))
        }
    }
}

/// Normalise while decoding: trim and lowercase every tag.
fn normalized_tags<'de, D: Deserializer<'de>>(d: D) -> Result<Vec<String>, D::Error> {
    let raw = Vec::<String>::deserialize(d)?;
    Ok(raw.into_iter().map(|t| t.trim().to_lowercase()).collect())
}

#[derive(Debug, Serialize, Deserialize)]
struct Item {
    status: Status,
    due: NaiveDate,
    created_at: DateTime<Utc>,
    #[serde(with = "chrono::serde::ts_seconds")]
    updated_at: DateTime<Utc>,
    #[serde(deserialize_with = "normalized_tags")]
    tags: Vec<String>,
}

#[derive(Debug, Deserialize)]
struct Signup {
    email: Email,
}

fn main() -> Result<(), serde_json::Error> {
    // An offset timestamp is converted to UTC; `due` is a plain calendar date.
    let item: Item = serde_json::from_str(
        r#"{"status": "doing", "due": "2030-01-31", "created_at": "2030-01-01T12:00:00+03:00",
            "updated_at": 1893456000, "tags": [" Docs ", "P1"]}"#,
    )?;
    println!("{item:?}");
    println!("{}", serde_json::to_string(&item)?);

    for bad in [
        r#"{"status": "blocked", "due": "2030-01-31", "created_at": "2030-01-01T00:00:00Z", "updated_at": 0, "tags": []}"#,
        r#"{"status": "todo", "due": "2030-02-30", "created_at": "2030-01-01T00:00:00Z", "updated_at": 0, "tags": []}"#,
        r#"{"status": "todo", "due": "2030-01-31", "created_at": "01/02/2030", "updated_at": 0, "tags": []}"#,
    ] {
        println!("{}", serde_json::from_str::<Item>(bad).unwrap_err());
    }

    // Validation lives in the type: invalid input never becomes an `Email`.
    let signup: Signup = serde_json::from_str(r#"{"email": "Ada@Example.com"}"#)?;
    println!("{}", signup.email.0);
    println!(
        "{}",
        serde_json::from_str::<Signup>(r#"{"email": "ada"}"#).unwrap_err()
    );
    Ok(())
}
