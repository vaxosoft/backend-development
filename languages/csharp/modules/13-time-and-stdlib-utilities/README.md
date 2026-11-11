# 13. Time, Utilities and Standard Library in C#

> Estimated time: 1.5-2h | Difficulty: Beginner-Intermediate | Prerequisites: 01, 06 (tasks, cancellation), 12 (`FakeTimeProvider`)

## Learning Outcomes
After this module you can:
- [ ] Store and exchange instants as `DateTimeOffset` (UTC) and convert to a time zone only to display or to decide "which calendar day"
- [ ] Handle DST gaps/overlaps and compute with `TimeSpan`, `DateOnly` and ISO 8601 text
- [ ] Schedule in-process work with `PeriodicTimer` and `BackgroundService`, and shut it down cleanly
- [ ] Generate ids and random values correctly (`Guid.CreateVersion7`, `RandomNumberGenerator`) and hash/sign with SHA-256 and HMAC
- [ ] Use `[GeneratedRegex]` safely (timeouts, `NonBacktracking`) and render simple text templates

## 1. Concept Overview
Time is the most common source of backend bugs. An **instant** (a point on the timeline) and a **local date/time** (what a wall clock shows in some zone) are different things: store and compare instants in UTC, keep the user's zone separately, and convert only at the edges. "Due on 2030-01-31" is a *calendar date* (no zone); "overdue" depends on what day it is *for the user*. Anything that reads the clock should get it from an injected abstraction so tests can control it.

The rest of the module is the everyday standard library: unique ids (random or time-ordered), randomness that is cryptographically strong when it must be, hashes and HMACs, regular expressions for validation and extraction, simple templating, and a way to run a job every N minutes inside the service without a scheduler product.

## 2. How C# Approaches It

| Need | BCL type | Notes |
|------|----------|-------|
| An instant | `DateTimeOffset` | carries an offset; compares by instant. Prefer it over `DateTime` |
| A calendar date / time of day | `DateOnly`, `TimeOnly` | no zone, no `Kind` ambiguity; serialized as `2030-01-31` |
| Durations | `TimeSpan` | `DateTimeOffset - DateTimeOffset` gives one |
| Zones | `TimeZoneInfo.FindSystemTimeZoneById("Europe/Berlin")`, `ConvertTime`, `IsInvalidTime`, `IsAmbiguousTime` | IANA ids (`Europe/Berlin`) work on Linux/macOS (needs the `tzdata` package) and, since .NET 6, on Windows too |
| Testable clock | `TimeProvider` (`GetUtcNow`, `GetTimestamp`, `CreateTimer`) | `TimeProvider.System` in production, `FakeTimeProvider` in tests |
| Periodic work | `PeriodicTimer`, `BackgroundService` | `await timer.WaitForNextTickAsync(ct)` in a loop |
| Ids | `Guid.NewGuid()` (v4), `Guid.CreateVersion7()` | v7 starts with a millisecond timestamp, so it sorts and indexes well |
| Randomness | `Random.Shared` (not secret), `RandomNumberGenerator` (secrets/tokens) | `Random(seed)` is repeatable |
| Hashing | `SHA256.HashData`, `HMACSHA256.HashData`, `Convert.ToHexStringLower`, `Base64Url` | MD5/SHA-1 only for checksums, never for security |
| Regex | `[GeneratedRegex]` source generator, `Regex` with `matchTimeout`, `RegexOptions.NonBacktracking` | compile-time, fast, no startup cost |
| Templating | raw string literals (`"""`), `$$"""` interpolation, `CompositeFormat`, `Regex.Replace` with a `MatchEvaluator` | Razor/Scriban-style engines are out of scope |

Idiomatic: take `TimeProvider` as a constructor parameter, use `PeriodicTimer` (not `Task.Delay` loops), and mark regexes `static partial` with `[GeneratedRegex(..., matchTimeoutMilliseconds: N)]`. Non-idiomatic: `DateTime.Now`/`DateTime.UtcNow` sprinkled through business logic, `System.Random` for tokens, string-concatenated dates.

## 3. Guided Examples
All commands run from `languages/csharp/`: `dotnet run --project modules/13-time-and-stdlib-utilities/examples -- <name>`.

### 3.1 Dates, offsets and zones
- **Goal:** instants vs local dates, DST edge cases, ISO 8601
- **Run:** `dotnet run --project modules/13-time-and-stdlib-utilities/examples -- 01-dates-and-zones`
- **Code:** [01_DatesAndZones.cs](examples/01_DatesAndZones.cs)
- **Walkthrough:** 23:30 UTC on Jan 31 is the *same instant* as 00:30 on Feb 1 in Berlin but a different calendar day, so a due date must be judged in the user's zone. Around the March DST change the Berlin offset jumps from +01:00 to +02:00; 02:30 on that day never exists (`IsInvalidTime`) and 02:30 on the October change happens twice (`IsAmbiguousTime`). The `"O"` format round-trips; a plain `DateTime` has `Kind == Unspecified`, which is why it is risky.
  ```csharp
  var local = TimeZoneInfo.ConvertTime(instant, berlin);
  ```
- **Expected output:**
  ```
  UTC    2030-01-31T23:30:00.0000000+00:00
  Berlin 2030-02-01T00:30:00.0000000+01:00
  same instant: True, same calendar day: False
  offset before DST change: 01:00:00, one hour later: 02:00:00
  2030-03-31 02:30 invalid: True
  2030-10-27 02:30 ambiguous: True
  ISO 8601 round trip: True
  due date end of day in Berlin as UTC: 2030-01-31T22:59:59.0000000Z
  elapsed: 2.12:30:00 = 60.5 hours
  DateTime.Kind of a plain DateTime: Unspecified
  ```

### 3.2 `PeriodicTimer`
- **Goal:** a drift-free, cancellable tick loop
- **Run:** `dotnet run --project modules/13-time-and-stdlib-utilities/examples -- 02-timers`
- **Code:** [02_Timers.cs](examples/02_Timers.cs)
- **Walkthrough:** ticks come on a fixed cadence regardless of how long the loop body takes (a `Task.Delay(period)` loop drifts by the body's duration), and a slow body never overlaps the next one; missed ticks coalesce into one. Cancelling the token makes `WaitForNextTickAsync` throw `OperationCanceledException`; a disposed timer returns `false`. The example uses the real clock with 50 ms periods and prints only stable facts.
- **Expected output:**
  ```
  tick 1
  tick 2
  tick 3
  3 ticks took at least 140 ms: True
  cancelled; ticks seen before cancellation: True
  disposed timer yields: False
  ```

### 3.3 Ids, randomness and hashing
- **Goal:** pick the right id/random/hash primitive
- **Run:** `dotnet run --project modules/13-time-and-stdlib-utilities/examples -- 03-ids-random-hash`
- **Code:** [03_IdsRandomHash.cs](examples/03_IdsRandomHash.cs)
- **Walkthrough:** v4 GUIDs are random; v7 embeds the timestamp in the first 48 bits (`01234567-89ab` is the millisecond value we passed), so later ids sort later as strings, which is friendly to database indexes. A seeded `Random` is repeatable (good for tests, never for secrets). `RandomNumberGenerator` is the cryptographic source. SHA-256 and HMAC outputs are checked against published test vectors.
- **Expected output:**
  ```
  Guid.NewGuid() is version 4
  v7 at fixed time: 01234567-89ab-7...
  later id sorts after (ordinal string order): True
  seeded Random repeats: True
  crypto random in range: True
  random token length: 32
  sha256(abc) = ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad
  hmac-sha256  = f7bc83f430538424b13298e6aa6fb143ef4d59a14946175997479dbc2d1a3cd8
  constant-time compare: True
  ```

### 3.4 Regex and templates
- **Goal:** generated regexes, ReDoS defence, small templates
- **Run:** `dotnet run --project modules/13-time-and-stdlib-utilities/examples -- 04-regex-and-templates`
- **Code:** [04_RegexAndTemplates.cs](examples/04_RegexAndTemplates.cs)
- **Walkthrough:** `[GeneratedRegex]` on a `static partial` method emits the matcher at compile time. `(a+)+$` on `aaaa…!` backtracks exponentially: with a match timeout it fails fast with `RegexMatchTimeoutException`; `NonBacktracking` answers in linear time. The template renderer is one regex plus a `MatchEvaluator` that HTML-encodes values; raw string literals (`"""`) keep multi-line text and JSON readable and `$$"""` lets `{{x}}` interpolate.
  ```csharp
  [GeneratedRegex(@"#(?<tag>[a-z0-9_-]{1,30})", RegexOptions.CultureInvariant, matchTimeoutMilliseconds: 200)]
  private static partial Regex TagPattern();
  ```
- **Expected output:**
  ```
  tags: docs,p1
  backtracking engine: gave up after the 100 ms timeout
  NonBacktracking engine: matched=False
  Hello &lt;Ada&gt;,
  you have 3 open tasks.
  missing value: no value for 'count'
  {"title":"Write docs","status":"todo"}
  [in_progress] Write docs
  ```

### 3.5 Task Tracker: overdue-reminder `BackgroundService`
- **Goal:** an in-process scheduled job that is testable and stops cleanly
- **Run:** `dotnet run --project modules/13-time-and-stdlib-utilities/examples -- 05-reminder-worker`
- **Code:** [05_ReminderWorker.cs](examples/05_ReminderWorker.cs)
- **Walkthrough:** the worker owns a `PeriodicTimer` created with the injected `TimeProvider`, so the demo drives it with `FakeTimeProvider.Advance(12h)` and gets exact, instant ticks. Each tick it reports newly overdue tasks once. It publishes "worker started" after creating the timer because advancing a fake clock *before* the timer exists would never fire it (a real clock has no such race). `StopAsync` cancels the token; the loop catches the cancellation and exits normally.
  ```csharp
  using var timer = new PeriodicTimer(TimeSpan.FromHours(12), time);
  while (await timer.WaitForNextTickAsync(stoppingToken)) { /* work */ }
  ```
- **Expected output:**
  ```
  worker started
  2030-01-03 10:00Z reminders: [Write docs]
  2030-01-03 22:00Z reminders: []
  2030-01-04 10:00Z reminders: [Ship]
  worker stopped
  ```

## 4. Common Pitfalls

| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| `DateTime.Now` / `DateTime.UtcNow` in server code | it is the shortest way, and it cannot be faked | inject `TimeProvider`; `GetUtcNow()` returns a `DateTimeOffset` |
| Storing `DateTime` with `Kind.Unspecified` | `new DateTime(...)` and many parsers leave `Kind` unset; `ToUniversalTime()` then assumes *local* | use `DateTimeOffset` for instants, `DateOnly` for dates |
| Comparing `instant.Date` across zones | `DateTimeOffset.Date` is the date *in its own offset* | `TimeZoneInfo.ConvertTime(now, zone)` first, then `.Date`/`DateOnly` |
| `while (true) { work(); await Task.Delay(period); }` | delays accumulate the work time (drift) and cannot be driven by a fake clock | `PeriodicTimer` with a `TimeProvider` |
| Forgetting that `PeriodicTimer` throws on cancellation | `WaitForNextTickAsync(ct)` throws `OperationCanceledException` | catch it in `ExecuteAsync`, and pass `stoppingToken` to the work too |
| Exception inside `ExecuteAsync` stops the host | an unhandled exception ends the `BackgroundService` (and by default the host) | catch per-iteration, log, continue |
| Fixed-offset time zones for "Europe/Berlin" | hard-coding `+01:00` ignores DST | `TimeZoneInfo` id, and `IsInvalidTime`/`IsAmbiguousTime` for local inputs |
| `new Random()` for tokens, or `Guid.NewGuid()` as a secret | looks random | `RandomNumberGenerator.GetBytes` for secrets; ids are identifiers, not secrets |
| `new Random()` per call in a hot path, or sharing one instance across threads | allocation churn; `Random` instances are not thread-safe | `Random.Shared` (thread-safe) or one instance per owner |
| `new Regex(userInput)` or patterns with nested quantifiers and no timeout | catastrophic backtracking = CPU denial of service | `matchTimeout`, `NonBacktracking`, never compile user-supplied patterns |

## 5. Coming From Other Languages
- **Java:** `DateTimeOffset`/`DateOnly`/`TimeSpan` ~ `OffsetDateTime`/`LocalDate`/`Duration`; legacy `DateTime` ~ `java.util.Date` (avoid). `TimeProvider` ~ `java.time.Clock`.
- **TypeScript/JS:** `Date` hides the zone problem; use `DateTimeOffset` like you would use Temporal's `Instant`/`ZonedDateTime`.
- **Go:** `time.Time` carries a location like `DateTimeOffset` does an offset; `time.Ticker` ~ `PeriodicTimer` (which coalesces instead of dropping silently); `clock` interfaces ~ `TimeProvider`.
- **Python:** naive vs aware `datetime` = `Kind.Unspecified` vs `DateTimeOffset`; `uuid.uuid7` ~ `Guid.CreateVersion7`; `secrets` ~ `RandomNumberGenerator`.

## 6. Exercises
Starter code and tests live in [exercises/](exercises/); solutions in [solutions/](solutions/). Tests are deterministic and use `FakeTimeProvider` or fixed instants. Exercise 1 uses the `Europe/Berlin` zone, which needs system tzdata (present on any normal Linux/macOS/Windows install).

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | Due dates in the user's zone | Easy | `TimeZoneInfo`, `DateOnly`, DST-aware instants | `dotnet test modules/13-time-and-stdlib-utilities/exercises --filter "FullyQualifiedName~.Ex1_"` |
| 2 | Slugs, hashtags and templates | Medium | `[GeneratedRegex]`, `MatchEvaluator`, HTML encoding | `dotnet test modules/13-time-and-stdlib-utilities/exercises --filter "FullyQualifiedName~.Ex2_"` |
| 3 | Periodic job runner | Stretch | `PeriodicTimer` + `TimeProvider`, error isolation, cooperative shutdown | `dotnet test modules/13-time-and-stdlib-utilities/exercises --filter "FullyQualifiedName~.Ex3_"` |

**Exercise 1 - [Ex1_DueDates.cs](exercises/Ex1_DueDates.cs)**
- **Task:** implement `LocalToday`, `IsOverdue`, `DaysUntilDue` and `EndOfDayUtc` relative to a `TimeZoneInfo`.
- **Acceptance:** the same instant gives different local dates in UTC+2 and UTC-5, the offset carried by the input is ignored, due-today is not overdue, `null` dates are handled, and the end of day in Berlin is 22:59:59Z in winter and 21:59:59Z in summer.
- <details><summary>Hint</summary><code>TimeZoneInfo.ConvertTime(now, zone).DateTime</code> is the local wall clock; <code>zone.GetUtcOffset(localDateTime)</code> gives the offset in force at that local time.</details>

**Exercise 2 - [Ex2_Text.cs](exercises/Ex2_Text.cs)**
- **Task:** `Slugify`, `ExtractTags` (1-30 chars, lower-case, distinct, ordered) and `Render` (`{{ name }}` placeholders, HTML-encoded values, `KeyNotFoundException` for unknown names).
- **Acceptance:** the slug table, 100k-character separator runs finish quickly, over-long tags ignored, substituted values are not re-processed.
- <details><summary>Hint</summary>One <code>[GeneratedRegex]</code> per job; <code>Regex.Replace(input, match =&gt; ...)</code> processes each match once, which also gives you the "no re-processing" property.</details>

**Exercise 3 - [Ex3_JobRunner.cs](exercises/Ex3_JobRunner.cs)**
- **Task:** run a job every interval (not at start), never overlapping itself, reporting failures to `onError` and continuing, and ending normally on cancellation, even mid-job.
- **Acceptance:** run counts driven by `FakeTimeProvider.Advance`, failure isolation, normal completion when cancelled idle or inside a job, maximum concurrency of 1.
- <details><summary>Hint</summary>Create the <code>PeriodicTimer</code> with the <code>TimeProvider</code> before the first await; catch <code>OperationCanceledException</code> only when <code>ct.IsCancellationRequested</code>.</details>

## 7. Recap
- Instants in UTC (`DateTimeOffset`), dates as `DateOnly`, zones only at the edges; inject `TimeProvider`.
- `PeriodicTimer` + `BackgroundService` is the built-in scheduler; handle errors per iteration and honour the stopping token.
- `Guid.CreateVersion7` for sortable ids, `RandomNumberGenerator` for secrets, SHA-256/HMAC for integrity.
- `[GeneratedRegex]` with a timeout (or `NonBacktracking`); keep templates tiny and encode output.
- When in doubt: `DateTimeOffset`, not `DateTime`.

## 8. Self-check Questions
1. A task due `2030-01-31` is created by a user in Auckland and checked by a server in UTC. Which two values do you need to decide whether it is overdue, and where is each stored?
2. What is the difference between an invalid and an ambiguous local time, and which `TimeZoneInfo` methods detect them?
3. Why does a `Task.Delay` loop drift and a `PeriodicTimer` not, and what happens to ticks missed while the body runs?
4. Why can't you simply `Advance` a `FakeTimeProvider` right after starting a hosted service, and how does the example avoid the race?
5. When is `Random.Shared` acceptable and when must you use `RandomNumberGenerator`?

## 9. Further Reading
- [DateTime, DateTimeOffset, TimeSpan and TimeZoneInfo: choosing between them](https://learn.microsoft.com/dotnet/standard/datetime/choosing-between-datetime)
- [TimeProvider and testing with `FakeTimeProvider`](https://learn.microsoft.com/dotnet/core/extensions/timeprovider)
- [.NET regular expression source generators](https://learn.microsoft.com/dotnet/standard/base-types/regular-expression-source-generators) and [best practices](https://learn.microsoft.com/dotnet/standard/base-types/best-practices-regex)
- [Background tasks with hosted services](https://learn.microsoft.com/aspnet/core/fundamentals/host/hosted-services)

---
**Next:** [14 Packaging and Running](../14-packaging-and-running/README.md)
