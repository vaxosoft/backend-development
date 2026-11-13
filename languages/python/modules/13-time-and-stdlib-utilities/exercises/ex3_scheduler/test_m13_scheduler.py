import asyncio
from collections.abc import Awaitable, Callable

import pytest
from m13_scheduler import SystemClock, run_every


class FakeClock:
    """Virtual time: sleeping advances the clock instantly and records the requested delay."""

    def __init__(self) -> None:
        self.now = 0.0
        self.sleeps: list[float] = []

    def monotonic(self) -> float:
        return self.now

    async def sleep(self, delay: float) -> None:
        self.sleeps.append(delay)
        self.now += delay
        await asyncio.sleep(0)


def recorder(
    clock: FakeClock, durations: list[float] | None = None
) -> tuple[list[float], Callable[[], Awaitable[None]]]:
    starts: list[float] = []

    async def job() -> None:
        starts.append(clock.now)
        if durations:
            clock.now += durations[len(starts) - 1]

    return starts, job


async def test_runs_immediately_then_at_fixed_rate() -> None:
    clock = FakeClock()
    starts, job = recorder(clock)
    await run_every(10, job, clock, runs=4)
    assert starts == [0, 10, 20, 30]
    assert clock.sleeps == [10, 10, 10]


async def test_job_duration_does_not_cause_drift() -> None:
    clock = FakeClock()
    starts, job = recorder(clock, durations=[3, 3, 3])
    await run_every(10, job, clock, runs=3)
    assert starts == [0, 10, 20]
    assert clock.sleeps == [7, 7]


async def test_missed_ticks_are_skipped_not_replayed() -> None:
    clock = FakeClock()
    starts, job = recorder(clock, durations=[25, 0, 0])
    await run_every(10, job, clock, runs=3)
    assert starts == [0, 30, 40]
    assert clock.sleeps == [5, 10]


async def test_errors_are_reported_and_the_schedule_continues() -> None:
    clock = FakeClock()
    seen: list[Exception] = []
    calls = 0

    async def flaky() -> None:
        nonlocal calls
        calls += 1
        if calls == 2:
            raise RuntimeError("boom")

    await run_every(5, flaky, clock, runs=3, on_error=seen.append)
    assert calls == 3
    assert [str(e) for e in seen] == ["boom"]


async def test_errors_without_handler_do_not_stop_the_loop() -> None:
    clock = FakeClock()
    calls = 0

    async def always_fails() -> None:
        nonlocal calls
        calls += 1
        raise ValueError("nope")

    await run_every(5, always_fails, clock, runs=3)
    assert calls == 3


async def test_cancellation_propagates_out_of_the_job() -> None:
    clock = FakeClock()
    started = asyncio.Event()
    handled: list[Exception] = []

    async def hangs() -> None:
        started.set()
        await asyncio.Event().wait()

    task = asyncio.create_task(run_every(5, hangs, clock, on_error=handled.append))
    for _ in range(10):
        await asyncio.sleep(0)
    assert started.is_set()
    task.cancel()
    with pytest.raises(asyncio.CancelledError):
        await task
    assert handled == []


@pytest.mark.parametrize("interval", [0, -1.5])
async def test_interval_must_be_positive(interval: float) -> None:
    async def job() -> None:
        raise AssertionError("must not run")

    with pytest.raises(ValueError, match="interval"):
        await run_every(interval, job, FakeClock())


async def test_works_with_the_real_clock() -> None:
    ticks = 0

    async def job() -> None:
        nonlocal ticks
        ticks += 1

    await asyncio.wait_for(run_every(0.001, job, SystemClock(), runs=3), timeout=2)
    assert ticks == 3
