import asyncio
import logging
from typing import Any

from .config import Config
from .errors import JobError, JobFinished, JobNotFound, PoolClosed, QueueFull
from .handlers import run_job
from .models import Job, JobStatus, JobType

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
        self._workers = [
            asyncio.create_task(self._worker(), name=f"worker-{n}")
            for n in range(self._config.workers)
        ]

    def submit(self, job_type: JobType, payload: dict[str, Any]) -> Job:
        """Register a queued job and put it on the queue.

        PoolClosed once shutdown has begun; QueueFull (and no stored job) when the queue is full.
        """
        if self._closing:
            raise PoolClosed
        job = Job(type=job_type, payload=payload)
        try:
            self._queue.put_nowait(job)
        except asyncio.QueueFull:
            raise QueueFull from None
        self._jobs[job.id] = job
        return job

    def get(self, job_id: str) -> Job | None:
        return self._jobs.get(job_id)

    def list_newest_first(self) -> list[Job]:
        return list(reversed(self._jobs.values()))

    def cancel(self, job_id: str) -> None:
        """Cancel a queued or running job.

        JobNotFound for an unknown id, JobFinished for a job in a final status.
        """
        job = self._jobs.get(job_id)
        if job is None:
            raise JobNotFound
        if job.status.is_final:
            raise JobFinished
        job.status = JobStatus.CANCELLED
        # A queued job stays on the queue; the worker that dequeues it skips it.
        if (task := self._running.get(job_id)) is not None:
            task.cancel()

    async def shutdown(self) -> None:
        """Stop accepting jobs, let queued and running jobs finish for at most
        `config.drain_timeout`, then cancel whatever is left and stop the workers."""
        self._closing = True
        try:
            async with asyncio.timeout(self._config.drain_timeout.total_seconds()):
                await self._queue.join()
        except TimeoutError:
            logger.warning("drain timeout reached, cancelling remaining jobs")
        for worker in self._workers:
            worker.cancel()
        await asyncio.gather(*self._workers, return_exceptions=True)
        for job in self._jobs.values():
            if job.status is JobStatus.QUEUED:
                job.status = JobStatus.CANCELLED

    async def _worker(self) -> None:
        while True:
            job = await self._queue.get()
            try:
                if job.status is JobStatus.QUEUED:
                    await self._process(job)
            finally:
                self._queue.task_done()

    async def _process(self, job: Job) -> None:
        job.status = JobStatus.RUNNING
        # The attempts run in their own task so that cancelling a job does not kill the worker.
        task = asyncio.create_task(self._attempt_loop(job))
        self._running[job.id] = task
        try:
            await asyncio.wait({task})
        except asyncio.CancelledError:  # shutdown cancels the worker itself
            task.cancel()
            job.status = JobStatus.CANCELLED
            raise
        finally:
            del self._running[job.id]
        if task.cancelled():
            job.status = JobStatus.CANCELLED
        elif (crash := task.exception()) is not None:
            job.status = JobStatus.FAILED
            job.error = "internal error"
            logger.error("job crashed", exc_info=crash, extra={"job_id": job.id})
        logger.info(
            "job finished",
            extra={"job_id": job.id, "status": job.status, "attempts": job.attempts},
        )

    async def _attempt_loop(self, job: Job) -> None:
        for attempt in range(1, self._config.max_attempts + 1):
            job.attempts = attempt
            try:
                async with asyncio.timeout(self._config.job_timeout.total_seconds()):
                    job.result = await run_job(job.type, job.payload)
            except TimeoutError:
                job.error = f"timed out after {self._config.job_timeout.total_seconds():g}s"
            except JobError as err:
                job.error = str(err)
            else:
                job.status = JobStatus.SUCCEEDED
                job.error = None
                return
            if attempt < self._config.max_attempts:
                await asyncio.sleep(self._config.retry_delay.total_seconds())
        job.status = JobStatus.FAILED
