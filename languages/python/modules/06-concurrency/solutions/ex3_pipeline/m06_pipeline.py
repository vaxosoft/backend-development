import asyncio
from collections.abc import Awaitable, Callable, Iterable


async def run_pipeline(
    items: Iterable[int],
    transform: Callable[[int], Awaitable[int]],
    *,
    workers: int,
    queue_size: int,
    timeout: float | None = None,
) -> list[int]:
    if workers < 1 or queue_size < 1:
        raise ValueError("workers and queue_size must be at least 1")
    queue: asyncio.Queue[tuple[int, int]] = asyncio.Queue(maxsize=queue_size)
    results: dict[int, int] = {}

    async def produce() -> None:
        for index, item in enumerate(items):
            await queue.put((index, item))
        queue.shutdown()  # consumers still drain what is queued, then get QueueShutDown

    async def consume() -> None:
        while True:
            try:
                index, item = await queue.get()
            except asyncio.QueueShutDown:
                return
            results[index] = await transform(item)

    async with asyncio.timeout(timeout), asyncio.TaskGroup() as group:
        group.create_task(produce())
        for _ in range(workers):
            group.create_task(consume())
    return [results[i] for i in range(len(results))]
