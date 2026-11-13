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

import time
from collections.abc import Callable
from dataclasses import dataclass


@dataclass(frozen=True, slots=True)
class Decision:
    allowed: bool
    retry_after: float
    remaining: int


class TokenBucketLimiter:
    def __init__(
        self, burst: int, refill_per_second: float, clock: Callable[[], float] = time.monotonic
    ) -> None:
        raise NotImplementedError

    def allow(self, key: str, cost: int = 1) -> Decision:
        raise NotImplementedError

    def prune(self) -> int:
        raise NotImplementedError
