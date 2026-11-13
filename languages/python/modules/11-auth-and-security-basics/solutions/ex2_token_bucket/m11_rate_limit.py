"""Exercise 2: a per-key token-bucket rate limiter.

`TokenBucketLimiter(burst, refill_per_second, clock)`:

* every key starts with a full bucket of `burst` tokens; tokens refill continuously at
  `refill_per_second` but never exceed `burst`
* `allow(key, cost=1)` takes `cost` tokens and returns `Decision(allowed=True, ...)`; if there
  are not enough tokens it takes NOTHING and returns `allowed=False` with `retry_after`, the
  seconds until `cost` tokens are available
* `Decision.remaining` is the whole tokens left after the call
* `prune()` forgets buckets that are completely full again and returns how many it removed,
  so the dict does not grow with every key ever seen
* `burst < 1` or `refill_per_second <= 0` raises ValueError
"""

import math
import time
from collections.abc import Callable
from dataclasses import dataclass


@dataclass(frozen=True, slots=True)
class Decision:
    allowed: bool
    retry_after: float
    remaining: int


@dataclass(slots=True)
class _Bucket:
    tokens: float
    updated: float


class TokenBucketLimiter:
    def __init__(
        self, burst: int, refill_per_second: float, clock: Callable[[], float] = time.monotonic
    ) -> None:
        if burst < 1 or refill_per_second <= 0:
            raise ValueError("burst must be >= 1 and refill_per_second > 0")
        self.burst = burst
        self.rate = refill_per_second
        self.clock = clock
        self._buckets: dict[str, _Bucket] = {}

    def _refilled(self, key: str, now: float) -> _Bucket:
        bucket = self._buckets.setdefault(key, _Bucket(float(self.burst), now))
        bucket.tokens = min(float(self.burst), bucket.tokens + (now - bucket.updated) * self.rate)
        bucket.updated = now
        return bucket

    def allow(self, key: str, cost: int = 1) -> Decision:
        bucket = self._refilled(key, self.clock())
        if bucket.tokens >= cost:
            bucket.tokens -= cost
            return Decision(True, 0.0, math.floor(bucket.tokens))
        return Decision(False, (cost - bucket.tokens) / self.rate, math.floor(bucket.tokens))

    def prune(self) -> int:
        now = self.clock()
        full = [key for key in self._buckets if self._refilled(key, now).tokens >= self.burst]
        for key in full:
            del self._buckets[key]
        return len(full)
