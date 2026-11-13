import asyncio
from functools import partial

import pytest
from m06_limited_gather import gather_limited


class Probe:
    """Counts how many jobs run at the same time."""

    def __init__(self) -> None:
        self.running = 0
        self.peak = 0
        self.started: list[int] = []
        self.cancelled: list[int] = []

    async def job(self, n: int, delay: float = 0.01) -> int:
        self.started.append(n)
        self.running += 1
        self.peak = max(self.peak, self.running)
        try:
            await asyncio.sleep(delay)
            return n * 2
        except asyncio.CancelledError:
            self.cancelled.append(n)
            raise
        finally:
            self.running -= 1


async def test_never_exceeds_the_limit_but_uses_all_slots() -> None:
    probe = Probe()
    results = await gather_limited([partial(probe.job, n) for n in range(9)], limit=3)
    assert results == [n * 2 for n in range(9)]
    assert probe.peak == 3


async def test_results_keep_input_order_even_if_completion_order_differs() -> None:
    probe = Probe()
    delays = [0.03, 0.01, 0.02]
    factories = [partial(probe.job, n, d) for n, d in enumerate(delays)]
    assert await gather_limited(factories, limit=3) == [0, 2, 4]


async def test_limit_one_runs_sequentially() -> None:
    probe = Probe()
    await gather_limited([partial(probe.job, n, 0.002) for n in range(4)], limit=1)
    assert probe.peak == 1
    assert probe.started == [0, 1, 2, 3]


async def test_empty_input() -> None:
    assert await gather_limited([], limit=2) == []


@pytest.mark.parametrize("limit", [0, -1])
async def test_invalid_limit(limit: int) -> None:
    with pytest.raises(ValueError, match=r"."):
        await gather_limited([], limit=limit)


async def test_failure_cancels_running_jobs_and_skips_waiting_ones() -> None:
    probe = Probe()

    async def failing() -> int:
        await asyncio.sleep(0.01)
        raise ValueError("bad job")

    factories = [
        lambda: probe.job(0, 5),
        failing,
        lambda: probe.job(2, 5),
        lambda: probe.job(3, 5),
        lambda: probe.job(4, 5),
    ]
    with pytest.raises(ExceptionGroup) as info:
        await gather_limited(factories, limit=2)
    assert info.group_contains(ValueError, match="bad job")
    assert 0 in probe.cancelled
    assert sorted(probe.cancelled) == sorted(probe.started)  # nothing was left running
    assert len(probe.started) <= 2  # at most one waiting job grabbed the freed slot
    assert probe.running == 0


async def test_no_tasks_are_left_behind() -> None:
    before = asyncio.all_tasks()
    probe = Probe()
    await gather_limited([partial(probe.job, n) for n in range(5)], limit=2)
    assert asyncio.all_tasks() == before
