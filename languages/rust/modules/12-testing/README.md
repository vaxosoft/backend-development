# 12. Testing in Rust

> Estimated time: 2h | Difficulty: Intermediate | Prerequisites: 01, 02, 03, 05 (SQLite basics), 07 (HTTP client)

## Learning Outcomes
After this module you can:
- [ ] Write unit tests and table-driven tests with `#[test]`, `assert_eq!`, `#[should_panic]` and `Result`-returning tests
- [ ] Test axum handlers in-process with `tower::ServiceExt::oneshot`, without opening a socket
- [ ] Isolate dependencies with small trait-based fakes, spies and an injected clock (no mocking framework)
- [ ] Run integration tests against an in-memory SQLite database and against a local fake HTTP server (`wiremock`)
- [ ] Write a reusable contract test suite that every implementation of a trait must pass
- [ ] Add a `criterion` benchmark and know where doc-tests and coverage fit

## 1. Concept Overview
A good test suite is a pyramid: many fast **unit tests** (pure functions, validation), fewer **component tests** (one handler or repository with fakes), a few **integration tests** (real database, real HTTP wire). Tests must be **deterministic** (no wall-clock time, no shared state, no real network), **independent** (any order, in parallel) and **fast**.

The way to get there is *dependency isolation*: the code under test receives time, storage and outbound HTTP through narrow interfaces, so a test can substitute a fake. A **fake** is a simple working implementation (in-memory store); a **stub** returns canned answers; a **spy** records calls. A **contract test** runs the same checks against every implementation (fake and real) so the fake cannot drift from the real thing.

## 2. How Rust Approaches It
- **Built in:** `cargo test` runs `#[test]` functions in parallel threads. Unit tests sit in `#[cfg(test)] mod tests` beside the code (they see private items); integration tests in `tests/*.rs` are separate crates that only see the public API; doc-tests (`///` examples) are compiled and run too.
- **Async:** `#[tokio::test]` gives each test its own runtime.
- **HTTP handlers:** an axum `Router` is a `tower::Service`, so `router.oneshot(request).await` runs the whole stack (routing, extractors, middleware) in memory. `http-body-util::BodyExt::collect` reads the body.
- **Fakes, not mocks:** define a trait at the boundary (`TaskStore`, `Clock`), implement it for production and for tests. No mocking framework, and the compiler checks the fake against the trait.
- **External HTTP:** `wiremock` starts a real local server on a random loopback port with expectations (`.expect(1)`, verified when the server is dropped).
- **Benchmarks:** `criterion` in `benches/` (`harness = false`). Coverage: `cargo llvm-cov` (a separate tool you install; not used here).
- **Layout of this module:** `src/lib.rs` is a small Task Tracker service to test. The lesson files in `examples/` are **test targets** (declared with `[[test]] path = "examples/..."` in [Cargo.toml](./Cargo.toml)), so you *run* them with `cargo test`.

## 3. Guided Examples
Run all commands from `languages/rust/`. The listed output uses `--test-threads=1` only to make the order stable; the "finished in" timing is omitted.

### 3.1 Unit and table-driven tests
- **Goal:** the shape of a test: arrange, act, assert, with readable failures.
- **Run:** `cargo test -p m12-testing --test unit_and_table -- --test-threads=1`
- **Code:** [examples/unit_and_table.rs](./examples/unit_and_table.rs); a private-item unit test lives in [src/lib.rs](./src/lib.rs) (`mod tests`), a doc-test on `slugify`.
- **Walkthrough:** a "table" is an array of `(input, expected)` tuples looped over with a message that names the row (`"input: {input:?}"`); Rust has no subtest runner, so the loop plus message is the idiom. `#[should_panic(expected = "...")]` should always name the message, or any panic passes. Tests can return `Result<(), E>` so `?` replaces `unwrap`. `#[ignore = "why"]` skips slow tests unless `-- --ignored`.
  ```rust
  for (input, expected) in cases {
      assert_eq!(slugify(input), expected, "input: {input:?}");
  }
  ```
- **Expected output:**
  ```
  test slugify_handles_a_megabyte_title ... ignored, slow: run with `cargo test -- --ignored`
  test slugify_table ... ok
  test status_round_trips_through_text ... ok
  test unknown_status_is_an_error ... ok
  test unwrap_on_invalid_input_panics_with_the_reason - should panic ... ok
  test validate_title_table ... ok
  test result: ok. 5 passed; 0 failed; 1 ignored; 0 measured; 0 filtered out
  ```

### 3.2 Handler tests with `oneshot`
- **Goal:** exercise routing, extractors, status codes and the error envelope with no socket.
- **Run:** `cargo test -p m12-testing --test handler_oneshot -- --test-threads=1`
- **Code:** [examples/handler_oneshot.rs](./examples/handler_oneshot.rs)
- **Walkthrough:** `router()` builds a *fresh* app and store per test, so tests cannot interfere. `oneshot` consumes the service, so the helper clones the `Router` (cheap: it is reference counted) to send several requests to the same state. Error cases are a table of `(method, uri, body, status, code)`.
  ```rust
  let resp = router.clone().oneshot(req.body(body).unwrap()).await.unwrap();
  let bytes = resp.into_body().collect().await.unwrap().to_bytes();
  ```
- **Expected output:**
  ```
  test create_then_fetch ... ok
  test error_cases_use_the_envelope ... ok
  test list_filters_by_status_and_delete_removes ... ok
  test result: ok. 3 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out
  ```

### 3.3 Fakes, stubs and spies
- **Goal:** isolate time and storage behind traits.
- **Run:** `cargo test -p m12-testing --test fakes_and_isolation -- --test-threads=1`
- **Code:** [examples/fakes_and_isolation.rs](./examples/fakes_and_isolation.rs)
- **Walkthrough:** `FakeClock` (shared `Arc<Mutex<DateTime>>`) makes `created_at` exact: `"2030-01-01T09:00:00Z"`, then `"…T10:30:00Z"` after `advance(90 min)`. `BrokenStore` always errors and proves a 500 never leaks `10.0.0.5:5432`. `SpyStore` wraps the real `MemoryStore` and records calls to show invalid input never reaches storage.
- **Expected output:**
  ```
  test created_at_comes_from_the_injected_clock ... ok
  test invalid_input_never_reaches_the_store ... ok
  test store_failure_becomes_a_500_without_details ... ok
  test result: ok. 3 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out
  ```

### 3.4 Integration tests with a database
- **Goal:** a real SQL engine per test, with no containers.
- **Run:** `cargo test -p m12-testing --test db_integration -- --test-threads=1`
- **Code:** [examples/db_integration.rs](./examples/db_integration.rs)
- **Walkthrough:** `SqliteTasks::new()` opens `sqlite::memory:` with **one** connection that is never recycled (each connection to `:memory:` is a separate database). Parallel tests therefore each get an empty database with ids starting at 1. The tests check schema `CHECK` constraints, that bound parameters neutralise `'); DROP TABLE`, and that dropping a transaction rolls it back. For PostgreSQL you would run the same tests against a throwaway database (for example `DATABASE_URL` pointing at a container) and skip them when it is unset.
- **Expected output:**
  ```
  test bound_parameters_neutralise_sql_injection ... ok
  test dropping_a_transaction_rolls_it_back ... ok
  test every_test_gets_its_own_database ... ok
  test schema_constraints_reject_bad_rows ... ok
  test result: ok. 4 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out
  ```

### 3.5 Outbound HTTP with `wiremock` (Task Tracker webhook)
- **Goal:** test an HTTP client against a local fake server, including failures.
- **Run:** `cargo test -p m12-testing --test wiremock_webhook -- --test-threads=1`
- **Code:** [examples/wiremock_webhook.rs](./examples/wiremock_webhook.rs)
- **Walkthrough:** `MockServer::start()` listens on a random port; `Mock::given(method("POST")).and(path(..)).and(body_partial_json(..))` describes the request, `respond_with` the reply, and `.expect(1)` fails the test on drop if the call did not happen exactly once. A 503 maps to `NotifyError::Status`; a closed port to `NotifyError::Transport`.
- **Expected output:**
  ```
  test a_server_error_is_reported ... ok
  test an_unreachable_server_is_a_transport_error ... ok
  test posts_the_event_as_json ... ok
  test result: ok. 3 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out
  ```

### Also in the module
- **Doc-test:** `cargo test -p m12-testing --doc` compiles the example in `slugify`'s docs.
- **Benchmark:** `cargo bench -p m12-testing --bench slugify` (add `-- --quick` for a fast run) prints `slugify/short time: [...]`. The numbers depend on your machine, so none are listed. Benchmark in release mode only, and compare before/after on the same machine.
- **Everything at once:** `cargo test -p m12-testing`.

## 4. Common Pitfalls
| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| `#[test] async fn` does not compile / test never awaits | `#[test]` has no runtime | `#[tokio::test]` |
| Tests share a `static`/global or a fixed file/port and fail randomly | `cargo test` runs tests in parallel threads | Build state per test (`router()`, `SqliteTasks::new()`), use port 0 and `tempfile` |
| `sqlite::memory:` data "disappears" or tables are missing | Each pooled connection is its own database; idle connections get recycled | `max_connections(1)`, `idle_timeout(None)`, `max_lifetime(None)` |
| `oneshot` used twice on the same router: "use of moved value" | `oneshot` takes `self` | `router.clone().oneshot(..)` |
| `#[should_panic]` without `expected` passes on any panic, even a typo | Wrong panic satisfies the attribute | Always give `expected = "message part"` |
| Helpers marked `#[cfg(test)]` in `src/` are invisible to `tests/` | Integration tests compile the library *without* `cfg(test)` | Put shared helpers in `tests/common/mod.rs` or a `pub` test-support module/feature |
| Real clock or `sleep` in tests | Convenient, but slow and flaky | Inject a `Clock`/`Sleeper` trait (see exercises 1 and 3) |
| `unwrap()` on a poisoned `Mutex` hides the original failure | One test thread panicked while holding the lock | Fresh state per test; or recover with `unwrap_or_else(\|e\| e.into_inner())` |
| Expecting `println!` output from a passing test | libtest captures output | `cargo test -- --nocapture` |
| A fake that silently differs from the real store | Nobody checked | Run one contract suite against both (exercise 2) |

## 5. Coming From Other Languages
- **Go:** table-driven tests are a `for` over a slice (no `t.Run` subtests built in); `httptest.NewRecorder` ↔ `oneshot`; interfaces defined at the consumer ↔ traits; `-race` has no equivalent because data races do not compile.
- **Java/C#:** no Mockito/Moq: write the fake. `#[should_panic]` ↔ `assertThrows`; `[Theory]/@ParameterizedTest` ↔ a loop or a `macro_rules!` table.
- **Python/TypeScript:** no monkey-patching or `jest.mock`; dependencies must be injected through generics (`impl Trait`) or `Arc<dyn Trait>`. That is why the design stays testable.
- **C++:** no test framework required; `cargo test` is built in, and tests are ordinary functions linked into a test binary.

## 6. Exercises
These are implementation tasks whose tests demonstrate a technique (you are not asked to write tests). Solutions are in [solutions/](./solutions).

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | Fake clock and session expiry | Easy | Injectable time, shared fake via `Arc<Mutex<_>>` | `cargo test -p m12-ex1-fake-clock` |
| 2 | Contract-tested in-memory store | Medium | Trait contract suite, concurrency check, "tests that test the tests" | `cargo test -p m12-ex2-store-contract` |
| 3 | Retry with an injected sleeper | Stretch | Deterministic async test, recording fake, overflow-safe backoff | `cargo test -p m12-ex3-retry-with-sleeper` |

**Exercise 1: [exercises/ex1-fake-clock](./exercises/ex1-fake-clock)**
- **Task:** implement `FakeClock::{advance, set}` and `Clock::now`, then `SessionStore::{create, validate, purge_expired}` (sliding expiry).
- **Acceptance:** time is frozen until advanced; clones share one timeline; a session dies exactly at `now >= expiry`; each successful validation extends it; purge counts only dead sessions.
- <details><summary>Hint</summary>The fake holds `Arc<Mutex<DateTime<Utc>>>`; `SessionStore` stores `(user, expiry)` per id.</details>

**Exercise 2: [exercises/ex2-store-contract](./exercises/ex2-store-contract)**
- **Task:** implement `MemoryStore` for `TaskStore`.
- **Acceptance:** `tests/contract.rs` is generic over `TaskStore`: ids start at 1 and are never reused, blank titles are rejected without consuming an id, list keeps order and filters, missing ids are reported, 4 threads get 200 unique ids. The same file proves the suite fails for three deliberately flawed stores.
- <details><summary>Hint</summary>Keep `last_id` separate from the map so deleting the newest task does not free its id.</details>

**Exercise 3: [exercises/ex3-retry-with-sleeper](./exercises/ex3-retry-with-sleeper)**
- **Task:** implement `RetryPolicy::delay_for` and `retry`.
- **Acceptance:** delays `10, 20, 40 ms` (capped), no sleep after the last attempt, fatal errors stop at once, `max_attempts = 0` still tries once, no overflow for huge retry numbers. A `RecordingSleeper` returns instantly, so the suite takes milliseconds.
- <details><summary>Hint</summary>Use `Duration::saturating_mul` and cap the shift amount before computing `2^n`.</details>

## 7. Recap
- Inject time, storage and outbound HTTP through traits; test with fakes, spies and local servers.
- `oneshot` tests a whole axum app in memory; build fresh state per test so parallelism is free.
- One contract suite for every implementation keeps fakes honest.
- Unit tests in `src/`, integration tests in `tests/`, doc-tests for public examples, `criterion` for performance.
- Reach for a fake when you own the boundary, `wiremock` when you don't, a real in-memory database when SQL behaviour matters.

## 8. Self-check Questions
1. Why can a unit test call a private function but a file in `tests/` cannot, and where would you put a helper both need?
2. A test passes alone but fails with the rest of the suite. Name three Rust-specific causes and how you would find them (`--test-threads=1` is the first step).
3. What goes wrong if `created_at` uses `Utc::now()` directly in the handler, and how does `Arc<dyn Clock>` change the test?
4. When is a hand-written fake better than `wiremock`, and when is it worse?

## 9. Further Reading
- The Rust Book, chapter 11 (Writing Automated Tests) and the `cargo test` reference
- [`tower::ServiceExt::oneshot`](https://docs.rs/tower/latest/tower/trait.ServiceExt.html), [`wiremock`](https://docs.rs/wiremock), [`criterion`](https://docs.rs/criterion)
- Rust API Guidelines: documentation and examples as tests

---
**Next:** [13 Time, Utilities and Standard Library](../13-time-and-stdlib-utilities/README.md)
