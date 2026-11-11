//! A reusable contract suite: every `TaskStore` implementation must pass the same checks.

use std::collections::BTreeSet;
use std::sync::{Arc, Mutex};
use std::thread;

use m12_ex2_store_contract::{MemoryStore, Status, StoreError, Task, TaskStore};

fn run_contract<S: TaskStore + 'static>(make: impl Fn() -> S) {
    ids_start_at_one_and_are_never_reused(&make());
    new_tasks_are_todo_and_can_be_fetched(&make());
    blank_titles_are_rejected(&make());
    list_keeps_insertion_order_and_filters(&make());
    set_status_and_delete_report_missing_ids(&make());
    concurrent_inserts_get_unique_ids(make());
}

fn ids_start_at_one_and_are_never_reused(store: &dyn TaskStore) {
    assert_eq!(store.insert("a").unwrap().id, 1);
    assert_eq!(store.insert("b").unwrap().id, 2);
    assert!(store.delete(2));
    assert_eq!(
        store.insert("c").unwrap().id,
        3,
        "ids must not be reused after a delete"
    );
}

fn new_tasks_are_todo_and_can_be_fetched(store: &dyn TaskStore) {
    let task = store.insert("write docs").unwrap();
    assert_eq!(task.status, Status::Todo);
    assert_eq!(store.get(task.id), Some(task));
    assert_eq!(store.get(999), None);
}

fn blank_titles_are_rejected(store: &dyn TaskStore) {
    assert_eq!(store.insert(""), Err(StoreError::BlankTitle));
    assert_eq!(store.insert("  \t "), Err(StoreError::BlankTitle));
    assert!(
        store.list(None).is_empty(),
        "a rejected insert must leave no trace"
    );
    assert_eq!(
        store.insert("ok").unwrap().id,
        1,
        "a rejected insert must not consume an id"
    );
}

fn list_keeps_insertion_order_and_filters(store: &dyn TaskStore) {
    for title in ["one", "two", "three"] {
        store.insert(title).unwrap();
    }
    store.set_status(2, Status::Done).unwrap();
    store.set_status(3, Status::InProgress).unwrap();

    let titles = |tasks: Vec<Task>| tasks.into_iter().map(|t| t.title).collect::<Vec<_>>();
    assert_eq!(titles(store.list(None)), ["one", "two", "three"]);
    assert_eq!(titles(store.list(Some(Status::Todo))), ["one"]);
    assert_eq!(titles(store.list(Some(Status::Done))), ["two"]);
    assert_eq!(titles(store.list(Some(Status::InProgress))), ["three"]);
}

fn set_status_and_delete_report_missing_ids(store: &dyn TaskStore) {
    let id = store.insert("x").unwrap().id;
    let updated = store.set_status(id, Status::Done).unwrap();
    assert_eq!((updated.id, updated.status), (id, Status::Done));
    assert_eq!(store.get(id).unwrap().status, Status::Done);
    assert_eq!(store.set_status(999, Status::Done), None);

    assert!(store.delete(id));
    assert!(!store.delete(id), "second delete finds nothing");
    assert_eq!(store.get(id), None);
}

fn concurrent_inserts_get_unique_ids<S: TaskStore + 'static>(store: S) {
    let store = Arc::new(store);
    let handles: Vec<_> = (0..4)
        .map(|t| {
            let store = Arc::clone(&store);
            thread::spawn(move || {
                (0..50)
                    .map(|i| store.insert(&format!("t{t}-{i}")).unwrap().id)
                    .collect::<Vec<_>>()
            })
        })
        .collect();
    let ids: BTreeSet<u64> = handles
        .into_iter()
        .flat_map(|h| h.join().unwrap())
        .collect();
    assert_eq!(ids.len(), 200);
    assert_eq!((ids.first(), ids.last()), (Some(&1), Some(&200)));
}

#[test]
fn memory_store_satisfies_the_contract() {
    run_contract(MemoryStore::new);
}

/// A simple Vec-backed store with an optional, deliberate flaw: proves the suite can fail.
struct FlawedStore {
    state: Mutex<(u64, Vec<Task>)>, // (last id handed out, tasks)
    flaw: Flaw,
}

#[derive(Clone, Copy)]
enum Flaw {
    None,
    ReusesIds,
    IgnoresFilter,
    AcceptsBlankTitles,
}

impl FlawedStore {
    fn new(flaw: Flaw) -> Self {
        Self {
            state: Mutex::new((0, Vec::new())),
            flaw,
        }
    }
}

impl TaskStore for FlawedStore {
    fn insert(&self, title: &str) -> Result<Task, StoreError> {
        if title.trim().is_empty() && !matches!(self.flaw, Flaw::AcceptsBlankTitles) {
            return Err(StoreError::BlankTitle);
        }
        let mut state = self.state.lock().unwrap();
        let id = match self.flaw {
            Flaw::ReusesIds => state.1.iter().map(|t| t.id).max().unwrap_or(0) + 1,
            _ => state.0 + 1,
        };
        state.0 = id;
        let task = Task {
            id,
            title: title.to_string(),
            status: Status::Todo,
        };
        state.1.push(task.clone());
        Ok(task)
    }

    fn get(&self, id: u64) -> Option<Task> {
        self.state
            .lock()
            .unwrap()
            .1
            .iter()
            .find(|t| t.id == id)
            .cloned()
    }

    fn list(&self, status: Option<Status>) -> Vec<Task> {
        let state = self.state.lock().unwrap();
        match (self.flaw, status) {
            (Flaw::IgnoresFilter, _) | (_, None) => state.1.clone(),
            (_, Some(s)) => state.1.iter().filter(|t| t.status == s).cloned().collect(),
        }
    }

    fn set_status(&self, id: u64, status: Status) -> Option<Task> {
        let mut state = self.state.lock().unwrap();
        let task = state.1.iter_mut().find(|t| t.id == id)?;
        task.status = status;
        Some(task.clone())
    }

    fn delete(&self, id: u64) -> bool {
        let mut state = self.state.lock().unwrap();
        let before = state.1.len();
        state.1.retain(|t| t.id != id);
        before != state.1.len()
    }
}

#[test]
fn the_contract_accepts_a_flawless_vec_store() {
    // guards against a contract that is stricter than the documented behaviour
    run_contract(|| FlawedStore::new(Flaw::None));
}

#[test]
#[should_panic(expected = "ids must not be reused")]
fn the_contract_catches_id_reuse() {
    run_contract(|| FlawedStore::new(Flaw::ReusesIds));
}

#[test]
#[should_panic]
fn the_contract_catches_an_ignored_filter() {
    run_contract(|| FlawedStore::new(Flaw::IgnoresFilter));
}

#[test]
#[should_panic]
fn the_contract_catches_blank_titles() {
    run_contract(|| FlawedStore::new(Flaw::AcceptsBlankTitles));
}
