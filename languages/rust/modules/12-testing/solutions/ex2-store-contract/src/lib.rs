//! Exercise 2: implement an in-memory store that satisfies a shared contract.
//!
//! The contract suite lives in `tests/contract.rs` and is generic over `TaskStore`, so the same
//! checks can later run against a SQLite-backed store.

use std::collections::BTreeMap;
use std::sync::{Mutex, MutexGuard};

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
    state: Mutex<State>,
}

#[derive(Default)]
struct State {
    last_id: u64,
    tasks: BTreeMap<u64, Task>,
}

impl MemoryStore {
    pub fn new() -> Self {
        Self::default()
    }

    fn lock(&self) -> MutexGuard<'_, State> {
        // a panic in another test thread must not hide this store's own state
        self.state.lock().unwrap_or_else(|e| e.into_inner())
    }
}

impl TaskStore for MemoryStore {
    fn insert(&self, title: &str) -> Result<Task, StoreError> {
        if title.trim().is_empty() {
            return Err(StoreError::BlankTitle);
        }
        let mut state = self.lock();
        state.last_id += 1;
        let task = Task {
            id: state.last_id,
            title: title.to_string(),
            status: Status::Todo,
        };
        state.tasks.insert(task.id, task.clone());
        Ok(task)
    }

    fn get(&self, id: u64) -> Option<Task> {
        self.lock().tasks.get(&id).cloned()
    }

    fn list(&self, status: Option<Status>) -> Vec<Task> {
        self.lock()
            .tasks
            .values()
            .filter(|t| status.is_none_or(|s| t.status == s))
            .cloned()
            .collect()
    }

    fn set_status(&self, id: u64, status: Status) -> Option<Task> {
        let mut state = self.lock();
        let task = state.tasks.get_mut(&id)?;
        task.status = status;
        Some(task.clone())
    }

    fn delete(&self, id: u64) -> bool {
        self.lock().tasks.remove(&id).is_some()
    }
}
