"""Token bucket rate limiter (one bucket per key)."""

import threading
from collections.abc import Callable
from dataclasses import dataclass


@dataclass(slots=True)
class _Bucket:
    tokens: float
    updated: float


class RateLimiter:
    def __init__(self, burst: int, refill_per_second: float, clock: Callable[[], float]) -> None:
        if burst < 1 or refill_per_second <= 0:
            raise ValueError("burst must be >= 1 and refill_per_second > 0")
        self._burst = burst
        self._refill_per_second = refill_per_second
        self._clock = clock
        self._buckets: dict[str, _Bucket] = {}
        self._lock = threading.Lock()

    def try_acquire(self, key: str) -> float:
        """Take one token for `key`.

        Return 0.0 when allowed, otherwise the seconds until a token is available.
        A new key starts with a full bucket of `burst` tokens; tokens refill continuously.
        """
        now = self._clock()
        with self._lock:
            bucket = self._buckets.setdefault(key, _Bucket(float(self._burst), now))
            elapsed = max(0.0, now - bucket.updated)
            bucket.tokens = min(self._burst, bucket.tokens + elapsed * self._refill_per_second)
            bucket.updated = now
            if bucket.tokens >= 1:
                bucket.tokens -= 1
                return 0.0
            return (1 - bucket.tokens) / self._refill_per_second
