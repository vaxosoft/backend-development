//! Traits, generics, trait objects and composition instead of inheritance.

use std::fmt;

trait Notifier {
    fn name(&self) -> &str;
    fn notify(&self, message: &str) -> String;

    // Default method: shared behaviour without a base class.
    fn notify_all(&self, messages: &[&str]) -> Vec<String> {
        messages.iter().map(|m| self.notify(m)).collect()
    }
}

struct Console;
struct Email {
    to: String,
}

impl Notifier for Console {
    fn name(&self) -> &str {
        "console"
    }
    fn notify(&self, message: &str) -> String {
        format!("[console] {message}")
    }
}

impl Notifier for Email {
    fn name(&self) -> &str {
        &self.to
    }
    fn notify(&self, message: &str) -> String {
        format!("[mail to {}] {message}", self.to)
    }
}

// Static dispatch: monomorphised per concrete type, no vtable.
fn send_static<N: Notifier>(notifier: &N, message: &str) -> String {
    notifier.notify(message)
}

// Dynamic dispatch: one compiled function, a vtable call per use.
fn send_all(notifiers: &[Box<dyn Notifier>], message: &str) {
    for notifier in notifiers {
        println!("{:>14}: {}", notifier.name(), notifier.notify(message));
    }
}

// A generic function with a trait bound and `where` clause.
fn largest<T>(items: &[T]) -> Option<&T>
where
    T: PartialOrd,
{
    let mut iter = items.iter();
    let first = iter.next()?;
    Some(iter.fold(first, |best, item| if item > best { item } else { best }))
}

// Composition: a Task *has* a notifier; there is no inheritance.
struct Task<N> {
    title: String,
    notifier: N,
}

impl<N: Notifier> Task<N> {
    fn complete(&self) -> String {
        self.notifier.notify(&format!("'{}' completed", self.title))
    }
}

struct Priority(u8);

impl fmt::Display for Priority {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "P{}", self.0)
    }
}

fn main() {
    println!("{}", send_static(&Console, "static dispatch"));

    let notifiers: Vec<Box<dyn Notifier>> = vec![
        Box::new(Console),
        Box::new(Email {
            to: "ada@example.com".into(),
        }),
    ];
    send_all(&notifiers, "dynamic dispatch");
    println!("{:?}", Console.notify_all(&["a", "b"]));

    println!("{:?}", largest(&[3, 9, 4]));
    println!("{:?}", largest(&["pear", "apple"]));
    println!("{:?}", largest::<f64>(&[]));

    let task = Task {
        title: "Write docs".into(),
        notifier: Email {
            to: "ada@example.com".into(),
        },
    };
    println!("{}", task.complete());
    println!("{} {}", Priority(1), Priority(3));
}
