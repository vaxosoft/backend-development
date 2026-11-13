"""json: what maps cleanly, what silently changes shape and what it refuses."""

import json
from datetime import UTC, datetime
from decimal import Decimal

task = {"id": 1, "title": "Write docs", "tags": ["docs", "p1"], "owner_id": None, "done": False}

text = json.dumps(task)
print(text)
print(json.loads(text) == task)

# Compact separators and stable key order are what you want on the wire and in tests.
print(json.dumps(task, separators=(",", ":"), sort_keys=True))

# Round trips are not always identities.
print(json.loads(json.dumps({"point": (1, 2), 7: "int key"})))

# Types the encoder refuses: it raises TypeError instead of guessing.
for value in (datetime(2030, 1, 1, 9, tzinfo=UTC), Decimal("1.10"), {"a", "b"}):
    try:
        json.dumps(value)
    except TypeError as err:
        print("TypeError:", err)


def encode_extra(value: object) -> object:
    """`default=` is only called for objects the encoder does not know."""
    if isinstance(value, datetime):
        return value.isoformat().replace("+00:00", "Z")
    if isinstance(value, set):
        return sorted(value)
    raise TypeError(f"cannot serialise {type(value).__name__}")


print(
    json.dumps(
        {"created_at": datetime(2030, 1, 1, 9, tzinfo=UTC), "tags": {"b", "a"}},
        default=encode_extra,
    )
)

# Malformed input raises json.JSONDecodeError (a ValueError) with a position.
try:
    json.loads('{"title": "x",}')
except json.JSONDecodeError as err:
    print("JSONDecodeError:", err.msg, "at column", err.colno)

# json happily accepts NaN, which is not valid JSON. Reject it on the way in and out.
print(json.loads("NaN"))
try:
    json.dumps(float("nan"), allow_nan=False)
except ValueError as err:
    print("ValueError:", err)
