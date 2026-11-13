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

DEFAULT_N = 2**14
R = 8
P = 1
DKLEN = 32


def hash_password(password: str, salt: bytes | None = None, n: int = DEFAULT_N) -> str:
    raise NotImplementedError


def verify_password(password: str, stored: str) -> bool:
    raise NotImplementedError


def needs_rehash(stored: str, n: int = DEFAULT_N) -> bool:
    raise NotImplementedError
