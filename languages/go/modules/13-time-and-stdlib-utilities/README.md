# 13. Time and Standard Library Utilities in Go

> Estimated time: 2h | Difficulty: Intermediate | Prerequisites: 01, 06

## Learning Outcomes
After this module you can:
- [ ] Parse, format and compare dates, durations and time zones without layout or DST surprises
- [ ] Use timers, tickers and `AfterFunc` correctly, and run periodic jobs in-process with clean shutdown
- [ ] Generate UUIDs, reproducible pseudo-random numbers and secure random tokens, and pick the right one
- [ ] Match and extract text with `regexp` and render text with `text/template`
- [ ] Hash, sign and verify data with SHA-256 and HMAC using constant-time comparison

## 1. Concept Overview
Backends are full of small utilities that look trivial and hide sharp edges: a "day" is not always 24 hours, a timer that is never stopped keeps working forever, a random number is not a secret, and comparing two MACs with `==` leaks timing. This module is a tour of the primitives you reuse in every service, with the one rule that matters for each.

Two cross-cutting habits make time-dependent code testable: **inject the clock** (a `Clock` interface or a `func() time.Time`, never `time.Now()` deep inside logic) and **inject the ticker** (so a test fires ticks by hand instead of sleeping). Exercises 1 and 3 practise exactly this.

## 2. How Go Approaches It

| Need | Tool |
|------|------|
| Instants, calendar dates | `time.Time` (value type; has a monotonic reading when it comes from `time.Now()`), `time.Date`, `Parse`, `Format`, `AddDate`, `time.UTC`, `LoadLocation` (import `time/tzdata` to embed the zone database) |
| Elapsed time | `time.Duration` (int64 nanoseconds), `time.Since`, `Truncate`, `Round` |
| Waiting and repeating | `time.NewTimer`, `time.NewTicker`, `time.AfterFunc`, `context.WithTimeout` |
| Unique ids | `github.com/google/uuid` (v4 random, v5 name-based, v7 time-ordered) |
| Random numbers | `math/rand/v2` (fast, seedable, **not** secret) and `crypto/rand` (`Text`, `Read`, `Int`) |
| Text matching | `regexp` (RE2: guaranteed linear time, no lookaround/backreferences) |
| Text templating | `text/template` (no escaping; `html/template` for HTML) |
| Hashing / signing | `crypto/sha256`, `crypto/hmac`, `crypto/subtle`, `encoding/hex`, `encoding/base64` |

Idioms: layouts are written with the **reference time** `Mon Jan 2 15:04:05 MST 2006` (use the constants `time.RFC3339`, `time.DateOnly`); store and transmit UTC, convert at the edges; compare instants with `Equal`/`Before`/`After`; since Go 1.23 timers and tickers are garbage-collected when unreferenced and `Stop`/`Reset` leave no stale value in the channel, but explicit `Stop` is still the rule. The only third-party package is `uuid` (the stdlib has no UUID type).

## 3. Guided Examples

### 3.1 Time basics
- **Goal:** layouts, durations, calendar arithmetic, zones, the monotonic clock.
- **Run:** `go run ./modules/13-time-and-stdlib-utilities/examples/01_time_basics`
- **Code:** [examples/01_time_basics/main.go](examples/01_time_basics/main.go)
  ```go
  t := time.Date(2030, time.January, 31, 9, 30, 0, 0, time.UTC)
  t.Format("2006-01-02")        // 2030-01-31
  t.AddDate(0, 1, 0)            // Jan 31 + 1 month normalises to 2030-03-03
  ```
- **Walkthrough:** `Parse` rejects `2030-02-30`. `Duration` arithmetic is exact nanoseconds while `AddDate` follows the calendar, which is why across the US spring-forward day "tomorrow" is `23h0m0s` away but `Add(24h)` lands an hour late. `winter == winter.UTC()` is `false` (different `Location` pointers) although `Equal` is `true`. A value from `time.Now()` differs from its `Round(0)` copy because the monotonic reading is stripped.
- **Expected output:**
  ```
  2030-01-31T09:30:00Z
  2030-01-31
  Thu 31 Jan 2030, 9:30AM
  parse invalid date: true parsing time "2030-02-30": day out of range
  1h30m30s 1.5083333333333333 2h0m0s 1h0m0s
  11:00:30 true
  2030-03-03
  2030-01-31T00:00:00Z
  2030-01-15T12:00:00-05:00 2030-07-15T12:00:00-04:00
  2030-01-15T17:00:00Z true false
  23h0m0s 2030-03-10T13:00:00-04:00
  has monotonic reading: true
  elapsed is non-negative: true
  true false true
  2030-01-01T09:00:00Z
  ```

### 3.2 Timers, tickers, AfterFunc
- **Goal:** wait, repeat and time out without leaks.
- **Run:** `go run ./modules/13-time-and-stdlib-utilities/examples/02_timers_tickers`
- **Code:** [examples/02_timers_tickers/main.go](examples/02_timers_tickers/main.go)
  ```go
  timer := time.NewTimer(0)
  defer timer.Stop()
  for {
      select {
      case <-ctx.Done():  return ctx.Err()
      case <-timer.C:     if ready() { return nil }; timer.Reset(interval)
      }
  }
  ```
- **Walkthrough:** `Ticker.Stop` ends the repetition; `Timer.Stop` reports whether it beat the firing; `AfterFunc` runs a callback in its own goroutine. The polling loop reuses **one** timer instead of calling `time.After` per iteration, and `ctx` provides the overall deadline. The idle-timeout loop resets the timer on every event.
- **Expected output:**
  ```
  ticks: 3
  stopped before firing: true
  timer fired after Reset
  cancelled AfterFunc: true
  AfterFunc calls: 1
  poll succeeded after 3 attempts, err: <nil>
  poll never ready: context deadline exceeded
  event a
  event b
  event c
  all events seen before the idle timeout
  ```

### 3.3 UUIDs, random numbers, regular expressions
- **Goal:** choose the right id/random source; compile regexps once.
- **Run:** `go run ./modules/13-time-and-stdlib-utilities/examples/03_ids_random_regex`
- **Code:** [examples/03_ids_random_regex/main.go](examples/03_ids_random_regex/main.go)
  ```go
  r := mrand.New(mrand.NewPCG(1, 2))   // reproducible: same seed, same numbers
  token := rand.Text()                 // crypto/rand: 26 base32 chars, 128 bits
  var tagRe = regexp.MustCompile(`#([a-z][a-z0-9-]{0,29})`)
  ```
- **Walkthrough:** v5 UUIDs are deterministic (same namespace and name), v4/v7 are random. The seeded PCG makes tests reproducible; the package-level `mrand.IntN` is randomly seeded and fine for jitter, never for secrets. `SubexpNames` lets named groups be read by name; `$name` works in `ReplaceAllString`. `#Release-1` is not matched because tags must start with a lowercase letter. An unanchored pattern matches anywhere (`\d+` matches `12a`).
- **Expected output:**
  ```
  v4 version: 4 len: 36
  round trip: true <nil>
  invalid: invalid UUID length: 10
  v5: b0bd5583-9c84-54d6-b511-991c6bc149cb true
  v7 version: 7 <nil>
  seeded: 76 61 78
  shuffled: [in_progress todo done]
  global in range: true
  token length: 26
  unbiased int in range: true <nil>
  [#docs #p1]
  p1
  year=2030 month=01 day=31 
  Write docs #docs #p1 due 2030-01-31 and ship #Release-1
  Write docs #DOCS #P1 due 2030-01-31 and   ship #Release-1
  Write docs #docs #p1 due 31/01/2030 and   ship #Release-1
  compile error: error parsing regexp: missing closing ): `(unclosed`
  anchored: false true
  ```

### 3.4 Templates and hashing
- **Goal:** render a notification with `text/template`; hash and sign with SHA-256 and HMAC.
- **Run:** `go run ./modules/13-time-and-stdlib-utilities/examples/04_templates_hashing`
- **Code:** [examples/04_templates_hashing/main.go](examples/04_templates_hashing/main.go)
  ```go
  mac := hmac.New(sha256.New, key)
  mac.Write([]byte("task:42"))
  ok := hmac.Equal(tag, expected) // constant time, never bytes.Equal / ==
  ```
- **Walkthrough:** `{{with}}...{{else}}` handles the empty list, `{{range $i, $t := .}}` iterates, pipelines (`| printf`) and a `FuncMap` extend the language. `Option("missingkey=error")` turns a typo into an error. `Execute` streams, so a failure mid-way has already written `"Hi "`: render to a buffer if partial output is unacceptable. SHA-256 is for integrity; HMAC adds a secret; passwords need bcrypt (module 11).
- **Expected output:**
  ```
  Hi Ada,
  you have 2 tasks:
  1. Write docs   due 2030-01-31 [docs, p1]
  2. Ship it      due 2030-02-01
  Hi Bob,
  nothing is due.
  missing key: template: digest:1:5: executing "digest" at <.Name>: map has no entry for key "Name" (partial output "Hi ")
  parse error: template: bad:1: unclosed action
  sha256: 2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824
  streamed equal: true
  hmac: vQHUGw86U51MPrvDejHX2CGAgGIqoJ9uoZEL_3R-CaI
  hmac.Equal: true
  same tag: true | truncated tag: false
  forged accepted: false
  ```

### 3.5 Task Tracker: reminders from an injected clock
- **Goal:** calendar-date urgency and a periodic in-process job, both deterministic.
- **Run:** `go run ./modules/13-time-and-stdlib-utilities/examples/05_task_reminders`
- **Code:** [examples/05_task_reminders/main.go](examples/05_task_reminders/main.go)
- **Walkthrough:** `Urgency` compares the UTC **date** of "now" with the parsed `due_date`, so a task due `2030-01-31` is "due today" at 23:00 and "overdue" a minute later, whatever the hour. `Clock` is an interface (`ClockFunc(time.Now)` in production); the demo uses a clock that steps one day per read. `Every` is a ticker loop that stops on `ctx.Done()` and cannot overlap itself because the callback runs inside the loop. Invalid dates are reported, not fatal. Exercise 3 turns this into a real scheduler.
- **Expected output:**
  ```
  run 1 2030-01-31T23:00:00Z
    #1 Write docs: overdue by 1 day(s)
    #2 Ship it: due in 1 day(s)
    #3 Plan: due today
    skip: task 6: parsing time "31/01/2030" as "2006-01-02": cannot parse "31/01/2030" as "2006"
  run 2 2030-02-01T23:00:00Z
    #1 Write docs: overdue by 2 day(s)
    #2 Ship it: due today
    #3 Plan: overdue by 1 day(s)
    skip: task 6: parsing time "31/01/2030" as "2006-01-02": cannot parse "31/01/2030" as "2006"
  scheduler stopped: context canceled
  ```

## 4. Common Pitfalls

| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| `Format("YYYY-MM-DD")` or `"2006-02-01"` | Layouts use the reference date `Mon Jan 2 15:04:05 MST 2006` (month 01, day 02), not `Y/M/D` letters | Use `time.DateOnly`, `time.RFC3339` or write the reference digits exactly |
| `a == b` on `time.Time` | The struct also holds the monotonic reading and a `*Location` | `a.Equal(b)`; `IsZero()` for the zero value |
| `now.Add(24 * time.Hour)` for "tomorrow" | `Duration` is absolute time; DST days are 23 or 25 hours | `AddDate(0, 0, 1)`, or compare calendar dates |
| `Jan 31 + 1 month = Mar 3` | `AddDate` normalises overflow instead of clamping | Clamp explicitly when "end of month" is the intent |
| `time.Parse` without a zone silently gives UTC | The layout has no offset, so Go assumes UTC | `ParseInLocation`, or require RFC 3339 with an offset |
| `time.After` in a `for`/`select` loop | Every call allocates a timer you cannot stop (leaks on older Go, wasteful on new) | One `time.NewTimer` + `Reset`, or `context.WithTimeout` |
| Ticker never stopped / job in a ticker loop that outlives its owner | A ticker is a runtime resource and a goroutine needs a `ctx` | `defer ticker.Stop()`; exit on `ctx.Done()` and wait for the goroutine |
| Slow consumer of `ticker.C` | The channel has capacity 1: missed ticks are dropped, not queued | Do slow work off the loop or accept skipping; never assume "exactly N runs" |
| `time.Sleep` in tests / `time.Now()` inside logic | Slow, flaky, untestable | Inject a `Clock`/ticker factory and fire it by hand |
| `math/rand` for tokens or ids | It is predictable by design | `crypto/rand.Text`/`Read`; `math/rand/v2` only for jitter, shuffles, tests |
| `regexp.MustCompile` inside a handler | Compiling is expensive and repeated per request | Package-level `var re = regexp.MustCompile(...)`; remember RE2 has no lookahead |
| `bytes.Equal(mac1, mac2)` or `==` on secrets | Early exit leaks timing | `hmac.Equal` / `subtle.ConstantTimeCompare` |
| `text/template` for HTML | It does no escaping, so user input becomes XSS | `html/template` |
| `time.Duration(n) * time.Second` written as `n * time.Second` | Type mismatch (`int` vs `Duration`) | Convert once: `time.Duration(n) * time.Second` |

## 5. Coming From Other Languages
- **Java:** `time.Time` is closest to `Instant` + `ZoneId` in one immutable value; `Duration` is a plain nanosecond count, not `Period`. There is no `LocalDate` type: the idiom is midnight UTC, formatted with `time.DateOnly`.
- **C#:** `time.Time` replaces `DateTimeOffset`; "Kind" confusion does not exist because the `Location` is always explicit. `System.Threading.Timer` maps to `time.AfterFunc`; `PeriodicTimer` maps to `time.Ticker`.
- **Python/JS:** no naive datetimes (a `Time` always has a location) and no `strftime` codes: the layout is an example of the output. `regexp` rejects lookbehind and backreferences that PCRE/JS allow.
- **Rust:** `chrono`/`time` crates cover the same ground; here they are all in the standard library. `rand` crate defaults map to `math/rand/v2` (seedable) vs `crypto/rand` (OS secure).

## 6. Exercises
Starter code and tests are in [exercises/](exercises/); reference solutions in [_solutions/](_solutions/).

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | Due dates | Easy | strict `Parse`, UTC calendar days, DST-safe arithmetic, RFC 3339 | `go test ./modules/13-time-and-stdlib-utilities/exercises/ex1_due_dates/` |
| 2 | Signed links | Medium | HMAC-SHA256, constant-time compare, expiry with injected `now` | `go test ./modules/13-time-and-stdlib-utilities/exercises/ex2_signed_links/` |
| 3 | Scheduler | Stretch | tickers, per-job goroutines, panic recovery, clean shutdown with a manual ticker | `go test -race ./modules/13-time-and-stdlib-utilities/exercises/ex3_scheduler/` |

### Exercise 1: Due dates
- **Task:** implement `Parse` (strict `YYYY-MM-DD`, midnight UTC), `DaysUntil` (calendar days between the UTC date of `now` and the due date), `IsOverdue` and `FormatTimestamp` (RFC 3339 UTC).
- **Acceptance:** `TestParse`, `TestParseRejects`, `TestDaysUntil`, `TestDaysUntilLeapYear`, `TestDaysUntilUsesTheUTCDateOfNow`, `TestDaysUntilAcrossDST`, `TestIsOverdue`, `TestFormatTimestamp`.
- <details><summary>Hint</summary>Build today's midnight with <code>time.Date(y, m, d, 0, 0, 0, 0, time.UTC)</code> from <code>now.UTC()</code>, then divide the difference by 24 hours; <code>time.Parse(time.DateOnly, s)</code> already rejects most bad input.</details>

### Exercise 2: Signed links
- **Task:** `Signer.Sign` builds `<taskID>.<expiresUnix>.<base64url HMAC>`; `Verify` checks structure, signature (constant time), then expiry, in that order.
- **Acceptance:** `TestNewRejectsEmptyKey`, `TestRoundTrip`, `TestTokenFormatIsPinned`, `TestSubSecondPartIgnored`, `TestExpiry`, `TestTamperingIsDetected`, `TestForgedExpiredTokenReportsBadSignature`, `TestMalformed`.
- <details><summary>Hint</summary>Compute the MAC over the first two parts as text, decode the third part with <code>base64.RawURLEncoding</code> and compare the raw bytes with <code>hmac.Equal</code>; expired means <code>!now.Before(expiry)</code>.</details>

### Exercise 3: Scheduler
- **Task:** `Scheduler.Run` runs each job on its own ticker and goroutine, never overlapping with itself, reports errors and recovered panics to `OnError`, validates jobs up front, and on cancellation stops every ticker and waits for running jobs.
- **Acceptance:** `TestRunsJobOnEveryTick`, `TestEveryJobUsesItsOwnInterval`, `TestSlowJobDoesNotBlockOthers`, `TestJobNeverOverlapsItself`, `TestErrorsAreReportedAndJobKeepsRunning`, `TestPanicIsRecovered`, `TestCancelWaitsForRunningJobAndStopsTickers`, `TestInvalidJobsStartNothing`, `TestZeroValueUsesRealTickers`, `TestNoJobsWaitsForCancellation`.
- <details><summary>Hint</summary>Validate in a first loop, then <code>wg.Go</code> one loop per job: <code>select</code> on <code>ctx.Done()</code> and the ticks channel; call the job inside a function with <code>defer recover()</code>; <code>&lt;-ctx.Done()</code> then <code>wg.Wait()</code> at the end.</details>

## 7. Recap
- Store and exchange UTC (RFC 3339, `YYYY-MM-DD`); compare with `Equal`; use `AddDate` and dates for calendar logic, `Duration` for elapsed time.
- Every timer, ticker and goroutine has an owner who stops it; reuse one `Timer` in loops.
- Inject the clock and the ticker so time-based code is tested without sleeping.
- `math/rand/v2` is for simulation and jitter, `crypto/rand` for anything secret; UUID v4 for random ids, v7 when sort order matters.
- When to reach for what: SHA-256 for integrity, HMAC for "made by us", bcrypt for passwords, `hmac.Equal` for every comparison of secrets.

## 8. Self-check Questions
1. Why does `winter == winter.UTC()` print `false` while `winter.Equal(winter.UTC())` is `true`?
2. A task is due `2030-03-11`, and the server runs in `America/New_York`. Why is "hours until due / 24" the wrong way to count days, and what do you compute instead?
3. What does a `time.Ticker` do when the receiver is slower than the interval, and how does that affect a job that takes longer than its period?
4. You must create a password-reset token. Which package generates it, and which comparison verifies it?
5. Why does exercise 3 take a `NewTicker` function instead of an interval, and what would the tests look like without it?

## 9. Further Reading
- [Package time](https://pkg.go.dev/time) (layouts, `Time` comparison notes) and [Go 1.23 timer changes](https://go.dev/wiki/Go123Timer)
- [math/rand/v2](https://pkg.go.dev/math/rand/v2), [crypto/rand](https://pkg.go.dev/crypto/rand), [regexp syntax](https://pkg.go.dev/regexp/syntax)
- [text/template](https://pkg.go.dev/text/template) and [google/uuid](https://pkg.go.dev/github.com/google/uuid)

---
**Next:** [14 Packaging and Running](../14-packaging-and-running/README.md)
