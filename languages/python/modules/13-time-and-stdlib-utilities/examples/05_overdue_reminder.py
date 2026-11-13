"""Task Tracker: a periodic in-process job that lists overdue tasks, using an injected clock."""

import asyncio
import logging
from collections.abc import Awaitable, Callable
from dataclasses import dataclass
from datetime import UTC, date, datetime, time, timedelta
from zoneinfo import ZoneInfo

log = logging.getLogger("reminders")


class FakeClock:
    """Test double for `datetime.now(UTC)`: time only moves when told to."""

    def __init__(self, start: datetime) -> None:
        self._now = start

    def now(self) -> datetime:
        return self._now

    def advance(self, delta: timedelta) -> None:
        self._now += delta


@dataclass(slots=True, frozen=True)
class Task:
    id: int
    title: str
    due_date: date | None


def is_overdue(task: Task, now: datetime, tz: ZoneInfo) -> bool:
    """A date-only deadline ends when that calendar day ends in the owner's zone."""
    if task.due_date is None:
        return False
    deadline = datetime.combine(task.due_date + timedelta(days=1), time.min, tzinfo=tz)
    return now >= deadline


async def run_every(
    interval: float, job: Callable[[], Awaitable[None]], *, runs: int | None = None
) -> None:
    """Run `job` now and then every `interval` seconds without drift; a failing job is logged."""
    loop = asyncio.get_running_loop()
    next_run = loop.time()
    count = 0
    while runs is None or count < runs:
        try:
            await job()
        except Exception:
            log.exception("job failed; keeping the schedule")
        count += 1
        next_run += interval  # fixed-rate: based on the schedule, not on when the job finished
        await asyncio.sleep(max(0.0, next_run - loop.time()))


async def main() -> None:
    zone = ZoneInfo("Europe/Berlin")
    tasks = [
        Task(1, "Write docs", date(2030, 1, 31)),
        Task(2, "Ship v1", date(2030, 2, 2)),
        Task(3, "Someday", None),
    ]
    # A fake clock that advances one day per tick keeps the output deterministic.
    clock = FakeClock(datetime(2030, 1, 31, 12, 0, tzinfo=UTC))

    async def remind() -> None:
        now = clock.now()
        late = [t.title for t in tasks if is_overdue(t, now, zone)]
        print(f"{now:%Y-%m-%d %H:%M}Z overdue: {late or 'nothing'}")
        clock.advance(timedelta(days=1))

    await run_every(0.01, remind, runs=4)


if __name__ == "__main__":
    asyncio.run(main())
