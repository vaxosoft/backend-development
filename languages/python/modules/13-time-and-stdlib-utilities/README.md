# 13. Time, Utilities and the Standard Library in Python

> Estimated time: 1.5-2 h | Difficulty: Intermediate | Prerequisites: 01, 06 (asyncio basics), 04 (wire format)

## Learning Outcomes
After this module you can:
- [ ] Tell aware from naive `datetime` values, store UTC, and convert to a user's zone only at the edges
- [ ] Use `zoneinfo` correctly around daylight saving time (wall-clock vs absolute arithmetic, `fold`)
- [ ] Choose between `uuid4`/`uuid7`, `random` and `secrets`, and between `hashlib` and `hmac`
- [ ] Write safe, compiled regular expressions and render text with `string.Template`
- [ ] Run a periodic job inside the service that is testable without real time

## 1. Concept Overview
Time bugs come from treating a *wall-clock reading* ("09:00") as if it were an *instant* ("09:00 in Berlin on 2030-01-01"). Backends store and compare instants (UTC), accept and emit them in an unambiguous format (RFC 3339: `2030-01-01T09:00:00Z`), and apply a time zone only when a human reads a date or when a date-only value such as `due_date` has to become a deadline.

The same "pick the right primitive" thinking applies to the small utilities every service needs: unique ids (random or time-ordered), randomness for simulations versus secrets, digests for identifying content versus keyed signatures, regexes for validating or extracting text, and in-process timers for background work.

```text
client (any zone) --RFC 3339--> parse, require offset --> UTC instant --> store / compare
                                                              |
due_date (date only) + owner's zone --> deadline instant -----+--> format as UTC "Z" on the way out
```

## 2. How Python Approaches It

| Need | Tool | Note |
|------|------|------|
| Instants, durations | `datetime`, `timedelta`, `UTC` | `datetime.now(UTC)`; `utcnow()` is deprecated and returns a naive value |
| Time zones | `zoneinfo.ZoneInfo` (+ `tzdata` wheel) | IANA names; never hand-roll offsets for named zones |
| Ids | `uuid.uuid4()`, `uuid.uuid7()` (3.14), `uuid5` | `uuid7` sorts by creation time, which is friendlier to database indexes |
| Randomness | `random.Random(seed)` vs `secrets` | `random` is predictable; tokens must come from `secrets` |
| Digests | `hashlib`, `hmac` | `hmac.compare_digest` for comparing secrets; password hashing is module 11 |
| Text | `re` (compile once), `string.Template` | `fullmatch` to validate; `Template` is safe for user-edited text |
| Timers | `asyncio.sleep`, `loop.time()`, tasks | one coroutine looping with `await asyncio.sleep(...)`; cancel it on shutdown |

Idiomatic: aware datetimes everywhere, `datetime.fromisoformat` for parsing (it accepts `Z` since 3.11), injecting a clock (a callable or small object) instead of calling `datetime.now()` deep inside logic. Non-idiomatic: `datetime.utcnow()`, naive datetimes, `time.sleep()` in a coroutine, `pytz`-style `localize()` code, patching the `datetime` class in tests.

Third-party: only `tzdata`, which supplies the IANA database on systems without one (slim containers, Windows).

## 3. Guided Examples

### 3.1 Aware vs naive datetimes
- **Goal:** know what an aware value guarantees and how it travels as RFC 3339.
- **Run:** `python modules/13-time-and-stdlib-utilities/examples/01_aware_datetimes.py`
- **Code:** [examples/01_aware_datetimes.py](./examples/01_aware_datetimes.py)
- **Walkthrough:** comparing an aware and a naive value raises `TypeError` instead of guessing. The same instant in `+02:00` compares equal to its UTC form. `timedelta` handles arithmetic; `isoformat()` plus `Z` is the wire format.
- **Expected output:**

  ```text
  aware: 2030-01-01T09:00:00+00:00
  naive: 2030-01-01T09:00:00
  naive.utcoffset(): None
  compare aware/naive -> can't compare offset-naive and offset-aware datetimes
  same instant: 2030-01-01T11:00:00+02:00 True
  due: 2030-01-31T12:00:00+00:00
  remaining: 30 days, 3:00:00 = 2602800.0 seconds
  wire: 2030-01-01T09:00:00Z
  parsed: 2030-01-01T09:00:00+00:00 True
  fromisoformat accepts Z: True
  now is aware: True
  ```

### 3.2 Time zones and daylight saving time
- **Goal:** convert at the edges and understand what DST does to arithmetic.
- **Run:** `python modules/13-time-and-stdlib-utilities/examples/02_time_zones.py`
- **Code:** [examples/02_time_zones.py](./examples/02_time_zones.py)

  ```python
  before = datetime(2030, 3, 30, 12, 0, tzinfo=berlin)
  wall = before + timedelta(days=1)  # same local time
  absolute = (before.astimezone(UTC) + timedelta(days=1)).astimezone(berlin)  # 24 real hours
  ```

- **Walkthrough:** arithmetic on an aware value keeps the wall clock, so "+1 day" across the spring-forward night is only 23 real hours. Convert to UTC first when you mean elapsed time. Nonexistent local times are not rejected (they resolve using the pre-transition offset) and ambiguous ones are chosen with `fold`. A `due_date` (a `date`) has no instant until you pick a zone; its deadline is the next local midnight.
- **Expected output:**

  ```text
  UTC:      2030-07-01T12:00:00+00:00
  Berlin:   2030-07-01T14:00:00+02:00
  New York: 2030-07-01T08:00:00-04:00
  wall clock +1 day: 2030-03-31T12:00:00+02:00
  absolute   +1 day: 2030-03-31T13:00:00+02:00
  nonexistent 02:30 -> 2030-03-31T01:30:00+00:00
  ambiguous 02:30 fold=0: 2030-10-27T02:30:00+02:00
  ambiguous 02:30 fold=1: 2030-10-27T02:30:00+01:00
  due 2030-01-31 ends at 2030-02-01T05:00:00+00:00
  ```

### 3.3 Ids, randomness and hashing
- **Goal:** pick the right primitive.
- **Run:** `python modules/13-time-and-stdlib-utilities/examples/03_ids_random_hashing.py`
- **Code:** [examples/03_ids_random_hashing.py](./examples/03_ids_random_hashing.py)
- **Walkthrough:** `uuid7()` ids are monotonic within the process, so they sort by creation time. A seeded `random.Random` is reproducible (great for tests, wrong for secrets); `secrets.token_urlsafe(16)` yields 22 URL-safe characters. `hashlib.sha256` fingerprints content; `hmac` adds a key, and `compare_digest` checks it in constant time. Random values (uuid4/7, token) are shown only by their properties.
- **Expected output:**

  ```text
  uuid4 version: 4
  uuid7 version: 7
  uuid7 sorts by time: True
  uuid5 is deterministic: cfbff0d1-9375-5685-968c-48ce8b15ae17
  seeded rng: [6, 1, 1, 6, 3]
  seeded choice: in_progress
  token length: 22 chars, hex: 32
  sha256: 345b88bffbe2a2d20b6ae1dd09ef4952cc3d74aef54078d81a503601d9bb1441
  blake2b-8: 3584baf7e1368c9c
  hmac: 87a36ec8c6b901b0...
  valid: True
  ```

### 3.4 Regular expressions and templates
- **Goal:** compile once, validate with `fullmatch`, and avoid catastrophic backtracking.
- **Run:** `python modules/13-time-and-stdlib-utilities/examples/04_regex_and_templates.py`
- **Code:** [examples/04_regex_and_templates.py](./examples/04_regex_and_templates.py)

  ```python
  SLUG = re.compile(r"[a-z0-9]+(?:-[a-z0-9]+)*")
  SLUG.fullmatch("write-docs")  # the whole string must match
  re.match(r"[a-z]+", "abc!!!")  # truthy: match() only anchors at the start
  ```

- **Walkthrough:** named groups make `finditer` results readable; `re.escape` turns user input into a literal; `Template.safe_substitute` leaves unknown placeholders alone. The last lines time a nested quantifier `(a+)+$` on 22 characters against a flat `a+$` on 5000: only the booleans are printed, because timings are not deterministic.
- **Expected output:**

  ```text
  tags: ['docs', 'p1']
  slug 'write-docs': True
  slug 'Write-docs': False
  slug 'write--docs': False
  slug 'docs\n': False
  match() is lenient: True
  masked: *****@example.com
  escaped: a\+b True
  Hi Ana, 'Write docs' is due on 2030-01-31.
  Hi Ana, '$title' is due on $due.
  nested quantifier is slower despite 200x less input: True
  ```

### 3.5 Task Tracker: overdue reminders on a schedule
- **Goal:** a periodic in-process job whose "now" is injected.
- **Run:** `python modules/13-time-and-stdlib-utilities/examples/05_overdue_reminder.py`
- **Code:** [examples/05_overdue_reminder.py](./examples/05_overdue_reminder.py)
- **Walkthrough:** `is_overdue` turns a date-only `due_date` into a deadline in the owner's zone (Europe/Berlin here). `run_every` keeps a fixed-rate schedule from `loop.time()`, logs a failing job and keeps going. `FakeClock` advances one day per tick, so the output never depends on the real date. In a service you would start `run_every` as a task in the FastAPI lifespan and cancel it on shutdown.
- **Expected output:**

  ```text
  2030-01-31 12:00Z overdue: nothing
  2030-02-01 12:00Z overdue: ['Write docs']
  2030-02-02 12:00Z overdue: ['Write docs']
  2030-02-03 12:00Z overdue: ['Write docs', 'Ship v1']
  ```

## 4. Common Pitfalls

| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| `datetime.utcnow()` / `utcfromtimestamp()` | They return naive values that later get treated as local time | `datetime.now(UTC)`, `datetime.fromtimestamp(ts, UTC)` |
| Naive datetimes in models or JSON | `datetime(2030, 1, 1)` has no `tzinfo`; Pydantic and the DB driver accept it silently | Reject at the boundary (`AwareDatetime` in Pydantic, or a check like exercise 2) |
| `datetime.now()` inside business logic | Hidden global makes tests time-dependent | Inject a clock object or `now: datetime` parameter |
| `dt.replace(tzinfo=zone)` to "convert" | `replace` relabels the wall clock, `astimezone` converts the instant | `dt.astimezone(zone)` for aware values, `replace` only for naive ones you know are in `zone` |
| `date.today()` or `datetime.now().date()` as "today" | Uses the server's zone, not the user's | `datetime.now(zone).date()` with the owner's `ZoneInfo` |
| `time.sleep()` in `async def` | Blocks the whole event loop, every request stalls | `await asyncio.sleep()`, or `asyncio.to_thread` for blocking work |
| `asyncio.create_task(loop())` without keeping the task | The task can be garbage collected and its error is lost | Keep a reference (set or `TaskGroup`), cancel it at shutdown, log inside the loop |
| `while True: do(); await asyncio.sleep(60)` | Drifts by the job's runtime and a raised error ends the loop silently | Schedule from a fixed start (exercise 3) and catch `Exception` (never `CancelledError`) |
| `random` for tokens or ids exposed to users | Mersenne Twister is predictable | `secrets.token_urlsafe()`; `uuid4`/`uuid7` are for identity, not for authorization |
| `re.match` for validation, nested quantifiers like `(a+)+` | `match` is not anchored at the end; backtracking is exponential on hostile input | `fullmatch`, flat patterns, limit input length before matching |
| Missing `tzdata` in slim images | `ZoneInfo("Europe/Berlin")` raises `ZoneInfoNotFoundError` | Add `tzdata` to dependencies |

## 5. Coming From Other Languages
- **JavaScript/TypeScript:** `Date` is always an instant; Python's naive `datetime` is not, so the aware/naive split is the main adjustment. There is no `Temporal` yet, but `zoneinfo` plus `timedelta` covers most of it.
- **Java/C#:** `datetime` aware ≈ `OffsetDateTime`/`DateTimeOffset`; a `date` ≈ `LocalDate`/`DateOnly`; `ZoneInfo` ≈ `ZoneId`/`TimeZoneInfo`. Python has no separate `Instant` type: use an aware UTC `datetime`.
- **Go:** `time.Time` always carries a location; Python can omit it (naive), which is the source of most bugs. `time.Ticker` maps to a coroutine loop with `asyncio.sleep`, not to a channel.
- **Ruby:** `Time` and `DateTime` mix zones similarly; Python's explicitness (`astimezone`) is closer to `Time#getlocal`.

## 6. Exercises
Starter code and tests are in [exercises/](./exercises/); reference solutions in [solutions/](./solutions/). Run them from `languages/python/`.

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | [Duration strings](./exercises/ex1_duration/m13_duration.py) | Easy | `timedelta`, compiled `re`, `fullmatch` | `pytest modules/13-time-and-stdlib-utilities/exercises/ex1_duration` |
| 2 | [Due dates and the wire format](./exercises/ex2_due_dates/m13_due.py) | Medium | aware datetimes, `zoneinfo`, DST | `pytest modules/13-time-and-stdlib-utilities/exercises/ex2_due_dates` |
| 3 | [Fixed-rate scheduler](./exercises/ex3_scheduler/m13_scheduler.py) | Stretch | asyncio loop, injected clock, cancellation | `pytest modules/13-time-and-stdlib-utilities/exercises/ex3_scheduler` |

**Exercise 1 - `parse_duration` / `format_duration`**
- **Task:** parse `1h30m`, `2d`, `90s`, `1d2h3m4s` into a `timedelta` and format one back (`0s` for zero).
- **Acceptance:** valid forms parse; empty text, spaces, repeated or out-of-order units, signs, fractions and trailing newlines raise `ValueError`; negative durations cannot be formatted; format then parse round-trips.
- <details><summary>Hint</summary>One regex with four optional named groups and <code>fullmatch</code>; then reject the empty string separately.</details>

**Exercise 2 - `to_wire`, `from_wire`, `deadline`, `is_overdue`, `time_left`**
- **Task:** emit/parse UTC RFC 3339 with whole seconds, require offsets, and turn a date-only due date into the instant that day ends in a given zone.
- **Acceptance:** `Z` output, naive input rejected, offsets normalised, the deadline differs per zone and shifts across the New York DST change (a 23-hour day), the boundary is inclusive, `None` is never overdue.
- <details><summary>Hint</summary>The deadline is the <em>next</em> local midnight: <code>datetime.combine(due + timedelta(days=1), time.min, tzinfo=tz)</code>, then <code>astimezone(UTC)</code>.</details>

**Exercise 3 - `run_every`**
- **Task:** implement a fixed-rate runner against a `Clock` protocol (`monotonic`, `sleep`).
- **Acceptance:** runs at 0, 10, 20...; job duration does not drift the schedule; missed ticks are skipped; errors go to `on_error` and the loop continues; cancellation propagates; no trailing sleep after the last run; `SystemClock` works with real time.
- <details><summary>Hint</summary>Keep <code>next_tick</code> as a running total (<code>+= interval</code>), never "now + interval"; catch <code>Exception</code>, which does not include <code>CancelledError</code>.</details>

## 7. Recap
- Store and compare aware UTC datetimes; parse and format RFC 3339; apply zones only at the edges.
- `zoneinfo` handles DST: arithmetic on aware values keeps the wall clock, so convert to UTC for elapsed time.
- `uuid7` for time-ordered ids, `secrets` for tokens, `hashlib`/`hmac` for digests and signatures.
- Compile regexes once, validate with `fullmatch`, avoid nested quantifiers.
- Periodic jobs are plain coroutines: fixed schedule, caught errors, cancelled at shutdown, clock injected.

## 8. Self-check Questions
1. Why does `datetime(2030, 1, 1, 9, 0) < datetime.now(UTC)` raise instead of returning a boolean?
2. A task is due `2030-03-10` for a New York user. Why is "`due_date` + 24 h" a wrong deadline?
3. When is `uuid4` the better choice than `uuid7`, and when is neither acceptable?
4. What is the difference between `dt.replace(tzinfo=z)` and `dt.astimezone(z)`?
5. Why does a `while True: ...; await asyncio.sleep(60)` loop drift, and what does fixed-rate scheduling change?

## 9. Further Reading
- [datetime](https://docs.python.org/3/library/datetime.html) and [zoneinfo](https://docs.python.org/3/library/zoneinfo.html)
- [uuid](https://docs.python.org/3/library/uuid.html), [secrets](https://docs.python.org/3/library/secrets.html), [re HOWTO](https://docs.python.org/3/howto/regex.html)
- [asyncio: sleeping and tasks](https://docs.python.org/3/library/asyncio-task.html)
- [RFC 3339](https://www.rfc-editor.org/rfc/rfc3339)

---
**Next:** [14 Packaging and Running](../14-packaging-and-running/README.md)
