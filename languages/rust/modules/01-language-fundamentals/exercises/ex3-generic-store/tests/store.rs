use m01_ex3_generic_store::{Entity, Store};

// Deliberately not `Clone`: the store must work with any `Entity`.
#[derive(Debug, PartialEq)]
struct Task {
    id: u32,
    title: &'static str,
    done: bool,
}

impl Entity for Task {
    fn id(&self) -> u32 {
        self.id
    }
}

fn task(id: u32, title: &'static str, done: bool) -> Task {
    Task { id, title, done }
}

#[test]
fn insert_get_and_replace() {
    let mut store = Store::new();
    assert!(store.insert(task(1, "a", false)).is_none());
    let old = store
        .insert(task(1, "b", true))
        .expect("replaced item is returned");
    assert_eq!(old.title, "a");
    assert_eq!(store.len(), 1);
    assert_eq!(store.get(1).map(|t| t.title), Some("b"));
    assert!(store.get(2).is_none());
}

#[test]
fn remove_returns_the_owned_item() {
    let mut store: Store<Task> = Store::new();
    store.insert(task(7, "x", false));
    assert_eq!(store.remove(7), Some(task(7, "x", false)));
    assert_eq!(store.remove(7), None);
    assert!(store.is_empty());
}

#[test]
fn update_mutates_in_place_through_a_closure() {
    let mut store = Store::new();
    store.insert(task(1, "a", false));
    assert!(store.update(1, |t| t.done = true));
    assert!(store.get(1).unwrap().done);
    assert!(
        !store.update(2, |t| t.done = true),
        "unknown id reports false"
    );
}

#[test]
fn filter_is_lazy_ordered_and_borrows() {
    let store: Store<Task> = [task(3, "c", true), task(1, "a", true), task(2, "b", false)]
        .into_iter()
        .collect();

    let done: Vec<u32> = store.filter(|t| t.done).map(|t| t.id).collect();
    assert_eq!(done, [1, 3]);

    let min_id = 2;
    let from: Vec<&str> = store
        .filter(move |t| t.id >= min_id)
        .map(|t| t.title)
        .collect();
    assert_eq!(from, ["b", "c"]);
}

#[test]
fn next_id_is_one_past_the_largest() {
    let mut store: Store<Task> = Store::new();
    assert_eq!(store.next_id(), 1);
    store.insert(task(5, "a", false));
    store.insert(task(2, "b", false));
    assert_eq!(store.next_id(), 6);
    store.remove(5);
    assert_eq!(store.next_id(), 3);
}

#[test]
fn collecting_deduplicates_by_id_keeping_the_last() {
    let store: Store<Task> = vec![task(1, "first", false), task(1, "second", false)]
        .into_iter()
        .collect();
    assert_eq!(store.len(), 1);
    assert_eq!(store.get(1).unwrap().title, "second");
}
