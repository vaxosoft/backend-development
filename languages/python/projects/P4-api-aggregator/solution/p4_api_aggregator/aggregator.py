"""The core of the project: validate ids, call the upstream, collect ordered outcomes."""

import asyncio
from dataclasses import dataclass
from typing import Any

import httpx

from .config import Config

MAX_IDS = 50


class BadRequestError(Exception):
    """The client sent an invalid `ids` parameter (rendered as 400 `bad_request`)."""


@dataclass(frozen=True, slots=True)
class Success:
    id: int
    data: Any


@dataclass(frozen=True, slots=True)
class Failure:
    id: int
    error: str


Outcome = Success | Failure


def parse_ids(raw: str | None) -> list[int]:
    """Turn "1,2,3" into [1, 2, 3]; raise BadRequestError if missing, empty, > 50 or non-numeric."""
    if raw is None or not raw.strip():
        raise BadRequestError("ids is required, e.g. ids=1,2,3")
    parts = [part.strip() for part in raw.split(",")]
    if len(parts) > MAX_IDS:
        raise BadRequestError(f"at most {MAX_IDS} ids are allowed")
    # isdecimal() also accepts non-ASCII digits, so check isascii() too
    if not all(p.isascii() and p.isdecimal() for p in parts):
        raise BadRequestError("ids must be comma-separated integers")
    return [int(p) for p in parts]


async def fetch_item(client: httpx.AsyncClient, config: Config, item_id: int) -> Outcome:
    """GET /items/{id} with a per-call timeout and up to `max_retries` retries.

    Retry network errors, timeouts and 5xx with exponential backoff
    (`backoff_base`, then twice that, ...). Never retry 4xx.
    Failure messages: "upstream status 500", "upstream timeout", "upstream unreachable".
    """
    error = "upstream unreachable"
    for attempt in range(config.max_retries + 1):
        if attempt > 0:
            await asyncio.sleep(config.backoff_base * 2 ** (attempt - 1))
        try:
            response = await client.get(f"/items/{item_id}", timeout=config.call_timeout)
        except httpx.TimeoutException:
            error = "upstream timeout"
            continue
        except httpx.TransportError:
            error = "upstream unreachable"
            continue
        if response.status_code >= 500:
            error = f"upstream status {response.status_code}"
            continue
        if not response.is_success:
            return Failure(item_id, f"upstream status {response.status_code}")
        try:
            return Success(item_id, response.json())
        except ValueError:
            return Failure(item_id, "invalid upstream response")
    return Failure(item_id, error)


async def aggregate(client: httpx.AsyncClient, config: Config, ids: list[int]) -> list[Outcome]:
    """Fetch every id concurrently (at most `max_concurrency` in flight).

    The result has one outcome per requested id, in request order. When
    `overall_deadline` seconds pass, outstanding calls are cancelled and their ids
    become Failure(id, "deadline exceeded"); finished calls keep their outcome.
    """
    outcomes: list[Outcome | None] = [None] * len(ids)
    gate = asyncio.Semaphore(config.max_concurrency)

    async def worker(index: int, item_id: int) -> None:
        async with gate:
            outcomes[index] = await fetch_item(client, config, item_id)

    try:
        async with asyncio.timeout(config.overall_deadline), asyncio.TaskGroup() as group:
            for index, item_id in enumerate(ids):
                group.create_task(worker(index, item_id))
    except TimeoutError:
        pass  # whatever finished is already in `outcomes`
    return [
        outcome or Failure(item_id, "deadline exceeded")
        for outcome, item_id in zip(outcomes, ids, strict=True)
    ]
