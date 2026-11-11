# Backend Development in Rust

Learn backend development with Rust (edition 2024) and the tokio/axum/sqlx stack: language constructs and the core libraries only (HTTP, no gRPC/GraphQL/brokers). Every lesson uses the same Task Tracker domain described in [../../framework/domain-and-projects.md](../../framework/domain-and-projects.md). Library choices and conventions are in [PROFILE.md](./PROFILE.md).

## Prerequisites
- Programming experience in any language. No Rust needed; module 01 front-loads ownership, `Result`, traits and iterators (it is the longest module: plan 2-3 h).
- [rustup](https://rustup.rs) with the stable toolchain (1.85 or newer), and a C compiler (`cc`/`gcc`; SQLite is compiled from source by `sqlx`).
- `curl` and an editor with rust-analyzer.

## Setup
All commands run from this folder (`languages/rust/`). The whole track is one Cargo workspace with a pinned `Cargo.lock`.

```bash
export CARGO_TARGET_DIR=/tmp/target      # optional; much faster on a Windows-mounted drive
cargo build -p m01-language-fundamentals --examples   # first build downloads dependencies
cargo run -p m01-language-fundamentals --example <name>
```

| Path | Contents |
|------|----------|
| `modules/NN-topic/` | crate `mNN-topic`, e.g. `m02-http-server` |
| `modules/NN-topic/README.md` | the lesson |
| `modules/NN-topic/examples/*.rs` | runnable examples: `cargo run -p m02-http-server --example hello` |
| `modules/NN-topic/exercises/exN-name/` | crate `mNN-exN-name`: starter in `src/lib.rs` (`todo!()`), tests in `tests/` |
| `modules/NN-topic/solutions/exN-name/src/` | reference solution files (same paths as the exercise) |
| `projects/PN-name/` | capstone: `README.md`, `starter/` (crate `pN-name`, tests in `starter/tests/`), `solution/` |

Working on an exercise:

```bash
cargo test -p m02-ex1-greeter         # red
# edit modules/02-http-server/exercises/ex1-greeter/src/lib.rs until green
```
Peek at the solution under `solutions/ex1-greeter/src/` only after a real attempt.

Quality commands: `cargo fmt --all`, `cargo clippy --workspace --all-targets -- -D warnings`. Note that `cargo test --workspace` fails by design until all exercises are solved; always test one package.

## Learning path

| # | Module | Time |
|---|--------|------|
| 01 | [Language fundamentals](./modules/01-language-fundamentals/README.md) | 2-3h |
| 02 | [HTTP server](./modules/02-http-server/README.md) | 2h |
| 03 | [REST API design](./modules/03-rest-api-design/README.md) | 2h |
| 04 | [Serialization](./modules/04-serialization/README.md) | 1.5-2h |
| 05 | [Database access](./modules/05-database-access/README.md) | 2h |
| 06 | [Concurrency](./modules/06-concurrency/README.md) | 2h |
| 07 | [HTTP client](./modules/07-http-client/README.md) | 1.5-2h |
| 08 | [File and stream I/O](./modules/08-file-and-stream-io/README.md) | 1.5-2h |
| 09 | [Configuration](./modules/09-configuration/README.md) | 1.5h |
| 10 | [Logging and diagnostics](./modules/10-logging-and-diagnostics/README.md) | 1.5-2h |
| 11 | [Auth and security basics](./modules/11-auth-and-security-basics/README.md) | 2h |
| 12 | [Testing](./modules/12-testing/README.md) | 2h |
| 13 | [Time and standard library utilities](./modules/13-time-and-stdlib-utilities/README.md) | 2h |
| 14 | [Packaging and running](./modules/14-packaging-and-running/README.md) | 2h |

Read 06 (concurrency) before writing your own async code beyond the module 02 handlers. Crate APIs here are recent; when something does not compile, check the docs for the pinned version (`cargo doc -p <crate> --open`).

## Capstone projects
Make the tests in `starter/tests/` pass by completing `starter/`.

| Project | Exercises modules | Run the tests |
|---------|-------------------|---------------|
| [P1 Hello HTTP](./projects/P1-hello-http/README.md) | 01, 02, 10 | `cargo test -p p1-hello-http` |
| [P2 Tasks CRUD API](./projects/P2-tasks-crud-api/README.md) | 02-05 | `cargo test -p p2-tasks-crud-api` |
| [P3 File service](./projects/P3-file-service/README.md) | 02, 08 | `cargo test -p p3-file-service` |
| [P4 API aggregator](./projects/P4-api-aggregator/README.md) | 06, 07 | `cargo test -p p4-api-aggregator` |
| [P5 URL shortener](./projects/P5-url-shortener/README.md) | 03, 05, 10, 11 | `cargo test -p p5-url-shortener` |
| [P6 Job worker pool](./projects/P6-job-worker-pool/README.md) | 06, 10, 13 | `cargo test -p p6-job-worker-pool` |
| [P7 Tested service, Docker, CI](./projects/P7-tested-service-docker-ci/README.md) | 09, 10, 12, 14 | `cargo test -p p7-tested-service-docker-ci` |

## Progress checklist
- [ ] 01 Language fundamentals
- [ ] 02 HTTP server
- [ ] 03 REST API design
- [ ] 04 Serialization
- [ ] 05 Database access
- [ ] 06 Concurrency
- [ ] 07 HTTP client
- [ ] 08 File and stream I/O
- [ ] 09 Configuration
- [ ] 10 Logging and diagnostics
- [ ] 11 Auth and security basics
- [ ] 12 Testing
- [ ] 13 Time and standard library utilities
- [ ] 14 Packaging and running
- [ ] P1 - P7 capstones

## Notes
- XML and YAML are intentionally skipped in module 04 (see [PROFILE.md](./PROFILE.md)).
- Docker is described in module 14 and project P7 but is not needed to run any test.
- PostgreSQL appears only as a compile-only section in module 05.
- `modules/00-workspace-check` and `projects/P0-anchor` are infrastructure crates; ignore them.
- Maintainers: `python3 ../../scripts/verify.py rust` verifies every exercise (starter fails, solution passes), fmt, clippy, and example builds.
