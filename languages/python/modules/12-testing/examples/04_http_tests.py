"""HTTP handler tests: TestClient, dependency_overrides and an async client.

Run: pytest modules/12-testing/examples/04_http_tests.py -vv --no-header
"""

from collections.abc import Iterator
from datetime import UTC, datetime

import httpx
import pytest
from fastapi import FastAPI
from fastapi.testclient import TestClient
from m12_app import create_app, get_service
from m12_tasks import InMemoryTaskRepository, TaskService

NOON = datetime(2030, 1, 1, 12, 0, tzinfo=UTC)


@pytest.fixture
def app() -> Iterator[FastAPI]:
    app = create_app()
    # Each test gets its own service: no shared state, a frozen clock, no real storage.
    service = TaskService(InMemoryTaskRepository(), clock=lambda: NOON)
    app.dependency_overrides[get_service] = lambda: service
    yield app
    app.dependency_overrides.clear()


@pytest.fixture
def client(app: FastAPI) -> TestClient:
    return TestClient(app)


def test_create_returns_201_location_and_the_task(client: TestClient) -> None:
    response = client.post("/tasks", json={"title": "  Write docs ", "tags": ["Docs", "docs"]})

    assert response.status_code == 201
    assert response.headers["Location"] == "/tasks/1"
    assert response.json() == {
        "id": 1,
        "title": "Write docs",
        "status": "todo",
        "tags": ["docs"],
        "created_at": "2030-01-01T12:00:00Z",
    }


def test_get_returns_what_was_created(client: TestClient) -> None:
    client.post("/tasks", json={"title": "Write docs"})

    response = client.get("/tasks/1")

    assert response.status_code == 200
    assert response.json()["title"] == "Write docs"


def test_unknown_id_uses_the_error_envelope(client: TestClient) -> None:
    response = client.get("/tasks/99")

    assert response.status_code == 404
    assert response.json() == {"error": {"code": "not_found", "message": "task 99 not found"}}


@pytest.mark.parametrize(
    "body",
    [{"title": "   "}, {"title": "x" * 201}, {"title": 5}, {}],
    ids=["blank", "too-long", "wrong-type", "missing"],
)
def test_invalid_bodies_are_422(client: TestClient, body: dict[str, object]) -> None:
    response = client.post("/tasks", json=body)

    assert response.status_code == 422
    assert response.json()["error"]["code"] == "validation_failed"


def test_malformed_json_is_a_400(client: TestClient) -> None:
    response = client.post("/tasks", content=b"{nope", headers={"Content-Type": "application/json"})

    assert response.status_code == 400
    assert response.json()["error"]["code"] == "bad_request"


async def test_async_client_talks_to_the_app_in_process(app: FastAPI) -> None:
    transport = httpx.ASGITransport(app=app)
    async with httpx.AsyncClient(transport=transport, base_url="http://test") as client:
        created = await client.post("/tasks", json={"title": "async"})
        listed = await client.get("/tasks")

    assert created.status_code == 201
    assert [t["title"] for t in listed.json()] == ["async"]


if __name__ == "__main__":
    raise SystemExit(pytest.main([__file__, "-vv", "--no-header"]))
