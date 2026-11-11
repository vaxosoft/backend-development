# Language Profile: Go

Constrains all generated Go content. Dependencies are pinned in [go.mod](./go.mod); do not add any while writing a module.

## 1. Overview
- **Version:** Go 1.27 (`go 1.27.2` in go.mod; anything >= 1.25 works for the lessons).
- **Paradigm / typing:** statically typed, garbage collected, compiled to a single binary. Procedural with structs + interfaces (structural typing), composition over inheritance, generics since 1.18.
- **Why backend:** the standard library ships a production-grade HTTP server, JSON, SQL interface, TLS and testing tools. Goroutines make concurrent I/O simple, and deployment is one static binary.

## 2. Toolchain
| Purpose | Tool | Command |
|---------|------|---------|
| Install / version manager | go.dev installer (or `go install golang.org/dl/...`) | `go version` |
| Package / dependency manager | Go modules | `go get <pkg>`, `go mod tidy` |
| Build | `go build` | `go build ./...` |
| Run | `go run` | `go run ./modules/02-http-server/examples/01_hello_mux` |
| Test | `go test` | `go test ./modules/02-http-server/exercises/ex1_routes/` |
| Race detector | `-race` (needs cgo/gcc) | `go test -race ./...` |
| Format | `gofmt` | `gofmt -l -w .` |
| Lint / static analysis | `go vet` (staticcheck is optional) | `go vet ./...` |
| Debug / profile | `dlv`, `pprof`, `go tool trace` | `go test -bench . -cpuprofile cpu.out` |

All commands run from `languages/go/` (the module root, module path `webdev`).

## 3. Project Conventions
- **Layout of this repo:** one module (`webdev`) at `languages/go/`. Examples are `package main` programs, one per directory: `modules/NN-topic/examples/NN_name/main.go`. Exercises are one package per directory: `modules/NN-topic/exercises/exN_name/{name.go,name_test.go}`. Solutions: `_solutions/exN_name/name.go` (only non-test files; the leading underscore makes the Go tool ignore the folder). Projects: `projects/PN-name/{starter,_solution,tests}/`; tests are in `package tests` importing `webdev/projects/PN-name/starter`.
- **Naming:** `MixedCaps`; exported = capitalised; short package names, no stutter (`task.Store`, not `task.TaskStore`); receivers 1-2 letters; interfaces named by behaviour and defined at the consumer.
- **Error handling:** return `error` as the last value; wrap with `fmt.Errorf("...: %w", err)`; compare with `errors.Is` / `errors.As`; sentinel errors for expected conditions; never ignore errors silently; `panic` only for programmer errors.
- **Docs:** `// Name does ...` doc comments on exported identifiers; package comment at top of one file.
- **Tests:** table-driven with `t.Run`, `t.Helper`, `t.Cleanup`, `t.TempDir`, `t.Parallel` where safe. Starter code uses `panic("TODO")` or zero-value returns so tests fail on assertions.
- **Context:** `context.Context` first parameter for anything that blocks or does I/O.

## 4. Library Choices per Topic
Standard library first. Third-party packages are listed with the reason.

| Module | Stdlib / Library | Version | Why |
|--------|------------------|---------|-----|
| 01 Fundamentals | stdlib (`errors`, `slices`, `maps`, `sort`, `strings`, `iter`) | std | core language |
| 02 HTTP server | `net/http` (Go 1.22+ `ServeMux` patterns like `GET /tasks/{id}`), `http.MaxBytesReader`, `mime/multipart` | std | no router needed; teaches the real stdlib |
| 03 REST design | `net/http`, `encoding/json`, hand-written validation, `ServeMux` | std | chi/gin intentionally skipped; the mux is enough |
| 04 Serialization | `encoding/json`, `encoding/xml`, `encoding/csv`, `net/url`; `gopkg.in/yaml.v3` | v3.0.1 | YAML is not in the stdlib |
| 05 Database access | `database/sql` + `modernc.org/sqlite` (pure Go, no cgo); `github.com/jackc/pgx/v5/stdlib` for the PostgreSQL section | v1.62.0 / v5.12.0 | SQLite needs no server; pgx is the standard PostgreSQL driver |
| 06 Concurrency | goroutines, channels, `sync`, `sync/atomic`, `context`; `golang.org/x/sync/errgroup` | v0.25.0 | errgroup is the idiomatic fan-out with errors |
| 07 HTTP client | `net/http` client, `httptest` | std | stdlib is the idiom; retries hand-written |
| 08 File and stream I/O | `os`, `io`, `bufio`, `io/fs`, `path/filepath`, `compress/gzip`, `encoding/base64`, `encoding/hex`, `crypto/sha256`, `embed` | std | |
| 09 Configuration | `os.Getenv`, `flag`, `encoding/json`, `gopkg.in/yaml.v3` | std / v3.0.1 | manual layered config shows the idiom |
| 10 Logging | `log/slog`, `net/http/pprof`, `expvar`, `runtime/debug` | std | slog is the standard structured logger |
| 11 Auth / security | `golang.org/x/crypto/bcrypt`, `github.com/golang-jwt/jwt/v5`, `golang.org/x/time/rate`, `crypto/rand`, `crypto/subtle`, `html/template`, `crypto/tls` | x/crypto v0.58.0, jwt v5.3.1, x/time v0.16.0 | no stdlib equivalents for bcrypt, JWT, token bucket |
| 12 Testing | `testing`, `net/http/httptest`, `testing/fstest`, fakes by hand | std | no testify/gomock: idiomatic Go compares with `==`/`reflect.DeepEqual`/`slices.Equal` and writes small fakes |
| 13 Time and utilities | `time`, `math/rand/v2`, `regexp`, `text/template`, `github.com/google/uuid`, `crypto/*`, `time.Ticker` | uuid v1.6.0 | uuid is not in the stdlib |
| 14 Packaging | `go build`, `-ldflags`, `GOOS/GOARCH`, `//go:embed`, `debug.ReadBuildInfo`, Dockerfile (multi-stage), GitHub Actions | std | |

**Database for examples:** SQLite (`modernc.org/sqlite`, in-memory or temp file, driver name `"sqlite"`). PostgreSQL appears only as a documented, non-executed swap (the example compiles; it runs only when `DATABASE_URL` is set and otherwise prints a skip message).

## 5. Language-specific Teaching Notes
| Module | Emphasis | Typical pitfall |
|--------|----------|-----------------|
| 01 | Zero values, value vs pointer receivers, slices (len/cap/aliasing), implicit interfaces, `error` as a value, `defer` | nil map write, loop-variable capture (fixed in 1.22 but explain), nil-interface-holding-nil-pointer, modifying a slice during `append` aliasing |
| 02 | `Handler` / `HandlerFunc`, middleware as `func(http.Handler) http.Handler`, `ServeMux` patterns, `http.Server` timeouts, graceful `Shutdown` | using `http.ListenAndServe` with no timeouts, writing headers after the body, forgetting to `return` after `http.Error` |
| 03 | Consistent JSON error envelope, `Location` header, validation errors as 422 with field map, pagination | returning 200 for errors, mixing 400/422 randomly, unbounded `limit` |
| 04 | struct tags, `omitempty` vs pointers, `json.RawMessage`, custom `MarshalJSON`, `DisallowUnknownFields` | `omitempty` on zero numbers, unexported fields silently skipped, decoding into `interface{}` gives float64 |
| 05 | `database/sql` pool, `QueryContext`, always `rows.Close` + `rows.Err`, `sql.Null*`, transactions with `defer tx.Rollback()` | leaked rows, `SetMaxOpenConns` and SQLite in-memory (each connection is a separate DB), string-concatenated SQL |
| 06 | goroutine lifecycle, channel ownership (the sender closes), `context` cancellation, `sync.WaitGroup`, race detector | goroutine leaks, closing a channel twice, copying a `sync.Mutex`, unsynchronised map access |
| 07 | reuse one `http.Client`, set `Timeout`, always `defer resp.Body.Close()` and drain, non-2xx is not an error | default client has no timeout, not closing body, retrying non-idempotent requests |
| 08 | `io.Reader`/`io.Writer` composition, `bufio`, `io.Copy`, `defer f.Close()` and write errors, `embed.FS` | reading a whole file into memory, ignoring `Close` error on writes, path traversal in `filepath.Join` |
| 09 | explicit precedence defaults < file < env < flags, fail fast on validation | reading env at call sites, `flag.Parse` in `init` |
| 10 | `slog` handlers and `With`, context-carried request ID, `recover` middleware, pprof exposure | logging secrets, pprof on a public port |
| 11 | constant-time compare, bcrypt cost, JWT `alg` checks, token bucket per key, parameterised SQL, `html/template` auto-escaping | trusting the JWT `alg` header, storing plaintext tokens, `text/template` for HTML |
| 12 | table tests, `httptest.NewRecorder` vs `NewServer`, fakes behind small interfaces, `testing.B`, `-cover`, `-race` | shared mutable state across `t.Parallel` tests, asserting on error strings |
| 13 | `time.Time` monotonic clock, `Duration`, layout `2006-01-02T15:04:05Z07:00`, `time.After` in loops | `time.After` leaking timers in loops, reference-time layout confusion, `math/rand` for secrets |
| 14 | `CGO_ENABLED=0`, `-ldflags "-s -w -X ..."`, multi-stage Docker, distroless/scratch, non-root | building with cgo then running on `scratch`, caching `go mod download` layer |

## 6. Comparison Anchors
Java/C# (exceptions vs error values, interfaces without `implements`), Python/JS (async vs goroutines), Rust (ownership vs GC).

## 7. Module Ordering Adjustments
Default order 01-14. Module 01 includes error handling and `defer` before HTTP. Module 06 (concurrency) is a prerequisite for 07-aggregation examples and for projects P4 and P6.

## 8. Out of Scope for This Language
`cgo`, `unsafe`, `reflect` internals, code generation (`go generate`, sqlc, ORMs like GORM), web frameworks (gin/echo/chi), gRPC, WebAssembly, assembly, build tags beyond a mention.
