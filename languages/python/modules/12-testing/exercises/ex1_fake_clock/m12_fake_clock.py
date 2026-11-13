"""Exercise 1: a controllable clock and a clock-driven cache.

`FakeClock` is the test double that makes time-dependent code deterministic:

* `FakeClock(start)` starts at `start` (default 2030-01-01 09:00 UTC); `start` must be
  timezone-aware, otherwise ValueError
* calling it returns the current fake time; time only moves when the test says so
* `advance(delta)` moves it forward; a negative delta raises ValueError (time never goes back)
* `set(moment)` jumps to an aware moment that is not earlier than the current one
  (ValueError otherwise)

`ExpiringCache(ttl, clock)` stores values that expire `ttl` after they were set:

* `set(key, value)` (re-setting a key restarts its ttl), `get(key)` -> value or None
* an entry is expired when `clock() >= set_time + ttl` (exactly at the ttl it is gone)
* `len(cache)` counts only live entries; `purge()` deletes expired entries and returns how many
"""

from collections.abc import Callable
from datetime import UTC, datetime, timedelta

DEFAULT_START = datetime(2030, 1, 1, 9, 0, tzinfo=UTC)


class FakeClock:
    def __init__(self, start: datetime = DEFAULT_START) -> None:
        raise NotImplementedError

    def __call__(self) -> datetime:
        raise NotImplementedError

    def advance(self, delta: timedelta) -> None:
        raise NotImplementedError

    def set(self, moment: datetime) -> None:
        raise NotImplementedError


class ExpiringCache[K, V]:
    def __init__(self, ttl: timedelta, clock: Callable[[], datetime]) -> None:
        raise NotImplementedError

    def set(self, key: K, value: V) -> None:
        raise NotImplementedError

    def get(self, key: K) -> V | None:
        raise NotImplementedError

    def __len__(self) -> int:
        raise NotImplementedError

    def purge(self) -> int:
        raise NotImplementedError
