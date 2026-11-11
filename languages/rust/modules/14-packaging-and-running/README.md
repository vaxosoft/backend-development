# 14. Packaging and Running in Rust

> Estimated time: 2h | Difficulty: Intermediate | Prerequisites: 02, 09 (configuration), 10 (logging), 12 (testing)

## Learning Outcomes
After this module you can:
- [ ] Build optimised release binaries, tune the release profile and embed build metadata
- [ ] Cross-compile for another target (for example a static `musl` binary)
- [ ] Write a multi-stage Dockerfile with layer caching, a non-root user and a health check
- [ ] Write a CI pipeline that builds, formats, lints and tests with a pinned toolchain and `--locked`
- [ ] Accept runtime flags and environment variables with clear precedence, validate them at start-up, and shut down on SIGTERM

> **Docker is not available on the machine this module was written on.** The Dockerfile, `.dockerignore` and CI file in [examples/deploy/](./examples/deploy) are written and explained but were **not run**. Exercise 3 checks Dockerfile rules as text, and the shipped Dockerfile passes that checker.

## 1. Concept Overview
Shipping a service means producing one reproducible artifact and running it the same way everywhere. Rust makes the first half easy: `cargo build --release` yields a single native binary with no runtime to install. What remains is **reproducibility** (pin the toolchain, commit `Cargo.lock`, build with `--locked`), **small, safe images** (multi-stage build, no compiler in production, non-root user), **observability of the artifact** (version and commit in the binary, health and readiness probes) and **runtime configuration** (flags and environment, validated before serving, graceful shutdown on SIGTERM because that is how orchestrators stop containers).

```
 source + Cargo.lock ──cargo build --release --locked──▶ binary ──COPY──▶ minimal image ──▶ registry ──▶ orchestrator
        ▲                                                                       │
   CI: fmt, clippy, test                                         HEALTHCHECK / probes, SIGTERM, flags + env
```

## 2. How Rust Approaches It
| Concern | Rust tool |
|---------|-----------|
| Optimised build | `cargo build --release` (profile `release`; output in `target/release/`) |
| Profile tuning | `[profile.release]`: `lto`, `codegen-units`, `strip`, `opt-level`, `panic` (workspace root manifest only) |
| Reproducibility | `rust-toolchain.toml` (channel), committed `Cargo.lock`, `--locked` on every cargo command in CI/Docker |
| Install a binary | `cargo install --path . --locked` (into `~/.cargo/bin`) |
| Cross-compile | `rustup target add x86_64-unknown-linux-musl` then `cargo build --release --target x86_64-unknown-linux-musl`; for C dependencies/linkers use `cross` or `cargo-zigbuild` |
| Build metadata | `env!("CARGO_PKG_VERSION")`, `option_env!`, and a `build.rs` that prints `cargo:rustc-env=KEY=value` |
| Flags + env | `clap` derive with `env = "NAME"` (flag > env > default) |
| Signals | `tokio::signal::{ctrl_c, unix::signal}` plus axum `with_graceful_shutdown` |

Release profile for a standalone service:
```toml
[profile.release]
opt-level = 3
lto = "thin"          # cross-crate inlining; "fat" is slower to build, rarely much faster
codegen-units = 1     # better optimisation, slower build
strip = "symbols"     # drop debug symbols: a much smaller binary
# panic = "abort"     # smaller still, but CatchPanicLayer (module 10) can no longer turn a panic into a 500
```
Two common runtime images: **distroless `cc`** (glibc, tiny, no shell; the example) or **`scratch` with a `musl` static binary** (smallest, but no CA certificates or timezone data unless you copy them in).

## 3. Guided Examples
Run all commands from `languages/rust/`.

### 3.1 Build info
- **Goal:** know what is running: version, profile, target, commit.
- **Run:** `cargo run -q -p m14-packaging-and-running --example build_info`, then `GIT_SHA=3f2a9c1 cargo run -q --release -p m14-packaging-and-running --example build_info`
- **Code:** [build.rs](./build.rs), [src/lib.rs](./src/lib.rs), [examples/build_info.rs](./examples/build_info.rs)
- **Walkthrough:** cargo runs `build.rs` first and gives it `TARGET` and `PROFILE`; printing `cargo:rustc-env=BUILD_TARGET=...` makes them visible to `env!("BUILD_TARGET")` in the crate. The commit comes from `option_env!("GIT_SHA")`, which CI sets (`GIT_SHA=$(git rev-parse --short HEAD) cargo build --release`). `rerun-if-env-changed` makes cargo rebuild when it changes.
  ```rust
  pub const BUILD: BuildInfo = BuildInfo { version: env!("CARGO_PKG_VERSION"), profile: env!("BUILD_PROFILE"), git_sha: option_env!("GIT_SHA"), .. };
  ```
- **Expected output** (x86_64 Linux; the target line differs on other machines):
  ```
  # debug
  name:    m14-packaging-and-running
  version: 0.1.0
  profile: debug
  target:  x86_64-unknown-linux-gnu
  commit:  unknown
  debug assertions: true
  pointer width: 64 bits

  # --release with GIT_SHA=3f2a9c1
  name:    m14-packaging-and-running
  version: 0.1.0
  profile: release
  target:  x86_64-unknown-linux-gnu
  commit:  3f2a9c1
  debug assertions: false
  pointer width: 64 bits
  ```

### 3.2 Runtime flags and environment
- **Goal:** one `Settings` struct filled from defaults, environment and flags, validated by the parser.
- **Run:** `cargo run -q -p m14-packaging-and-running --example flags_and_env -- <flags>`
- **Code:** [examples/flags_and_env.rs](./examples/flags_and_env.rs)
- **Walkthrough:** `#[arg(long, env = "ADDR", default_value = ...)]` gives `--addr`, `ADDR` and a default in one line, with that precedence. Types do the validation: `SocketAddr` rejects `nonsense`, a `ValueEnum` rejects `trace`, `value_parser!(u16).range(1..=256)` rejects `0`. Invalid input exits with status 2 *before* anything starts. (Exercise 2 hand-rolls the same precedence with an injected environment so it can be tested without touching real process state.)
  ```rust
  #[arg(long, env = "WORKERS", default_value_t = 4, value_parser = clap::value_parser!(u16).range(1..=256))]
  workers: u16,
  ```
- **Expected output:**
  ```
  $ cargo run -q ... --example flags_and_env
  Settings {
      addr: 127.0.0.1:8080,
      log_level: Info,
      workers: 4,
      shutdown_timeout_secs: 10,
  }

  $ ADDR=0.0.0.0:9000 WORKERS=8 cargo run -q ... --example flags_and_env
  Settings {
      addr: 0.0.0.0:9000,
      log_level: Info,
      workers: 8,
      shutdown_timeout_secs: 10,
  }

  $ ADDR=0.0.0.0:9000 cargo run -q ... --example flags_and_env -- --addr 127.0.0.1:7000 --log-level warn
  Settings {
      addr: 127.0.0.1:7000,
      log_level: Warn,
      workers: 4,
      shutdown_timeout_secs: 10,
  }

  $ cargo run -q ... --example flags_and_env -- --workers 0      # exit status 2
  error: invalid value '0' for '--workers <WORKERS>': 0 is not in 1..=256

  For more information, try '--help'.

  $ cargo run -q ... --example flags_and_env -- --log-level trace      # exit status 2
  error: invalid value 'trace' for '--log-level <LOG_LEVEL>'
    [possible values: debug, info, warn, error]

  For more information, try '--help'.
  ```

### 3.3 Dockerfile and CI (written, not run)
- **Goal:** read and understand the deployment files.
- **Files:** [examples/deploy/Dockerfile](./examples/deploy/Dockerfile), [examples/deploy/.dockerignore](./examples/deploy/.dockerignore), [examples/deploy/ci.yml](./examples/deploy/ci.yml)
- **Walkthrough:**
  - **Two stages.** `build` has the compiler and is thrown away; the final stage is distroless with only the binary.
  - **Layer caching without extra tools.** Copy `Cargo.toml` + `Cargo.lock`, build a dummy `main.rs` (this downloads and compiles all dependencies into a cached layer), then copy the real `src/` and build again. Dependencies are rebuilt only when the manifests change. `touch src/main.rs` makes cargo see the sources as newer than the dummy build.
  - **`--locked`** everywhere: the build fails instead of silently updating `Cargo.lock`.
  - **Non-root:** `USER nonroot:nonroot`. **`.dockerignore`** keeps `target/` (gigabytes) and `.git` out of the build context.
  - **Health check without curl:** the binary has a `--healthcheck` mode (see 3.4); distroless images have no shell or `curl`.
  - **CI:** triggers on push and pull request, pins the toolchain (keep it equal to `rust-toolchain.toml` and the Dockerfile `FROM`), then `fmt --check`, `clippy -D warnings`, `test --locked`, a release build, and an image build job.
- **Check it:** the exercise-3 linter accepts the shipped Dockerfile (the solution's tests were run against it while writing this module).

### 3.4 Task Tracker service as a deployable unit
- **Goal:** probes, version endpoint, runtime settings and signal handling in one binary.
- **Run:** `ADDR=127.0.0.1:20710 cargo run -q -p m14-packaging-and-running --example tracker_service` (stop with Ctrl-C or `kill -TERM <pid>`)
- **Code:** [examples/tracker_service.rs](./examples/tracker_service.rs)
- **Walkthrough:**
  - The Tokio runtime is built by hand (`Builder::new_multi_thread().worker_threads(n)`) so the worker count is a setting.
  - `/healthz` says the process is alive; `/readyz` flips to `503` as soon as shutdown starts so a load balancer stops sending traffic, then `with_graceful_shutdown` lets in-flight requests finish, bounded by `SHUTDOWN_TIMEOUT_SECS`.
  - `--healthcheck` probes `GET /healthz` with plain `std::net::TcpStream` and exits `0` or `1`: that is what the Dockerfile `HEALTHCHECK` runs.
- **Try it:**
  ```bash
  curl -si http://127.0.0.1:20710/healthz
  curl -si http://127.0.0.1:20710/readyz
  curl -si http://127.0.0.1:20710/version
  curl -s  http://127.0.0.1:20710/tasks
  ADDR=127.0.0.1:20710 target/debug/examples/tracker_service --healthcheck; echo "exit=$?"
  ```
- **Expected output** (headers trimmed to the status line and content type):
  ```
  HTTP/1.1 200 OK
  content-type: text/plain; charset=utf-8

  ok
  HTTP/1.1 200 OK
  content-type: text/plain; charset=utf-8

  ready
  HTTP/1.1 200 OK
  content-type: application/json

  {"name":"m14-packaging-and-running","profile":"debug","version":"0.1.0"}
  {"items":[{"id":1,"status":"todo","title":"Write docs"}],"total":1}
  exit=0
  ```
  On the server terminal, after Ctrl-C or SIGTERM:
  ```
  m14-packaging-and-running 0.1.0 (debug) listening on 127.0.0.1:20710
  shutting down (up to 5s for in-flight requests)
  bye
  ```
  (`--healthcheck` against a port where nothing listens exits `1`.)

## 4. Common Pitfalls
| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| `COPY . .` then `cargo build` in Docker: every code change re-downloads and recompiles all dependencies | The dependency layer is invalidated by any source change | Copy manifests first, build a dummy `main.rs`, then copy `src/` (see the Dockerfile) |
| Build context is gigabytes / image contains `target/` | No `.dockerignore`; `target/` is huge | `.dockerignore` with `target/` and `.git/`; copy only the binary into the final stage |
| Builds differ between laptop and CI | Floating toolchain, `Cargo.lock` ignored or silently updated | `rust-toolchain.toml`, commit `Cargo.lock`, `--locked` in CI and Docker |
| Binary from `rust:` image fails with `GLIBC_x.y not found` in a smaller image | Dynamically linked against the builder's glibc | Same distro family for build and runtime (distroless `cc-debian12`), or a static `musl` build |
| `musl` image: TLS/HTTPS fails or timezones are missing | `scratch` has no CA bundle or tzdata | Copy `/etc/ssl/certs` (or use `rustls` with `webpki-roots`) and zoneinfo, or keep distroless |
| Panics no longer become 500s after `panic = "abort"` | Abort kills the process, `CatchPanicLayer` cannot catch | Keep `panic = "unwind"` for servers |
| Profile settings in a member crate are ignored | Cargo only reads `[profile.*]` from the workspace root | Put them in the root `Cargo.toml` |
| Container ignores `docker stop` for 10 s, then is killed | The process runs as PID 1 and has no SIGTERM handler, or the shell form `CMD cmd` wraps it in `sh` | Handle SIGTERM (`tokio::signal`) and use the exec form `ENTRYPOINT ["/bin/app"]` |
| Listening on `127.0.0.1` inside a container | Default dev address; the port mapping cannot reach loopback | `ADDR=0.0.0.0:8080` in the image |
| Benchmarking or load-testing the debug build | `cargo run` defaults to the unoptimised `dev` profile | `--release` |

## 5. Coming From Other Languages
- **Go:** `go build` also yields one binary, but Go links statically by default; Rust links glibc dynamically unless you target `musl`. `-ldflags "-X main.version=..."` ↔ `build.rs` + `env!`/`option_env!`. `GOOS/GOARCH` ↔ `--target`.
- **Java/C#:** no runtime or `.jar`/`publish` step: the release binary *is* the artifact, so the runtime image can be a few MB. `Directory.Packages.props`/Maven pinning ↔ `Cargo.lock` + `--locked`.
- **Python/TypeScript:** no virtualenv, `node_modules` or interpreter in the image; the cost moves to build time (so cache dependencies as in the Dockerfile).
- **C++:** cross-compilation needs a linker for the target (`cross` or `cargo-zigbuild` packages one), as with any native toolchain, but cargo handles dependencies.

## 6. Exercises
Starters compile and fail with `todo!()`; solutions are in [solutions/](./solutions). No Docker or network is needed.

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | Release versions and image tags | Easy | Strict version parsing, tagging policy for stable vs pre-release | `cargo test -p m14-ex1-release-versions` |
| 2 | Runtime settings | Medium | flag > env > default precedence, validation, production rules, injected environment | `cargo test -p m14-ex2-runtime-settings` |
| 3 | Dockerfile linter | Stretch | Parse a Dockerfile (continuations, stages) and enforce the multi-stage / pinned / non-root / health-check / `--locked` rules | `cargo test -p m14-ex3-dockerfile-lint` |

**Exercise 1: [exercises/ex1-release-versions](./exercises/ex1-release-versions)**
- **Task:** implement `Version::parse`, `Display`, `bump` and `image_tags`.
- **Acceptance:** `v1.4.2` and `1.5.0-rc.1` parse; leading zeros, missing parts, `+build` and junk are rejected; stable releases produce `1.4.2, 1.4, 1[, latest]`; pre-releases only their exact tag.
- <details><summary>Hint</summary>`split_once('-')` separates the pre-release from the numbers, so a pre-release may itself contain dashes.</details>

**Exercise 2: [exercises/ex2-runtime-settings](./exercises/ex2-runtime-settings)**
- **Task:** implement `resolve(args, env)` returning `Settings` or a precise `SettingsError`.
- **Acceptance:** defaults differ per environment; env beats default; flags beat env (`--k v` and `--k=v`); bad flags and values are reported with the field name; production rejects `debug` logging whichever source supplied it.
- <details><summary>Hint</summary>Parse the flags into a map first, then for each setting do `flags.get(k).or_else(|| env(VAR))`, and validate the combined result last.</details>

**Exercise 3: [exercises/ex3-dockerfile-lint](./exercises/ex3-dockerfile-lint)**
- **Task:** implement `lint(dockerfile) -> Vec<Finding>` for five rules.
- **Acceptance:** a good Dockerfile has no findings; a naive single-stage one trips every rule with the right line numbers; only the final stage decides `USER` and `HEALTHCHECK`; continuation lines, comments, lowercase keywords and `--platform` are handled.
- <details><summary>Hint</summary>First turn the text into `(line, KEYWORD, args)` instructions by joining lines that end in `\`; every rule is then a simple pass over that list.</details>

## 7. Recap
- `cargo build --release --locked` + a committed `Cargo.lock` + a pinned toolchain = reproducible artifact.
- Multi-stage Docker builds ship only the binary; cache dependencies via the dummy-`main` layer; run as non-root.
- Bake version/commit into the binary and expose `/healthz`, `/readyz`, `/version`.
- Flags > env > defaults, validated at start-up; handle SIGTERM and drain.
- Reach for a `musl`/`scratch` image when size matters most, distroless `cc` when you want fewer surprises.

## 8. Self-check Questions
1. Why does the Dockerfile build a dummy `main.rs` first, and what exactly invalidates each of its two build layers?
2. What does `--locked` protect you from, and where should it appear?
3. Why should `/readyz` fail *before* the server stops accepting connections during shutdown?
4. A teammate sets `panic = "abort"` in the member crate's `Cargo.toml` to shrink the binary. Name two reasons that is a bad idea.

## 9. Further Reading
- The Cargo Book: Profiles, Build Scripts, Cross-compilation; `rustup target`
- Docker docs: multi-stage builds, `HEALTHCHECK`, `.dockerignore`; GitHub Actions docs for `rust-toolchain` and caching
- [`clap` derive tutorial](https://docs.rs/clap/latest/clap/_derive/index.html)

---
**Next:** [Rust track overview](../../README.md)
