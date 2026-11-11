# 09. Configuration and Environment in Go

> Estimated time: 1.5h | Difficulty: Beginner-Intermediate | Prerequisites: 01, 02, 04

## Learning Outcomes
After this module you can:
- [ ] Read environment variables correctly (unset vs empty) and parse them into typed values.
- [ ] Use the `flag` package with a private `FlagSet`, and detect which flags the user actually passed.
- [ ] Load JSON/YAML config files strictly (unknown keys are errors).
- [ ] Layer defaults < file < environment < flags explicitly, and validate the result once, reporting every problem.
- [ ] Keep secrets out of logs, JSON and `fmt` output with a `Secret` type.
- [ ] Wire a service by hand: load config once in `main`, pass dependencies as constructor/function arguments.

## 1. Concept Overview
Configuration is everything that changes between deployments without changing code: addresses, timeouts, log levels, credentials. The twelve-factor rule of thumb: **defaults in code, overrides from a file, environment for deployment-specific values and secrets, flags for one-off operator overrides**. The service should read all of it **once at startup**, merge it in a fixed order, validate it, and **fail fast** with a message that names every wrong value. After that, configuration is plain immutable data handed to the parts that need it.

```
defaults  ->  config file  ->  environment  ->  command-line flags   (later wins)
                                  |
                  validate once -> Config value -> constructors (dependency injection)
```

## 2. How Go Approaches It

| Tool | Role |
|------|------|
| `os.LookupEnv` | env access that distinguishes unset from empty (`os.Getenv` cannot) |
| `flag` (`NewFlagSet`, `Visit`) | flags; `Visit` iterates only the flags that were **set**, which is what layering needs |
| `encoding/json` (`DisallowUnknownFields`), `gopkg.in/yaml.v3` (`KnownFields(true)`) | strict file decoding into structs |
| `time.ParseDuration`, `strconv` | typed parsing with errors instead of silent zero values |
| `errors.Join` | collect all validation problems into one error |
| plain structs + constructors | dependency injection: no container, no framework |

Idiomatic: one `Config` struct built by a `Load(args, lookup, readFile)` function whose inputs are injected (so tests need no `os.Setenv`); validation in a `Validate() error` method; no global config, no `os.Getenv` scattered through handlers; no `flag.Parse` in `init`. A config library (viper, envconfig) is deliberately not used: the manual version is ~100 lines and shows the precedence rules.

## 3. Guided Examples
Run all commands from `languages/go/`. Examples 1-4 are deterministic programs; example 5 is a server.

### 3.1 Environment variables
- **Goal:** unset vs empty, typed parsing, injectable lookup.
- **Run:** `go run ./modules/09-configuration/examples/01_env_vars`
- **Code:** [examples/01_env_vars/main.go](examples/01_env_vars/main.go)
```go
type Lookup func(key string) (string, bool) // same shape as os.LookupEnv
raw, ok := lookup(key)
if !ok || raw == "" { return def, nil }
```
- **Walkthrough:** `Getenv` returns `""` for both unset and empty; `LookupEnv` tells them apart. Parse helpers take a `Lookup`, so tests pass a map instead of mutating the process environment. Parse failures name the variable.
- **Expected output:**
  ```
  Getenv:    missing="" empty=""
  LookupEnv: missing set = false | empty set = true
  port: 9090 <nil>
  default port: 8080 <nil>
  bad port: DEMO_BAD_PORT: "eighty" is not an integer
  timeout: 1.5s <nil>
  fake env port: 1
  listen on :9090
  ```

### 3.2 Flags
- **Goal:** a private `FlagSet`, a repeatable custom flag, knowing what was set.
- **Run:** `go run ./modules/09-configuration/examples/02_flags`
- **Code:** [examples/02_flags/main.go](examples/02_flags/main.go)
```go
fs := flag.NewFlagSet("tasks", flag.ContinueOnError)
fs.Var(&o.tags, "tag", "tag to include (repeatable)") // *stringList implements flag.Value
fs.Visit(func(f *flag.Flag) { set[f.Name] = true })    // only flags present in args
```
- **Walkthrough:** `ContinueOnError` returns errors instead of exiting the process, which makes parsing testable; output is redirected with `SetOutput`. The `timeout` flag keeps its default and is **not** in `set`, so a caller can tell "user said 5s" from "nobody said anything".
- **Expected output:**
  ```
  parsed: :9000 5s true [docs p1] err: <nil>
  addr set: true | timeout set: false
  error: invalid value "soon" for flag -timeout: parse error
  --- usage for -h ---
  Usage of tasks:
    -addr address
      	listen address (default "127.0.0.1:8080")
    -tag value
      	tag to include (repeatable)
    -timeout duration
      	request timeout (default 5s)
    -verbose
      	verbose logging
  help error: flag: help requested
  ```

### 3.3 Config files, strictly
- **Goal:** JSON and YAML into one struct; typos must fail.
- **Run:** `go run ./modules/09-configuration/examples/03_config_files`
- **Code:** [examples/03_config_files/main.go](examples/03_config_files/main.go)
```go
cfg := defaults()            // absent keys keep their defaults
dec := json.NewDecoder(r); dec.DisallowUnknownFields()
// yaml: dec.KnownFields(true)
```
- **Walkthrough:** decoding *on top of* a defaults struct makes missing keys harmless. A `Duration` type implementing `UnmarshalText` accepts `"2s"` in both formats. Non-strict decoding silently ignores `adr`, the classic "why is my setting not applied" bug.
- **Expected output:**
  ```
  json           addr=:9000 level=info timeout=2s tags=[a b]
  yaml           addr=:9100 level=debug timeout=750ms tags=[docs]
  json typo      error: json: unknown field "adr"
  yaml typo      error: yaml: unmarshal errors:
    line 1: field adr not found in type main.Config
  json loose     addr=127.0.0.1:8080 err=<nil> (typo ignored!)
  bad duration   error: time: invalid duration "soon"
  ```

### 3.4 Layered configuration with sources
- **Goal:** explicit precedence, remembering where each value came from, validating everything at once.
- **Run:** `go run ./modules/09-configuration/examples/04_layered_config`
- **Code:** [examples/04_layered_config/main.go](examples/04_layered_config/main.go)
```go
cfg, src := defaults()
applyFile(&cfg, src, path, readFile)  // pointers in fileConfig: nil = key absent
applyEnv(&cfg, src, lookup)
fs.Visit(func(f *flag.Flag) { ... }) // flags last, only if passed
return cfg, src, errors.Join(errs...)
```
- **Walkthrough:** the scenarios feed `Load` fake args, env and files, so the demo is deterministic and needs no real files. `APIToken` is a `Secret`, so printing it shows `[redacted]`. The `invalid` scenario shows three problems reported together.
- **Expected output:**
  ```
  == defaults only
  addr            = 127.0.0.1:8080   (default)
  log level       = info             (default)
  request timeout = 5s               (default)
  api token       = [redacted]       (default)
  == file
  addr            = :9000            (file app.yaml)
  log level       = debug            (file app.yaml)
  request timeout = 2s               (file app.yaml)
  api token       = [redacted]       (default)
  == file + env
  addr            = :9000            (file app.yaml)
  log level       = warn             (env LOG_LEVEL)
  request timeout = 2s               (file app.yaml)
  api token       = [redacted]       (env API_TOKEN)
  == file + env + flag
  addr            = 127.0.0.1:7000   (flag -addr)
  log level       = warn             (env LOG_LEVEL)
  request timeout = 2s               (file app.yaml)
  api token       = [redacted]       (default)
  == invalid
  invalid configuration:
  log level "loud": want debug|info|warn|error
  request timeout 2h0m0s: want 1ns..1m
  addr "nowhere": want host:port
  == missing file
  invalid configuration:
  config file nope.yaml: file does not exist
  ```

### 3.5 A Task Tracker service wired by hand
- **Goal:** config loaded once, dependencies passed explicitly, secret redacted, fail-fast startup.
- **Run:**
  ```bash
  API_TOKEN=s3cret ADDR=127.0.0.1:8080 go run ./modules/09-configuration/examples/05_task_service
  ```
  Stop with Ctrl-C (graceful shutdown). Without `API_TOKEN` the program refuses to start.
- **Code:** [examples/05_task_service/main.go](examples/05_task_service/main.go)
```go
cfg, err := LoadConfig(os.LookupEnv)            // the only place that touches the environment
srv := &http.Server{Handler: NewHandler(cfg, &memStore{}, logger), ...}
```
- **Walkthrough:** `main` is the *composition root*: it builds the logger, store and handler and connects them. `NewHandler` takes a small `Store` interface, so tests can pass a fake. `Secret` implements `MarshalText`, `String` and `LogValue`, so `/config` and the startup log line cannot leak the token.
- **Expected output** (`curl -s -w ' [%{http_code}]\n' ...`):
  ```
  $ curl localhost:8080/config
  {"addr":"127.0.0.1:8080","log_level":"info","api_token":"[redacted]"} [200]
  $ curl localhost:8080/tasks                                   # no token
  {"error":{"code":"unauthorized","message":"missing or invalid credentials"}} [401]
  $ curl -XPOST localhost:8080/tasks -H 'Authorization: Bearer s3cret' -d '{"title":"Write docs"}'
  {"id":1,"title":"Write docs"} [201]
  $ curl localhost:8080/tasks -H 'Authorization: Bearer s3cret'
  {"items":[{"id":1,"title":"Write docs"}]} [200]
  ```
  Startup failure (exit status 2):
  ```
  $ LOG_LEVEL=loud go run ./modules/09-configuration/examples/05_task_service
  configuration error:
  LOG_LEVEL "loud": want debug|info|warn|error
  API_TOKEN is required
  ```

## 4. Common Pitfalls

| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| `os.Getenv("X") == ""` to mean "unset" | `Getenv` cannot tell unset from empty | `os.LookupEnv`, then decide how to treat `""` |
| Flag defaults overriding env/file values | `flag.Parse` fills every flag with its default, so "not passed" looks like "passed" | `fs.Visit` (only set flags) or compare with `fs.Lookup`; apply flags last |
| `"metrics": false` in a file ignored | Decoding into plain `bool` fields cannot tell absent from `false` when merging layers | Decode onto the lower layer's struct, or use pointer fields (`*bool`) for the file shape |
| Typos silently ignored | `json.Unmarshal` skips unknown keys | `DisallowUnknownFields` / `KnownFields(true)` |
| `flag.Parse()` in `init` or global `flag.CommandLine` | Runs in tests/imports, shared global state | Private `FlagSet` parsed from `os.Args[1:]` inside `main`/`Load` |
| `os.Getenv` deep inside handlers | Hidden inputs, impossible to test, validated late | Read once in `Load`; pass a `Config` value |
| `time.Duration` field in JSON/YAML | JSON sees an integer of nanoseconds (`5` = 5ns) | Wrapper type with `UnmarshalText`, or a string field + `ParseDuration` |
| Logging the whole config at startup | A `%+v` of a struct prints the token | `Secret` type with `String/MarshalText/LogValue` |
| Returning only the first validation error | Operator fixes one, restarts, hits the next | `errors.Join` all problems; exit non-zero before listening |
| Secrets in files committed to git / in flags | Flags show in `ps`; files get committed | Secrets via env (or mounted file); never as flags |

## 5. Coming From Other Languages
- **Java/Spring `@ConfigurationProperties`, .NET `IOptions`, Python `pydantic-settings`:** Go has no binding framework. A struct, a few `Lookup` calls and `Validate()` do the same job, in one visible place. Layering order is code you can read, not framework magic.
- **Node `dotenv`/`process.env`:** env values are always strings; here they are parsed into typed fields once, with errors.
- **DI containers (Spring, ASP.NET, Dagger):** Go wires dependencies by hand in `main` (constructors take interfaces defined by the consumer). It is more typing and zero magic; a failure is a compile error, not a runtime "no bean found".
- **Rust `clap` + `config`:** same layering idea; Go's `flag` is far smaller and has no derive macros.

## 6. Exercises
Starter code and tests are in [exercises/](exercises/); reference solutions in [_solutions/](_solutions/).

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | Typed env helpers | Easy | `Lookup`, unset vs empty, typed errors | `go test ./modules/09-configuration/exercises/ex1_env_parse/` |
| 2 | Layered `Load` | Medium | precedence, `flag.Visit`, absent vs zero | `go test ./modules/09-configuration/exercises/ex2_layered/` |
| 3 | Validation and secrets | Stretch | `errors.Join` + walking error trees, redaction on every output path | `go test ./modules/09-configuration/exercises/ex3_validate_redact/` |

**Exercise 1: typed env helpers**
- **Task:** implement `String`, `Int`, `Bool`, `Duration`, `List` in [exercises/ex1_env_parse/envparse.go](exercises/ex1_env_parse/envparse.go).
- **Acceptance:** unset/blank gives the default; values are trimmed; bad values return a `*ParseError` with `Key`/`Value` and the cause (`errors.Is(err, strconv.ErrSyntax)`); negative durations rejected.
- <details><summary>Hint</summary>One private helper returning `(trimmedValue, present)` removes most of the repetition.</details>

**Exercise 2: layered `Load`**
- **Task:** implement `Load` in [exercises/ex2_layered/layered.go](exercises/ex2_layered/layered.go) following the layer rules in the doc comment.
- **Acceptance:** precedence tables; unset flags do not reset env values; a flag equal to its default still overrides; `"metrics": false` honoured; every failure satisfies `errors.Is(err, ErrInvalid)` and returns a zero `Config`.
- <details><summary>Hint</summary>Register flags with the already-merged value as default, parse, then copy back only the ones `fs.Visit` reports. Use pointer fields in the file struct.</details>

**Exercise 3: validation and secrets**
- **Task:** make `Secret` safe on all output paths, implement `Settings.Validate` and `Problems` in [exercises/ex3_validate_redact/settings.go](exercises/ex3_validate_redact/settings.go).
- **Acceptance:** `%v %+v %s %q %#v`, `json.Marshal` and both slog handlers never show the token; `Validate` reports all problems in field order without leaking values; `Problems` walks wrapped and nested joined errors.
- <details><summary>Hint</summary>`GoString` covers `%#v`, `MarshalText` covers JSON, `LogValue` covers slog. For `Problems`, type-switch on `interface{ Unwrap() []error }` and `interface{ Unwrap() error }`.</details>

## 7. Recap
- Defaults < file < env < flags, implemented in one `Load` function with injected inputs.
- Read the environment once; unset and empty are different; flags count only when passed.
- Decode files strictly; validate once and report every problem; fail before listening.
- Model secrets as a type that redacts itself on every output path.
- Wire dependencies by hand in `main`; consumers define the interfaces they need.

## 8. Self-check Questions
1. A user runs `app -timeout 5s` while `TIMEOUT=30s` is set and the default is also `5s`. What should the effective timeout be, and why does a naive `flag.Duration("timeout", 5*time.Second, "")` get this wrong?
2. Why does decoding a config file into a plain struct lose the information needed for layering, and which two techniques fix it?
3. Why is `Load(args, lookup, readFile)` easier to test than a function that calls `os.Getenv` and `os.ReadFile`?
4. Your struct has `APIToken string` and a developer logs it with `slog.Info("cfg", "cfg", cfg)`. List two code changes that prevent the leak.

## 9. Further Reading
- [`flag` package](https://pkg.go.dev/flag) (see `FlagSet.Visit`) and [`os.LookupEnv`](https://pkg.go.dev/os#LookupEnv)
- [The Twelve-Factor App: Config](https://12factor.net/config)
- [`log/slog` `LogValuer`](https://pkg.go.dev/log/slog#LogValuer)

---
**Next:** [10 Logging and Diagnostics](../10-logging-and-diagnostics/README.md)
