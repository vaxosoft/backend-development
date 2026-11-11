# 12. Testing in C#

> Estimated time: 2h | Difficulty: Intermediate | Prerequisites: 01, 02 (DI, `WebApplication`), 05 (SQLite), 11 (time-dependent rules)

## Learning Outcomes
After this module you can:
- [ ] Write unit tests with xUnit `[Fact]`/`[Theory]` (table-driven) and the built-in `Assert`
- [ ] Test HTTP endpoints in memory with `WebApplicationFactory<Program>` and replace services per test
- [ ] Isolate dependencies with NSubstitute substitutes and hand-written fakes, and know which to pick
- [ ] Make time deterministic with `TimeProvider`/`FakeTimeProvider`
- [ ] Run integration tests against in-memory SQLite without leaking state between tests
- [ ] Write a BenchmarkDotNet benchmark and run it in Release

## 1. Concept Overview
A useful test suite is a pyramid: many fast **unit tests** on pure logic, fewer **component tests** that run a real HTTP pipeline in memory with the slow or non-deterministic parts replaced (clock, network, notifier), and a few **integration tests** that use a real database engine. Tests must be deterministic (no wall clock, no random, no network), isolated (no shared state, any order) and fast.

The tools are the same everywhere: a **fake** is a small working implementation (an in-memory store); a **mock/substitute** is generated and records calls so you can verify an interaction; a **contract test** runs one set of assertions against every implementation of an interface so a fake can be trusted. Benchmarks answer a different question ("how fast?") and have their own runner because naive stopwatch timing lies.

## 2. How C# Approaches It

| Need | Tool | Notes |
|------|------|-------|
| Test framework | xUnit 2.9.3 (`[Fact]`, `[Theory]`, `[InlineData]`, `[MemberData]`) | a **new class instance per test**, so instance fields are isolated; `IClassFixture<T>` shares expensive setup deliberately |
| Assertions | xUnit `Assert` | `Assert.Equal`, `Assert.Throws<T>`, `Assert.Single`, `Assert.All`. FluentAssertions is no longer free, so this repo does not use it |
| HTTP tests | `Microsoft.AspNetCore.Mvc.Testing` (`WebApplicationFactory<Program>`) | runs the real pipeline on `TestServer`: no sockets, no ports |
| Substitutes | NSubstitute 6.2.0 | `Substitute.For<T>()`, `.Returns`, `.Received()`, `Arg.Any/Is/Do` |
| Time | `TimeProvider` + `FakeTimeProvider` (`Microsoft.Extensions.TimeProvider.Testing`) | `Advance()` fires timers and `Task.Delay(.., provider)` with no real waiting |
| Database | in-memory SQLite via `Microsoft.Data.Sqlite` | one open connection per test = one fresh database |
| Benchmarks | BenchmarkDotNet 0.15.8 | must run in `-c Release`; handles warm-up and statistics |

Idiomatic: depend on small interfaces (`ITaskStore`) and `TimeProvider`, register them in DI, and swap them with `ConfigureTestServices`. For minimal APIs the app needs `public partial class Program;` (top-level statements generate an internal `Program`) so the test project can name it. Non-idiomatic: `DateTime.UtcNow` inside business code, static singletons, `Thread.Sleep` in tests, and tests that depend on execution order.

Layout note: unlike other modules, this one has a third project, [app/](app/M12.TaskApi.csproj) (a small Task Tracker API: `ITaskStore`, `SqliteTaskStore`, `Program`). It is the system under test for both the examples and the exercises, just like a real solution where tests reference the web project. Run everything from `languages/csharp/`.

## 3. Guided Examples
Run with `dotnet run --project modules/12-testing/examples -- <name>`. The examples are plain programs that perform what a test would do, so you can see each technique's output.

### 3.1 Deterministic time with `FakeTimeProvider`
- **Goal:** control "now", timers and delays from the test
- **Run:** `dotnet run --project modules/12-testing/examples -- 01-fake-time`
- **Code:** [01_FakeTime.cs](examples/01_FakeTime.cs)
- **Walkthrough:** everything takes a `TimeProvider`; the fake starts at a fixed instant, `Advance` moves it. Timers fire for every period that elapsed, and a one-hour `Task.Delay(…, clock)` completes instantly once advanced.
  ```csharp
  var clock = new FakeTimeProvider(new DateTimeOffset(2030, 1, 1, 9, 0, 0, TimeSpan.Zero));
  clock.Advance(TimeSpan.FromMinutes(9));
  ```
- **Expected output:**
  ```
  created_at is exactly 2030-01-01T09:00:00.0000000+00:00
  at +0m   session alive: True
  at +9m   session alive: True
  at +11m  session alive: False
  timer ticks after 3.5 virtual seconds: 3
  one-hour delay completed before advancing: False
  one-hour delay completed after advancing: True
  ```

### 3.2 NSubstitute substitutes
- **Goal:** replace a dependency and verify interactions
- **Run:** `dotnet run --project modules/12-testing/examples -- 02-substitutes`
- **Code:** [02_Substitutes.cs](examples/02_Substitutes.cs)
- **Walkthrough:** `Substitute.For<ITaskStore>()` implements the interface; unconfigured calls return defaults (`null` here, completed tasks for async). `Returns` stubs, `Received(1)` / `DidNotReceive()` verify. Prefer asserting on results; verify calls only when the call *is* the behaviour (a notification, a write).
  ```csharp
  store.GetAsync(1, Arg.Any<CancellationToken>()).Returns(Item(1, "todo"));
  await store.Received(1).SetStatusAsync(1, "done", Arg.Any<CancellationToken>());
  ```
- **Expected output:**
  ```
  unknown id -> False
  todo task -> True
  store received SetStatusAsync(1, "done") exactly once
  already done -> False
  store failure surfaces as: db down
  ```

### 3.3 In-memory SQLite isolation
- **Goal:** why an in-memory database must keep its connection open, and how to get a fresh one per test
- **Run:** `dotnet run --project modules/12-testing/examples -- 03-sqlite-isolation`
- **Code:** [03_SqliteIsolation.cs](examples/03_SqliteIsolation.cs), store in [SqliteTaskStore.cs](app/SqliteTaskStore.cs)
- **Walkthrough:** `Data Source=:memory:` is private to one connection; `Close()` destroys the data. `SqliteTaskStore` owns one open connection for its lifetime, so `new SqliteTaskStore()` in a test is a fresh, isolated database.
- **Expected output:**
  ```
  same connection sees the table: 1 row
  after Close()+Open() the database is gone: SQLite Error 1: 'no such table: t'.
  first store has 1 task, second has 0
  ```

### 3.4 BenchmarkDotNet
- **Goal:** measure two implementations properly
- **Run:** `dotnet run -c Release --project modules/12-testing/examples -- 04-benchmark` (about 45 s; Debug builds are rejected by the optimizer validator)
- **Code:** [04_Benchmark.cs](examples/04_Benchmark.cs)
- **Walkthrough:** `[Benchmark]` methods on a public class; BenchmarkDotNet does warm-up, many iterations and statistics. The example uses `Job.ShortRun` in-process and a quiet logger so it prints only the stable conclusion. Normally you call `BenchmarkRunner.Run<T>()` and read the table (mean, allocated memory); absolute numbers differ per machine, so assert on shape, never in a unit test.
- **Expected output:**
  ```
  benchmarks run: 2
  fastest: Join
  ```

### 3.5 Task Tracker: `WebApplicationFactory<Program>`
- **Goal:** test the real HTTP pipeline in memory with a fake clock
- **Run:** `dotnet run --project modules/12-testing/examples -- 05-in-memory-http`
- **Code:** [05_InMemoryHttp.cs](examples/05_InMemoryHttp.cs), app in [app/Program.cs](app/Program.cs)
- **Walkthrough:** the factory boots `Program` on `TestServer`; `ConfigureTestServices` runs after the app's own registrations, so the fake `TimeProvider` wins and `created_at` is exact. `CreateClient()` gives an `HttpClient` wired to the server. (A console example must `cd` to the output folder first; `dotnet test` already runs there.) Errors are `application/problem+json` (the C# equivalent of the shared error envelope).
  ```csharp
  await using var factory = new WebApplicationFactory<Program>().WithWebHostBuilder(host =>
      host.ConfigureTestServices(s => s.AddSingleton<TimeProvider>(clock)));
  using var client = factory.CreateClient();
  ```
- **Expected output:**
  ```
  POST /tasks -> 201 Location=/tasks/1
  {"id":1,"title":"Write docs","status":"todo","due_date":"2030-01-31","created_at":"2030-01-01T09:00:00Z"}
  POST /tasks (empty title) -> 422 application/problem+json
  GET /tasks/99 -> 404
  GET /tasks -> {"items":[{"id":1,"title":"Write docs","status":"todo","due_date":"2030-01-31","created_at":"2030-01-01T09:00:00Z"}],"total":1}
  ```

In a real test class this is `public class TasksApiTests(WebApplicationFactory<Program> factory) : IClassFixture<WebApplicationFactory<Program>>`: the factory (and its server) is shared by the class, so give each test its own data (or its own `WithWebHostBuilder` factory, as the exercise does). Code coverage comes from `dotnet test --collect:"XPlat Code Coverage"`, which needs the `coverlet.collector` package; this repo does not pin it, so it is only mentioned here.

## 4. Common Pitfalls

| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| `static` fields or a static in-memory store shared by tests | xUnit runs test **classes** in parallel; statics are shared by all | instance fields (new instance per test); `[Collection]` only when sharing is intended |
| Closing the in-memory SQLite connection between steps | `:memory:` data lives and dies with the connection | keep one open connection per test (see 3.3), or `Mode=Memory;Cache=Shared` with a keeper connection |
| `WebApplicationFactory<Program>` does not compile: "Program is inaccessible" | top-level statements generate an internal `Program` | add `public partial class Program;` to the app |
| Replacing a service in `ConfigureServices` and the original still wins | the app registered later, or you replaced the wrong lifetime | use `ConfigureTestServices` (runs after the app's registrations); `AddSingleton` the fake for the same service type |
| `DateTime.UtcNow` / `Task.Delay` in code under test, then `Thread.Sleep` in the test | no seam for time | inject `TimeProvider`; `FakeTimeProvider.Advance` |
| Asserting `Received()` on everything | substitutes make verification easy | assert on outputs and state; verify calls only for side effects, and use `Arg.Any<CancellationToken>()` for incidental args |
| Mocking what you do not own (e.g. `HttpClient`) | looks convenient | fake the `HttpMessageHandler` or call a loopback server; wrap and fake your own interface |
| Benchmarks in Debug, or timing with `Stopwatch` in a test | JIT tiers, no warm-up, noisy | BenchmarkDotNet with `-c Release`; do not assert on timings in unit tests |
| `async void` test methods or forgetting `await` | the test "passes" before the assertion runs | return `Task`; `await Assert.ThrowsAsync<T>(...)` |

## 5. Coming From Other Languages
- **Java:** xUnit `[Fact]` ~ JUnit `@Test`; `[Theory]` ~ `@ParameterizedTest`; NSubstitute ~ Mockito; `WebApplicationFactory` ~ `@SpringBootTest` + `MockMvc`/`TestRestTemplate`. JUnit makes one instance per class by default; xUnit makes one per test.
- **TypeScript/JS:** `[Theory]` ~ `it.each`; NSubstitute ~ `jest.fn()`; `FakeTimeProvider.Advance` ~ `jest.advanceTimersByTime`; the factory ~ `supertest(app)` without a listening port.
- **Go:** table-driven tests with `t.Run` are `[Theory]`; `httptest.NewServer` ~ `TestServer`; Go fakes are plain structs implementing small interfaces, which C# does as well (3.1 and Exercise 3).
- **Python:** `pytest.mark.parametrize` ~ `[Theory]`; fixtures ~ constructor + `IDisposable`/`IClassFixture`; `freezegun` ~ `FakeTimeProvider`, but explicit.

## 6. Exercises
Starter code and tests live in [exercises/](exercises/); solutions in [solutions/](solutions/). Because a starter must fail, these are implementation tasks whose acceptance tests demonstrate a testing technique. Tests reference the system under test in [app/](app/).

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | Task validator | Easy | pure rules, `[Theory]` with `InlineData`/`MemberData` | `dotnet test modules/12-testing/exercises --filter "FullyQualifiedName~.Ex1_"` |
| 2 | Reminder service | Medium | NSubstitute + `FakeTimeProvider` isolation | `dotnet test modules/12-testing/exercises --filter "FullyQualifiedName~.Ex2_"` |
| 3 | In-memory store | Stretch | fake validated by a shared contract; plugged into `WebApplicationFactory` | `dotnet test modules/12-testing/exercises --filter "FullyQualifiedName~.Ex3_"` |

**Exercise 1 - [Ex1_TaskValidator.cs](exercises/Ex1_TaskValidator.cs)**
- **Task:** return a field -> message dictionary: `title` must be 1-200 characters after trimming, `due_date` must not be before `today`.
- **Acceptance:** the theory tables (null/blank/200/201-character titles, due dates around `today`), exact messages, and a valid task yields no errors.
- <details><summary>Hint</summary>A property pattern such as <code>task.Title?.Trim().Length is not (>= 1 and <= 200)</code> handles null and the range in one expression.</details>

**Exercise 2 - [Ex2_ReminderService.cs](exercises/Ex2_ReminderService.cs)**
- **Task:** notify once per overdue (due before today's UTC date, not `done`) task, in due-date order, using the injected clock and store; keep going when one notification fails and report all failures in an `AggregateException`.
- **Acceptance:** substitutes for `ITaskStore` and `IReminderNotifier`, a `FakeTimeProvider` that moves a task from "due today" to "overdue", ordering via `Arg.Do`, failure isolation, cancellation token pass-through.
- <details><summary>Hint</summary>Compute <code>DateOnly.FromDateTime(time.GetUtcNow().UtcDateTime)</code> once, filter and sort with LINQ, then loop with try/catch collecting exceptions.</details>

**Exercise 3 - [Ex3_InMemoryTaskStore.cs](exercises/Ex3_InMemoryTaskStore.cs)**
- **Task:** implement a thread-safe in-memory `ITaskStore` that behaves like `SqliteTaskStore` (ids from 1, never reused, status `todo`, ordered by id).
- **Acceptance:** the shared `Ex3_TaskStoreContract` passes for your store **and** for the SQLite store (proving the fake is faithful), 50 concurrent adds get distinct ids, and two `WebApplicationFactory<Program>` tests run the HTTP API on your store with a fake clock.
- <details><summary>Hint</summary>A <code>lock</code> around a <code>SortedDictionary</code> plus a monotonically increasing counter is enough; return a snapshot list from <code>ListAsync</code>.</details>

## 7. Recap
- Inject seams (`ITaskStore`, `TimeProvider`) and test through them; never reach for static clocks.
- `[Theory]` gives table-driven tests; xUnit creates a new class instance per test, so keep state in instance fields.
- `WebApplicationFactory<Program>` + `ConfigureTestServices` tests the real pipeline in memory.
- Fakes need a contract test; substitutes are for verifying side effects.
- Reach for BenchmarkDotNet (Release) for performance questions, never for assertions.

## 8. Self-check Questions
1. Why does `public partial class Program;` appear in a web app, and what breaks without it?
2. When would you write a fake instead of an NSubstitute substitute, and how do you keep the fake honest?
3. Two test classes both use a `static` list as a store; why can the suite be flaky even though each test is correct alone?
4. `ConfigureServices` vs `ConfigureTestServices`: which one lets a test override the app's registration, and why?
5. Why does `FakeTimeProvider.Advance(TimeSpan.FromHours(1))` complete `Task.Delay(TimeSpan.FromHours(1), clock)` immediately?

## 9. Further Reading
- [Integration tests in ASP.NET Core](https://learn.microsoft.com/aspnet/core/test/integration-tests)
- [xUnit.net: shared context and parallelism](https://xunit.net/docs/shared-context)
- [NSubstitute docs](https://nsubstitute.github.io/help/getting-started/) and [BenchmarkDotNet](https://benchmarkdotnet.org/articles/overview.html)

---
**Next:** [13 Time and Standard Library Utilities](../13-time-and-stdlib-utilities/README.md)
