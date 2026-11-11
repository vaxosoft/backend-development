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
    fn from_str(text: &str) -> Result<Self, Self::Err> {
        match text {
            "todo" => Ok(Status::Todo),
            "in_progress" => Ok(Status::InProgress),
            "done" => Ok(Status::Done),
            other => Err(ParseError::UnknownStatus(other.to_string())),
        }
    }
}

impl fmt::Display for Status {
    /// The inverse of `from_str`.
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(match self {
            Status::Todo => "todo",
            Status::InProgress => "in_progress",
            Status::Done => "done",
        })
    }
}

impl Status {
    /// Workflow order: todo -> in_progress -> done -> (nothing).
    pub fn next(self) -> Option<Status> {
        match self {
            Status::Todo => Some(Status::InProgress),
            Status::InProgress => Some(Status::Done),
            Status::Done => None,
        }
    }
}

/// Parses `title|status|tag1,tag2`. The title is trimmed and must not be empty;
/// tags are trimmed and empty tags are dropped.
pub fn parse_task(line: &str) -> Result<Task, ParseError> {
    let fields: Vec<&str> = line.split('|').collect();
    let [title, status, tags] = fields[..] else {
        return Err(ParseError::FieldCount(fields.len()));
    };
    let title = title.trim();
    if title.is_empty() {
        return Err(ParseError::EmptyTitle);
    }
    let status = status.trim().parse()?;
    let tags = tags
        .split(',')
        .map(str::trim)
        .filter(|t| !t.is_empty())
        .map(String::from)
        .collect();
    Ok(Task {
        title: title.to_string(),
        status,
        tags,
    })
}

/// Parses many lines. Blank lines and lines starting with `#` are skipped but
/// still count towards line numbers. Stops at the first invalid line.
pub fn parse_all(text: &str) -> Result<Vec<Task>, LineError> {
    text.lines()
        .enumerate()
        .filter(|(_, line)| !line.trim().is_empty() && !line.starts_with('#'))
        .map(|(index, line)| {
            parse_task(line).map_err(|source| LineError {
                line: index + 1,
                source,
            })
        })
        .collect()
}
