# 13. Time and Standard-Library Utilities in TypeScript

> Estimated time: 1.5-2h | Difficulty: Intermediate | Prerequisites: [01 Language fundamentals](../01-language-fundamentals/README.md), [06 Concurrency](../06-concurrency/README.md)

## Learning Outcomes

After this module you can:

- [ ] Represent instants, calendar dates, zoned date-times and durations with `Temporal`, and convert legacy `Date` values at the edges
- [ ] Do time-zone and daylight-saving-safe calendar math, and format times for humans (`Intl`) and machines (RFC 3339)
- [ ] Use timers (`node:timers/promises`, `AbortSignal`, `unref()`) and write an in-process scheduler that does not drift or overlap
- [ ] Generate UUIDs and random numbers securely, compute hashes/HMACs, and use regular expressions with named groups and tagged templates
- [ ] Make time-dependent code deterministic by injecting a clock

## 1. Concept Overview

Time bugs come from mixing up four different ideas: an **instant** (one point on the UTC timeline), a **calendar date** (`2030-01-31`, no time, no zone), a **wall-clock time in a zone** (needs a tz database to become an instant) and a **duration**. Store and transmit instants (RFC 3339, UTC) and plain dates (`due_date`); convert to a zone only to display or to do calendar arithmetic ("tomorrow at 09:00 Berlin time"), because a day is not always 24 hours.

The rest of the standard library covers small jobs every service needs: timers and periodic background jobs (that must stop cleanly on shutdown), identifiers and randomness (cryptographically secure unless you only need a dice roll), hashing (ETags, signatures) and text handling (regex, templates).

Code that reads the clock directly is hard to test. Pass a clock (`{ now(): Temporal.Instant }`) into the code that needs it.

## 2. How TypeScript Approaches It

| Need          | Use                                                                 | Notes                                                                        |
| ------------- | ------------------------------------------------------------------- | ---------------------------------------------------------------------------- |
| Point in time | `Temporal.Instant`                                                  | `Temporal.Now.instant()`, `.toString()` gives RFC 3339 `...Z`                |
| Calendar date | `Temporal.PlainDate`                                                | the type of `due_date`; `{ overflow: 'reject' }` to refuse `2030-02-30`      |
| Zoned time    | `Temporal.ZonedDateTime`                                            | DST-aware `add`, `disambiguation` option for gaps/overlaps                   |
| Span          | `Temporal.Duration`                                                 | `.total('minutes')`, `.round({ largestUnit })`                               |
| Legacy API    | `Date`, `Intl.DateTimeFormat`                                       | still needed for `Intl` formatting and libraries; `date.toTemporalInstant()` |
| Timers        | `node:timers/promises` (`setTimeout`, `setInterval` async iterator) | cancel with `AbortSignal`; `timer.unref()` for background timers             |
| IDs / random  | `crypto.randomUUID()`, `crypto.randomInt()`, `crypto.randomBytes()` | `Math.random()` is never for secrets                                         |
| Hashing       | `crypto.createHash`, `createHmac`                                   | passwords need scrypt (module 11), not a plain hash                          |
| Text          | `RegExp` named groups, `matchAll`, tagged template literals         |                                                                              |

`Temporal` is built into Node 26 (no package), and TypeScript 7 ships its types, so it is used directly as a global. On Node 24 you would need a polyfill, so run this module on Node 26 (`engines` allows 24, which has no `Temporal`). No third-party libraries are used (no moment, dayjs, date-fns, uuid, nanoid).

## 3. Guided Examples

### 3.1 Dates, old and new

- **Goal:** see the `Date` traps (0-based months, silent overflow, UTC-vs-local parsing) and the `Temporal` equivalents.
- **Run:** `node modules/13-time-and-stdlib-utilities/examples/01_dates_old_and_new.ts`
- **Code:** [01_dates_old_and_new.ts](examples/01_dates_old_and_new.ts)
- **Walkthrough:** the script pins `process.env.TZ` so the output is identical on every machine. `new Date('2030-01-01')` is midnight UTC, `new Date('2030-01-01T00:00')` is midnight _local_. `PlainDate.add({ months: 1 })` clamps to Feb 28, `overflow: 'reject'` throws `RangeError`.

  ```ts
  const jan31 = Temporal.PlainDate.from('2030-01-31');
  jan31.add({ months: 1 }).toString(); // 2030-02-28
  ```
- **Expected output:**

  ```
  Date month for January: 0
  Temporal month for January: 1
  Date Jan 31 + 1 month: 3-3
  Temporal Jan 31 + 1 month: 2030-02-28
  overflow: reject -> RangeError
  Date("2030-01-01"):       2030-01-01T00:00:00.000Z
  Date("2030-01-01T00:00"): 2030-01-01T05:00:00.000Z
  Instant: 2030-01-01T09:00:00Z 1893488400000
  PlainDate: 2030-01-31 4 31
  Due + 14 days: 2030-02-14
  Days between: 30
  Compare: 0 true
  Total hours: 90.5
  Balanced: P3DT18H30M
  Invalid Date: Invalid Date
  Temporal rejects 2030-02-30: true
  Date -> Instant: 1970-01-01T00:00:00Z
  Instant -> Date: 2030-01-01T09:00:00.000Z
  ```

### 3.2 Time zones, DST and `Intl`

- **Goal:** do calendar math in a zone and format for humans.
- **Run:** `node modules/13-time-and-stdlib-utilities/examples/02_time_zones_and_intl.ts`
- **Code:** [02_time_zones_and_intl.ts](examples/02_time_zones_and_intl.ts)
- **Walkthrough:** Berlin skips 02:00-03:00 on 2030-03-31, so that day has 23 hours: `add({ days: 1 })` keeps the wall clock (`00:00`), `add({ hours: 24 })` does not (`01:00`). A local time inside the gap is moved forward (`compatible`), `reject` throws; a time in the autumn overlap exists twice (`earlier`/`later`). Always pass `locale` and `timeZone` to `Intl.DateTimeFormat`, never rely on the server defaults.
- **Expected output:**

  ```
  UTC               2030-03-30T22:30:00+00:00[UTC]
  Europe/Berlin     2030-03-30T23:30:00+01:00[Europe/Berlin]
  Asia/Tokyo        2030-03-31T07:30:00+09:00[Asia/Tokyo]
  America/New_York  2030-03-30T18:30:00-04:00[America/New_York]
  +1 day : 2030-04-01T00:00:00+02:00[Europe/Berlin]
  +24 h  : 2030-04-01T01:00:00+02:00[Europe/Berlin]
  hours elapsed between the two midnights: 23
  hoursInDay on 2030-03-31: 23
  gap, compatible : 2030-03-31T03:30:00+02:00[Europe/Berlin]
  gap, earlier    : 2030-03-31T01:30:00+01:00[Europe/Berlin]
  gap, reject     : RangeError
  overlap, earlier: 2030-10-27T02:30:00+02:00[Europe/Berlin]
  overlap, later  : 2030-10-27T02:30:00+01:00[Europe/Berlin]
  Saturday, March 30, 2030 at 6:30 PM
  Samstag, 30. März 2030 um 23:30
  Sunday, 31 March 2030 at 07:30
  Mar 31, 2030, 7:30 AM
  {"created_at":"2030-03-30T22:30:00Z"}
  offset input normalised to UTC: 2030-03-30T22:30:00Z
  ```

### 3.3 Timers and intervals

- **Goal:** sleep, cancel and repeat without leaking timers.
- **Run:** `node modules/13-time-and-stdlib-utilities/examples/03_timers_and_intervals.ts`
- **Code:** [03_timers_and_intervals.ts](examples/03_timers_and_intervals.ts)
- **Walkthrough:** `await setTimeout(ms)` from `node:timers/promises` replaces callback timers; an `AbortSignal` cancels it (the promise rejects with `AbortError`). `for await (const _ of setInterval(ms))` is an interval you `break` out of. Plain `setInterval` starts a new run even when the previous one is still busy, a loop with `await` does not. `unref()` lets the process exit while the timer is pending.

  ```ts
  const housekeeping = setInterval(() => sweep(), 60_000);
  housekeeping.unref();
  ```
- **Expected output:**

  ```
  woke up - waited at least 15 ms: true
  cancelled sleep -> AbortError
  interval ticks: 3
  setInterval overlaps: true
  sequential loop overlaps: false
  exiting without waiting for the unref()ed timer
  ```

### 3.4 IDs, randomness, hashing, regex and templates

- **Goal:** the everyday `node:crypto` helpers and regex features.
- **Run:** `node modules/13-time-and-stdlib-utilities/examples/04_ids_hashing_regex.ts`
- **Code:** [04_ids_hashing_regex.ts](examples/04_ids_hashing_regex.ts)
- **Walkthrough:** random values change per run, so the script prints only their shape. `randomInt(1, 7)` has an exclusive upper bound. Named groups give `match.groups.year`; a `/g` regex stored in a variable keeps `lastIndex` between `test()` calls. A tagged template (the `html` tag function) escapes every interpolated value.
- **Expected output:**

  ```
  uuid shape ok: true
  dice roll in 1..6: true
  token length: 22
  sha256: 26c516c90fe44af8161fb52d8492716b196823d9243814982cd1e5d486369ade
  sha1 : a9993e364706816aba3e25717850c26c9cd0d89d
  hmac : 60816bea639d1ba7
  groups: { year: '2030', month: '01', day: '31' }
  no match: null
  issue refs: [ 12, 7, 101 ]
  replace: Fix issue-12 and issue-7, then close issue-101
  test twice with /g: true false
  <li>&#60;script&#62;alert(&#34;x&#34;)&#60;/script&#62;</li>
  Task 1: WRITE DOCS..|
  ```

### 3.5 Task Tracker deadlines with an injected clock

- **Goal:** time-dependent domain logic that is testable, plus a background sweep.
- **Run:** `node modules/13-time-and-stdlib-utilities/examples/05_tracker_deadlines.ts`
- **Code:** [05_tracker_deadlines.ts](examples/05_tracker_deadlines.ts)
- **Walkthrough:** `createTask` receives a `Clock`, so `created_at` is a fixed RFC 3339 string in the demo. `due_date` is validated with `overflow: 'reject'`. `dueState` compares the plain due date with "today in Europe/Berlin", not with UTC. The sweep reads a stepping clock that advances one day per tick, so a real 5 ms interval produces a reproducible "days" story.
- **Expected output:**

  ```
  {"id":1,"title":"Write docs","status":"todo","due_date":"2030-01-31","tags":[],"owner_id":null,"created_at":"2030-01-01T09:00:00Z"}
  sorted: 4:2030-01-30 1:2030-01-31 2:2030-02-02 3:null
  rejected due date: true
  tick 0 (2030-01-30): newly overdue [] due today [4]
  tick 1 (2030-01-31): newly overdue [4] due today [1]
  tick 2 (2030-02-01): newly overdue [1] due today []
  tick 3 (2030-02-02): newly overdue [] due today [2]
  ```

## 4. Common Pitfalls

| Mistake                                                                              | Why it happens                                                     | Fix                                                                             |
| ------------------------------------------------------------------------------------ | ------------------------------------------------------------------ | ------------------------------------------------------------------------------- |
| `new Date(2030, 1, 1)` is February, not January                                      | `Date` months are 0-based, days are 1-based                        | use `Temporal.PlainDate.from({ year, month, day })` (1-based)                   |
| `new Date('2030-01-01')` and `new Date('2030-01-01T00:00')` differ by the UTC offset | date-only ISO strings are UTC, date-times without offset are local | parse with `Temporal.Instant.from` (requires an offset) or `PlainDate.from`     |
| `date.setMonth(date.getMonth() + 1)` on Jan 31 lands in March                        | `Date` overflows silently                                          | `PlainDate.add({ months: 1 })` clamps, or pass `overflow: 'reject'`             |
| `add({ hours: 24 })` is not "tomorrow"                                               | days are 23 or 25 hours on DST changes                             | do calendar math with `{ days: 1 }` on a `ZonedDateTime`                        |
| `JSON.stringify(new Date())` or `toString()` in an API                               | `Date#toString` is locale/zone dependent                           | send `instant.toString()` (RFC 3339 UTC) and keep `due_date` as `YYYY-MM-DD`    |
| `Math.random()` for tokens or ids                                                    | it is a fast, predictable PRNG                                     | `randomUUID()`, `randomBytes()`, `randomInt()` from `node:crypto`               |
| `setInterval(async () => ...)` piles up runs and swallows errors                     | the interval does not await the callback, a rejection is unhandled | a loop with `await` + `timers/promises`, try/catch around the task (exercise 3) |
| Process never exits / tests hang                                                     | a pending `setInterval` keeps the event loop alive                 | `abort()` an `AbortSignal` on shutdown and `unref()` housekeeping timers        |
| `regex.test()` alternates `true`/`false`                                             | a `/g` or `/y` regex keeps `lastIndex`                             | drop the flag for `test`, or create the regex where it is used                  |
| `new Intl.DateTimeFormat().format(d)` differs per machine                            | defaults come from the server's locale and zone                    | always pass `locale` and `timeZone`                                             |

## 5. Coming From Other Languages

- **JavaScript:** `Temporal` replaces `Date`, moment and dayjs for new code; `Date` stays only at API edges.
- **Java/C#:** `Instant`, `LocalDate`, `ZonedDateTime`, `Duration` map almost one-to-one to `java.time` and `NodaTime` (C# `DateOnly` is `PlainDate`).
- **Python:** `PlainDate` ~ `datetime.date`, `ZonedDateTime` ~ an aware `datetime` with `zoneinfo`, `PlainDateTime` ~ a naive `datetime`; an `Instant` is always UTC.
- **Go:** `time.Time` mixes instant and zone, `Temporal` separates them; measure elapsed time with `performance.now()` (monotonic). A `time.Ticker` corresponds to `for await (... of setInterval())`.

## 6. Exercises

Starter code is in [exercises/](exercises/), reference solutions in [solutions/](solutions/). Run every test command from `languages/typescript/`.

| #   | Title                                                          | Difficulty | Skill practiced                               | Run tests                                                                                  |
| --- | -------------------------------------------------------------- | ---------- | --------------------------------------------- | ------------------------------------------------------------------------------------------ |
| 1   | [Due dates](exercises/ex1_due_dates/due-dates.ts)              | Easy       | strict `PlainDate` parsing, "today" in a zone | `node --test "modules/13-time-and-stdlib-utilities/exercises/ex1_due_dates/**/*.test.ts"`  |
| 2   | [Next daily run](exercises/ex2_next_run/next-run.ts)           | Medium     | `ZonedDateTime`, DST gaps and overlaps        | `node --test "modules/13-time-and-stdlib-utilities/exercises/ex2_next_run/**/*.test.ts"`   |
| 3   | [Fixed-rate scheduler](exercises/ex3_fixed_rate/fixed-rate.ts) | Stretch    | injected clock, no drift, `AbortSignal`       | `node --test "modules/13-time-and-stdlib-utilities/exercises/ex3_fixed_rate/**/*.test.ts"` |

**Exercise 1 - Due dates.** Task: implement `parseDueDate` (only `YYYY-MM-DD`, real days), `daysUntilDue` and `isOverdue`. Acceptance: all tests in `due-dates.test.ts`, including the Tokyo/New York cases where "today" differs. Hint: `Temporal.PlainDate.from` accepts more formats than the API allows; check the shape with a regex first.

**Exercise 2 - Next daily run.** Task: `nextDailyRun(after, zone, { hour, minute })` returns the first instant strictly after `after`; plus `upcomingRuns`. Acceptance: tests for plain days, other zones, the spring-forward gap and the fall-back overlap (runs once). Hint: build candidates from a `PlainDate` with `toZonedDateTime({ timeZone, plainTime })` and compare instants.

**Exercise 3 - Fixed-rate scheduler.** Task: `runFixedRate` runs a task at `start + n * interval` without drift or overlap, skips missed ticks after an overrun, reports errors to `onError` and resolves when the signal aborts (also while sleeping). Acceptance: tests with a fake clock (virtual time) and one with the real clock. Hint: derive the next tick from the _start_ time and the tick counter, never from "now + interval".

## 7. Recap

- Store instants as RFC 3339 UTC and dates as `YYYY-MM-DD`; convert to zones only for display and calendar math.
- `Temporal` makes the four time concepts distinct types; `Date` remains only for interop.
- Background loops need cancellation (`AbortSignal`), `unref()`, no overlap and error handling.
- Use `node:crypto` for ids, randomness and hashes; inject clocks to test time.
- When to reach for what: `Instant` for "when it happened", `PlainDate` for "which day", `ZonedDateTime` for "at 09:00 in their city".

## 8. Self-check Questions

1. Why is `due_date` a `PlainDate` and `created_at` an `Instant`? What breaks if you swap them?
2. A job must run daily at 02:30 Europe/Berlin. What happens on the two DST change days, and how does your code decide?
3. Why does `setInterval(async () => { await work(); }, 1000)` misbehave when `work()` sometimes takes 3 seconds?
4. When is it acceptable to hash with SHA-256 and when must you use scrypt?

## 9. Further Reading

- [MDN: Temporal](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Temporal) and the [Temporal cookbook](https://tc39.es/proposal-temporal/docs/cookbook.html)
- [Node.js: timers promises API](https://nodejs.org/api/timers.html#timers-promises-api) and [crypto](https://nodejs.org/api/crypto.html)
- [MDN: Intl.DateTimeFormat](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Intl/DateTimeFormat)

---

**Next:** [14 Packaging and Running](../14-packaging-and-running/README.md)
