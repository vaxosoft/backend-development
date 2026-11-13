"""Exercise 3 (Stretch): fetch many items concurrently with a bounded number in flight."""

import asyncio
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
    limit = asyncio.Semaphore(max_concurrency)

    async def fetch_one(item_id: int) -> Item | ItemError:
        async with limit:
            try:
                response = await client.get(f"/items/{item_id}")
            except httpx.TimeoutException:
                return ItemError(item_id, "upstream timeout")
            except httpx.TransportError:
                return ItemError(item_id, "upstream unreachable")
        if not response.is_success:
            return ItemError(item_id, f"upstream status {response.status_code}")
        return Item(item_id, response.json())

    async with asyncio.TaskGroup() as group:
        tasks = [group.create_task(fetch_one(item_id)) for item_id in ids]
    outcomes = [task.result() for task in tasks]
    return Report(
        [o for o in outcomes if isinstance(o, Item)],
        [o for o in outcomes if isinstance(o, ItemError)],
    )
