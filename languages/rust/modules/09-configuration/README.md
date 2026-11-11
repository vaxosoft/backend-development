# 09. Configuration and Environment in Rust

> Estimated time: 1.5h | Difficulty: Intermediate | Prerequisites: 01, 02, 04

## Learning Outcomes
After this module you can:
- [ ] Read environment variables and flags into a typed, validated config struct
- [ ] Load a TOML config file with `serde` and reject unknown keys and wrong types
- [ ] Layer sources with the precedence defaults < file < environment < command line
- [ ] Keep secrets out of logs and `Debug` output with `secrecy::SecretString`
- [ ] Wire an application by hand from its config in one place (the composition root)

## 1. Concept Overview
A service should behave differently per environment (laptop, CI, production) **without recompiling**. The twelve-factor rule: configuration lives outside the code, usually in environment variables, optionally in a file, and flags for one-off overrides. The robust pattern has four steps: **collect** each source into a partial value, **merge** them by precedence, **validate** once (reporting every problem together) and then pass an immutable, typed `Config` to the rest of the program. Secrets are configuration too, but must never be printed.

```
defaults  <  config file (TOML)  <  environment variables  <  command-line flags
                         \_____ merge field by field _____/
                                      |
                              validate -> Config  ->  build_app(&config)  (wiring)
```

## 2. How Rust Approaches It

| Piece | Role |
|-------|------|
| `std::env::var` | read one variable; `Err(NotPresent)` vs `Err(NotUnicode)` |
| `clap` (derive + `env`) | flags with typed parsing, `--help`, `--version`; `#[arg(env = "TT_ADDR")]` adds the variable fallback |
| `serde` + `toml` | file -> struct; `#[serde(default, deny_unknown_fields)]` |
| `Option<T>` per layer | "this layer has no opinion" is `None`, so merging is `higher.or(lower)` |
| `secrecy::SecretString` | redacting `Debug`, explicit `expose_secret()` |
| `thiserror` | error enums for config problems |

- **Idiomatic:** parse into types at the edge (`SocketAddr`, `Duration`, enums), never pass raw strings around; read the environment **once** at startup, not at call sites; inject the lookup (`impl Fn(&str) -> Option<String>`) so tests never touch the real environment.
- **Why a lookup function?** `std::env::set_var` is `unsafe` in Rust 2024 (it races with other threads), and this workspace forbids `unsafe`. A closure over a `HashMap` is safer *and* easier to test.
- **No config framework:** layering is hand-written here (profile choice) so the mechanism is visible; it is about 20 lines.
- **Dependency injection** is plain Rust: constructors take `Arc<dyn Trait>` values and one function wires them. No container, no reflection.

## 3. Guided Examples

### 3.1 Environment variables into a typed struct
- **Goal:** inject the lookup, parse with a generic helper, collect all errors.
- **Run:** `cargo run -p m09-configuration --example 01_env_lookup`
- **Code:** [examples/01_env_lookup.rs](examples/01_env_lookup.rs)
- **Walkthrough:**
  ```rust
  fn parse<T: FromStr>(lookup: &impl Fn(&str) -> Option<String>, key: &'static str, default: T, errors: &mut Vec<ConfigError>) -> T
  ```
  returns the default when unset, pushes a `ConfigError` (with the key and the offending value) when invalid, so the user fixes everything in one go. The last line reads the real environment; it prints `NotPresent` unless you set `TT_ADDR`.
- **Expected output:**
  ```
  defaults:  addr=127.0.0.1:8080 timeout=5s max_page=100
  overrides: addr=0.0.0.0:9000 timeout=250ms max_page=100
  invalid:   TT_ADDR="localhost": invalid socket address syntax
  invalid:   TT_TIMEOUT_MS="-1": invalid digit found in string
  invalid:   TT_MAX_PAGE="0": must be at least 1
  TT_ADDR in this process: Err(NotPresent)
  ```

### 3.2 A TOML file with serde
- **Goal:** nested sections, defaults for missing keys, strict rejection of mistakes.
- **Run:** `cargo run -p m09-configuration --example 02_toml_file`
- **Code:** [examples/02_toml_file.rs](examples/02_toml_file.rs)
- **Walkthrough:** `#[serde(default)]` on a struct fills absent keys from `Default`; `deny_unknown_fields` turns the typo `adr` into an error instead of silently using the default address; enums with `rename_all = "lowercase"` give a precise "expected one of" message. A missing file (`io::Error`) is a different failure from a parse error: report them differently.
- **Expected output:**
  ```
  full:    addr=0.0.0.0:9000 timeout=2500ms db=sqlite://tasks.db pool=5 log=Debug/Json
  partial: addr=0.0.0.0:1 timeout=5000ms db=sqlite::memory: pool=1 log=Info/Text
  empty:   addr=127.0.0.1:8080 timeout=5000ms db=sqlite::memory: pool=1 log=Info/Text
  --- typo error:
  TOML parse error at line 2, column 1
    |
  2 | adr = "0.0.0.0:1"
    | ^^^
  unknown field `adr`, expected `addr` or `request_timeout_ms`

  --- type error:
  TOML parse error at line 2, column 19
    |
  2 | max_connections = "many"
    |                   ^^^^^^
  invalid type: string "many", expected u32

  --- level error:
  TOML parse error at line 2, column 9
    |
  2 | level = "verbose"
    |         ^^^^^^^^^
  unknown variant `verbose`, expected one of `debug`, `info`, `warn`, `error`

  read error kind: NotFound
  ```

### 3.3 Flags with clap (and the env fallback)
- **Goal:** typed flags with `env`, defaults, range validation.
- **Run:** `cargo run -p m09-configuration --example 03_clap_flags`, then `TT_ADDR=0.0.0.0:9000 TT_LOG_LEVEL=warn cargo run -p m09-configuration --example 03_clap_flags`
- **Code:** [examples/03_clap_flags.rs](examples/03_clap_flags.rs)
- **Walkthrough:** `#[arg(long, env = "TT_ADDR", default_value = "127.0.0.1:8080")]` gives the order flag > variable > default inside clap. `value_parser!(u64).range(1..=60_000)` validates ranges. The example uses `try_parse_from` so errors are returned (a real `main` calls `Cli::parse()`, which prints and exits with code 2).
- **Expected output** (no `TT_*` variables set):
  ```
  $ tasktracker
    addr=127.0.0.1:8080 level=Info timeout=5000ms config=None verbose=0
  $ tasktracker --addr 0.0.0.0:9000 --log-level debug -vv
    addr=0.0.0.0:9000 level=Debug timeout=5000ms config=None verbose=2
  $ tasktracker --addr=[::1]:3000 --config tasks.toml --timeout-ms 250
    addr=[::1]:3000 level=Info timeout=250ms config=Some("tasks.toml") verbose=0
  $ tasktracker --addr localhost
    error: invalid value 'localhost' for '--addr <ADDR>': invalid socket address syntax
  $ tasktracker --timeout-ms 0
    error: invalid value '0' for '--timeout-ms <TIMEOUT_MS>': 0 is not in 1..=60000
  $ tasktracker --log-level loud
    error: invalid value 'loud' for '--log-level <LOG_LEVEL>'
  ```
  With the variables set, the first line becomes `addr=0.0.0.0:9000 level=Warn ...` and `--addr`/`--log-level` flags still win.

### 3.4 Layered configuration with validation
- **Goal:** defaults < file < env < flags, then validate once.
- **Run:** `cargo run -p m09-configuration --example 04_layered_config`
- **Code:** [examples/04_layered_config.rs](examples/04_layered_config.rs)
- **Walkthrough:** one `Partial` struct (all fields `Option`) derives both `serde::Deserialize` (file) and `clap::Parser` (flags). `lower.overridden_by(higher)` is field-wise `higher.or(lower)`. `finish()` applies defaults and validates **after** merging, so a bad value in the file that the environment overrides is not an error.
- **Expected output:**
  ```
  defaults:     127.0.0.1:8080 5s sqlite::memory: Info
  + file:       0.0.0.0:9000 5s sqlite://tasks.db Warn
  + env:        0.0.0.0:7000 250ms sqlite://tasks.db Warn
  + flags:      127.0.0.1:1234 250ms sqlite://tasks.db Warn
  bad values:   rejected: timeout_ms must be 1-60000, got 0; database_url must start with sqlite: or postgres://, got "mysql://db"
  bad env:      TT_TIMEOUT_MS="soon" is not valid
  ```

### 3.5 Secrets and manual wiring
- **Goal:** `SecretString` in config; a composition root that builds the app from it.
- **Run:** `cargo run -p m09-configuration --example 05_secrets_and_wiring`
- **Code:** [examples/05_secrets_and_wiring.rs](examples/05_secrets_and_wiring.rs)
- **Walkthrough:** `Config` derives `Debug` and still prints `[REDACTED]`; the only `expose_secret()` is inside `App::is_admin`. `build_app(&config, clock)` is the single function that knows the concrete types (`MemoryStore`, `FixedClock`); tests and demos inject a `FixedClock`, production passes `SystemClock`. Error messages say what is wrong with a secret, never its value.
- **Expected output:**
  ```
  missing:  TT_ADMIN_TOKEN is required
  too short: TT_ADMIN_TOKEN must have at least 8 characters (got 3)
  debug:    Config { addr: 127.0.0.1:8080, admin_token: SecretBox<str>([REDACTED]) }
  listens on 127.0.0.1:8080
  created:  (1, 1893456000)
  created:  (2, 1893456000)
  stored:   2
  admin?    right=true wrong=false
  system clock is sane: true
  ```

## 4. Common Pitfalls

| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| `std::env::var("X")` scattered through handlers | It is a global, easy to call anywhere, and untestable | Read once at startup into a `Config`; inject a lookup closure in tests |
| `std::env::set_var` in a test | It compiles in edition 2021 but is `unsafe` in 2024 and races with parallel tests | Pass a `HashMap`-backed lookup instead |
| `#[derive(Debug)]` on a struct holding a `String` password | Derive prints every field; one `{:?}` or `tracing` field leaks it | Store `SecretString`; reveal with `expose_secret()` at the single use site |
| Defaults applied before merging | A lower layer's default then "overrides" nothing and masks real values | Layer `Option` fields, apply defaults last in `finish()` |
| Typos silently ignored | serde ignores unknown keys by default | `#[serde(deny_unknown_fields)]` on every config struct |
| `.ok()` on `env::var` hides `NotUnicode`; an empty `PORT=` becomes a parse error | Both look like "not set" to the eye | Treat blank as unset on purpose; report non-Unicode explicitly |
| Fail on the first error | `?` stops at the first problem | Collect all errors into a `Vec` and print them together |
| Secret in the file in git / in `argv` | Convenient; visible in `ps` and history | Environment or `*_FILE` secret files; never commit them (exercise 3) |
| Validating before merge | A bad file value fails even when overridden | Validate the merged result once |

## 5. Coming From Other Languages
- **Go:** `os.Getenv` + `flag` + a hand-rolled struct; here `clap` replaces `flag`, `serde` replaces `encoding/json`-style tags, and typed parsing happens in the type system instead of `strconv`.
- **Python:** `pydantic-settings`/`argparse` combined; Rust's `Option` layers play the role of "unset" sentinels.
- **TypeScript:** `dotenv` + `zod` validation; here `toml` + `serde` + the compiler give the schema.
- **Java/C#:** `IConfiguration` / Spring `@ConfigurationProperties` are frameworks that do layering and binding; Rust does it with ~20 lines and no reflection. Their DI containers correspond to the manual composition root in 3.5.
- **C++:** `getenv` returns a raw pointer or null; `env::var` returns a `Result`.

## 6. Exercises
All tests are pure (injected lookups; `tempfile` only in exercise 3).

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | Env config | Easy | typed parsing, defaults, collecting errors | `cargo test -p m09-ex1-env-config` |
| 2 | Layered settings | Medium | serde TOML layer, merge precedence, validate-after-merge | `cargo test -p m09-ex2-layered-settings` |
| 3 | Secret sources | Stretch | `*_FILE` secrets, `SecretString`, redaction, no leaks in errors | `cargo test -p m09-ex3-secret-sources` |

**Exercise 1: `AppConfig::from_lookup`** ([starter](exercises/ex1-env-config/src/lib.rs), [tests](exercises/ex1-env-config/tests/env_config.rs))
- **Task:** implement `parse_duration` (`500ms`, `30s`, `2m`), `parse_bool`, and `from_lookup` (defaults, one required key, trimming, blank = unset, all errors in key order).
- **Acceptance:** all eight tests pass.
<details><summary>Hint</summary>Split the string at the first non-digit; collect errors in a <code>Vec</code> and return the config only if it is empty.</details>

**Exercise 2: layered `Settings`** ([starter](exercises/ex2-layered-settings/src/lib.rs), [tests](exercises/ex2-layered-settings/tests/layered.rs))
- **Task:** `Layer::from_toml`, `from_lookup`, `merge`, `resolve` (defaults and range checks) and `load` (file < env < cli).
- **Acceptance:** precedence per field, validation after merging, unknown TOML keys and bad env values reported by name.
<details><summary>Hint</summary><code>higher.field.or(self.field)</code> for each field; <code>toml::from_str</code> with <code>deny_unknown_fields</code> is already declared.</details>

**Exercise 3: `load_secret` and `DbConfig`** ([starter](exercises/ex3-secret-sources/src/lib.rs), [tests](exercises/ex3-secret-sources/tests/secrets.rs))
- **Task:** `NAME` or `NAME_FILE` (one trailing newline stripped), conflict and empty detection, and a `DbConfig` whose password stays a `SecretString` (connection string is a secret; `redacted_url` is for logs).
- **Acceptance:** nothing prints the password: not `Debug`, not errors.
<details><summary>Hint</summary><code>strip_suffix('\n')</code> then <code>strip_suffix('\r')</code> removes exactly one line ending; call <code>expose_secret()</code> only inside the two methods that need it.</details>

Reference solutions: [solutions/](solutions/) (overlay onto `exercises/`).

## 7. Recap
- Parse configuration into types once at startup; pass the typed `Config` around.
- Layer `Option`s, merge by precedence, validate the merged result, report every error.
- `deny_unknown_fields` and typed enums turn typos into startup failures instead of production surprises.
- Wrap secrets in `SecretString`; expose them in one place; keep them out of errors and logs.
- Wire dependencies by hand in one `build_app`; swap fakes in tests.

## 8. Self-check Questions
1. Why does the merge step use `Option` fields rather than the final types with defaults?
2. A value is invalid in the TOML file but correct in `TT_*` env. Should startup fail? Where does your code decide?
3. What does `#[serde(default)]` do on a struct, and what does `deny_unknown_fields` add?
4. Why is the environment lookup injected as a closure instead of calling `env::var` directly?
5. Name two ways a `String` secret can leak that a `SecretString` prevents, and one way it cannot.

## 9. Further Reading
- [clap derive tutorial](https://docs.rs/clap/latest/clap/_derive/_tutorial/index.html)
- [serde container attributes](https://serde.rs/container-attrs.html) and [`toml`](https://docs.rs/toml)
- [`secrecy`](https://docs.rs/secrecy); The Twelve-Factor App, "III. Config"

---
**Next:** [10 Logging and Diagnostics](../10-logging-and-diagnostics/README.md)
