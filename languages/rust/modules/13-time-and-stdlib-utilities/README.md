# 13. Time, Utilities and the Standard Library in Rust

> Estimated time: 2h | Difficulty: Intermediate | Prerequisites: 01, 06 (tokio basics), 04 (serialization helps)

## Learning Outcomes
After this module you can:
- [ ] Choose between instants (`DateTime<Utc>`), calendar dates (`NaiveDate`) and durations, and store/transmit time as UTC
- [ ] Convert to and from time zones with `chrono-tz` and handle DST gaps and overlaps explicitly
- [ ] Generate UUIDs (v4/v7) and random values, and know when a seeded generator is appropriate
- [ ] Use regular expressions safely (compile once, linear time) and render text/HTML with `minijinja`
- [ ] Compute SHA-256 and HMAC digests and verify them
- [ ] Run an in-process periodic job with `tokio::time::interval` and stop it cleanly

## 1. Concept Overview
Time is the biggest source of avoidable bugs in services. Three different things are all called "time": an **instant** (a point on the UTC timeline: `created_at`), a **calendar date or wall-clock time** (a due date `2030-01-31`, "every day at 08:30"), and a **duration** (a timeout of 1.5 s). Servers store and exchange instants in UTC (RFC 3339), keep zones as presentation details, and never call the system clock deep inside logic that needs testing: they pass `now` in.

The rest of the module is the "small tools" drawer: identifiers (UUID), randomness, regex, templates, hashing/HMAC, and background scheduling inside the process (no external scheduler).

| Need | Use |
|------|-----|
| Timestamp in the API | `DateTime<Utc>` ↔ RFC 3339 `"2030-01-01T09:00:00Z"` |
| Due date, birthday | `NaiveDate` ↔ `"2030-01-31"` |
| Show time to a user | `dt.with_timezone(&tz)` |
| Timeout / sleep | `std::time::Duration` (`tokio::time` uses it) |
| Sortable ids | UUID v7; opaque random ids: v4 |
| Tokens, slugs | `rand::rng()` (CSPRNG) |
| Integrity / ETag | SHA-256; authenticity: HMAC-SHA-256 |

## 2. How Rust Approaches It
- **`std` has `Instant`/`SystemTime`/`Duration` only**: no calendar, no zones, no UUID, no regex, no RNG. The standard ecosystem fills the gap: `chrono` (+`chrono-tz`), `uuid`, `rand`, `regex`, `minijinja`, `sha2`/`hmac`.
- **Three `Duration`s:** `std::time::Duration` (unsigned; timeouts, `sleep`, `Instant`), `chrono::Duration` = `TimeDelta` (signed; calendar math), and tokio re-exports std's. Convert with `Duration::from_std` / `.to_std()` (negative values fail).
- **Types prevent mix-ups:** `NaiveDate` has no zone, `DateTime<Utc>` always does; `Utc.with_ymd_and_hms(..)` returns a `LocalResult` (use `.unwrap()` only for known-good constants). Zone conversions return `LocalResult::{Single, Ambiguous, None}` and the compiler makes you handle them.
- **Clock injection:** pass `now: DateTime<Utc>` (or an `Instant`) as an argument; the exercises never read the clock.
- **`rand` 0.10:** `rand::rng()` is the thread-local CSPRNG; extension methods (`random_range`, `fill`, `sample_iter`) come from `RngExt`; `StdRng::seed_from_u64` is reproducible (tests only).
- **Regex:** the `regex` crate is linear time (no catastrophic backtracking, no look-around). Compile once in a `LazyLock<Regex>`.
- **Templates:** `minijinja` (Jinja syntax) auto-escapes templates named `*.html`.
- **Scheduling:** `tokio::time::interval` + `select!` + `CancellationToken`. `interval`'s **first tick fires immediately**.

## 3. Guided Examples
Run all commands from `languages/rust/`.

### 3.1 Dates, instants and durations
- **Goal:** the vocabulary: parse, add, subtract, format, and which type to use.
- **Run:** `cargo run -q -p m13-time-and-stdlib-utilities --example dates_and_durations`
- **Code:** [examples/dates_and_durations.rs](./examples/dates_and_durations.rs)
- **Walkthrough:** an offset input (`+01:00`) is normalised to UTC. `NaiveDate - NaiveDate` yields a `Duration` in days. `checked_add_months` clamps to the end of February (calendar math), while `+ Duration::days(30)` is exact elapsed time. `Duration::from_std(..)` bridges the two duration types.
  ```rust
  let parsed = DateTime::parse_from_rfc3339("2030-01-01T10:30:00+01:00")?;
  let due = NaiveDate::from_ymd_opt(2030, 1, 31).unwrap();
  let days_left = (due - created_at.date_naive()).num_days();
  ```
- **Expected output:**
  ```
  created_at: 2030-01-01T09:00:00Z
  due_date:   2030-01-31
  offset input in UTC: 2030-01-01T09:30:00Z
  bad date: true
  created + 36h: 2030-01-02T21:00:00Z
  days until due: 30
  waited: 2160 minutes (129600s)
  Jan 31 + 1 month: Some(2030-02-28)
  Jan 31 + 30 days: 2030-03-02
  weekday of due: Thu, is weekend: false
  custom: Tue 01 Jan 2030, 09:00
  std 1.5s -> chrono 1500ms
  a negative chrono duration cannot become a std one: true
  ```

### 3.2 Time zones and DST
- **Goal:** convert for display and face the DST edge cases.
- **Run:** `cargo run -q -p m13-time-and-stdlib-utilities --example time_zones`
- **Code:** [examples/time_zones.rs](./examples/time_zones.rs)
- **Walkthrough:** `"Europe/Berlin".parse::<Tz>()` handles user-supplied names (unknown names are `Err`). The same wall-clock noon is UTC+1 on 29/30 March 2030 and UTC+2 on the 31st (DST began at 02:00). 02:30 on that day never existed (`LocalResult::None`); 02:30 on 27 October happens twice (`Ambiguous`). Decide a policy for each case, as exercise 2 does.
  ```rust
  match berlin.from_local_datetime(&naive) {
      LocalResult::Single(t) => t,
      LocalResult::Ambiguous(first, _second) => first,
      LocalResult::None => /* shift forward past the gap */,
  }
  ```
- **Expected output:**
  ```
  Europe/Berlin     2030-07-01T14:00:00+02:00
  America/New_York  2030-07-01T08:00:00-04:00
  Asia/Kolkata      2030-07-01T17:30:00+05:30
  Mars/Olympus      unknown zone
  Berlin 2030-03-29 12:00 -> 2030-03-29T11:00:00Z
  Berlin 2030-03-30 12:00 -> 2030-03-30T11:00:00Z
  Berlin 2030-03-31 12:00 -> 2030-03-31T10:00:00Z
  02:30 on 2030-03-31 (clocks jump 02:00 -> 03:00): does not exist
  02:30 on 2030-10-27 (clocks go back 03:00 -> 02:00): ambiguous: 2030-10-27T02:30:00+02:00 or 2030-10-27T02:30:00+01:00
  ```

### 3.3 UUIDs, randomness and hashing
- **Goal:** ids, random values and digests, printing only properties of random values.
- **Run:** `cargo run -q -p m13-time-and-stdlib-utilities --example ids_random_hashing`
- **Code:** [examples/ids_random_hashing.rs](./examples/ids_random_hashing.rs)
- **Walkthrough:** v7 UUIDs begin with a millisecond timestamp, so later ids sort later (good for database indexes); the example builds one from a fixed timestamp to show the prefix. `rand::rng()` produces slugs and dice; a *seeded* `StdRng` gives the same shuffle twice (tests/simulations, never secrets). SHA-256 proves integrity, HMAC proves who made it: `verify_slice` checks the tag in constant time.
  ```rust
  let mut mac = Hmac::<Sha256>::new_from_slice(b"shared-secret")?;
  mac.update(b"task.created:42");
  mac.verify_slice(&tag)?; // constant time
  ```
- **Expected output** (random values are not printed):
  ```
  v4 version=4 text length=36
  v7 version=7 early<late: true
  v7 timestamp prefix: 01ba60d3-3800
  parse round trip: true
  bad uuid: true
  slug length=7 alphanumeric=true
  die in 1..=6: true
  same seed, same shuffle: true
  choose from slice: true
  next_u32 bytes differ per call: true
  sha256: ebe8d4d1f04f23b91ed079ef1cdbe37ffa6b9eec136a7ea9a7f186849f4e448a
  hmac:   98b7cde7a30ab5536fce361e3ad5ea6304012357f4ca6a7c20673f04dde12c8a
  verify ok: true
  verify forged: false
  ```

### 3.4 Regex and templates
- **Goal:** extract structure from text and render output without string concatenation.
- **Run:** `cargo run -q -p m13-time-and-stdlib-utilities --example regex_and_templates`
- **Code:** [examples/regex_and_templates.rs](./examples/regex_and_templates.rs)
- **Walkthrough:** patterns live in `static ... LazyLock<Regex>` (compiled once, `expect` is justified because the pattern is a constant). Named groups (`(?P<year>..)`) are read with `&caps["year"]`. A bad pattern is `Err`, never a panic, when it comes from input. `card.html` auto-escapes `<script>`; `reminder.txt` does not, and strict undefined behaviour turns a forgotten variable into an error.
  ```rust
  static TAG: LazyLock<Regex> = LazyLock::new(|| Regex::new(r"#([a-z][a-z0-9_-]{0,29})\b").expect("static pattern is valid"));
  ```
- **Expected output:**
  ```
  tags: ["bug", "release-2"]
  due: year=2030 month=01 day=31
  redacted: Fix login #bug before #release-2 (due <date>). Also #Bug and #bug again, not C#.
  '#Bug' (uppercase) matches the lowercase-only pattern: false
  invalid pattern: true
  Hi Ada, 2 task(s) due:
  - Write docs (2030-01-31)
  - Ship <v1> (2030-02-01)
  <li class="task">&lt;script&gt;alert(1)&lt;&#x2f;script&gt;</li>
  strict mode rejects a missing variable: true
  ```

### 3.5 Task Tracker overdue reminders (background job)
- **Goal:** a periodic in-process job with clean cancellation.
- **Run:** `cargo run -q -p m13-time-and-stdlib-utilities --example overdue_scheduler`
- **Code:** [examples/overdue_scheduler.rs](./examples/overdue_scheduler.rs)
- **Walkthrough:** `run_every` owns an `interval`, sets `MissedTickBehavior::Skip` (the default `Burst` would fire a catch-up run for every missed period after a slow run) and selects between the next tick and `cancel.cancelled()`. The job uses a pretend "today" that advances per run, so the overdue list grows deterministically. Cancelling from inside the job ends the loop; `await` on the `JoinHandle` returns the run count.
  ```rust
  tokio::select! {
      _ = ticker.tick() => { runs += 1; job(runs); }
      _ = cancel.cancelled() => return runs,
  }
  ```
- **Expected output:**
  ```
  first tick immediate: true
  run 1 (2030-01-11): overdue = ["Pay invoices"]
  run 2 (2030-01-12): overdue = ["Pay invoices"]
  run 3 (2030-01-13): overdue = ["Pay invoices", "Renew domain"]
  stopped after 3 runs; at least two periods elapsed: true
  ```

## 4. Common Pitfalls
| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| `Local::now()` / `chrono::Local` in a server | It uses the host's zone, which differs between laptop, container and prod | Use `Utc::now()`; convert with an explicit `Tz` |
| `tokio::time::interval` runs its job *immediately* | The first `tick().await` completes at once | Call `ticker.tick().await` once before the loop, or use `interval_at(Instant::now() + period, period)` |
| Burst of catch-up runs after a slow job | Default `MissedTickBehavior::Burst` | `set_missed_tick_behavior(Skip)` (or `Delay`) |
| `from_local_datetime(..).unwrap()` panics twice a year | The result is `None` in a gap and `Ambiguous` in an overlap | Match all three `LocalResult` variants |
| `NaiveDateTime` stored/serialised as "a timestamp" | No zone, so it cannot identify an instant | Use `DateTime<Utc>` for instants, `NaiveDate` only for calendar dates |
| `chrono::Duration` passed to `tokio::time::sleep` | `sleep` takes `std::time::Duration` | `.to_std()` (it fails for negative values: handle it) |
| `Regex::new(..)` inside a handler | Compilation is expensive and is repeated per request | `static X: LazyLock<Regex>` |
| `thread_rng`/old `rand` examples do not compile | rand 0.10 renamed things (`rand::rng()`, `RngExt::random_range`) | Read the docs for the pinned version |
| Using a seeded or non-crypto source for tokens | Predictable output | `rand::rng()` (CSPRNG) or the OS RNG; seeded only for tests |
| Comparing HMAC tags with `==` | Timing side channel | `Mac::verify_slice` |

## 5. Coming From Other Languages
- **Go:** `time.Time` carries a zone and a monotonic reading in one type; in Rust the type is explicit (`DateTime<Utc>` vs `Instant`). `time.Ticker` ↔ `interval`; `context` cancellation ↔ `CancellationToken`. Go's `regexp` is also linear-time (RE2); both lack look-around.
- **Java/C#:** `Instant`/`DateTimeOffset` ↔ `DateTime<Utc>`/`DateTime<FixedOffset>`; `LocalDate` ↔ `NaiveDate`; `ZoneId` ↔ `Tz`. `java.util.regex` backtracks; Rust's does not.
- **Python/TypeScript:** no implicit local time: naive and aware values are different types, so "naive vs aware" `TypeError`s become compile errors. `datetime.now()` ↔ `Utc::now()`.
- **C++:** `chrono` is the standard name but the Rust crate is separate; zone support (`chrono-tz`) embeds the tz database in the binary.

## 6. Exercises
Starters compile and fail with `todo!()`; solutions are in [solutions/](./solutions). No test depends on the real clock.

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | Text utilities | Easy | Regex tags, slugs, minijinja text vs HTML escaping | `cargo test -p m13-ex1-text-utils` |
| 2 | Due dates and zones | Medium | Strict date parsing, `LocalResult`, start/end of day across DST | `cargo test -p m13-ex2-due-dates-and-zones` |
| 3 | Scheduler logic | Stretch | Schedules, injected time, skip-missed-runs policy | `cargo test -p m13-ex3-scheduler` |

**Exercise 1: [exercises/ex1-text-utils](./exercises/ex1-text-utils)**
- **Task:** implement `extract_tags`, `slugify`, `render_reminder`, `render_card_html`.
- **Acceptance:** tags are lowercase, unique, ordered, capped (30 chars, 10 tags) and ignore `C#`/`a#b`; slugs collapse non-alphanumerics; plain text is not escaped but HTML is.
- <details><summary>Hint</summary>`Regex::captures_iter` gives each match's group 1; the template *name* decides escaping.</details>

**Exercise 2: [exercises/ex2-due-dates-and-zones](./exercises/ex2-due-dates-and-zones)**
- **Task:** implement `parse_due_date`, `parse_zone`, `days_until`, `is_overdue`, `local_today`, `start_of_day_utc`, `end_of_day_utc`.
- **Acceptance:** padding is enforced (`2030-1-5` is rejected); due today is not overdue; Berlin days are 23/25 hours on DST days; São Paulo's missing midnight and Beirut's repeated 23:59:59 are handled.
- <details><summary>Hint</summary>For a gap, try local times 30 minutes later until `from_local_datetime` is not `None`; for an overlap pick `first` or `second` on purpose.</details>

**Exercise 3: [exercises/ex3-scheduler](./exercises/ex3-scheduler)**
- **Task:** implement `Schedule::{every, next_after}` and `Scheduler::{add, due, next_wakeup}`.
- **Acceptance:** non-positive intervals are rejected; `DailyAt` is strictly after the reference; after a long pause a job runs **once**, then waits for the next slot; ready jobs return in registration order.
- <details><summary>Hint</summary>Reschedule from `now`, not from the old `next_run`: that is what prevents a burst.</details>

## 7. Recap
- Store instants as UTC, dates as `NaiveDate`, zones only at the edges; pass `now` in.
- DST makes local times missing or repeated: handle `LocalResult` explicitly.
- UUID v7 for sortable ids, `rand::rng()` for secrets, seeded RNGs only for tests.
- Compile regexes once; templates auto-escape by template name.
- `interval` fires first immediately; choose a missed-tick policy and cancel via `CancellationToken`.

## 8. Self-check Questions
1. A task is "due 2030-03-31 end of day" for a user in Berlin. How many hours does that day have, and which UTC instant is the deadline?
2. Why is `chrono::Duration` not accepted by `tokio::time::sleep`, and what can go wrong when converting?
3. What would happen to a 10-minute job after a 2-hour freeze with `Burst` vs `Skip`?
4. When would you pick UUID v4 over v7, and why does v7 help a B-tree index?

## 9. Further Reading
- [`chrono`](https://docs.rs/chrono), [`chrono-tz`](https://docs.rs/chrono-tz), [`tokio::time`](https://docs.rs/tokio/latest/tokio/time/index.html)
- RFC 3339 (timestamps), RFC 9562 (UUID v7), the IANA tz database
- [`regex` crate docs](https://docs.rs/regex): syntax and performance guarantees

---
**Next:** [14 Packaging and Running](../14-packaging-and-running/README.md)
