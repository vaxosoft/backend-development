"""Password hashing (scrypt) and JWT helpers."""

from .config import Config


class InvalidTokenError(Exception):
    pass


def hash_password(password: str, n: int) -> str:
    """Return "scrypt$<n>$<r>$<p>$<salt b64>$<hash b64>" using a fresh random 16-byte salt."""
    raise NotImplementedError("TODO: hashlib.scrypt with a random salt")


def verify_password(password: str, stored: str) -> bool:
    """Check `password` against a value made by hash_password, in constant time."""
    raise NotImplementedError("TODO: parse the stored parameters, recompute, hmac.compare_digest")


def issue_token(config: Config, user_id: int) -> str:
    """HS256 JWT with `sub` (the user id as a string), `iat` and `exp` (iat + token_ttl_seconds)."""
    raise NotImplementedError("TODO: jwt.encode")


def decode_token(config: Config, token: str) -> int:
    """Return the user id; raise InvalidTokenError when the token is bad, forged or expired."""
    raise NotImplementedError("TODO: jwt.decode with algorithms=['HS256'] and required claims")
