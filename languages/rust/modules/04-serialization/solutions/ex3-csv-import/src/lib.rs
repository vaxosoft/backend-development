//! Exercise 3: CSV import/export with a custom `with` module and validation of decoded data.
//!
//! CSV is flat: `csv` cannot write a `Vec<String>` into one cell, so tags travel
//! as `docs;p1` through the `tags_csv` module. Decoding is not validation: a row
//! can decode fine and still break the domain rules, so `from_csv` validates.

use std::collections::HashSet;

use chrono::NaiveDate;
use serde::{Deserialize, Deserializer, Serialize, Serializer};

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum Status {
    Todo,
    InProgress,
    Done,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct TaskRow {
    pub id: i64,
    pub title: String,
    pub status: Status,
    pub due_date: Option<NaiveDate>,
    #[serde(with = "tags_csv")]
    pub tags: Vec<String>,
    pub owner_id: Option<i64>,
}

#[derive(Debug, thiserror::Error)]
pub enum ImportError {
    #[error("malformed csv: {0}")]
    Csv(#[from] csv::Error),
    /// `row` is 1-based and counts data rows only (the header is row 0).
    #[error("row {row}: field `{field}` {message}")]
    Invalid {
        row: usize,
        field: &'static str,
        message: String,
    },
}

/// `["docs", "p1"]` <-> `docs;p1`, `[]` <-> the empty string.
pub mod tags_csv {
    use super::*;

    pub fn serialize<S: Serializer>(tags: &[String], serializer: S) -> Result<S::Ok, S::Error> {
        serializer.serialize_str(&tags.join(";"))
    }

    pub fn deserialize<'de, D: Deserializer<'de>>(
        deserializer: D,
    ) -> Result<Vec<String>, D::Error> {
        let joined = String::deserialize(deserializer)?;
        if joined.is_empty() {
            return Ok(Vec::new());
        }
        Ok(joined.split(';').map(str::to_owned).collect())
    }
}

/// Header row first, then one line per task.
pub fn to_csv(rows: &[TaskRow]) -> Result<String, csv::Error> {
    let mut writer = csv::Writer::from_writer(Vec::new());
    for row in rows {
        writer.serialize(row)?;
    }
    let bytes = writer.into_inner().map_err(|e| e.into_error())?;
    Ok(String::from_utf8(bytes).expect("csv output of valid UTF-8 strings is UTF-8"))
}

/// Decode every row, then validate: title 1-200 chars once trimmed; at most 10
/// tags, each 1-30 chars, lowercase, no duplicates. Report the first problem.
pub fn from_csv(input: &str) -> Result<Vec<TaskRow>, ImportError> {
    let mut reader = csv::Reader::from_reader(input.as_bytes());
    let mut rows = Vec::new();
    for (index, record) in reader.deserialize::<TaskRow>().enumerate() {
        let row = record?;
        validate(&row, index + 1)?;
        rows.push(row);
    }
    Ok(rows)
}

fn validate(row: &TaskRow, number: usize) -> Result<(), ImportError> {
    let invalid = |field, message: &str| ImportError::Invalid {
        row: number,
        field,
        message: message.to_owned(),
    };
    let title_len = row.title.trim().chars().count();
    if !(1..=200).contains(&title_len) {
        return Err(invalid("title", "must be 1-200 characters"));
    }
    if row.tags.len() > 10 {
        return Err(invalid("tags", "must contain at most 10 tags"));
    }
    let mut seen = HashSet::new();
    for tag in &row.tags {
        if !(1..=30).contains(&tag.chars().count()) {
            return Err(invalid("tags", "each tag must be 1-30 characters"));
        }
        if *tag != tag.to_lowercase() {
            return Err(invalid("tags", "tags must be lowercase"));
        }
        if !seen.insert(tag) {
            return Err(invalid("tags", "tags must be unique"));
        }
    }
    Ok(())
}
