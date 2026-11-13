"""The core of the project: validate ids, call the upstream, collect ordered outcomes."""

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
    raise NotImplementedError("TODO: parse and validate the ids query parameter")


async def fetch_item(client: httpx.AsyncClient, config: Config, item_id: int) -> Outcome:
    """GET /items/{id} with a per-call timeout and up to `max_retries` retries.

    Retry network errors, timeouts and 5xx with exponential backoff
    (`backoff_base`, then twice that, ...). Never retry 4xx.
    Failure messages: "upstream status 500", "upstream timeout", "upstream unreachable".
    """
    raise NotImplementedError("TODO: call the upstream with retries")


async def aggregate(client: httpx.AsyncClient, config: Config, ids: list[int]) -> list[Outcome]:
    """Fetch every id concurrently (at most `max_concurrency` in flight).

    The result has one outcome per requested id, in request order. When
    `overall_deadline` seconds pass, outstanding calls are cancelled and their ids
    become Failure(id, "deadline exceeded"); finished calls keep their outcome.
    """
    raise NotImplementedError("TODO: run fetch_item concurrently with a bound and a deadline")
