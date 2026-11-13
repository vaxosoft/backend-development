"""Rate limiting with a token bucket, driven by an injected clock so the demo is deterministic."""

import math

from m11_security import RateLimiter


class FakeClock:
    def __init__(self) -> None:
        self.now = 100.0

    def __call__(self) -> float:
        return self.now


def main() -> None:
    clock = FakeClock()
    limiter = RateLimiter(burst=3, refill_per_second=1.0, clock=clock)

    for attempt in range(1, 6):
        wait = limiter.check("user-1")
        verdict = "allowed" if wait == 0 else f"429, Retry-After: {math.ceil(wait)}"
        print(f"t=+0.0s request {attempt}: {verdict}")

    print(f"other user is unaffected: {limiter.check('user-2') == 0}")

    clock.now += 2.5  # 2.5 tokens refilled
    for _ in range(3):
        print(f"t=+2.5s: {'allowed' if limiter.check('user-1') == 0 else 'denied'}")


if __name__ == "__main__":
    main()
