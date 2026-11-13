"""Exercise 2 (Medium): retry with exponential backoff, for idempotent requests only."""

import time
from collections.abc import Callable

import httpx

IDEMPOTENT_METHODS = frozenset({"GET", "HEAD", "PUT", "DELETE", "OPTIONS"})


def request_with_retry(
    client: httpx.Client,
    method: str,
    url: str,
    *,
    retries: int = 2,
    base_delay: float = 0.1,
    sleep: Callable[[float], None] = time.sleep,
) -> httpx.Response:
    """Send a request, retrying up to `retries` extra times.

    - Retry on httpx.TransportError and on 5xx responses. Never retry 4xx.
    - Only methods in IDEMPOTENT_METHODS are retried; others get a single attempt.
    - Before retry number n (n = 1, 2, ...) call sleep(base_delay * 2 ** (n - 1)).
    - When attempts run out: re-raise the last transport error, or for a 5xx call
      response.raise_for_status() so the caller gets httpx.HTTPStatusError.
      A non-idempotent request that fails with 5xx simply returns that response.
    - Return the first response that is not a 5xx.
    """
    raise NotImplementedError("TODO: implement request_with_retry")
