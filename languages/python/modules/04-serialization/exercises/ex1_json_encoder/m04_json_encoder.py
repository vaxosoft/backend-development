"""Exercise 1: a JSON encoder with the project's wire rules."""


def encode_json(value: object) -> str:
    """Return compact, key-sorted JSON for `value`.

    Rules:
    - separators without spaces, keys sorted, non-ASCII characters kept as they are
    - aware datetime -> UTC string with seconds precision and a trailing "Z"
    - naive datetime -> ValueError
    - date -> "YYYY-MM-DD", Decimal -> string (keeps "1.10" as "1.10")
    - set / frozenset -> sorted list
    - NaN and infinity -> ValueError
    - anything else the json module does not know -> TypeError
    """
    raise NotImplementedError
