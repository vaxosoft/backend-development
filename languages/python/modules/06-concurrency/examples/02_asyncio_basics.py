"""asyncio basics: coroutines, gather, TaskGroup, background tasks and blocking calls."""

import asyncio
import time


async def fetch(name: str, delay: float) -> str:
    await asyncio.sleep(delay)
    return f"{name} done"


async def main() -> None:
    # Calling a coroutine function does not run it; it creates a coroutine object.
    coro = fetch("never started", 0)
    print(type(coro).__name__)
    coro.close()  # without this (or an await) Python warns "coroutine ... was never awaited"

    # Sequential awaits add up; concurrent tasks overlap.
    start = time.perf_counter()
    for name in ("a", "b", "c"):
        await fetch(name, 0.05)
    sequential = time.perf_counter() - start

    start = time.perf_counter()
    results = await asyncio.gather(fetch("a", 0.05), fetch("b", 0.05), fetch("c", 0.05))
    concurrent = time.perf_counter() - start
    print(results, "| overlapped:", concurrent < sequential / 2)

    # TaskGroup (3.11+) is structured: the block ends only when every task has finished.
    async with asyncio.TaskGroup() as group:
        slow = group.create_task(fetch("slow", 0.03))
        fast = group.create_task(fetch("fast", 0.01))
    print(fast.result(), "/", slow.result())

    # create_task() alone keeps only a weak reference: hold on to background tasks.
    background: set[asyncio.Task[str]] = set()
    task = asyncio.create_task(fetch("background", 0.01))
    background.add(task)
    task.add_done_callback(background.discard)
    print(await task)

    # A blocking call stalls the whole event loop; to_thread keeps it responsive.
    ticks = 0

    async def ticker() -> None:
        nonlocal ticks
        while True:
            await asyncio.sleep(0.005)
            ticks += 1

    ticking = asyncio.create_task(ticker())
    await asyncio.sleep(0)  # let the ticker start
    time.sleep(0.05)  # BAD: nothing else can run meanwhile
    print("ticks during time.sleep:", ticks)
    await asyncio.to_thread(time.sleep, 0.05)  # GOOD: runs in a worker thread
    print("ticks during to_thread:", "several" if ticks >= 3 else ticks)
    ticking.cancel()


asyncio.run(main())
