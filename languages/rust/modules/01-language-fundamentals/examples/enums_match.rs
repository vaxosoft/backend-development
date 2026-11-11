//! Enums with data, `match`, `Option` and `Result`.

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum Status {
    Todo,
    InProgress,
    Done,
}

impl Status {
    const fn as_str(self) -> &'static str {
        match self {
            Status::Todo => "todo",
            Status::InProgress => "in_progress",
            Status::Done => "done",
        }
    }
}

#[derive(Debug)]
enum Event {
    Created { title: String },
    Moved { from: Status, to: Status },
    Tagged(Vec<String>),
    Deleted,
}

fn describe(event: &Event) -> String {
    // `match` must be exhaustive: adding a variant breaks the build until handled.
    match event {
        Event::Created { title } => format!("created '{title}'"),
        Event::Moved { from, to } if from == to => "no-op move".to_string(),
        Event::Moved { from, to } => format!("moved {} -> {}", from.as_str(), to.as_str()),
        Event::Tagged(tags) if tags.is_empty() => "tags cleared".to_string(),
        Event::Tagged(tags) => format!("tagged {}", tags.join(",")),
        Event::Deleted => "deleted".to_string(),
    }
}

fn parse_status(text: &str) -> Option<Status> {
    match text {
        "todo" => Some(Status::Todo),
        "in_progress" => Some(Status::InProgress),
        "done" => Some(Status::Done),
        _ => None,
    }
}

fn parse_id(text: &str) -> Result<u32, std::num::ParseIntError> {
    text.trim().parse::<u32>()
}

fn main() {
    let events = [
        Event::Created {
            title: "Write docs".into(),
        },
        Event::Moved {
            from: Status::Todo,
            to: Status::InProgress,
        },
        Event::Moved {
            from: Status::Done,
            to: Status::Done,
        },
        Event::Tagged(vec!["docs".into(), "p1".into()]),
        Event::Tagged(vec![]),
        Event::Deleted,
    ];
    for event in &events {
        println!("{}", describe(event));
    }

    // Option: absence is a value, not null.
    for text in ["done", "finished"] {
        match parse_status(text) {
            Some(status) => println!("{text} -> {status:?}"),
            None => println!("{text} -> unknown"),
        }
    }
    let status = parse_status("finished").unwrap_or(Status::Todo);
    println!("default: {}", status.as_str());

    // `if let` / `let ... else` when only one arm matters.
    if let Some(Status::Done) = parse_status("done") {
        println!("finished task");
    }
    let Some(status) = parse_status("in_progress") else {
        return;
    };
    println!("let-else: {status:?}");

    // Result: failure is a value too.
    for text in ["42", " 7 ", "abc", "-1"] {
        match parse_id(text) {
            Ok(id) => println!("{text:?} -> id {id}"),
            Err(err) => println!("{text:?} -> error: {err}"),
        }
    }
}
