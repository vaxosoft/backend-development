"""Password hashing (scrypt) and JWT helpers."""

import base64
import hashlib
import hmac
import secrets
import time

import jwt

from .config import Config

SCRYPT_R = 8
SCRYPT_P = 1
SALT_BYTES = 16
KEY_BYTES = 32
ALGORITHM = "HS256"


class InvalidTokenError(Exception):
    pass


def _scrypt(password: str, salt: bytes, n: int, r: int, p: int) -> bytes:
    return hashlib.scrypt(password.encode(), salt=salt, n=n, r=r, p=p, dklen=KEY_BYTES)


def hash_password(password: str, n: int) -> str:
    """Return "scrypt$<n>$<r>$<p>$<salt b64>$<hash b64>" using a fresh random 16-byte salt."""
    salt = secrets.token_bytes(SALT_BYTES)
    digest = _scrypt(password, salt, n, SCRYPT_R, SCRYPT_P)
    b64 = base64.b64encode
    return f"scrypt${n}${SCRYPT_R}${SCRYPT_P}${b64(salt).decode()}${b64(digest).decode()}"


def verify_password(password: str, stored: str) -> bool:
    """Check `password` against a value made by hash_password, in constant time."""
    try:
        scheme, n, r, p, salt, expected = stored.split("$")
        if scheme != "scrypt":
            return False
        # the cost parameters come from the stored value, so old hashes keep verifying
        digest = _scrypt(password, base64.b64decode(salt), int(n), int(r), int(p))
        return hmac.compare_digest(digest, base64.b64decode(expected))
    except ValueError:
        return False


def issue_token(config: Config, user_id: int) -> str:
    """HS256 JWT with `sub` (the user id as a string), `iat` and `exp` (iat + token_ttl_seconds)."""
    now = int(time.time())
    claims = {"sub": str(user_id), "iat": now, "exp": now + config.token_ttl_seconds}
    return jwt.encode(claims, config.jwt_secret, algorithm=ALGORITHM)


def decode_token(config: Config, token: str) -> int:
    """Return the user id; raise InvalidTokenError when the token is bad, forged or expired."""
    try:
        claims = jwt.decode(
            token,
            config.jwt_secret,
            algorithms=[ALGORITHM],  # never trust the algorithm named in the token header
            options={"require": ["sub", "exp"]},
        )
        return int(claims["sub"])
    except (jwt.InvalidTokenError, ValueError) as err:
        raise InvalidTokenError("invalid token") from err
