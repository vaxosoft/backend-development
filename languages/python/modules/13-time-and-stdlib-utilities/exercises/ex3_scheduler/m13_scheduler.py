"""A fixed-rate in-process scheduler that is testable without real time."""

import asyncio
import time
from collections.abc import Awaitable, Callable
from typing import Protocol


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
    raise NotImplementedError
