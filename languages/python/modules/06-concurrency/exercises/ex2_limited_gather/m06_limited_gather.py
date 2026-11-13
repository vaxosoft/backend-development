"""Exercise 2: run many awaitables concurrently, but never more than `limit` at once."""

from collections.abc import Awaitable, Callable, Sequence


async def gather_limited[T](factories: Sequence[Callable[[], Awaitable[T]]], limit: int) -> list[T]:
    """Call each factory and await the result, with at most `limit` awaitables running at once.

    - results are returned in the order of `factories`, not in completion order
    - a factory is only called when a slot is free (waiting jobs have not started)
    - if one job raises, the jobs still running are cancelled, waiting jobs are abandoned
      (not all of them may run), and the error propagates as an ExceptionGroup (what
      asyncio.TaskGroup raises)
    - limit < 1 raises ValueError
    """
    raise NotImplementedError
