//! Ownership, moves, borrowing and `String` vs `&str`.

fn consume(title: String) -> usize {
    title.len()
}

fn shout(title: &str) -> String {
    title.to_uppercase()
}

fn append_suffix(title: &mut String) {
    title.push_str(" (draft)");
}

// Elision gives the result the lifetime of `text`: no copy, no allocation.
fn first_word(text: &str) -> &str {
    text.split_whitespace().next().unwrap_or("")
}

fn main() {
    let title = String::from("Write docs");

    // Borrowing: the caller keeps ownership.
    println!("shout: {}", shout(&title));
    println!("first word: {}", first_word(&title));

    // Mutable borrow: exactly one `&mut` at a time, and no shared borrows alongside it.
    let mut draft = title.clone();
    append_suffix(&mut draft);
    println!("draft: {draft}");

    // Move: `consume` takes ownership, so `title` can no longer be used here.
    let len = consume(title);
    println!("consumed {len} bytes");
    // println!("{title}"); // error[E0382]: borrow of moved value: `title`

    // Copy types (integers, bool, char, &T) are duplicated instead of moved.
    let a = 5;
    let b = a;
    println!("a={a} b={b}");

    // `&str` can point into a String, a literal, or a slice of either.
    let owned = String::from("in_progress");
    let literal: &str = "done";
    let slice: &str = &owned[..2];
    println!("{owned} / {literal} / {slice}");

    // Moving into a closure that outlives this scope needs `move`.
    let prefix = String::from("task");
    let label = move |id: u32| format!("{prefix}-{id}");
    println!("{}", label(7));
}
