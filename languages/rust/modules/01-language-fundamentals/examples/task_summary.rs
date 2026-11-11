//! Task Tracker domain: structs, modules and visibility, collections,
//! iterators, closures, and a first look at `async fn` (no runtime needed).

use std::collections::{BTreeMap, BTreeSet, HashMap};

mod tracker {
    use std::fmt;

    #[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, PartialOrd, Ord)]
    pub enum Status {
        Todo,
        InProgress,
        Done,
    }

    impl fmt::Display for Status {
        fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
            // `pad` (unlike `write_str`) honours width/alignment such as `{:<12}`.
            f.pad(match self {
                Status::Todo => "todo",
                Status::InProgress => "in_progress",
                Status::Done => "done",
            })
        }
    }

    #[derive(Debug, Clone)]
    pub struct Task {
        pub id: u32,
        pub title: String,
        pub status: Status,
        pub tags: Vec<String>,
        // Private field: only this module can construct a Task via `Task::new`.
        due_day: Option<u32>,
    }

    impl Task {
        pub fn new(
            id: u32,
            title: &str,
            status: Status,
            tags: &[&str],
            due_day: Option<u32>,
        ) -> Self {
            Task {
                id,
                title: title.trim().to_string(),
                status,
                tags: tags.iter().map(|t| t.to_string()).collect(),
                due_day,
            }
        }

        pub fn is_overdue(&self, today: u32) -> bool {
            self.status != Status::Done && self.due_day.is_some_and(|due| due < today)
        }
    }
}

use tracker::{Status, Task};

fn seed() -> Vec<Task> {
    vec![
        Task::new(1, "Write docs", Status::Todo, &["docs", "p1"], Some(10)),
        Task::new(
            2,
            "Fix login bug",
            Status::InProgress,
            &["bug", "p1"],
            Some(3),
        ),
        Task::new(3, "  Release v1  ", Status::Done, &["release"], Some(2)),
        Task::new(4, "Refactor router", Status::Todo, &["tech-debt"], None),
        Task::new(5, "Update deps", Status::Todo, &["chore", "p1"], Some(1)),
    ]
}

async fn double_async(x: u32) -> u32 {
    x * 2
}

fn main() {
    let tasks = seed();

    // Iterator chain: lazy until `collect`. Closures borrow `today` from the scope.
    let today = 5;
    let overdue: Vec<&str> = tasks
        .iter()
        .filter(|t| t.is_overdue(today))
        .map(|t| t.title.as_str())
        .collect();
    println!("overdue: {overdue:?}");

    // BTreeMap keeps keys sorted, so the output order is deterministic.
    let mut by_status: BTreeMap<Status, Vec<u32>> = BTreeMap::new();
    for task in &tasks {
        by_status.entry(task.status).or_default().push(task.id);
    }
    for (status, ids) in &by_status {
        println!("{status:<12} {ids:?}");
    }

    let tags: BTreeSet<&str> = tasks
        .iter()
        .flat_map(|t| t.tags.iter().map(String::as_str))
        .collect();
    println!("tags: {tags:?}");

    // Ownership in collections: `into_iter` consumes, `iter` borrows.
    let titles: HashMap<u32, String> = tasks.into_iter().map(|t| (t.id, t.title)).collect();
    let mut ids: Vec<_> = titles.keys().copied().collect();
    ids.sort_unstable();
    println!("first title: {:?}", titles.get(&ids[0]));
    println!("missing: {:?}", titles.get(&99));

    // `async fn` returns a lazy future; nothing runs until it is polled.
    // `block_on` is a minimal executor; servers use tokio (module 02).
    let future = double_async(21);
    println!("async result: {}", futures::executor::block_on(future));

    let total: u32 = (1..=5).filter(|n| n % 2 == 1).map(|n| n * n).sum();
    println!("sum of odd squares: {total}");
}
