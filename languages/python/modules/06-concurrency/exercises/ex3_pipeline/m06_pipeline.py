"""Exercise 3: a bounded async pipeline with a worker pool, back-pressure and a deadline."""

from collections.abc import Awaitable, Callable, Iterable


async def run_pipeline(
    items: Iterable[int],
    transform: Callable[[int], Awaitable[int]],
    *,
    workers: int,
    queue_size: int,
    timeout: float | None = None,
) -> list[int]:
    """Apply `transform` to every item using `workers` concurrent workers.

    - a producer pulls items from `items` into a queue of at most `queue_size` entries and
      must wait when it is full (back-pressure: do not read the whole input up front)
    - workers take items from the queue; when the input is exhausted they all stop
      (hint: asyncio.Queue.shutdown())
    - the result list is in input order
    - if `transform` raises, everything else is cancelled and the error propagates as an
      ExceptionGroup (asyncio.TaskGroup behaviour)
    - if `timeout` seconds pass, cancel everything and raise TimeoutError
    - workers < 1 or queue_size < 1 raises ValueError
    - no task may be left running when the function returns or raises
    """
    raise NotImplementedError
