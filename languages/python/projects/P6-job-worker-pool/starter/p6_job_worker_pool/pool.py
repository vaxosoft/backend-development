import asyncio
import logging
from typing import Any

from .config import Config
from .models import Job, JobType

logger = logging.getLogger("p6_job_worker_pool")


class WorkerPool:
    """A fixed number of asyncio workers fed by a bounded queue of job ids.

    Everything runs on one event loop, so the methods that do not await need no locks.
    """

    def __init__(self, config: Config) -> None:
        self._config = config
        self._jobs: dict[str, Job] = {}  # insertion order = submission order
        self._queue: asyncio.Queue[Job] = asyncio.Queue(maxsize=config.queue_capacity)
        self._workers: list[asyncio.Task[None]] = []
        self._running: dict[str, asyncio.Task[None]] = {}  # job id -> task executing its attempts
        self._closing = False

    async def start(self) -> None:
        """Spawn `config.workers` worker tasks. Keep references to them."""
        raise NotImplementedError("TODO: start")

    def submit(self, job_type: JobType, payload: dict[str, Any]) -> Job:
        """Register a queued job and put it on the queue.

        PoolClosed once shutdown has begun; QueueFull (and no stored job) when the queue is full.
        """
        raise NotImplementedError("TODO: submit")

    def get(self, job_id: str) -> Job | None:
        raise NotImplementedError("TODO: get")

    def list_newest_first(self) -> list[Job]:
        raise NotImplementedError("TODO: list_newest_first")

    def cancel(self, job_id: str) -> None:
        """Cancel a queued or running job.

        JobNotFound for an unknown id, JobFinished for a job in a final status.
        """
        raise NotImplementedError("TODO: cancel")

    async def shutdown(self) -> None:
        """Stop accepting jobs, let queued and running jobs finish for at most
        `config.drain_timeout`, then cancel whatever is left and stop the workers."""
        raise NotImplementedError("TODO: shutdown")
