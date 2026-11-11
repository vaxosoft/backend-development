# 01. Language Fundamentals in C#

> Estimated time: 1.5-2h | Difficulty: Beginner | Prerequisites: .NET 10 SDK installed (`dotnet --version`), no earlier module

## Learning Outcomes
After this module you can:
- [ ] Choose between `class`, `struct`, `record` and `record struct` and predict copy/equality behaviour
- [ ] Model a domain with enums, interfaces, composition, generics with constraints and `switch` expressions
- [ ] Handle errors with exceptions for exceptional cases and result values for expected failures
- [ ] Release resources deterministically with `using`
- [ ] Use LINQ without falling into deferred-execution traps, and read nullable-reference-type warnings
- [ ] Build, format, test and run a project with the `dotnet` CLI

## 1. Concept Overview
Every backend language needs the same toolbox: types to model data, control flow, functions, a way to
abstract behaviour (interfaces/generics), a strategy for errors and resources, collections with iteration,
and tooling. This module covers that toolbox in the C# flavour used by every later module.

The key design question is **value vs reference semantics**: does assignment copy the data or the
pointer to it? It decides equality, mutation visibility and what is safe to share between requests.

## 2. How C# Approaches It

| Need | C# tool |
|------|---------|
| Reference type, identity equality | `class` (use `sealed` by default) |
| Small immutable value, copied on assignment | `struct` / `readonly record struct` |
| Data carrier with value equality, `with` copies | `record` (reference type), `record struct` |
| Closed set of choices | `enum` + `switch` expression; `abstract record` hierarchy when cases carry data |
| Contracts, composition | `interface`, constructor injection, primary constructors |
| Reusable algorithms | generics with constraints (`where T : IComparable<T>`, `notnull`, `IParsable<T>`) |
| Missing values | nullable reference types (`string?`), `is null`, `?.`, `??` |
| Errors | exceptions (derive from `Exception`), `TryParse` pattern, small `Result<T>` type |
| Resources | `IDisposable` + `using` declarations |
| Collections | `List<T>`, `Dictionary<K,V>`, `HashSet<T>`, collection expressions `[1, 2]`, LINQ |

Idiomatic: records for DTOs, `required`/`init` for immutable data, `sealed` classes, file-scoped
namespaces, pattern matching instead of `is`/cast chains. Avoid: public mutable structs, `null!` to silence
the compiler, catching `Exception` to swallow it, `ToLower()` on culture-sensitive data.
Everything here is in the BCL; there are no third-party packages. `dotnet build` is the linter (warnings are errors,
see `Directory.Build.props`).

| Tooling task | Command (from `languages/csharp/`) |
|--------------|---------|
| Build / lint (Roslyn analyzers) | `dotnet build modules/01-language-fundamentals/examples` |
| Run | `dotnet run --project modules/01-language-fundamentals/examples -- 01-values` |
| Test | `dotnet test modules/01-language-fundamentals/exercises` |
| Format | `dotnet format whitespace modules/01-language-fundamentals/examples/M01.Examples.csproj` |
| Add a package | `dotnet add package <Id>`, then move the version to `Directory.Packages.props` (central package management) |
| Debug | VS Code C# Dev Kit / Visual Studio; `dotnet-counters`/`dotnet-trace` for diagnostics |

## 3. Guided Examples
All examples are in one console project; a tiny dispatcher in [Program.cs](examples/Program.cs) picks one by name.

### 3.1 Values and references
- **Goal:** copy vs alias vs value equality
- **Run:** `dotnet run --project modules/01-language-fundamentals/examples -- 01-values`
- **Code:** [01_ValuesAndReferences.cs](examples/01_ValuesAndReferences.cs)
- **Walkthrough:** `var p2 = p1` copies a struct, `var c2 = c1` copies only the reference. `==` on a class
  compares identity; on a record it compares every property. `list[0]` on a `List<struct>` returns a copy,
  so mutating it changes nothing.
  ```csharp
  var m3 = m1 with { Amount = 12m };   // non-destructive mutation of a record
  ```
- **Expected output:**
  ```
  struct copy:      p1.X=1 p2.X=99
  class alias:      c1.Value=5
  class ==:         False (reference identity)
  record ==:        True, with-copy differs: True
  record ToString:  Money { Amount = 12, Currency = EUR }
  record struct set: 2 distinct points
  string ==:        True, same object: False
  struct in list:   points[0].X=1 copy.X=42
  ```

### 3.2 Pattern matching, enums and nullability
- **Goal:** replace `if`/cast chains with `switch` expressions; let the compiler track `null`
- **Run:** `dotnet run --project modules/01-language-fundamentals/examples -- 02-patterns`
- **Code:** [02_PatternMatching.cs](examples/02_PatternMatching.cs)
- **Walkthrough:** relational (`< 0`), property (`{ Length: 0 }`), positional (`Circle(var r)`) and list
  (`[var first, .., var last]`) patterns. In `Greeting`, after `name is null or ""` the compiler knows `name`
  is non-null.
  ```csharp
  static double Area(Shape shape) => shape switch
  {
      Circle(var r) => Math.PI * r * r,
      Rect(var w, var h) => w * h,
      _ => throw new NotSupportedException(),
  };
  ```
- **Expected output:**
  ```
  Todo -> not started
  InProgress -> working
  Done -> finished
  Describe: nothing
  Describe: negative int
  Describe: even int 8
  Describe: odd int 7
  Describe: empty string
  Describe: string of 3
  Describe: Double
  Summarize: empty list
  Summarize: list of one: 1
  Summarize: list from 1 to 3
  circle area: 3.14
  rect area:   6.00
  Hello, stranger
  Hello, Ada
  length via ?. and ??: -1
  parsed enum: Done
  ```

### 3.3 Errors and disposal
- **Goal:** exceptions vs result values; `using` and `finally`
- **Run:** `dotnet run --project modules/01-language-fundamentals/examples -- 03-errors`
- **Code:** [03_ErrorsAndDisposal.cs](examples/03_ErrorsAndDisposal.cs)
- **Walkthrough:** a bad port is *expected* input, so `ParsePort` returns `Result<int>.Ok/Error`; a missing task
  is exceptional, so `Find` throws a custom `NotFoundException`. `catch ... when` filters by condition without
  unwinding. `using var` disposes in reverse order, even when an exception flies.
- **Expected output:**
  ```
  port 8080
  error: 'http' is not a valid port
  error: '70000' is not a valid port
  error: '' is not a valid port
  caught: task 42 not found
  finally runs
  using declaration:
    db: query
    cache: disposed
    db: disposed
  after dispose (reverse order): boom
  guard clause: name
  ```

### 3.4 LINQ, generics and closures
- **Goal:** deferred execution, generic constraints, closure capture
- **Run:** `dotnet run --project modules/01-language-fundamentals/examples -- 04-linq-generics`
- **Code:** [04_LinqAndGenerics.cs](examples/04_LinqAndGenerics.cs)
- **Walkthrough:** a LINQ query is a *recipe*; nothing runs until it is enumerated, and each enumeration
  re-runs the work. `Largest<T>` works for any `IComparable<T>`. A `for` loop shares one variable across
  closures, `foreach` creates a new one per iteration.
- **Expected output:**
  ```
  after building the query: 0 produced
  first enumeration:  [10, 30], produced 3
  second enumeration: produced 6 (the work was repeated)
  Largest ints:    9
  Largest strings: zebra
  index[2] = test
  length 3: api, sql, tcp
  length 4: http, json
  for-loop closures:     3,3,3
  foreach-loop closures: 0,1,2
  ```

### 3.5 The Task Tracker domain
- **Goal:** put it together: record + enum + interface + decorator + LINQ
- **Run:** `dotnet run --project modules/01-language-fundamentals/examples -- 05-task-tracker`
- **Code:** [05_TaskTrackerDomain.cs](examples/05_TaskTrackerDomain.cs)
- **Walkthrough:** `TaskItem` is an immutable record with `required` members; updates use `with`.
  `AuditedStore` wraps any `ITaskStore` (composition over inheritance) and records an audit trail.
  `Find` returns `TaskItem?` so callers must deal with "not found".
- **Expected output:**
  ```
  #1 Write docs     done        2030-01-31 [docs,p1]
  #2 Fix login bug  in_progress no due date [bug]
  #3 Release        todo        2030-02-01 []
  open by due date: Release > Fix login bug
  todo: 1
  in_progress: 1
  done: 1
  missing id 99: null
  audit: add 'Write docs'; add 'Fix login bug'; add 'Release'; update #2; update #1
  ```

## 4. Common Pitfalls

| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| `a == b` on two equal-looking class instances is `false` | classes compare references unless they override `==` | use a `record`, or compare keys explicitly |
| Mutating a `struct` returned from a list/property has no effect | the indexer/getter returns a copy (`list[0].X = 1` is even a compile error) | make structs `readonly`, or use a class |
| Enumerating a LINQ query twice runs the work twice (or returns different data) | deferred execution | `.ToList()` once, then reuse |
| `foo!` / `null!` everywhere | silences nullable analysis instead of fixing the flow | return `T?`, check `is null`, or make the member `required` |
| `(TaskState)7` is accepted | enums are just integers; undefined values are legal | `Enum.IsDefined` or a `switch` with a throwing default |
| `catch (Exception) { }` or `throw ex;` | hides bugs / resets the stack trace | catch the specific type; rethrow with `throw;` |
| `$"{x}"` and `ToLower()` depend on the current culture | culture-sensitive APIs by default | `ToLowerInvariant()`, `CultureInfo.InvariantCulture` |
| Closure over a `for` variable sees the final value | one variable for the whole loop | copy into a local inside the loop or use `foreach` |
| Forgetting `using` on `IDisposable` | no destructor-style cleanup in a GC language | `using var x = ...;` (analyzer CA2000/IDE0063 help) |

## 5. Coming From Other Languages
- **Java:** records resemble Java records but `struct`s and `record struct`s have no Java equivalent;
  there are no checked exceptions; generics are reified (no type erasure, `typeof(T)` works).
- **TypeScript/JS:** LINQ (`Where`/`Select`/`OrderBy`) mirrors array methods but is lazy; `?.` and `??` behave as in TS;
  nullability is checked by the compiler, not by convention.
- **Go:** no multiple-return errors: use exceptions plus `TryParse`-style `bool` + `out`; `using` is the
  analogue of `defer` for cleanup; interfaces are implemented explicitly, not structurally.
- **Python:** `with` is `using`; dataclasses are records; `Enum.Parse` is stricter than you expect.

## 6. Exercises
Tests are in [exercises/](exercises/); reference solutions in [solutions/](solutions/). Copy a solution over the
starter only after trying.

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | Tag normalizer | Easy | strings, LINQ, deferred execution | `dotnet test modules/01-language-fundamentals/exercises --filter "FullyQualifiedName~.Ex1_"` |
| 2 | Task state rules | Medium | enums, `TryParse` pattern, `switch` expressions | `dotnet test modules/01-language-fundamentals/exercises --filter "FullyQualifiedName~.Ex2_"` |
| 3 | `Result<T>` | Stretch | generics, constraints (`IParsable<T>`), records, functional composition | `dotnet test modules/01-language-fundamentals/exercises --filter "FullyQualifiedName~.Ex3_"` |

### Exercise 1: Tag normalizer ([Ex1_TagNormalizer.cs](exercises/Ex1_TagNormalizer.cs))
- **Task:** implement `TagNormalizer.Normalize`: trim, lower-case, drop blank/null, de-duplicate keeping first position.
- **Acceptance:** all `Ex1_TagNormalizerTests` pass, including "reads the source exactly once".
- **Hint:** <details><summary>show</summary>Build the pipeline with `Where`/`Select`/`Distinct` and materialize it once with a collection expression `[.. query]`.</details>

### Exercise 2: Task state rules ([Ex2_TaskStates.cs](exercises/Ex2_TaskStates.cs))
- **Task:** parse/format the wire names `todo`, `in_progress`, `done` and encode which transitions are allowed.
- **Acceptance:** all `Ex2_TaskStatesTests` pass (numbers and wrong casing are rejected, undefined enum values throw).
- **Hint:** <details><summary>show</summary>`Enum.TryParse` is too lenient (it accepts `"1"`). Use `switch` expressions; a tuple pattern `(from, to)` expresses the transition table.</details>

### Exercise 3: `Result<T>` ([Ex3_Result.cs](exercises/Ex3_Result.cs))
- **Task:** implement `Map`, `Bind`, `Match`, `Result.Ok/Fail/Ensure`, the generic `Result.Parse<T> where T : IParsable<T>`, and `Result.Combine` that accumulates all errors.
- **Acceptance:** all `Ex3_ResultTests` pass.
- **Hint:** <details><summary>show</summary>`this switch { Ok(var v) => ..., Failure(var e) => ... }` inside the record; `T.TryParse(text, CultureInfo.InvariantCulture, out var value)` is a static abstract call.</details>

## 7. Recap
- `class` = identity, `record` = value equality, `struct` = copy; default to records for data, `sealed` classes for services.
- Make illegal states unrepresentable: enums + `switch` expressions without a lazy default, `required` members, nullable annotations.
- Exceptions for the unexpected, `TryX`/`Result<T>` for the expected; always dispose with `using`.
- LINQ is lazy: materialize once when the data is used more than once.
- Reach for generics + constraints before reaching for `object` or code duplication.

## 8. Self-check Questions
1. Why does `list[0].X = 1` not compile for a `List<MutablePoint>`, and what would you change?
2. Two `Money` records have equal fields; two `Counter` classes also do. Which `==` is true and why?
3. A LINQ query is enumerated in a `foreach` and then `.Count()` is called. How many times does the source run, and what do you change?
4. When would you throw a custom exception and when return a `Result<T>`?
5. What does `where T : notnull` guarantee that `where T : class` does not?

## 9. Further Reading
- [C# language reference](https://learn.microsoft.com/dotnet/csharp/language-reference/) (records, patterns, nullable reference types)
- [Nullable reference types](https://learn.microsoft.com/dotnet/csharp/nullable-references)
- [Framework design guidelines: exceptions](https://learn.microsoft.com/dotnet/standard/design-guidelines/exceptions)

---
**Next:** [02 HTTP Server](../02-http-server/README.md)
