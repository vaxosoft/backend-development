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


def _require_aware(moment: datetime) -> datetime:
    if moment.tzinfo is None:
        raise ValueError("naive datetimes are not allowed")
    return moment


class FakeClock:
    def __init__(self, start: datetime = DEFAULT_START) -> None:
        self._now = _require_aware(start)

    def __call__(self) -> datetime:
        return self._now

    def advance(self, delta: timedelta) -> None:
        if delta < timedelta(0):
            raise ValueError("time cannot go backwards")
        self._now += delta

    def set(self, moment: datetime) -> None:
        if _require_aware(moment) < self._now:
            raise ValueError("time cannot go backwards")
        self._now = moment


class ExpiringCache[K, V]:
    def __init__(self, ttl: timedelta, clock: Callable[[], datetime]) -> None:
        self._ttl = ttl
        self._clock = clock
        self._entries: dict[K, tuple[V, datetime]] = {}  # value, expires_at

    def set(self, key: K, value: V) -> None:
        self._entries[key] = (value, self._clock() + self._ttl)

    def _alive(self, expires_at: datetime) -> bool:
        return self._clock() < expires_at

    def get(self, key: K) -> V | None:
        entry = self._entries.get(key)
        if entry is None or not self._alive(entry[1]):
            return None
        return entry[0]

    def __len__(self) -> int:
        return sum(1 for _, expires_at in self._entries.values() if self._alive(expires_at))

    def purge(self) -> int:
        expired = [k for k, (_, expires_at) in self._entries.items() if not self._alive(expires_at)]
        for key in expired:
            del self._entries[key]
        return len(expired)
