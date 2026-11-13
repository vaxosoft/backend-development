"""Bounded queues, worker pools and fan-out/fan-in with asyncio."""

import asyncio


async def producer(queue: asyncio.Queue[int], count: int) -> None:
    for i in range(count):
        await queue.put(i)  # waits while the queue is full: back-pressure
    queue.shutdown()  # tells consumers "no more items" once the queue is drained


async def worker(
    queue: asyncio.Queue[int], results: list[tuple[int, int]], state: dict[str, int]
) -> None:
    while True:
        try:
            item = await queue.get()
        except asyncio.QueueShutDown:
            return
        state["busy"] += 1
        state["max_busy"] = max(state["max_busy"], state["busy"])
        state["max_queued"] = max(state["max_queued"], queue.qsize())
        await asyncio.sleep(0.005)
        results.append((item, item * item))
        state["busy"] -= 1


async def pool_demo() -> None:
    queue: asyncio.Queue[int] = asyncio.Queue(maxsize=2)
    results: list[tuple[int, int]] = []
    state = {"busy": 0, "max_busy": 0, "max_queued": 0}
    async with asyncio.TaskGroup() as group:
        group.create_task(producer(queue, 12))
        for _ in range(3):
            group.create_task(worker(queue, results, state))
    print(sorted(results)[:4], "... total", len(results))
    print(
        "max busy workers:", state["max_busy"], "| queue limit respected:", state["max_queued"] <= 2
    )


async def semaphore_demo() -> None:
    limit = asyncio.Semaphore(2)
    in_flight = 0
    peak = 0

    async def call(n: int) -> int:
        nonlocal in_flight, peak
        async with limit:  # at most 2 of these run at once
            in_flight += 1
            peak = max(peak, in_flight)
            await asyncio.sleep(0.005)
            in_flight -= 1
            return n * 10

    results = await asyncio.gather(*(call(n) for n in range(6)))
    print(results, "| peak in flight:", peak)


async def fan_in_demo() -> None:
    async def job(name: str, delay: float) -> str:
        await asyncio.sleep(delay)
        return name

    jobs = [job("slow", 0.03), job("fast", 0.01), job("medium", 0.02)]
    order = [await finished for finished in asyncio.as_completed(jobs)]
    print("completion order:", order)


async def main() -> None:
    await pool_demo()
    await semaphore_demo()
    await fan_in_demo()


asyncio.run(main())
