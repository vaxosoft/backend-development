//! Exercise 2: implement an in-memory store that satisfies a shared contract.
//!
//! The contract suite lives in `tests/contract.rs` and is generic over `TaskStore`, so the same
//! checks can later run against a SQLite-backed store.

use std::collections::BTreeMap;
use std::sync::Mutex;

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Status {
    Todo,
    InProgress,
    Done,
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Task {
    pub id: u64,
    pub title: String,
    pub status: Status,
}

#[derive(Debug, PartialEq, Eq, thiserror::Error)]
pub enum StoreError {
    #[error("title must not be blank")]
    BlankTitle,
}

pub trait TaskStore: Send + Sync {
    /// New tasks start as `Todo`. Ids start at 1, grow by one and are never reused.
    /// A blank (empty or whitespace-only) title is rejected.
    fn insert(&self, title: &str) -> Result<Task, StoreError>;
    fn get(&self, id: u64) -> Option<Task>;
    /// Insertion order; `Some(status)` keeps only tasks with that status.
    fn list(&self, status: Option<Status>) -> Vec<Task>;
    /// `None` when the id does not exist.
    fn set_status(&self, id: u64, status: Status) -> Option<Task>;
    /// True when something was deleted.
    fn delete(&self, id: u64) -> bool;
}

#[derive(Default)]
pub struct MemoryStore {
    #[allow(dead_code)] // used by your implementation
    state: Mutex<State>,
}

#[derive(Default)]
#[allow(dead_code)]
struct State {
    last_id: u64,
    tasks: BTreeMap<u64, Task>,
}

impl MemoryStore {
    pub fn new() -> Self {
        Self::default()
    }
}

impl TaskStore for MemoryStore {
    fn insert(&self, _title: &str) -> Result<Task, StoreError> {
        todo!("reject blank titles, assign the next id, start as Todo")
    }

    fn get(&self, _id: u64) -> Option<Task> {
        todo!()
    }

    fn list(&self, _status: Option<Status>) -> Vec<Task> {
        todo!()
    }

    fn set_status(&self, _id: u64, _status: Status) -> Option<Task> {
        todo!()
    }

    fn delete(&self, _id: u64) -> bool {
        todo!()
    }
}
