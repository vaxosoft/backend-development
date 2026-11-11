//! Job types, request validation and the JSON representation.

use std::collections::BTreeMap;

use chrono::{DateTime, Utc};
use serde::Serialize;
use serde_json::Value;

pub const MAX_SLEEP_MS: u64 = 5000;

/// What a job does. Parsed from `{"type": ..., "payload": {...}}`.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum JobSpec {
    Reverse { text: String },
    Sleep { ms: u64 },
    Fail,
}

impl JobSpec {
    pub fn kind(&self) -> &'static str {
        match self {
            JobSpec::Reverse { .. } => "reverse",
            JobSpec::Sleep { .. } => "sleep",
            JobSpec::Fail => "fail",
        }
    }

    /// Validates a request body; the error maps field names to messages.
    pub fn parse(body: &Value) -> Result<JobSpec, BTreeMap<String, String>> {
        let invalid = |field: &str, msg: &str| BTreeMap::from([(field.to_owned(), msg.to_owned())]);
        let payload = body.get("payload").unwrap_or(&Value::Null);
        match body.get("type").and_then(Value::as_str) {
            Some("reverse") => match payload.get("text").and_then(Value::as_str) {
                Some(text) => Ok(JobSpec::Reverse {
                    text: text.to_owned(),
                }),
                None => Err(invalid("payload.text", "is required and must be a string")),
            },
            Some("sleep") => match payload.get("ms").and_then(Value::as_u64) {
                Some(ms) if ms <= MAX_SLEEP_MS => Ok(JobSpec::Sleep { ms }),
                Some(_) => Err(invalid("payload.ms", "must be at most 5000")),
                None => Err(invalid(
                    "payload.ms",
                    "is required and must be a non-negative integer",
                )),
            },
            Some("fail") => Ok(JobSpec::Fail),
            _ => Err(invalid("type", "must be one of: reverse, sleep, fail")),
        }
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum JobStatus {
    Queued,
    Running,
    Succeeded,
    Failed,
    Cancelled,
}

impl JobStatus {
    pub fn is_finished(self) -> bool {
        matches!(
            self,
            JobStatus::Succeeded | JobStatus::Failed | JobStatus::Cancelled
        )
    }
}

/// The JSON shape returned by `GET /jobs/{id}`.
#[derive(Debug, Clone, Serialize)]
pub struct JobView {
    pub id: String,
    #[serde(rename = "type")]
    pub kind: &'static str,
    pub status: JobStatus,
    pub attempts: u32,
    pub result: Option<String>,
    pub error: Option<String>,
    pub created_at: DateTime<Utc>,
}
