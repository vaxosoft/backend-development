import os
import sys
import time
from collections.abc import Callable
from dataclasses import dataclass, field


def stdout_sink(line: str) -> None:
    sys.stdout.write(line + "\n")
    sys.stdout.flush()


@dataclass(frozen=True, slots=True)
class Config:
    jwt_secret: str
    db_path: str = ":memory:"
    token_ttl_seconds: int = 3600
    scrypt_n: int = 2**14
    rate_burst: int = 5
    rate_refill_per_second: float = 1.0
    # injectable so tests control time and capture the access log
    clock: Callable[[], float] = field(default=time.monotonic, repr=False)
    log_sink: Callable[[str], None] = field(default=stdout_sink, repr=False)

    @classmethod
    def from_env(cls) -> Config:
        secret = os.environ.get("JWT_SECRET", "")
        if len(secret) < 32:
            raise SystemExit("JWT_SECRET must be set and at least 32 characters long")
        return cls(
            jwt_secret=secret,
            db_path=os.environ.get("DB_PATH", "shortener.db"),
            rate_burst=int(os.environ.get("RATE_BURST", "5")),
            rate_refill_per_second=float(os.environ.get("RATE_REFILL_PER_SECOND", "1.0")),
        )
