import os
from dataclasses import dataclass
from datetime import timedelta


@dataclass(slots=True, frozen=True)
class Config:
    workers: int = 4
    queue_capacity: int = 16
    max_attempts: int = 3
    retry_delay: timedelta = timedelta(milliseconds=200)
    job_timeout: timedelta = timedelta(seconds=30)
    drain_timeout: timedelta = timedelta(seconds=10)

    @classmethod
    def from_env(cls) -> Config:
        """Read WORKERS, QUEUE_CAPACITY, MAX_ATTEMPTS and *_MS durations from the environment."""
        default = cls()

        def ms(name: str, fallback: timedelta) -> timedelta:
            raw = os.environ.get(name)
            return fallback if raw is None else timedelta(milliseconds=int(raw))

        return cls(
            workers=int(os.environ.get("WORKERS", default.workers)),
            queue_capacity=int(os.environ.get("QUEUE_CAPACITY", default.queue_capacity)),
            max_attempts=int(os.environ.get("MAX_ATTEMPTS", default.max_attempts)),
            retry_delay=ms("RETRY_DELAY_MS", default.retry_delay),
            job_timeout=ms("JOB_TIMEOUT_MS", default.job_timeout),
            drain_timeout=ms("DRAIN_TIMEOUT_MS", default.drain_timeout),
        )
