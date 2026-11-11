# Language Profile: Rust

Constrains all generated Rust content. Dependencies are pinned in the workspace [Cargo.toml](./Cargo.toml) (`[workspace.dependencies]`) and [Cargo.lock](./Cargo.lock). Member crates use `dep = { workspace = true }`; do not add new dependencies while writing a module.

## 1. Overview
- **Version:** Rust stable, edition 2024 (rustc 1.85+; developed with 1.99).
- **Paradigm / typing:** statically typed, no GC, ownership and borrowing with compile-time memory safety. Multi-paradigm with traits (not classes), enums with data (algebraic types), `Result`/`Option` instead of exceptions/null, generics monomorphised.
- **Why backend:** predictable latency, low memory, fearless concurrency (data races are compile errors), a rich async ecosystem (tokio, axum, sqlx). It is the hardest of the three languages to start with, so the lessons explain ownership in context.

## 2. Toolchain

| Purpose | Tool | Command |
|---------|------|---------|
| Install / version manager | rustup | `rustup update`, `rustc --version` |
| Package / dependency manager | cargo (workspace + `Cargo.lock`) | `cargo add` is NOT used here, see above |
| Build | cargo | `cargo build -p m02-http-server` |
| Run an example | cargo | `cargo run -p m02-http-server --example hello` |
| Test | cargo test | `cargo test -p m02-ex1-query-params` |
| Format | rustfmt | `cargo fmt --all` |
| Lint / static analysis | clippy | `cargo clippy --workspace --all-targets -- -D warnings` |
| Docs | rustdoc | `cargo doc --no-deps --open` |
| Debug / profile | `rust-lldb`/`rust-gdb`, `RUST_BACKTRACE=1`, `cargo bench` (criterion) | |

All commands run from `languages/rust/`. Set `CARGO_TARGET_DIR` to a Linux path (for example `/tmp/target`) when working on a Windows-mounted drive; builds are much faster.

## 3. Project Conventions
- **Layout of this repo:** one Cargo workspace at `languages/rust/`.
  - Each module is a crate: `modules/NN-topic/Cargo.toml`, package `mNN-topic` (e.g. `m02-http-server`), with an almost-empty `src/lib.rs` and the examples in `examples/*.rs` (auto-discovered, run with `--example <file-stem>`).
  - Each exercise is a crate: `modules/NN-topic/exercises/exN-name/` with package name `mNN-exN-name`, code in `src/lib.rs`, tests in `tests/` (integration tests; solutions only replace `src/`). Starter code compiles and uses `todo!()`; tests then fail with a panic.
  - Solutions: `modules/NN-topic/solutions/exN-name/src/...` (overlay onto the exercise).
  - Projects: `projects/PN-name/starter` (package `pN-name`, with `tests/` inside) and `projects/PN-name/solution/` (overlay on `starter/`).
  - `modules/00-workspace-check` and `projects/P0-anchor` are infrastructure (keep workspace globs valid); do not edit them.
- **Naming:** `snake_case` functions/modules, `UpperCamelCase` types/traits, `SCREAMING_SNAKE` constants. Prefer `&str`/`&[T]` parameters, return owned types.
- **Error handling:** `Result<T, E>` and `?`. Libraries and the domain layer use `thiserror` enums. Binaries and examples may use `anyhow`. Implement `IntoResponse` for the HTTP error type. `unwrap`/`expect` only in tests, `main` setup, or when an invariant is proven (and say why).
- **Docs:** `///` on public items, `//!` at module top; doc examples must compile.
- **Lints:** the workspace forbids `unsafe_code`. Starter code must not trigger clippy warnings that the solution would not (use `_arg` names).

## 4. Library Choices per Topic

| Module | Stdlib / Library | Version (resolved in lock) | Why |
|--------|------------------|---------|-----|
| 01 Fundamentals | std (`Option`, `Result`, collections, iterators, traits) | std | |
| 02 HTTP server | `axum` 0.8 (+ `tokio` 1, `tower`, `tower-http`) | axum 0.8, tokio 1, tower 0.5, tower-http 0.7 | the de facto framework on tokio; closest to "stdlib-like" |
| 03 REST design | `axum` extractors, `serde`, hand-written validation, `thiserror` | | keep validation explicit; no validator derive crate |
| 04 Serialization | `serde`, `serde_json`, `csv`, form via axum `Form`, `toml` | serde 1, serde_json 1, csv 1, toml 1 | serde is the standard. XML and YAML are skipped (no stable maintained serde crate worth teaching) and noted in the lesson |
| 05 Database access | `sqlx` 0.9 with `sqlite` (examples) and `postgres` (compile-only section), runtime-checked `query`/`query_as` + `FromRow`, `migrate!` | sqlx 0.9 | async, pool built in, same API for SQLite and PostgreSQL; no ORM |
| 06 Concurrency | `std::thread`, `std::sync` (`Arc`, `Mutex`, `RwLock`, `mpsc`, atomics), `tokio` (`spawn`, `select!`, `mpsc`, `JoinSet`, `Semaphore`), `tokio-util::CancellationToken`, `futures` | tokio 1, tokio-util 0.7, futures 0.3 | |
| 07 HTTP client | `reqwest` 0.13 (rustls), hand-written retry with backoff, `wiremock` for tests | reqwest 0.13, wiremock 0.6 | |
| 08 File and stream I/O | `std::fs`, `std::io` (`Read`, `Write`, `BufReader`), `tokio::fs`, `flate2`, `base64`, `hex`, `sha2`, `tempfile`, `include_str!`/`include_bytes!` | flate2 1, base64 0.23, hex 0.4, sha2 0.11, tempfile 3 | |
| 09 Configuration | `std::env`, `clap` (derive + `env`), `serde` + `toml`, `secrecy` | clap 4, toml 1, secrecy 0.10 | layering is hand-written to show the pattern; no `config`/`figment` crate |
| 10 Logging | `tracing`, `tracing-subscriber` (env-filter, json), `tower-http` `TraceLayer`, `CatchPanicLayer`, atomics for a metrics counter | tracing 0.1, tracing-subscriber 0.3 | |
| 11 Auth / security | `argon2`, `jsonwebtoken`, `axum-extra` (cookies), `rand`, `sha2`+`hmac` (API keys), hand-written token bucket rate limiter | argon2 0.6, jsonwebtoken 11, axum-extra 0.12, hmac 0.13 | |
| 12 Testing | built-in `#[test]`/`#[tokio::test]`, `tower::ServiceExt::oneshot`, `http-body-util`, trait-based fakes, `wiremock`, `criterion` (benchmarks), `tempfile` | criterion 0.8 | no mocking framework: traits + fakes |
| 13 Time and utilities | `chrono`, `chrono-tz`, `uuid`, `rand`, `regex`, `minijinja`, `tokio::time` (`interval`) | chrono 0.4, chrono-tz 0.10, uuid 1, rand 0.10, regex 1, minijinja 3 | std has no calendar/zone/uuid/regex |
| 14 Packaging | `cargo build --release`, profiles, `--target`, `cargo install`, multi-stage Dockerfile, GitHub Actions | std | |

Crate APIs change between versions; when code does not compile, read the docs for the pinned version (`cargo doc -p <crate>` or docs.rs) rather than guessing.

**Database for examples:** SQLite via `sqlx` (`sqlite::memory:` kept alive by its pool, or a temp file). PostgreSQL is shown as a documented swap (the `postgres` feature is enabled so the example compiles; it only connects when `DATABASE_URL` is set).

## 5. Language-specific Teaching Notes

| Module | Emphasis | Typical pitfall |
|--------|----------|-----------------|
| 01 | ownership/move/borrow, `String` vs `&str`, `Option`/`Result`, enums + `match`, traits and generics, iterators/closures, lifetimes only as far as needed | "use of moved value", returning a reference to a local, `clone()` to silence the borrow checker, `unwrap()` in handlers |
| 02 | handlers as async fns with extractors, `State`, `Router`, layers, `with_graceful_shutdown`, extractor order (body extractor last) | body-consuming extractor not last, non-`Send` future held across `.await`, `State` needs `Clone` |
| 03 | `IntoResponse` for a typed `ApiError`, status codes via `(StatusCode, Json<T>)`, `Query` defaults | `Json` rejection returns plain text unless mapped, `Query` of a `Vec` |
| 04 | derive `Serialize`/`Deserialize`, `#[serde(rename_all, default, skip_serializing_if, deny_unknown_fields)]`, `Option` vs missing, `chrono` serde, custom `with` modules | `Option<Option<T>>` for PATCH, untagged enum error messages |
| 05 | `sqlx::query_as`, `FromRow`, binding with `?`, pool setup, transactions (`begin`/`commit`, rollback on drop), migrations | every separate `connect`/pool of `sqlite::memory:` gets its own empty database, and the database vanishes when its last connection closes (keep one connection alive); holding a transaction across `.await` on shared state |
| 06 | `Send + 'static` bounds, `Arc<Mutex<_>>`, never hold a `std::sync::Mutex` guard across `.await`, `JoinSet`, `select!` cancel safety, bounded channels | blocking the runtime thread (`std::thread::sleep`, heavy CPU) -> `spawn_blocking`, forgetting to join/abort tasks, deadlock with two locks |
| 07 | one `reqwest::Client`, timeouts, `error_for_status`, retry only idempotent calls with exponential backoff and jitter | creating a client per request, no timeout, unbounded body read |
| 08 | `Read`/`Write` traits, `BufReader`/`BufWriter` (flush on drop swallows errors), streaming with `tokio::io` | reading a whole file into memory, forgetting `flush`, path traversal |
| 09 | typed config structs, precedence defaults < file < env < CLI, secrets in `secrecy::SecretString` | `Debug` printing secrets, parsing env at call sites |
| 10 | `tracing` spans vs events, `#[instrument]`, `EnvFilter`, request-id span, panic = 500 via `CatchPanicLayer` | forgetting to keep the subscriber guard, logging secrets |
| 11 | constant-time comparison, `Argon2` with random salt, JWT validation settings (alg, exp), `axum` middleware via `from_fn`, SQL injection is prevented by binding | blocking the runtime with Argon2 (use `spawn_blocking`), default JWT validation misconfiguration |
| 12 | integration tests in `tests/`, `oneshot`, traits for fakes, `#[tokio::test]`, `criterion`, doc-tests | tests sharing a global (use per-test state), `#[cfg(test)]` helpers not visible to `tests/` |
| 13 | `chrono` `DateTime<Utc>` vs `NaiveDateTime`, `Duration` types (`std` vs `chrono` vs `tokio`), `interval` first tick fires immediately | `Local` time in servers, `interval` missed-tick behaviour |
| 14 | `--release`, `lto`, `strip`, `musl` target, Docker layer caching with `cargo chef`-free approach (dummy `main`), non-root | forgetting `--locked`, `target/` in the image |

## 6. Comparison Anchors
Go (goroutines vs async tasks, `error` vs `Result`), C++ (RAII, move semantics), TypeScript/Python (async/await, enums vs unions), Java/C# (traits vs interfaces).

## 7. Module Ordering Adjustments
Module 01 is the longest: it front-loads ownership, `Result`, traits, iterators and a first look at `async`/`.await` (no runtime yet). Module 06 should be read before 02-style async code is written independently; module 02 uses only `async fn` handlers and defers `Send`/`select!` detail to 06.

## 8. Out of Scope for This Language
`unsafe`, macros (`macro_rules!` writing, proc macros), advanced lifetimes/GATs/HRTB, `no_std`, FFI, ORMs (diesel, sea-orm), other web frameworks (actix, rocket), gRPC, WebAssembly, XML/YAML (no stable maintained serde crate is taught).
