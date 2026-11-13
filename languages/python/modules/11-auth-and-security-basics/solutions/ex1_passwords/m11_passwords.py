"""Exercise 1: password hashing with scrypt.

Stored format (one string): `scrypt$<n>$<r>$<p>$<salt base64>$<hash base64>`, e.g.
`scrypt$16384$8$1$c2FsdA==$...`. r is 8, p is 1, the derived key is 32 bytes, the salt is
16 random bytes unless one is passed in.

* `hash_password(password, salt=None, n=DEFAULT_N)` -> stored string (use `hashlib.scrypt`)
* `verify_password(password, stored)` -> bool. It must use the parameters stored in the
  string (not the defaults), compare with `hmac.compare_digest`, and return False (never
  raise) for any malformed string.
* `needs_rehash(stored, n=DEFAULT_N)` -> True when the hash was made with a different cost
  than `n` (or cannot be parsed), so a login can upgrade it.
"""

import base64
import hashlib
import hmac
import secrets

DEFAULT_N = 2**14
R = 8
P = 1
DKLEN = 32


def hash_password(password: str, salt: bytes | None = None, n: int = DEFAULT_N) -> str:
    salt = salt if salt is not None else secrets.token_bytes(16)
    digest = hashlib.scrypt(password.encode(), salt=salt, n=n, r=R, p=P, dklen=DKLEN)
    salt_b64 = base64.b64encode(salt).decode()
    return f"scrypt${n}${R}${P}${salt_b64}${base64.b64encode(digest).decode()}"


def _parse(stored: str) -> tuple[int, int, int, bytes, bytes]:
    scheme, n, r, p, salt_b64, hash_b64 = stored.split("$")
    if scheme != "scrypt":
        raise ValueError("unknown scheme")
    return int(n), int(r), int(p), base64.b64decode(salt_b64), base64.b64decode(hash_b64)


def verify_password(password: str, stored: str) -> bool:
    try:
        n, r, p, salt, expected = _parse(stored)
        actual = hashlib.scrypt(password.encode(), salt=salt, n=n, r=r, p=p, dklen=len(expected))
    except ValueError:  # includes binascii.Error and unpacking errors
        return False
    return hmac.compare_digest(actual, expected)


def needs_rehash(stored: str, n: int = DEFAULT_N) -> bool:
    try:
        return _parse(stored)[0] != n
    except ValueError:
        return True
