"""Timeouts, cancellation and failure inside a TaskGroup."""

import asyncio


async def slow_job(name: str, log: list[str]) -> str:
    try:
        log.append(f"{name}: started")
        await asyncio.sleep(10)
        return "finished"
    except asyncio.CancelledError:
        log.append(f"{name}: cancelled")
        raise  # always re-raise, otherwise the canceller waits for a task that pretends to continue
    finally:
        log.append(f"{name}: cleanup")


async def failing_job() -> None:
    await asyncio.sleep(0.01)
    raise ValueError("bad input")


async def main() -> None:
    log: list[str] = []

    # asyncio.timeout cancels the body when the deadline passes and raises TimeoutError.
    try:
        async with asyncio.timeout(0.02):
            await slow_job("timed", log)
    except TimeoutError:
        print("timed out")
    print(log)

    # Cancelling a task you own: cancel() then await it to let the cleanup run.
    log.clear()
    task = asyncio.create_task(slow_job("manual", log))
    await asyncio.sleep(0)
    task.cancel()
    try:
        await task
    except asyncio.CancelledError:
        print("task cancelled:", task.cancelled())
    print(log)

    # When one task in a TaskGroup fails, its siblings are cancelled and the errors are
    # re-raised together as an ExceptionGroup, handled with except*.
    log.clear()
    try:
        async with asyncio.TaskGroup() as group:
            group.create_task(slow_job("sibling", log))
            group.create_task(failing_job())
    except* ValueError as eg:
        print("ValueError handled:", [str(e) for e in eg.exceptions])
    print(log)


asyncio.run(main())
