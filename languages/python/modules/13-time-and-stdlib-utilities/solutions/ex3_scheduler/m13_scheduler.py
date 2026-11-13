"""A fixed-rate in-process scheduler that is testable without real time."""

import asyncio
import logging
import math
import time
from collections.abc import Awaitable, Callable
from typing import Protocol

log = logging.getLogger(__name__)


class Clock(Protocol):
    def monotonic(self) -> float: ...

    async def sleep(self, delay: float) -> None: ...


class SystemClock:
    def monotonic(self) -> float:
        return time.monotonic()

    async def sleep(self, delay: float) -> None:
        await asyncio.sleep(delay)


async def run_every(
    interval: float,
    job: Callable[[], Awaitable[None]],
    clock: Clock,
    *,
    runs: int | None = None,
    on_error: Callable[[Exception], None] | None = None,
) -> None:
    """Run `job` immediately, then at fixed-rate ticks `interval` seconds apart.

    - Ticks are computed from the schedule (start + n * interval), so the job's own duration
      does not make the schedule drift.
    - A tick that is strictly in the past when the job finishes is skipped (no catch-up burst).
    - An Exception from the job goes to `on_error` (if given) and the schedule continues.
      Cancellation (CancelledError) is never swallowed.
    - With `runs` set, return right after that many executions, without a trailing sleep.
    - ValueError when `interval` is not positive.
    """
    if interval <= 0:
        raise ValueError("interval must be positive")
    next_tick = clock.monotonic()
    count = 0
    while runs is None or count < runs:
        try:
            await job()
        except Exception as err:
            if on_error is not None:
                on_error(err)
            else:
                log.exception("scheduled job failed")
        count += 1
        if count == runs:
            return
        next_tick += interval
        now = clock.monotonic()
        if next_tick < now:
            next_tick += math.ceil((now - next_tick) / interval) * interval
        await clock.sleep(max(0.0, next_tick - now))
