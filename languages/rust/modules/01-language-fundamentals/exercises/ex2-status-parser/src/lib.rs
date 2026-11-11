//! Exercise 2: enums, `FromStr`/`Display`, custom errors and `?`.
//!
//! Line format: `title|status|tag1,tag2` (see the tests for details).

use std::fmt;
use std::str::FromStr;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Status {
    Todo,
    InProgress,
    Done,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Task {
    pub title: String,
    pub status: Status,
    pub tags: Vec<String>,
}

#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
pub enum ParseError {
    #[error("unknown status '{0}'")]
    UnknownStatus(String),
    #[error("title must not be empty")]
    EmptyTitle,
    #[error("expected 3 fields separated by '|', got {0}")]
    FieldCount(usize),
}

/// A `ParseError` annotated with its 1-based line number.
#[derive(Debug, PartialEq, Eq, thiserror::Error)]
#[error("line {line}: {source}")]
pub struct LineError {
    pub line: usize,
    #[source]
    pub source: ParseError,
}

impl FromStr for Status {
    type Err = ParseError;

    /// Accepts exactly `todo`, `in_progress` and `done`.
    fn from_str(_text: &str) -> Result<Self, Self::Err> {
        todo!("match on the text, return ParseError::UnknownStatus otherwise")
    }
}

impl fmt::Display for Status {
    /// The inverse of `from_str`.
    fn fmt(&self, _f: &mut fmt::Formatter<'_>) -> fmt::Result {
        todo!("write the wire name")
    }
}

impl Status {
    /// Workflow order: todo -> in_progress -> done -> (nothing).
    pub fn next(self) -> Option<Status> {
        todo!("return the following status")
    }
}

/// Parses `title|status|tag1,tag2`. The title is trimmed and must not be empty;
/// tags are trimmed and empty tags are dropped.
pub fn parse_task(_line: &str) -> Result<Task, ParseError> {
    todo!("split on '|', check the field count, then use `?` on the status parse")
}

/// Parses many lines. Blank lines and lines starting with `#` are skipped but
/// still count towards line numbers. Stops at the first invalid line.
pub fn parse_all(_text: &str) -> Result<Vec<Task>, LineError> {
    todo!("enumerate lines, map errors to LineError, collect into a Result")
}
