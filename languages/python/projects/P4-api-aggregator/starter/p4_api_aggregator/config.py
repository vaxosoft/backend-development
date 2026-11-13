import os
from dataclasses import dataclass


@dataclass(frozen=True, slots=True)
class Config:
    upstream_url: str
    max_concurrency: int = 4
    call_timeout: float = 2.0
    max_retries: int = 2
    backoff_base: float = 0.1
    overall_deadline: float = 10.0

    @classmethod
    def from_env(cls) -> Config:
        return cls(
            upstream_url=os.environ.get("UPSTREAM_URL", "http://127.0.0.1:9000"),
            max_concurrency=int(os.environ.get("MAX_CONCURRENCY", "4")),
            call_timeout=float(os.environ.get("CALL_TIMEOUT", "2.0")),
            max_retries=int(os.environ.get("MAX_RETRIES", "2")),
            backoff_base=float(os.environ.get("BACKOFF_BASE", "0.1")),
            overall_deadline=float(os.environ.get("OVERALL_DEADLINE", "10.0")),
        )
