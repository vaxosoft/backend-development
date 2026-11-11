# 01. Language Fundamentals in Go

> Estimated time: 2h | Difficulty: Beginner | Prerequisites: none (Go 1.25+ installed, see [PROFILE](../../PROFILE.md))

## Learning Outcomes
After this module you can:
- [ ] Declare values with `var`, `:=`, `const` and `iota`, and predict their zero values
- [ ] Explain slice length, capacity and aliasing, and use maps and strings safely
- [ ] Choose between value and pointer receivers; satisfy interfaces implicitly; compose with embedding
- [ ] Write a small generic function and consume an `iter.Seq`
- [ ] Return, wrap and inspect errors with `%w`, `errors.Is` and `errors.As`; use `defer`
- [ ] Format, vet, build and run Go code from the command line

## 1. Concept Overview
Every backend language needs the same toolbox: types, control flow, data structures, abstraction (interfaces/generics), error handling, a memory model and a way to organise code. Go keeps each of them deliberately small: one loop (`for`), one way to abstract behaviour (interfaces), errors as ordinary return values, and a garbage collector so you rarely think about freeing memory.

What you *do* have to think about is **value vs reference semantics**: assignment copies, so the question is always "what exactly got copied?" (a struct, or just a slice header pointing at shared data).

## 2. How Go Approaches It

| Concern | Go idiom | Packages |
|---------|----------|----------|
| Enums | defined type + `iota` constants (+ `String()`) | built-in |
| Zero values | every type is usable when declared; design structs so zero is useful | built-in |
| Collections | slice, map, array; helpers in `slices` / `maps` | `slices`, `maps`, `sort` |
| Abstraction | small interfaces declared by the **consumer**, satisfied implicitly | built-in |
| Reuse | composition via embedding (no inheritance) | built-in |
| Generics | type parameters + constraints (`any`, `comparable`, custom) | `cmp`, `iter` |
| Errors | `error` return value, sentinels, custom types, wrapping | `errors`, `fmt` |
| Cleanup | `defer` (LIFO), `recover` only at boundaries | built-in |
| Memory | GC; pointers exist but no pointer arithmetic; escape analysis picks stack/heap | built-in |
| Tooling | `go build/run/test`, `gofmt`, `go vet`, `dlv` | toolchain |

Non-idiomatic: getters/setters everywhere, `interface{}` where generics fit, a `Manager`/`Factory` class hierarchy, panicking for expected failures, ignoring returned errors.

All commands run from `languages/go/` (module path `webdev`).

## 3. Guided Examples

### 3.1 Values, zero values, constants, control flow
- **Goal:** the building blocks: zero values, `iota`, `switch`, `for`, closures.
- **Run:** `go run ./modules/01-language-fundamentals/examples/01_values_and_flow`
- **Code:** [examples/01_values_and_flow/main.go](examples/01_values_and_flow/main.go)
  ```go
  const (
      Todo Status = iota
      InProgress
      Done
  )
  ```
- **Walkthrough:** `var` without a value gives the zero value (`0`, `""`, `false`, `nil`). `switch` has no fall-through. `for` is the only loop; `range 3` iterates an int. The closure loop prints `0 1 2` because Go 1.22 gives every iteration its own variable (earlier versions shared one and printed `3 3 3`).
- **Expected output:**
  ```
  zero values: 0 "" false 0.0 <nil> true true
  trimmed: "Write docs"
  default status: todo | constant: done | unknown: Status(7)
  divmod: 3 2
  -2 -> overdue
  0 -> due today
  5 -> upcoming
  0 1 2 
  0=a 1=b 2=c 
  0 1 2 
  counter: 3
  ```

### 3.2 Slices, maps and strings
- **Goal:** see aliasing, capacity and nil-map rules with your own eyes.
- **Run:** `go run ./modules/01-language-fundamentals/examples/02_collections`
- **Code:** [examples/02_collections/main.go](examples/02_collections/main.go)
  ```go
  a := make([]int, 3, 4)
  b := append(a, 99)  // fits in cap: shares a's backing array
  c := append(a, 100) // overwrites b[3]
  ```
- **Walkthrough:** a slice is `(pointer, len, cap)`. `append` reuses the array while capacity allows, so `b` and `c` alias. Once capacity is exceeded a new array is allocated and aliasing stops. Sub-slices share memory too; `slices.Clone` or `nums[:2:2]` breaks the link. Map iteration order is random, so keys are sorted with `slices.Sorted(maps.Keys(m))`. `len("héllo")` counts bytes, not characters.
- **Expected output:**
  ```
  shared backing array: 100 100
  after realloc: 0 -1
  nums after append on sub-slice: [1 2 42 4 5]
  clone leaves nums alone: [1 2 42 4 5] [1 2 7]
  len/cap: 2 true
  missing key: 0 false
  and=1 fast=1 fun=1 go=2 is=2 
  read from nil map: 0
  bytes: 6 runes: 5
  0:h 1:é 
  [0][1][2]
  [{Bo 30} {Cy 30} {Al 25}]
  ```

### 3.3 Structs, interfaces, embedding, generics
- **Goal:** methods and receivers, implicit interfaces, composition, generic functions.
- **Run:** `go run ./modules/01-language-fundamentals/examples/03_structs_interfaces`
- **Code:** [examples/03_structs_interfaces/main.go](examples/03_structs_interfaces/main.go)
  ```go
  func (t Task) Label() string { ... } // value receiver: a copy
  func (t *Task) Complete()    { ... } // pointer receiver: the original
  func Sum[T Number](xs []T) T { ... }
  ```
- **Walkthrough:** `for _, task := range tasks` hands you a *copy*, so `task.Complete()` is lost; index into the slice instead. `AuditedNotifier` embeds a `Notifier` and overrides one method (promotion, not inheritance). The last lines show the nil-interface trap: a function returning a nil `*myErr` as `error` yields a non-nil interface.
- **Expected output:**
  ```
  [x] Write docs
  after range copy: false false
  after index: true true
  email to ann@example.com: hi
  log: hi
  #2 log: two
  a log notifier
  6 0.75
  1,4,9
  err == nil? false
  ```

### 3.4 Errors and defer
- **Goal:** errors as values: sentinels, custom types, wrapping, `defer`, `recover`.
- **Run:** `go run ./modules/01-language-fundamentals/examples/04_errors_defer`
- **Code:** [examples/04_errors_defer/main.go](examples/04_errors_defer/main.go)
  ```go
  return fmt.Errorf("find task %d: %w", id, ErrNotFound) // keeps the chain
  if errors.As(err, &fe) { ... }                          // extract a type
  ```
- **Walkthrough:** `%w` wraps, so `errors.Is(err, ErrNotFound)` still works after adding context at each layer. `errors.As` finds a `*FieldError` or a `*strconv.NumError` anywhere in the chain. Deferred calls run in reverse order; a deferred closure that assigns the named result `err` turns a panic into an error.
- **Expected output:**
  ```
  find task 2: not found
  is not found: true | unwrapped: true
  field error on priority
  priority "high": strconv.Atoi: parsing "high": invalid syntax | as NumError: true
  title: required
  tags: too many
  body done
  deferred 2
  deferred 1
  deferred 0
  5 <nil>
  0 recovered: runtime error: integer divide by zero
  ```

### 3.5 Task Tracker: a store in plain Go
- **Goal:** combine everything in the running domain: types, a consumer-side interface, validation, closures, iterators.
- **Run:** `go run ./modules/01-language-fundamentals/examples/05_task_store`
- **Code:** [examples/05_task_store/main.go](examples/05_task_store/main.go)
  ```go
  type Store interface {
      Create(t Task) (Task, error)
      Get(id int) (Task, error)
      All() iter.Seq[Task]
  }
  ```
- **Walkthrough:** `MemStore` satisfies `Store` without declaring it; `var store Store = NewMemStore()` is the compile-time proof. `Create` clones `Tags` so callers cannot alias stored data. `Validate` uses `errors.Join`, which returns `nil` for an empty list. `Filter` is a generic iterator adapter, and `WithStatus` is a closure returning a predicate.
- **Expected output:**
  ```
  created #1 "Write docs" (todo)
  created #2 "Review PR" (in_progress)
  created #3 "Ship it" (done)
  rejected: true - create: title must be 1-200 characters: invalid value
  lookup: task 42: not found
  open tasks: Write docs; Review PR;
  in progress: 1
  status "blocked": invalid value
  ```

### Tooling cheat sheet
```bash
gofmt -l -w modules/01-language-fundamentals   # format (lists changed files)
go vet ./modules/01-language-fundamentals/...  # static analysis
go build ./modules/01-language-fundamentals/...
```
Debugging: `dlv debug ./modules/01-language-fundamentals/examples/05_task_store` (Delve), or `fmt.Printf("%+v\n", v)` / `%#v` for quick inspection.

## 4. Common Pitfalls

| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| `panic: assignment to entry in nil map` | `var m map[string]int` is a nil map: readable, not writable | `m := map[string]int{}` or `make` |
| `append` overwrites another slice's data | slices share a backing array while `cap` allows | `slices.Clone`, or `s[:n:n]` to cap capacity |
| Mutating a `range` variable has no effect | `for _, t := range ts` copies each element | index (`ts[i].Done = true`) or range over `[]*T` |
| `err != nil` is true for a "nil" error | an interface holding a typed nil pointer is not nil | return the literal `nil`; never return a nil `*MyErr` as `error` |
| Goroutine/closure sees the "last" loop value | before Go 1.22 the loop variable was shared (old code, old `go.mod`) | rely on `go 1.22+` in `go.mod`, or copy `i := i` in legacy code |
| Mixed value and pointer receivers; value type does not satisfy interface | pointer-receiver methods are not in the method set of `T` | pick one receiver kind per type; use `*T` when any method mutates |
| Comparing errors with `==` or `err.Error()` | wrapping changes the value and text | `errors.Is` / `errors.As` |
| Ignoring `defer` in a loop | deferred calls run at function end, not iteration end | move the body into a function |
| `len(s)` is not the character count | strings are bytes; UTF-8 runes vary in width | `utf8.RuneCountInString` or `[]rune(s)` |

## 5. Coming From Other Languages
- **Java / C#:** no exceptions for expected failures: functions return `(value, error)`. No `implements`: a type satisfies an interface by having the methods, and interfaces are defined where they are used. No inheritance; embed instead. `struct` is a value type (like C# `struct`), copied on assignment.
- **Python / JS:** statically typed and compiled; no `None` per type but a typed zero value; slices are views like NumPy slices, not copies. No `try/except`; `defer` replaces `finally`/context managers.
- **Rust:** a garbage collector instead of ownership; no `Option`/`Result` types: `nil` and `error`. Pointers are always safe to copy; aliasing rules are your job (see the slice pitfalls). Generics are monomorphised-ish but far simpler (no traits with associated types).

## 6. Exercises
Starter code and tests live in [exercises/](exercises/); reference solutions are in [_solutions/](_solutions/) (same relative paths). Work in `exercises/`, then compare.

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | [Tags](exercises/ex1_tags/tags.go) | Easy | slices, maps, aliasing | `go test ./modules/01-language-fundamentals/exercises/ex1_tags/` |
| 2 | [Validation errors](exercises/ex2_errors/task.go) | Medium | custom errors, `errors.Is/As`, nil interface | `go test ./modules/01-language-fundamentals/exercises/ex2_errors/` |
| 3 | [Generic store](exercises/ex3_store/store.go) | Stretch | value vs reference, generics, `iter.Seq` | `go test ./modules/01-language-fundamentals/exercises/ex3_store/` |

**Exercise 1: Tags**
- **Task:** implement `Normalize` (trim, lowercase, drop empties, de-duplicate keeping order, never `nil`, input untouched) and `Window` (first *n* tags that cannot be clobbered by `append`).
- **Acceptance:** all tests in `tags_test.go`, including the "does not modify input" and "append does not clobber" tests.
- <details><summary>Hint</summary>Build a new slice with <code>make([]string, 0, len(in))</code>; for <code>Window</code> look up the three-index slice expression <code>s[low:high:max]</code>.</details>

**Exercise 2: Validation errors**
- **Task:** implement `FieldErrors` (`Error`, `Is`), `ParseStatus`, `Validate` and `Repo.Get` so that sentinel matching works through wrapping and a valid task yields a true `nil`.
- **Acceptance:** `errors.Is(err, ErrValidation)`, `errors.As(err, &*FieldErrors)`, sorted deterministic message, boundary titles (200/201 chars after trimming).
- <details><summary>Hint</summary>Collect problems in a map and return <code>nil</code> explicitly when it is empty, instead of returning a typed nil pointer. A custom <code>Is(target error) bool</code> method lets <code>errors.Is</code> match a sentinel.</details>

**Exercise 3: Generic store**
- **Task:** finish `Store` (copying `Tags` in and out, ids from 1, ordered snapshot iterator) and the generic `Filter`/`Map` iterator adapters.
- **Acceptance:** no aliasing through `Create`/`Get`/`All`; `All` stops on `break`; `Filter`/`Map` are lazy and stop early; 50 concurrent creates are safe.
- <details><summary>Hint</summary>An <code>iter.Seq[T]</code> is <code>func(yield func(T) bool)</code>: return from it as soon as <code>yield</code> returns false. <code>slices.Values</code> and <code>slices.Clone</code> help.</details>

## 7. Recap
- Every type has a usable zero value; `:=` for locals, `const` + `iota` for enums.
- Slices are views: know when `append` aliases, and clone when you hand data across a boundary.
- Interfaces are implicit and small, defined by the consumer; compose with embedding.
- Errors are values: wrap with `%w`, test with `errors.Is/As`, never compare strings.
- Reach for generics for containers/algorithms and interfaces for behaviour; for everything else a plain struct and function is enough.

## 8. Self-check Questions
1. Why does `b := append(a, 99); c := append(a, 100)` change `b[3]`, and what changes when `cap(a)` is 3?
2. A function declared `func f() error` ends with `var e *MyErr; return e`. Why is `f() == nil` false, and how do you write it correctly?
3. When must a method use a pointer receiver, and what happens to a `T` value stored in an interface when the method set needs `*T`?
4. What is the difference between `errors.Is` and `errors.As`, and why does `%w` matter for both?
5. Why does `for _, t := range tasks { t.Done = true }` not update the slice, and which two fixes exist?

## 9. Further Reading
- [A Tour of Go](https://go.dev/tour/) and [Effective Go](https://go.dev/doc/effective_go)
- [Go Slices: usage and internals](https://go.dev/blog/slices-intro)
- [Working with Errors in Go 1.13](https://go.dev/blog/go1.13-errors)
- [Range over function iterators](https://go.dev/blog/range-functions)

---
**Next:** [02 HTTP Server](../02-http-server/README.md)
