"""Retry with exponential backoff and jitter: what to retry, what never to retry."""

import random
from collections.abc import Callable

import httpx
from m07_helpers import Reply, Request, json_reply, serve

IDEMPOTENT = frozenset({"GET", "HEAD", "PUT", "DELETE", "OPTIONS"})
RETRY_STATUSES = frozenset({429, 502, 503, 504})


def send_with_retry(
    client: httpx.Client,
    request: httpx.Request,
    *,
    retries: int = 3,
    base_delay: float = 0.1,
    sleep: Callable[[float], None],
    rng: random.Random,
) -> httpx.Response:
    attempts = retries + 1 if request.method in IDEMPOTENT else 1
    for attempt in range(attempts):
        last = attempt == attempts - 1
        try:
            response = client.send(request)
        except httpx.TransportError:
            if last:
                raise
        else:
            if response.status_code not in RETRY_STATUSES or last:
                return response
            response.close()
        # "full jitter": a random wait up to the exponential ceiling avoids synchronised retries
        sleep(rng.uniform(0, base_delay * 2**attempt))
    raise AssertionError("unreachable")


def main() -> None:
    calls: dict[str, int] = {}

    def flaky(request: Request) -> Reply:
        calls[request.path] = calls.get(request.path, 0) + 1
        if request.path == "/flaky" and calls[request.path] < 3:
            return json_reply({"error": "busy"}, status=503)
        if request.path == "/bad":
            return json_reply({"error": "no such thing"}, status=404)
        if request.path == "/down":
            return json_reply({"error": "down"}, status=503)
        return json_reply({"ok": True})

    waits: list[float] = []
    rng = random.Random(7)

    def run(method: str, path: str) -> None:
        request = client.build_request(method, path)
        waits.clear()
        calls.clear()
        response = send_with_retry(client, request, sleep=waits.append, rng=rng)
        outcome = f"{response.status_code} after {calls[path]} call(s), waits={len(waits)}"
        print(f"{method} {path}: {outcome}")

    with serve(flaky) as base_url, httpx.Client(base_url=base_url) as client:
        run("GET", "/flaky")  # 503, 503, 200
        run("GET", "/bad")  # 4xx is never retried
        run("GET", "/down")  # gives up and returns the last 503
        run("POST", "/down")  # POST is not idempotent: a single attempt

    ceilings = [0.1 * 2**n for n in range(3)]
    print("ceilings:", ceilings)


if __name__ == "__main__":
    main()
