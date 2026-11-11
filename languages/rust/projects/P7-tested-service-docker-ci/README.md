# P7: Tested Service with Dockerfile and CI

Take a small Tasks service and make it *shippable*: validated configuration, liveness/readiness/metrics endpoints, a multi-stage non-root Dockerfile, and a CI pipeline, with tests that pin all of it down.

Modules exercised: [09 Configuration](../../modules/09-configuration/README.md), [10 Logging and diagnostics](../../modules/10-logging-and-diagnostics/README.md), [12 Testing](../../modules/12-testing/README.md), [14 Packaging and running](../../modules/14-packaging-and-running/README.md).

> Docker is not run by the tests (and may not be installed on your machine). The Dockerfile and the workflow are checked as **text** by the "repo hygiene" tests, which is a legitimate technique: rules that live in a file can be asserted from a test.

## What is given and what you write

| Path | State | Content |
|------|-------|---------|
| `src/lib.rs`, `src/tasks.rs` | given | `AppState`, `build_app`, `/healthz`, `GET/POST /tasks`, `GET /tasks/{id}`, `TaskStore` trait + `MemoryStore` |
| `src/probe.rs`, `src/main.rs` | given | `probe` (a std-only `GET /healthz` client), startup, graceful shutdown, `healthcheck` subcommand |
| `src/config.rs` | **TODO** | `Config::from_lookup`: validate `PORT`, `LOG_LEVEL`, `ENV` |
| `src/ops.rs` | **TODO** | `/readyz`, `/metrics`, request-counting middleware |
| `Dockerfile`, `.dockerignore`, `.github/workflows/ci.yml` | **missing** | create them in `starter/` |

## Requirements

### Endpoints (all JSON errors use the shared envelope)

| Request | Response |
|---------|----------|
| `GET /healthz` | `200 {"status":"ok"}` (liveness, always) |
| `GET /readyz` | `503 {"status":"not_ready"}` until `AppState::set_ready(true)`, then `200 {"status":"ready"}` |
| `GET /metrics` | `200`, `Content-Type: text/plain; charset=utf-8`, body `requests_total <n>\n`; `n` counts **every** request seen so far, including this one |
| `POST /tasks` `{"title"}` | `201` + `Location: /tasks/{id}`; title trimmed, 1-200 chars else `422 validation_failed`; malformed JSON `400 bad_request` |
| `GET /tasks/{id}`, `GET /tasks` | `200` task / `{"items":[...],"total":N}`; unknown id `404 not_found` |

### Configuration

| Variable | Valid | Default |
|----------|-------|---------|
| `PORT` | integer 1-65535 | `8080` |
| `LOG_LEVEL` | `debug`, `info`, `warn`, `error` | `info` |
| `ENV` | `dev`, `staging`, `prod` | `dev` |

An unset variable means the default; a set-but-invalid one makes `from_lookup` return `ConfigError { var, value, expected }` whose message names all three: `invalid PORT="abc": expected an integer between 1 and 65535`. `main` prints it and exits with status 2 before binding anything. `from_lookup` takes a closure instead of reading `std::env`, so tests need no global state.

### Packaging (files in `starter/`)

- `Dockerfile`: at least two `FROM`s, the first named (`AS build`) and running `cargo build --release --locked`; the final stage is a minimal runtime image (not `rust:*`, no compiler) that `COPY --from=build`s the binary, runs as a **non-root** `USER`, has `EXPOSE <port>` and a `HEALTHCHECK` (the binary's `healthcheck` subcommand, because slim images have no `curl`). Every base image has an explicit version tag (no `latest`).
- `.dockerignore`: at least `target` and `.git`.
- `.github/workflows/ci.yml`: runs on `push` and `pull_request`; steps build, `cargo fmt -- --check`, `cargo clippy -- -D warnings`, `cargo test`; installs a **pinned** toolchain (`toolchain: "<major.minor>"`) equal to the version in the Dockerfile's `rust:<version>-...` tag.

Docker reads `.dockerignore` and resolves `COPY` from the *build context*. This crate takes its dependency versions from the workspace manifest, so in this repository the context is `languages/rust/`:

```bash
docker build -f projects/P7-tested-service-docker-ci/starter/Dockerfile -t tasks-service .
docker run --rm -p 8080:8080 tasks-service
```

For a real build, put the `.dockerignore` rules in the context root (or name the file `Dockerfile.dockerignore` next to the Dockerfile). GitHub only runs workflows from the repository root's `.github/workflows/`; in this repo that is where you would copy `ci.yml`.

## Run and test

From `languages/rust/`:

```bash
ADDR=127.0.0.1:8080 cargo run -p p7-tested-service-docker-ci
curl -i localhost:8080/readyz
curl -i -X POST localhost:8080/tasks -d '{"title":"Write docs"}'
curl localhost:8080/metrics

PORT=abc cargo run -p p7-tested-service-docker-ci     # configuration error, exit status 2
cargo test -p p7-tested-service-docker-ci
```

`ADDR` (host:port) overrides the listen address; without it the service listens on `0.0.0.0:$PORT`, which is what a container needs.

## Acceptance criteria

| Criterion | Tests |
|-----------|-------|
| Tasks service works (given, must keep passing) | `created_task_is_returned_by_get_and_list`, `unknown_task_is_404_with_error_envelope`, `empty_title_is_422_with_field_message`, `healthz_always_answers_ok`, `probe_succeeds_against_a_running_server` |
| Readiness | `readyz_is_503_until_the_app_is_ready`, `readyz_is_200_once_the_app_is_ready` |
| Metrics | `metrics_is_plain_text_in_the_expected_format`, `metrics_counts_every_request_including_its_own` |
| Configuration defaults and valid values | `empty_environment_gives_the_defaults`, `valid_values_are_read` |
| Invalid configuration fails with a clear error | `non_numeric_port_is_rejected_with_a_message_naming_variable_and_value`, `port_outside_1_to_65535_is_rejected`, `unknown_log_level_is_rejected`, `unknown_environment_name_is_rejected` |
| Dockerfile rules | `dockerfile_is_multi_stage_with_a_named_build_stage`, `build_stage_compiles_a_locked_release_binary`, `final_stage_is_a_minimal_runtime_image_that_copies_the_binary`, `base_images_are_pinned_to_a_version`, `final_stage_runs_as_a_non_root_user`, `final_stage_documents_the_port_and_a_health_probe` |
| `.dockerignore` | `dockerignore_keeps_build_output_and_git_data_out_of_the_context` |
| CI rules | `ci_runs_on_push_and_pull_request`, `ci_checks_build_format_lint_and_tests`, `ci_pins_the_same_toolchain_as_the_dockerfile_builds_with` |

In the starter the behaviour tests for tasks pass, everything else fails (`PORT=abc` is silently accepted, `/readyz` is a 404, the files do not exist).

## Suggested steps

1. `config.rs`: make the six config tests pass.
2. `ops.rs`: `/readyz`, `/metrics`, the middleware.
3. `Dockerfile` and `.dockerignore`, driven by the hygiene tests (read the failure messages).
4. `ci.yml`.

<details>
<summary>Hints</summary>

- `raw.parse::<u16>()` already rejects `-1` and `65536`; `0` needs an extra check.
- A small generic helper `choice(var, raw, default, &[("debug", LogLevel::Debug), ...])` removes the duplication between `LOG_LEVEL` and `ENV`.
- The counter lives in `AppState` (`count_request`, `requests_total`); a middleware added with `from_fn_with_state` in `build_app` already wraps every route.
- `([(header::CONTENT_TYPE, "text/plain; charset=utf-8")], body)` is a complete response.
- Dockerfile skeleton: `FROM rust:<ver>-slim-bookworm AS build`, `COPY . .`, `RUN cargo build --release --locked -p p7-tested-service-docker-ci`; then `FROM debian:bookworm-slim`, `RUN useradd ...`, `COPY --from=build ...`, `USER`, `EXPOSE`, `HEALTHCHECK CMD ["tasks-service","healthcheck"]`, `ENTRYPOINT`.
- The env var named `ENV` collides with the Dockerfile keyword only visually: `ENV ENV=prod` is valid.
- Workflow skeleton: `actions/checkout`, `dtolnay/rust-toolchain@master` with `toolchain:` and `components: rustfmt, clippy`, then one `run:` per check. The crate is part of a workspace, so set `defaults.run.working-directory: languages/rust`.
</details>

## Stretch goals (not tested)

- Report **all** invalid variables at once instead of the first.
- Add a dependency-caching layer (a dummy `main.rs` build before copying the sources) and measure the rebuild time.
- Build for `x86_64-unknown-linux-musl` and end on `FROM scratch`/`distroless` (what changes for `useradd` and the `HEALTHCHECK`?).
- Add `cargo audit`/`cargo deny` and a `cargo build --release` artifact upload to the workflow.
- A `/readyz` that turns 503 again while the service drains (the given `main` already calls `set_ready(false)` on shutdown).

## Reference solution

`solution/` contains `src/config.rs`, `src/ops.rs`, `Dockerfile`, `.dockerignore` and `.github/workflows/ci.yml`; copy them over `starter/` after your own attempt.
