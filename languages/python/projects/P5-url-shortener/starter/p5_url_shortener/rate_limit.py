"""Token bucket rate limiter (one bucket per key)."""

from collections.abc import Callable


class RateLimiter:
    def __init__(self, burst: int, refill_per_second: float, clock: Callable[[], float]) -> None:
        self._burst = burst
        self._refill_per_second = refill_per_second
        self._clock = clock

    def try_acquire(self, key: str) -> float:
        """Take one token for `key`.

        Return 0.0 when allowed, otherwise the seconds until a token is available.
        A new key starts with a full bucket of `burst` tokens; tokens refill continuously.
        """
        raise NotImplementedError("TODO: per-key token bucket guarded by a lock")
