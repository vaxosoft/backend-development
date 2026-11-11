# 14. Packaging and Running in Go

> Estimated time: 1.5-2h | Difficulty: Intermediate | Prerequisites: 02, 09, 10, 12

## Learning Outcomes
After this module you can:
- [ ] Build a static, stamped binary with `go build` (`CGO_ENABLED`, `-trimpath`, `-ldflags "-s -w -X ..."`) and cross-compile with `GOOS`/`GOARCH`
- [ ] Read build metadata at run time with `debug.ReadBuildInfo` and expose it as a version endpoint
- [ ] Embed templates and seed data into the binary with `//go:embed`
- [ ] Write a multi-stage, non-root Dockerfile with a cached dependency layer and a health check
- [ ] Describe a basic CI pipeline (restore, format, vet, test, build, image) and configure runtime flags and production settings

## 1. Concept Overview
Packaging turns source into one artifact that behaves identically on a laptop, in CI and in production. The steps are always the same: **build** reproducibly, **stamp** the artifact with what it is (version, commit), **ship** it in the smallest image that runs it safely (non-root, no shell), **configure** it at start-up (flags and environment, validated), and let **CI** run the same commands on every push.

```
source -> go build (CGO off, ldflags) -> static binary -> multi-stage Docker -> registry -> run with ADDR / flags
                         ^                                      ^
                  CI: gofmt, vet, test, build            HEALTHCHECK / readiness probe
```

Docker is **not** installed on the machine this course was written on: the Dockerfile and the CI workflow here are written, explained and linted by an exercise, but not executed. Everything else runs.

## 2. How Go Approaches It
| Need | Tool |
|------|------|
| Compile | `go build -o bin/app ./cmd/app`; the result is one self-contained executable |
| Static binary | `CGO_ENABLED=0` (no libc dependency, so it runs on `scratch` or distroless) |
| Smaller, reproducible | `-trimpath` (no local paths), `-ldflags "-s -w"` (drop symbol table and DWARF) |
| Stamp the version | `-ldflags "-X main.version=1.2.3"` on a package-level `string` variable, or `debug.ReadBuildInfo()` (module version, `vcs.revision`, `vcs.time`, `vcs.modified`) |
| Cross-compile | `GOOS=linux GOARCH=arm64 go build ...` (list targets: `go tool dist list`) |
| Bundle files | `//go:embed` into `string`, `[]byte` or `embed.FS` |
| Settings | `flag`, `os.Getenv`, validated once at start-up; runtime knobs `GOMEMLIMIT`, `GOMAXPROCS`, `debug.SetMemoryLimit` |
| Image | multi-stage Dockerfile: `golang` build stage, `distroless/static:nonroot` (or `scratch`) final stage |
| Pipeline | GitHub Actions: `actions/setup-go` with `go-version-file: go.mod`, then `gofmt`, `go vet`, `go test -race`, `go build`, `docker build` |

Idioms: put `main` in its own package so the application code is importable and testable; keep `Parse(args, getenv, out)`-style functions free of `os` globals; build with the toolchain pinned by `go.mod`; ship the same artifact you tested. No third-party packages are used in this module.

## 3. Guided Examples

### 3.1 Version stamping and release builds
- **Goal:** link-time variables, build info, cross-compiling.
- **Run:** `go run ./modules/14-packaging-and-running/examples/01_version`, then
  `CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags "-s -w -X main.version=1.2.3 -X main.commit=abc123" -o bin/version ./modules/14-packaging-and-running/examples/01_version && ./bin/version`
- **Code:** [examples/01_version/main.go](examples/01_version/main.go)
  ```go
  var (
      version = "dev"   // go build -ldflags "-X main.version=1.2.3"
      commit  = "none"
  )
  info, ok := debug.ReadBuildInfo()
  ```
- **Walkthrough:** `-X` overwrites an uninitialised-by-function `string` variable in the named package (`main.version`, or a full import path such as `webdev/internal/build.Version`). `go run` and `go build -buildvcs=false` do not stamp VCS data; a plain `go build` inside a git checkout adds `vcs.revision`, `vcs.time` and `vcs.modified`, which is why the demo disables it for a reproducible output. `go version -m bin/version` prints the same metadata for any binary. Cross-compile with e.g. `GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go build -o bin/version.exe ...`; you cannot run it here, but it builds.
- **Expected output:**
  ```
  version=dev commit=none date=unknown
  main module: webdev
  package: webdev/modules/14-packaging-and-running/examples/01_version
  vcs revision: not stamped
  ```
  and for the release build:
  ```
  version=1.2.3 commit=abc123 date=unknown
  main module: webdev
  package: webdev/modules/14-packaging-and-running/examples/01_version
  vcs revision: not stamped
  ```

### 3.2 Runtime flags and production settings
- **Goal:** defaults < environment < flags, validated before anything starts.
- **Run:** `go run ./modules/14-packaging-and-running/examples/02_runtime_flags -addr :7000 -shutdown-timeout 30s -mem-limit-mib 256`
- **Code:** [examples/02_runtime_flags/main.go](examples/02_runtime_flags/main.go)
  ```go
  fs := flag.NewFlagSet("service", flag.ContinueOnError)
  fs.StringVar(&c.Addr, "addr", envOr(getenv, "ADDR", "127.0.0.1:8080"), "listen address (env ADDR)")
  ```
- **Walkthrough:** the environment value becomes the flag's *default*, so a flag always wins. A private `FlagSet` with `ContinueOnError` returns errors instead of exiting, which makes `Parse` testable; `-h` yields `flag.ErrHelp`. Invalid values exit with status 2 and a message, never a half-started server. `debug.SetMemoryLimit` sets the soft limit that `GOMEMLIMIT` would: keep it ~10% below the container limit so the GC works harder before the OOM killer acts.
- **Expected output:**
  ```
  addr=:7000 env=dev shutdown=30s mem-limit=256 MiB
  ```
  `ADDR=0.0.0.0:9000 APP_ENV=prod go run ./modules/14-packaging-and-running/examples/02_runtime_flags` prints `addr=0.0.0.0:9000 env=prod shutdown=10s mem-limit=off`; `-env staging` prints `error: -env must be dev or prod, got "staging"` and exits with status 2.

### 3.3 Embedding assets
- **Goal:** ship templates and seed data inside the binary.
- **Run:** `go run ./modules/14-packaging-and-running/examples/03_embed_assets`
- **Code:** [examples/03_embed_assets/main.go](examples/03_embed_assets/main.go), assets in [examples/03_embed_assets/assets/](examples/03_embed_assets/assets/)
  ```go
  //go:embed assets
  var assets embed.FS

  //go:embed assets/seed.json
  var seedJSON []byte
  ```
- **Walkthrough:** the directive must sit directly above a package-level variable; patterns are relative to the source file and cannot use `..`. `embed.FS` is a read-only `fs.FS`, so `fs.WalkDir`, `fs.Sub`, `template.ParseFS` and `http.FileServerFS` all work on it. Files starting with `.` or `_` are skipped unless you use the `all:` prefix. Embedded content is fixed at compile time, which is exactly what lets a `scratch` image run without a data volume.
- **Expected output:**
  ```
  embedded: assets/seed.json
  embedded: assets/static/index.html
  embedded: assets/templates/banner.txt.tmpl
  seed tasks: 2 first: Write docs
  index.html: 3 lines
  Task Tracker dev (2 seed tasks)
  missing file is a normal fs error: open assets/missing.txt: file does not exist
  ```

### 3.4 Task Tracker service, Dockerfile and CI
- **Goal:** a deployable service plus the files that package it.
- **Run:** `ADDR=127.0.0.1:8080 go run ./modules/14-packaging-and-running/examples/04_service` (stop with Ctrl-C)
- **Code:** [main.go](examples/04_service/main.go), [Dockerfile](examples/04_service/Dockerfile), [Dockerfile.dockerignore](examples/04_service/Dockerfile.dockerignore), [ci.yml](examples/04_service/ci.yml), [seed.json](examples/04_service/seed.json)
  ```dockerfile
  FROM golang:1.27 AS build
  COPY go.mod go.sum ./
  RUN go mod download                    # cached until the dependencies change
  COPY modules/.../04_service ./modules/.../04_service
  RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/service ./modules/.../04_service

  FROM gcr.io/distroless/static-debian12:nonroot
  COPY --from=build /out/service /service
  USER nonroot:nonroot
  EXPOSE 8080
  HEALTHCHECK CMD ["/service", "-healthcheck"]
  ENTRYPOINT ["/service"]
  ```
- **Walkthrough:** the service honours `ADDR`, has explicit server timeouts, and on SIGINT/SIGTERM flips `/readyz` to 503 first and then calls `Shutdown` so in-flight requests finish. A distroless image has no shell or `curl`, so the binary itself implements the probe: `service -healthcheck` GETs `/healthz` on `$ADDR`'s port and exits 0/1. The Dockerfile copies `go.mod`/`go.sum` before the source so the dependency layer survives code edits; `CGO_ENABLED=0` is required because the final image has no libc. `ci.yml` (copy it to `.github/workflows/`) triggers on push and pull requests, takes the Go version from `go.mod`, and runs format check, vet, race tests, build and an image build. Build the image from `languages/go` with `docker build -f modules/14-packaging-and-running/examples/04_service/Dockerfile -t tasks:dev .` (not run here).
- **Expected output** (server started with `ADDR=127.0.0.1:22151`; logs go to stderr):
  ```
  $ curl -si 127.0.0.1:22151/readyz | sed -n '1p;$p'
  HTTP/1.1 200 OK
  {"status":"ready"}
  $ curl -s 127.0.0.1:22151/version
  {"commit":"none","version":"dev"}
  $ curl -s 127.0.0.1:22151/tasks
  {"items":[{"id":1,"title":"Write docs","status":"todo","due_date":"2030-01-31","tags":["docs","p1"],"owner_id":null,"created_at":"2030-01-01T09:00:00Z"},{"id":2,"title":"Ship it","status":"in_progress","due_date":null,"tags":[],"owner_id":1,"created_at":"2030-01-02T10:30:00Z"}],"total":2,"limit":20,"offset":0}
  $ curl -si -X POST 127.0.0.1:22151/tasks | sed -n 1p
  HTTP/1.1 405 Method Not Allowed
  $ ADDR=127.0.0.1:22151 service -healthcheck; echo $?
  0
  ```
  After `kill -TERM <pid>` the log shows `INFO listening addr=127.0.0.1:22151 version=dev` and `INFO shutting down`, and the process exits with status 0.

## 4. Common Pitfalls
| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| `exec /app: no such file or directory` on `scratch`/distroless | cgo is on by default when a C compiler exists, so the binary links libc dynamically | `CGO_ENABLED=0` in the build stage (and `GOOS=linux` when building on macOS/Windows) |
| `-X main.version=...` has no effect | Wrong package path, the variable is a `const`, is not a `string`, or is initialised by a function call | `var version = "dev"`; use the full import path for non-main packages; check with `go version -m` / `strings` |
| `COPY . .` before `go mod download` | Any source edit invalidates the dependency layer, so every build re-downloads modules | Copy `go.mod go.sum`, download, then copy the source |
| Final stage runs as root, or the image is `golang:latest` | Convenience: the build image works out of the box | Distroless `:nonroot` or `USER 10001`; pin tags (`golang:1.27`), never `latest` |
| `HEALTHCHECK CMD curl ...` on a distroless/scratch image | There is no curl or shell in the image | Make the binary probe itself (`-healthcheck`) or use the orchestrator's HTTP probe |
| `//go:embed` fails: `pattern ...: no matching files found` / `cannot embed ... in different module` | Paths are relative to the source file, cannot use `..`, and files in other modules or named `.x`/`_x` are excluded | Put assets in or under the package directory; `all:` prefix for dotfiles |
| Reading config files from the working directory at run time | Works with `go run`, breaks inside the container | Embed, or take an explicit path flag/env |
| `ListenAndServe` on `localhost:8080` inside a container | The port is bound to the container's loopback, unreachable from the host | Bind `0.0.0.0:8080` in the image (`ENV ADDR=0.0.0.0:8080`), keep `127.0.0.1` for local dev |
| Ignoring SIGTERM | The orchestrator waits 10-30 s, then SIGKILLs: in-flight requests are lost | `signal.NotifyContext(..., os.Interrupt, syscall.SIGTERM)` and `Server.Shutdown` |
| Different toolchains locally and in CI | `go-version: stable` drifts | `go-version-file: go.mod` and a matching `golang:<version>` build image |

## 5. Coming From Other Languages
- **Java/C#:** there is no JVM/CLR to install and no JAR/DLL set to ship: the output is one native executable per OS/arch (like a self-contained, trimmed publish). `-Xmx`-style tuning becomes `GOMEMLIMIT`/`GOMAXPROCS`.
- **Python/JS:** no virtualenv or `node_modules` in the image, so the runtime stage can be empty (`scratch`/distroless). Build once per target platform instead of installing on the target.
- **Rust:** same static-binary story (`musl` targets vs `CGO_ENABLED=0`); cross-compiling needs no extra toolchain for pure Go code, which is easier than `cross`/target installs.

## 6. Exercises
Starter code and tests are in [exercises/](exercises/); reference solutions in [_solutions/](_solutions/).

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | Build info | Easy | `debug.BuildInfo`, ldflags fallback, a version endpoint | `go test ./modules/14-packaging-and-running/exercises/ex1_buildinfo/` |
| 2 | Runtime config | Medium | `flag.FlagSet`, env precedence, `errors.Join`, production rules | `go test ./modules/14-packaging-and-running/exercises/ex2_runtime_config/` |
| 3 | Dockerfile lint | Stretch | the rules of a good Go image, checked as text (no Docker needed) | `go test ./modules/14-packaging-and-running/exercises/ex3_dockerfile_lint/` |

### Exercise 1: Build info
- **Task:** implement `FromBuildInfo` (ldflags version, else module version, else `dev`; 7-character commit; `vcs.time`; `vcs.modified`), `Info.String` and the JSON `Handler`.
- **Acceptance:** `TestFromBuildInfo`, `TestString`, `TestHandler`, `TestHandlerRejectsOtherMethods`.
- <details><summary>Hint</summary>Loop over <code>bi.Settings</code> with a <code>switch s.Key</code>; <code>"(devel)"</code> is what the toolchain reports for a local build, so it is not a real version.</details>

### Exercise 2: Runtime config
- **Task:** implement `Parse(args, getenv, out)`: defaults < `PORT`/`LOG_LEVEL`/`ENV`/`SHUTDOWN_TIMEOUT` < flags, validation of every field, no debug logging in prod, all problems reported together as `*FieldError`s joined with `errors.Join`.
- **Acceptance:** `TestDefaults`, `TestPrecedenceDefaultsEnvFlags`, `TestAllProblemsReportedTogether`, `TestPortBounds`, `TestBadEnvironmentValues`, `TestShutdownTimeoutLimit`, `TestProductionForbidsDebug`, `TestHelpAndMalformedFlags`, `TestParseDoesNotWriteToStdout`.
- <details><summary>Hint</summary>Apply the environment to a <code>Config</code> first and pass its fields as the defaults of the <code>FlagSet</code> variables; collect errors in a slice and return <code>errors.Join(errs...)</code> (it is <code>nil</code> for an empty slice).</details>

### Exercise 3: Dockerfile lint
- **Task:** implement `Lint(src)` with seven rules (`multi-stage`, `pinned-base`, `non-root`, `expose`, `healthcheck`, `deps-before-source`, `static-binary`) over a Dockerfile given as text. The module's own [Dockerfile](examples/04_service/Dockerfile) must come out clean.
- **Acceptance:** all tests in `dockerlint_test.go`, e.g. `TestEverythingWrong`, `TestPinnedBase`, `TestNonRoot`, `TestContinuationsAndComments`, `TestDepsBeforeSource`, `TestStaticBinaryNeedsCgoDisabled`.
- <details><summary>Hint</summary>First write a small parser that joins backslash continuations and groups instructions into stages (a <code>[]stage</code> with the <code>FROM</code> and its instructions); every rule then becomes a few lines over that structure.</details>

## 7. Recap
- `CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=..."` gives a small, static, stamped binary; `GOOS`/`GOARCH` cross-compile it.
- Configure once at start-up: defaults < env < flags, validate, fail fast with a clear message and exit status 2.
- Multi-stage image: cached `go mod download` layer, minimal non-root final stage, a probe that works without a shell.
- Handle SIGTERM: fail readiness, drain with `Shutdown`, exit 0.
- CI runs the same commands as your laptop: gofmt, vet, test, build, then image. When to reach for what: `embed` for files the program needs, env/flags for anything that differs per environment.

## 8. Self-check Questions
1. Why can a binary built on your laptop fail with "no such file or directory" on a `scratch` image, and which two settings prevent it?
2. In 3.2 the environment value is passed as the flag *default*. What precedence does that produce, and what would break if the env were applied after `fs.Parse`?
3. Why does the Dockerfile copy `go.mod` and `go.sum` in a separate step, and what does that change for a developer editing one handler?
4. The service flips `/readyz` to 503 before calling `Shutdown`. What problem does that ordering solve?
5. What does `debug.ReadBuildInfo` return for `go run`, and why must a release pipeline still pass `-X main.version`?

## 9. Further Reading
- [Go command: build flags](https://pkg.go.dev/cmd/go#hdr-Compile_packages_and_dependencies) and [`cmd/link -X`](https://pkg.go.dev/cmd/link)
- [Package embed](https://pkg.go.dev/embed) and [runtime/debug.ReadBuildInfo](https://pkg.go.dev/runtime/debug#ReadBuildInfo)
- [Dockerfile best practices](https://docs.docker.com/build/building/best-practices/), [distroless images](https://github.com/GoogleContainerTools/distroless), [Go GC guide (memory limit)](https://go.dev/doc/gc-guide)
- [GitHub Actions: building and testing Go](https://docs.github.com/en/actions/use-cases-and-examples/building-and-testing/building-and-testing-go)

---
**Next:** back to the [Go track overview](../../README.md)
