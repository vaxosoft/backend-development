"""Security helpers shared by the examples of module 11 (standard library + PyJWT only)."""

import base64
import hashlib
import hmac
import secrets
import time
from collections.abc import Callable
from dataclasses import dataclass
from datetime import UTC, datetime, timedelta
from typing import Any

import jwt

SCRYPT_N, SCRYPT_R, SCRYPT_P = 2**14, 8, 1  # ~16 MiB and a few dozen ms per hash
JWT_ALGORITHM = "HS256"


def _b64(raw: bytes) -> str:
    return base64.b64encode(raw).decode("ascii")


def hash_password(password: str, *, salt: bytes | None = None) -> str:
    """Return `scrypt$n$r$p$salt$hash`: the parameters travel with the hash, so they can change."""
    salt = salt if salt is not None else secrets.token_bytes(16)
    digest = hashlib.scrypt(
        password.encode(), salt=salt, n=SCRYPT_N, r=SCRYPT_R, p=SCRYPT_P, dklen=32
    )
    return f"scrypt${SCRYPT_N}${SCRYPT_R}${SCRYPT_P}${_b64(salt)}${_b64(digest)}"


def verify_password(password: str, stored: str) -> bool:
    try:
        scheme, n, r, p, salt_b64, hash_b64 = stored.split("$")
        if scheme != "scrypt":
            return False
        expected = base64.b64decode(hash_b64)
        actual = hashlib.scrypt(
            password.encode(),
            salt=base64.b64decode(salt_b64),
            n=int(n),
            r=int(r),
            p=int(p),
            dklen=len(expected),
        )
    except ValueError:
        return False
    return hmac.compare_digest(actual, expected)


def new_api_key() -> tuple[str, str]:
    """Return (key shown to the user once, sha256 hex digest that is stored)."""
    key = "tt_" + secrets.token_urlsafe(32)
    return key, hash_api_key(key)


def hash_api_key(key: str) -> str:
    return hashlib.sha256(key.encode()).hexdigest()  # keys are high-entropy: no scrypt needed


def issue_token(
    user_id: int, role: str, secret: str, *, ttl: timedelta = timedelta(hours=1)
) -> str:
    now = datetime.now(UTC)
    claims = {"sub": str(user_id), "role": role, "iat": now, "exp": now + ttl}
    return jwt.encode(claims, secret, algorithm=JWT_ALGORITHM)


def decode_token(token: str, secret: str) -> dict[str, Any]:
    """Raises `jwt.InvalidTokenError` (or a subclass) for anything that is not a valid token."""
    return jwt.decode(
        token,
        secret,
        algorithms=[JWT_ALGORITHM],  # never trust the header's "alg"
        options={"require": ["exp", "sub"]},
    )


@dataclass(slots=True)
class Bucket:
    tokens: float
    updated: float


class RateLimiter:
    """Token bucket per key: `burst` tokens, refilled at `refill_per_second`."""

    def __init__(
        self, burst: int, refill_per_second: float, clock: Callable[[], float] = time.monotonic
    ) -> None:
        self.burst = burst
        self.rate = refill_per_second
        self.clock = clock
        self.buckets: dict[str, Bucket] = {}

    def check(self, key: str) -> float:
        """Return 0.0 if the request is allowed, else the seconds to wait."""
        now = self.clock()
        bucket = self.buckets.setdefault(key, Bucket(float(self.burst), now))
        bucket.tokens = min(self.burst, bucket.tokens + (now - bucket.updated) * self.rate)
        bucket.updated = now
        if bucket.tokens >= 1:
            bucket.tokens -= 1
            return 0.0
        return (1 - bucket.tokens) / self.rate
