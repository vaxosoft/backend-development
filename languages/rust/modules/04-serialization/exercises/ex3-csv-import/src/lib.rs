//! Exercise 3: CSV import/export with a custom `with` module and validation of decoded data.
//!
//! CSV is flat: `csv` cannot write a `Vec<String>` into one cell, so tags travel
//! as `docs;p1` through the `tags_csv` module. Decoding is not validation: a row
//! can decode fine and still break the domain rules, so `from_csv` validates.

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
        let _ = (tags, serializer);
        todo!("join with ';' and serialize as one string")
    }

    pub fn deserialize<'de, D: Deserializer<'de>>(
        deserializer: D,
    ) -> Result<Vec<String>, D::Error> {
        let _ = deserializer;
        todo!("read one string and split it on ';' (empty string = no tags)")
    }
}

/// Header row first, then one line per task.
pub fn to_csv(rows: &[TaskRow]) -> Result<String, csv::Error> {
    let _ = rows;
    todo!("use csv::Writer over a Vec<u8>")
}

/// Decode every row, then validate: title 1-200 chars once trimmed; at most 10
/// tags, each 1-30 chars, lowercase, no duplicates. Report the first problem.
pub fn from_csv(input: &str) -> Result<Vec<TaskRow>, ImportError> {
    let _ = input;
    todo!("use csv::Reader::from_reader(input.as_bytes()) and validate each row")
}
