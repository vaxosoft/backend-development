"""Identifiers, randomness and hashing: pick the right tool for each job."""

import hashlib
import hmac
import random
import secrets
import uuid

# uuid4: 122 random bits. uuid7: a millisecond timestamp prefix, so ids sort by creation time.
a, b = uuid.uuid7(), uuid.uuid7()
print("uuid4 version:", uuid.uuid4().version)
print("uuid7 version:", a.version)
print("uuid7 sorts by time:", a < b)
print("uuid5 is deterministic:", uuid.uuid5(uuid.NAMESPACE_DNS, "example.com"))

# random is a seedable, predictable generator: good for simulations and tests, never for secrets.
rng = random.Random(42)
print("seeded rng:", [rng.randint(1, 6) for _ in range(5)])
print("seeded choice:", random.Random(7).choice(["todo", "in_progress", "done"]))

# secrets draws from the operating system: tokens, session ids, password reset links.
token = secrets.token_urlsafe(16)
print("token length:", len(token), "chars, hex:", len(secrets.token_hex(16)))

# hashlib: digests identify content (ETag, checksums). Not for storing passwords (module 11).
print("sha256:", hashlib.sha256(b"Write docs").hexdigest())
print("blake2b-8:", hashlib.blake2b(b"Write docs", digest_size=8).hexdigest())

# hmac signs a message with a key; compare_digest avoids timing leaks.
key = b"server-secret"
signature = hmac.new(key, b"task:1", hashlib.sha256).hexdigest()
print("hmac:", signature[:16] + "...")
print(
    "valid:", hmac.compare_digest(signature, hmac.new(key, b"task:1", hashlib.sha256).hexdigest())
)
