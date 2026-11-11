# 01. Language Fundamentals in Rust

> Estimated time: 2-3h | Difficulty: Beginner (the longest module: it front-loads ownership) | Prerequisites: none (Rust installed via rustup)

## Learning Outcomes
After this module you can:
- [ ] Explain move, borrow and clone, and pick `String` or `&str` for a parameter
- [ ] Model data with structs and enums and consume them exhaustively with `match`
- [ ] Return `Result`/`Option` and propagate failures with `?` and a `thiserror` enum
- [ ] Write generic functions and traits, and choose static (`T: Trait`) or dynamic (`dyn Trait`) dispatch
- [ ] Use iterators, closures and the standard collections instead of index loops
- [ ] Organise code in modules with visibility, and use `cargo`, `rustfmt` and `clippy`

## 1. Concept Overview
Every backend language answers the same questions: what are the types, how do values flow between functions, how do failures travel, and who frees memory. Rust answers the last one at compile time: every value has exactly one owner, and the compiler checks that borrows never outlive it. There is no garbage collector and no null; absence is `Option<T>` and failure is `Result<T, E>`.

The payoff for a server is predictable latency and the removal of whole bug classes (use-after-free, data races, null dereferences). The cost is that you must learn to think in ownership first. Everything else in this track (axum handlers, shared state, async tasks) builds on the four ideas in this module: **ownership, enums, traits, iterators**.

```
String (owner) --move--> consume(String)     the old binding is dead
      |--&--> shout(&str)                    many shared borrows, or
      |--&mut--> append(&mut String)         exactly one mutable borrow
```

## 2. How Rust Approaches It
| Need | Rust tool |
|------|-----------|
| Memory safety without GC | ownership, borrowing, lifetimes (mostly inferred, "elided") |
| Absence / failure | `Option<T>`, `Result<T, E>`, `?` operator; no exceptions, no null |
| Data modelling | `struct`, `enum` with data, `match` (exhaustive), `impl` blocks |
| Polymorphism | traits + generics (static) or `Box<dyn Trait>` (dynamic); composition, no inheritance |
| Collections | `Vec`, `HashMap`, `BTreeMap`, `HashSet`; iterator adapters (`map`, `filter`, `collect`) |
| Errors in libraries | custom `enum` + `thiserror`; binaries may use `anyhow` |
| Tooling | `cargo build/run/test`, `cargo fmt`, `cargo clippy -- -D warnings`, `rust-gdb`/`rust-lldb` |

Idiomatic: borrow (`&str`, `&[T]`) in parameters, return owned values, `?` instead of `match` on every `Result`, iterator chains instead of manual index loops. Avoid: `.clone()` to silence the borrow checker, `.unwrap()` outside tests and proven invariants, `Box<dyn Trait>` when a generic would do.

Libraries here: `std` plus `thiserror` (derive `Error`/`Display`) and `futures` only for a one-line `block_on` in the last example.

## 3. Guided Examples
All commands run from `languages/rust/`.

### 3.1 Ownership and borrowing
- **Goal:** moves, shared and mutable borrows, `String` vs `&str`.
- **Run:** `cargo run -p m01-language-fundamentals --example ownership`
- **Code:** [examples/ownership.rs](examples/ownership.rs)
```rust
fn consume(title: String) -> usize { title.len() }     // takes ownership
fn shout(title: &str) -> String { title.to_uppercase() } // borrows
// let len = consume(title); println!("{title}");  // error[E0382]: borrow of moved value
```
- **Walkthrough:** `shout(&title)` lends the string, so `title` stays usable. `consume(title)` moves it; using `title` afterwards is a compile error. Integers are `Copy`, so `let b = a` leaves both usable. `first_word` returns a `&str` slice of its input: elision ties the result's lifetime to the argument, no allocation. The closure needs `move` because `label` could outlive `prefix`.
- **Expected output:**
  ```
  shout: WRITE DOCS
  first word: Write
  draft: Write docs (draft)
  consumed 10 bytes
  a=5 b=5
  in_progress / done / in
  task-7
  ```

### 3.2 Enums and `match`
- **Goal:** enums that carry data, exhaustive matching, `Option`, `Result`, `let ... else`.
- **Run:** `cargo run -p m01-language-fundamentals --example enums_match`
- **Code:** [examples/enums_match.rs](examples/enums_match.rs)
```rust
match event {
    Event::Created { title } => format!("created '{title}'"),
    Event::Moved { from, to } if from == to => "no-op move".to_string(),
    Event::Moved { from, to } => format!("moved {} -> {}", from.as_str(), to.as_str()),
    Event::Tagged(tags) if tags.is_empty() => "tags cleared".to_string(),
    Event::Tagged(tags) => format!("tagged {}", tags.join(",")),
    Event::Deleted => "deleted".to_string(),
}
```
- **Walkthrough:** each variant has its own shape; the compiler rejects a `match` that forgets one, so adding a variant finds every place to update. Guards (`if ...`) refine an arm. `parse_status` returns `Option<Status>`; `unwrap_or` supplies a default. `str::parse::<u32>()` returns a `Result` whose error prints as text.
- **Expected output:**
  ```
  created 'Write docs'
  moved todo -> in_progress
  no-op move
  tagged docs,p1
  tags cleared
  deleted
  done -> Done
  finished -> unknown
  default: todo
  finished task
  let-else: InProgress
  "42" -> id 42
  " 7 " -> id 7
  "abc" -> error: invalid digit found in string
  "-1" -> error: invalid digit found in string
  ```

### 3.3 Traits, generics and composition
- **Goal:** a trait with a default method, static vs dynamic dispatch, a bounded generic, composition.
- **Run:** `cargo run -p m01-language-fundamentals --example traits_generics`
- **Code:** [examples/traits_generics.rs](examples/traits_generics.rs)
```rust
fn send_static<N: Notifier>(notifier: &N, message: &str) -> String { notifier.notify(message) }
fn send_all(notifiers: &[Box<dyn Notifier>], message: &str) { /* vtable call per item */ }
fn largest<T: PartialOrd>(items: &[T]) -> Option<&T> { /* ... */ }
struct Task<N> { title: String, notifier: N }   // has-a, not is-a
```
- **Walkthrough:** `send_static` is compiled once per concrete type (fast, no allocation). `Vec<Box<dyn Notifier>>` mixes different types at the cost of a pointer and a vtable call. `notify_all` is a default method every implementor inherits. `largest` works for any `PartialOrd` type, including `&str`; an empty slice gives `None`, not a crash.
- **Expected output:**
  ```
  [console] static dispatch
         console: [console] dynamic dispatch
  ada@example.com: [mail to ada@example.com] dynamic dispatch
  ["[console] a", "[console] b"]
  Some(9)
  Some("pear")
  None
  [mail to ada@example.com] 'Write docs' completed
  P1 P3
  ```

### 3.4 Errors with `Result`, `?` and `thiserror`
- **Goal:** a custom error enum, `?` with automatic `From` conversion, error `source()` chains.
- **Run:** `cargo run -p m01-language-fundamentals --example errors`
- **Code:** [examples/errors.rs](examples/errors.rs)
```rust
#[derive(Debug, thiserror::Error)]
enum StartupError {
    #[error("bad configuration")]
    Config(#[from] ConfigError),   // generates From<ConfigError>, so `?` converts
}
fn start(pairs: &[(&str, &str)]) -> Result<String, StartupError> {
    let port = parse_port(pairs)?;
    Ok(format!("listening on :{port}"))
}
```
- **Walkthrough:** `?` returns early with `Err(From::from(e))`. `#[source]` keeps the underlying `ParseIntError`, and `report` walks the `source()` chain to print every cause. Libraries return typed errors like these; `main` or handlers decide what to do with them (module 03 turns them into HTTP responses).
- **Expected output:**
  ```
  ok: listening on :8080
  error: bad configuration
    caused by: missing key 'port'
  error: bad configuration
    caused by: invalid port 'http'
    caused by: invalid digit found in string
  error: bad configuration
    caused by: port 80 is reserved
  ```

### 3.5 Task Tracker: modules, collections, iterators, async
- **Goal:** put the pieces together on the shared domain; see visibility and a first `async fn`.
- **Run:** `cargo run -p m01-language-fundamentals --example task_summary`
- **Code:** [examples/task_summary.rs](examples/task_summary.rs)
```rust
let overdue: Vec<&str> = tasks.iter()
    .filter(|t| t.is_overdue(today))
    .map(|t| t.title.as_str())
    .collect();
by_status.entry(task.status).or_default().push(task.id);   // BTreeMap<Status, Vec<u32>>
```
- **Walkthrough:** `mod tracker` keeps `due_day` private, so a `Task` can only be built through `Task::new` (visibility is per module, `pub` opts in). `iter()` borrows, `into_iter()` consumes. `BTreeMap` keeps keys sorted, which makes output deterministic. An `async fn` returns a lazy future; `futures::executor::block_on` is the smallest possible executor. Servers use tokio from module 02.
- **Expected output:**
  ```
  overdue: ["Fix login bug", "Update deps"]
  todo         [1, 4, 5]
  in_progress  [2]
  done         [3]
  tags: {"bug", "chore", "docs", "p1", "release", "tech-debt"}
  first title: Some("Write docs")
  missing: None
  async result: 42
  sum of odd squares: 35
  ```

## 4. Common Pitfalls
| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| `error[E0382]: borrow of moved value` | passing a `String`/`Vec` by value moves it | pass `&x`, or return the value back, or `clone()` only if you really need two copies |
| Reflexive `.clone()` to quiet the borrow checker | the error looks like a nuisance | restructure: borrow for the call, shorten the borrow's scope, or take ownership once |
| `error[E0106]: missing lifetime specifier` / returning `&` to a local | the reference would outlive the value | return an owned `String`, or tie the output to an input (`fn f(x: &str) -> &str`) |
| `&String` / `&Vec<T>` parameters | habit from other languages | take `&str` / `&[T]`; callers can pass more kinds of data |
| `.unwrap()` in request handlers | it is quick | return `Result` and use `?`; panics are for broken invariants |
| `&s[0..1]` panics on non-ASCII text | strings are UTF-8 bytes, not chars | use `.chars()`, `char_indices()` or `str::get(..)` |
| Mutating a collection while iterating it | borrow rules forbid it (compile error) | collect the changes first, or use `retain`/`drain` |
| Forgetting `move` on a closure that outlives its scope | the closure borrows locals by default | add `move` so it owns what it captures |

## 5. Coming From Other Languages
- **Go:** `Result<T, E>` + `?` replaces `if err != nil`; enums replace sentinel values; no zero values, so you must initialise. Goroutines vs async tasks arrive in module 06.
- **C++:** moves are the default and destructive (the source is unusable); RAII is `Drop`; the borrow checker replaces manual lifetime reasoning.
- **TypeScript/Python:** enums carry data like tagged unions and `match` is exhaustive; there is no `null`/`None` surprise because `Option` is in the type.
- **Java/C#:** traits are like interfaces, but can be implemented for types you do not own; generics are monomorphised (no boxing or erasure); no inheritance, so share behaviour with default methods and composition.

## 6. Exercises
Starter code and tests live in `exercises/`; reference solutions in `solutions/` (same relative paths).

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | [Word stats](exercises/ex1-word-stats) | Easy | borrowing, `&str` slices, iterators, `BTreeMap` | `cargo test -p m01-ex1-word-stats` |
| 2 | [Status parser](exercises/ex2-status-parser) | Medium | enums, `FromStr`/`Display`, custom errors, `?` | `cargo test -p m01-ex2-status-parser` |
| 3 | [Generic store](exercises/ex3-generic-store) | Stretch | traits, generics, closures, returning `impl Iterator` | `cargo test -p m01-ex3-generic-store` |

**1. Word stats.** *Task:* implement `word_counts`, `longest_word` and `top_words`. *Acceptance:* all tests in `tests/word_stats.rs`, including that `longest_word` returns a slice of the input (no copy). *Hint:* <details><summary>nudge</summary>`*map.entry(key).or_insert(0) += 1`; for the first-wins tie, `max_by_key` keeps the last maximum, so fold instead.</details>

**2. Status parser.** *Task:* implement `FromStr` and `Display` for `Status`, `Status::next`, `parse_task` and `parse_all`. *Acceptance:* `tests/status_parser.rs`; errors carry line numbers and keep their cause via `source()`. *Hint:* <details><summary>nudge</summary>`let [a, b, c] = fields[..] else { return Err(..) };` destructures a slice; `.collect::<Result<Vec<_>, _>>()` stops at the first `Err`.</details>

**3. Generic store.** *Task:* finish `Store<T: Entity>`: `insert`, `remove`, `update`, `filter`, `next_id` and `FromIterator`. *Acceptance:* `tests/store.rs` (note the test type is deliberately not `Clone`). *Hint:* <details><summary>nudge</summary>`filter` must return a lazy iterator that borrows `self`: `self.items.values().filter(move |i| pred(i))`.</details>

## 7. Recap
- Every value has one owner; borrow with `&`/`&mut`, move or `clone` only when you mean it.
- Enums + exhaustive `match` model states; `Option`/`Result` + `?` replace null and exceptions.
- Traits give polymorphism; generics are static, `dyn Trait` is dynamic; compose instead of inheriting.
- Iterator chains with closures are the default way to transform collections.
- Reach for `&str`/`&[T]` in parameters, typed `thiserror` errors in libraries, `cargo clippy -- -D warnings` before every commit.

## 8. Self-check Questions
1. Why does `let b = a;` leave `a` usable for an `i32` but not for a `String`?
2. When would you choose `Vec<Box<dyn Trait>>` over a generic `T: Trait`, and what does it cost?
3. What does `?` do on a `Result<T, E1>` inside a function returning `Result<U, E2>`, and what must exist for it to compile?
4. Why is `fn longest(a: &str, b: &str) -> &str` rejected, and how do you fix it?
5. Why does `BTreeMap` give deterministic output while `HashMap` does not?

## 9. Further Reading
- [The Rust Book](https://doc.rust-lang.org/book/): chapters 4 (ownership), 6 (enums), 9 (errors), 10 (generics, traits, lifetimes), 13 (iterators)
- [Rust by Example](https://doc.rust-lang.org/rust-by-example/)
- [thiserror docs](https://docs.rs/thiserror) and the [Rust API guidelines](https://rust-lang.github.io/api-guidelines/)

---
**Next:** [02 HTTP Server](../02-http-server/README.md)
