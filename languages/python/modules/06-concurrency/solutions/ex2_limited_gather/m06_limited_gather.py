import asyncio
from collections.abc import Awaitable, Callable, Sequence


async def gather_limited[T](factories: Sequence[Callable[[], Awaitable[T]]], limit: int) -> list[T]:
    if limit < 1:
        raise ValueError("limit must be at least 1")
    slots = asyncio.Semaphore(limit)

    async def run(factory: Callable[[], Awaitable[T]]) -> T:
        async with slots:  # the factory is called only after a slot is free
            return await factory()

    async with asyncio.TaskGroup() as group:
        tasks = [group.create_task(run(f)) for f in factories]
    return [task.result() for task in tasks]
