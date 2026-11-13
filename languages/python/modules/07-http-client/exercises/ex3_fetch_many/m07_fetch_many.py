"""Exercise 3 (Stretch): fetch many items concurrently with a bounded number in flight."""

from collections.abc import Sequence
from dataclasses import dataclass

import httpx


@dataclass(slots=True, frozen=True)
class Item:
    id: int
    data: object


@dataclass(slots=True, frozen=True)
class ItemError:
    id: int
    error: str


@dataclass(slots=True, frozen=True)
class Report:
    results: list[Item]
    errors: list[ItemError]


async def fetch_many(
    client: httpx.AsyncClient, ids: Sequence[int], *, max_concurrency: int = 4
) -> Report:
    """GET /items/{id} for every id, with at most `max_concurrency` requests in flight.

    - `results` and `errors` both keep the order of `ids`.
    - A 2xx response gives Item(id, response.json()).
    - Any other status gives ItemError(id, "upstream status <code>").
    - httpx.TimeoutException gives ItemError(id, "upstream timeout"); any other
      httpx.TransportError gives ItemError(id, "upstream unreachable").
    - One failing id must not stop or fail the others.
    """
    raise NotImplementedError("TODO: implement fetch_many")
