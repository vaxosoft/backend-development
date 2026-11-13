from collections.abc import Callable
from datetime import UTC, datetime, timedelta
from typing import Any

import pytest
from fastapi.testclient import TestClient
from m03_listing import create_app

SEED: list[dict[str, Any]] = [
    {"title": "Write docs", "status": "todo", "due_date": "2030-02-01", "tags": ["docs"]},
    {"title": "Fix bug", "status": "in_progress", "due_date": "2030-01-15", "tags": ["bug", "p1"]},
    {"title": "Ship release", "status": "todo", "due_date": None, "tags": ["p1"]},
    {"title": "Review PR", "status": "done", "due_date": "2030-01-20", "tags": ["review"]},
    {"title": "Plan sprint", "status": "todo", "due_date": "2030-01-10", "tags": ["p1"]},
]


def stepping_clock() -> Callable[[], datetime]:
    ticks = iter(range(1000))
    return lambda: datetime(2030, 1, 1, 9, 0, tzinfo=UTC) + timedelta(minutes=next(ticks))


@pytest.fixture
def client() -> TestClient:
    client = TestClient(create_app(stepping_clock()))
    for body in SEED:
        assert client.post("/tasks", json=body).status_code == 201
    return client


def titles(response: Any) -> list[str]:
    assert response.status_code == 200, response.text
    return [item["title"] for item in response.json()["items"]]


def test_default_order_is_newest_first_with_page_metadata(client: TestClient) -> None:
    body = client.get("/tasks").json()
    assert [t["id"] for t in body["items"]] == [5, 4, 3, 2, 1]
    assert (body["total"], body["limit"], body["offset"]) == (5, 20, 0)


def test_sort_by_created_at_ascending(client: TestClient) -> None:
    assert titles(client.get("/tasks?sort=created_at"))[0] == "Write docs"


def test_sort_by_due_date_puts_undated_last_in_both_directions(client: TestClient) -> None:
    assert titles(client.get("/tasks?sort=due_date")) == [
        "Plan sprint",
        "Fix bug",
        "Review PR",
        "Write docs",
        "Ship release",
    ]
    assert titles(client.get("/tasks?sort=-due_date")) == [
        "Write docs",
        "Review PR",
        "Fix bug",
        "Plan sprint",
        "Ship release",
    ]


def test_filters_combine_with_and(client: TestClient) -> None:
    assert titles(client.get("/tasks?status=todo&tag=p1&sort=created_at")) == [
        "Ship release",
        "Plan sprint",
    ]
    assert titles(client.get("/tasks?q=REVIEW")) == ["Review PR"]
    assert titles(client.get("/tasks?tag=nope")) == []


def test_total_counts_matches_before_paging(client: TestClient) -> None:
    body = client.get("/tasks?status=todo&limit=1&offset=1&sort=created_at").json()
    assert [t["title"] for t in body["items"]] == ["Ship release"]
    assert (body["total"], body["limit"], body["offset"]) == (3, 1, 1)


def test_pages_cover_everything_exactly_once(client: TestClient) -> None:
    seen: list[str] = []
    for offset in (0, 2, 4):
        seen += titles(client.get(f"/tasks?limit=2&offset={offset}&sort=created_at"))
    assert seen == [s["title"] for s in SEED]


def test_offset_past_the_end_is_an_empty_page(client: TestClient) -> None:
    body = client.get("/tasks?offset=50").json()
    assert body["items"] == []
    assert body["total"] == 5


def test_limit_above_the_maximum_is_clamped(client: TestClient) -> None:
    body = client.get("/tasks?limit=1000").json()
    assert body["limit"] == 100
    assert len(body["items"]) == 5


@pytest.mark.parametrize(
    "query", ["limit=0", "limit=-1", "limit=abc", "offset=-1", "sort=title", "status=finished"]
)
def test_bad_query_values_are_400(client: TestClient, query: str) -> None:
    response = client.get(f"/tasks?{query}")
    assert response.status_code == 400
    assert response.json()["error"]["code"] == "bad_request"


def test_ties_are_broken_by_ascending_id() -> None:
    frozen = datetime(2030, 1, 1, tzinfo=UTC)
    client = TestClient(create_app(lambda: frozen))
    for title in "abc":
        client.post("/tasks", json={"title": title})
    for sort in ("created_at", "-created_at"):
        assert titles(client.get(f"/tasks?sort={sort}")) == ["a", "b", "c"]


def test_post_without_key_creates_every_time(client: TestClient) -> None:
    first = client.post("/tasks", json={"title": "same"})
    second = client.post("/tasks", json={"title": "same"})
    assert first.status_code == second.status_code == 201
    assert second.json()["id"] == first.json()["id"] + 1
    assert first.headers["location"] == f"/tasks/{first.json()['id']}"


def test_idempotency_key_replays_the_first_response(client: TestClient) -> None:
    headers = {"Idempotency-Key": "k-1"}
    first = client.post("/tasks", json={"title": "Pay"}, headers=headers)
    second = client.post("/tasks", json={"title": "Pay"}, headers=headers)
    assert first.status_code == second.status_code == 201
    assert second.json() == first.json()
    assert second.headers["location"] == first.headers["location"]
    assert "idempotency-replayed" not in first.headers
    assert second.headers["idempotency-replayed"] == "true"
    assert client.get("/tasks").json()["total"] == 6


def test_idempotency_key_with_different_body_is_409(client: TestClient) -> None:
    headers = {"Idempotency-Key": "k-2"}
    client.post("/tasks", json={"title": "Pay"}, headers=headers)
    response = client.post("/tasks", json={"title": "Pay later"}, headers=headers)
    assert response.status_code == 409
    assert response.json()["error"]["code"] == "conflict"
    assert client.get("/tasks").json()["total"] == 6


def test_different_keys_are_independent(client: TestClient) -> None:
    a = client.post("/tasks", json={"title": "Pay"}, headers={"Idempotency-Key": "a"})
    b = client.post("/tasks", json={"title": "Pay"}, headers={"Idempotency-Key": "b"})
    assert a.json()["id"] != b.json()["id"]
