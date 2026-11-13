import asyncio
from collections.abc import Iterator

import pytest
from m06_pipeline import run_pipeline


async def double(n: int) -> int:
    await asyncio.sleep(0.001)
    return n * 2


async def test_results_are_in_input_order() -> None:
    async def uneven(n: int) -> int:
        await asyncio.sleep(0.02 if n % 2 == 0 else 0.001)  # evens finish last
        return n * 10

    assert await run_pipeline(range(10), uneven, workers=4, queue_size=2) == [
        n * 10 for n in range(10)
    ]


async def test_empty_input() -> None:
    assert await run_pipeline([], double, workers=2, queue_size=1) == []


async def test_concurrency_equals_the_number_of_workers() -> None:
    running = 0
    peak = 0

    async def tracked(n: int) -> int:
        nonlocal running, peak
        running += 1
        peak = max(peak, running)
        await asyncio.sleep(0.01)
        running -= 1
        return n

    await run_pipeline(range(12), tracked, workers=3, queue_size=2)
    assert peak == 3


async def test_producer_is_throttled_by_the_bounded_queue() -> None:
    pulled = 0
    gate = asyncio.Event()

    def source() -> Iterator[int]:
        nonlocal pulled
        for i in range(1000):
            pulled += 1
            yield i

    async def blocked(n: int) -> int:
        await gate.wait()
        return n

    task = asyncio.create_task(run_pipeline(source(), blocked, workers=2, queue_size=3))
    await asyncio.sleep(0.05)
    # 2 items in workers + 3 queued + 1 held by the producer while it waits for space
    assert 5 <= pulled <= 6
    gate.set()
    assert len(await task) == 1000


async def test_transform_error_cancels_everything() -> None:
    cancelled: list[int] = []
    before = asyncio.all_tasks()

    async def transform(n: int) -> int:
        if n == 0:
            await asyncio.sleep(0.005)
            raise ValueError("bad item")
        try:
            await asyncio.sleep(5)
        except asyncio.CancelledError:
            cancelled.append(n)
            raise
        return n

    with pytest.raises(ExceptionGroup) as info:
        await run_pipeline(range(100), transform, workers=3, queue_size=2)
    assert info.group_contains(ValueError, match="bad item")
    assert sorted(cancelled) == [1, 2]  # the other two workers were mid-transform
    assert asyncio.all_tasks() == before


async def test_timeout_cancels_workers_and_raises_timeout_error() -> None:
    before = asyncio.all_tasks()

    async def forever(n: int) -> int:
        await asyncio.sleep(10)
        return n

    with pytest.raises(TimeoutError):
        await run_pipeline(range(5), forever, workers=2, queue_size=2, timeout=0.02)
    assert asyncio.all_tasks() == before


async def test_generous_timeout_does_not_interfere() -> None:
    assert await run_pipeline(range(3), double, workers=2, queue_size=1, timeout=5) == [0, 2, 4]


@pytest.mark.parametrize(("workers", "queue_size"), [(0, 1), (1, 0)])
async def test_invalid_sizes(workers: int, queue_size: int) -> None:
    with pytest.raises(ValueError, match=r"."):
        await run_pipeline(range(3), double, workers=workers, queue_size=queue_size)
