"""Task Tracker: bulk import with bounded concurrency, per-item timeouts and a request id."""

import asyncio
import contextvars
from dataclasses import dataclass

request_id: contextvars.ContextVar[str] = contextvars.ContextVar("request_id", default="-")


@dataclass(frozen=True, slots=True)
class Outcome:
    index: int
    ok: bool
    detail: str


class TaskStore:
    """In-memory store: the lock makes "read the counter, then write it" one step."""

    def __init__(self) -> None:
        self._lock = asyncio.Lock()
        self._next_id = 1
        self.tasks: dict[int, str] = {}

    async def add(self, title: str) -> int:
        async with self._lock:
            task_id = self._next_id
            await asyncio.sleep(
                0
            )  # suspension point: without the lock, tasks could interleave here
            self._next_id += 1
            self.tasks[task_id] = title
            return task_id


async def notify_owner(title: str) -> None:
    await asyncio.sleep(0.2 if "slow" in title else 0.005)  # pretend this is an HTTP call


async def import_one(index: int, title: str, store: TaskStore, limit: asyncio.Semaphore) -> Outcome:
    async with limit:
        try:
            if not title.strip():
                raise ValueError("title must be 1-200 characters")
            task_id = await store.add(title.strip())
            async with asyncio.timeout(0.05):
                await notify_owner(title)
            return Outcome(index, True, f"[{request_id.get()}] task {task_id}: {title}")
        except TimeoutError:
            return Outcome(index, False, f"[{request_id.get()}] notification timed out: {title}")
        except ValueError as err:
            return Outcome(index, False, f"[{request_id.get()}] rejected: {err}")


async def import_tasks(titles: list[str], concurrency: int = 3) -> list[Outcome]:
    store = TaskStore()
    limit = asyncio.Semaphore(concurrency)
    # One failure must not cancel the others, so each worker returns an Outcome instead of raising.
    async with asyncio.TaskGroup() as group:
        jobs = [group.create_task(import_one(i, t, store, limit)) for i, t in enumerate(titles)]
    return [job.result() for job in jobs]


async def handle_request(rid: str) -> list[Outcome]:
    request_id.set(rid)  # tasks created afterwards inherit a copy of the context
    return await import_tasks(["Write docs", "", "slow report", "Deploy"])


async def main() -> None:
    # Each request runs in its own task, so each has its own context value.
    first, second = await asyncio.gather(
        asyncio.create_task(handle_request("req-1")), asyncio.create_task(handle_request("req-2"))
    )
    for outcome in first:
        print(outcome.index, outcome.ok, outcome.detail)
    print("second request ids:", sorted({o.detail[1:].split("]")[0] for o in second}))
    print("outside any request:", request_id.get())


asyncio.run(main())
