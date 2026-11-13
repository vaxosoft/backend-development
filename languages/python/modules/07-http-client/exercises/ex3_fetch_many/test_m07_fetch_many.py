import asyncio

import httpx
from m07_fetch_many import Item, ItemError, fetch_many


class FakeUpstream:
    """Async MockTransport handler that records how many requests overlap."""

    def __init__(self, failures: dict[int, int | Exception] | None = None) -> None:
        self.failures = failures or {}
        self.current = 0
        self.peak = 0
        self.requested: list[int] = []

    async def __call__(self, request: httpx.Request) -> httpx.Response:
        item_id = int(request.url.path.rsplit("/", 1)[1])
        self.requested.append(item_id)
        self.current += 1
        self.peak = max(self.peak, self.current)
        try:
            await asyncio.sleep(0.005)
        finally:
            self.current -= 1
        outcome = self.failures.get(item_id)
        if isinstance(outcome, Exception):
            raise outcome
        if outcome is not None:
            return httpx.Response(outcome, json={"error": "boom"})
        return httpx.Response(200, json={"id": item_id, "name": f"item-{item_id}"})


def make_client(upstream: FakeUpstream) -> httpx.AsyncClient:
    return httpx.AsyncClient(transport=httpx.MockTransport(upstream), base_url="http://up.test")


async def test_results_keep_request_order() -> None:
    upstream = FakeUpstream()
    async with make_client(upstream) as client:
        report = await fetch_many(client, [3, 1, 2])

    assert report.results == [
        Item(3, {"id": 3, "name": "item-3"}),
        Item(1, {"id": 1, "name": "item-1"}),
        Item(2, {"id": 2, "name": "item-2"}),
    ]
    assert report.errors == []


async def test_never_exceeds_max_concurrency_but_does_overlap() -> None:
    upstream = FakeUpstream()
    async with make_client(upstream) as client:
        report = await fetch_many(client, list(range(1, 10)), max_concurrency=3)

    assert len(report.results) == 9
    assert upstream.peak == 3


async def test_partial_failures_are_reported_per_id() -> None:
    upstream = FakeUpstream({2: 500, 3: 404})
    async with make_client(upstream) as client:
        report = await fetch_many(client, [1, 2, 3, 4])

    assert [item.id for item in report.results] == [1, 4]
    assert report.errors == [
        ItemError(2, "upstream status 500"),
        ItemError(3, "upstream status 404"),
    ]


async def test_transport_failures_are_classified() -> None:
    upstream = FakeUpstream(
        {1: httpx.ReadTimeout("slow"), 2: httpx.ConnectError("refused"), 3: 200}
    )
    async with make_client(upstream) as client:
        report = await fetch_many(client, [1, 2, 3])

    assert report.errors == [ItemError(1, "upstream timeout"), ItemError(2, "upstream unreachable")]
    assert [item.id for item in report.results] == [3]


async def test_empty_input_makes_no_requests() -> None:
    upstream = FakeUpstream()
    async with make_client(upstream) as client:
        report = await fetch_many(client, [])

    assert (report.results, report.errors, upstream.requested) == ([], [], [])
