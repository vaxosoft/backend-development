from datetime import UTC, datetime

import pytest
from fastapi import FastAPI
from fastapi.testclient import TestClient
from m12_tasks_app import Task, create_app, get_clock, get_store

NOON = datetime(2030, 1, 1, 12, 0, tzinfo=UTC)


class SpyStore:
    """A fake that also records its calls, so tests can check what the handler passed in."""

    def __init__(self, seed: list[Task] | None = None) -> None:
        self.tasks = {t.id: t for t in seed or []}
        self.added: list[tuple[str, datetime]] = []

    def add(self, title: str, created_at: datetime) -> Task:
        self.added.append((title, created_at))
        task = Task(len(self.tasks) + 100, title, "todo", created_at)
        self.tasks[task.id] = task
        return task

    def get(self, task_id: int) -> Task | None:
        return self.tasks.get(task_id)

    def list_all(self) -> list[Task]:
        return list(self.tasks.values())


@pytest.fixture
def store() -> SpyStore:
    return SpyStore()


@pytest.fixture
def app(store: SpyStore) -> FastAPI:
    app = create_app()
    app.dependency_overrides[get_store] = lambda: store
    app.dependency_overrides[get_clock] = lambda: lambda: NOON
    return app


@pytest.fixture
def client(app: FastAPI) -> TestClient:
    return TestClient(app)


def test_create_uses_the_injected_store_and_clock(client: TestClient, store: SpyStore) -> None:
    response = client.post("/tasks", json={"title": "  Write docs "})

    assert response.status_code == 201
    assert response.headers["Location"] == "/tasks/100"
    assert response.json() == {
        "id": 100,
        "title": "Write docs",
        "status": "todo",
        "created_at": "2030-01-01T12:00:00Z",
    }
    assert store.added == [("Write docs", NOON)]


def test_the_real_store_is_never_touched_when_overridden(client: TestClient) -> None:
    client.post("/tasks", json={"title": "isolated"})
    client.get("/tasks")

    assert get_store().list_all() == []


def test_get_returns_a_seeded_task(app: FastAPI) -> None:
    seeded = SpyStore([Task(7, "seeded", "done", NOON)])
    app.dependency_overrides[get_store] = lambda: seeded

    response = TestClient(app).get("/tasks/7")

    assert response.status_code == 200
    assert response.json()["status"] == "done"


def test_unknown_task_is_a_404_envelope(client: TestClient) -> None:
    response = client.get("/tasks/5")

    assert response.status_code == 404
    assert response.json() == {"error": {"code": "not_found", "message": "task 5 not found"}}


def test_list_wraps_items_and_total(app: FastAPI) -> None:
    seeded = SpyStore([Task(1, "a", "todo", NOON), Task(2, "b", "todo", NOON)])
    app.dependency_overrides[get_store] = lambda: seeded

    body = TestClient(app).get("/tasks").json()

    assert body["total"] == 2
    assert [item["title"] for item in body["items"]] == ["a", "b"]


@pytest.mark.parametrize("payload", [{"title": "   "}, {"title": "x" * 201}, {}])
def test_invalid_payloads_are_422_and_never_reach_the_store(
    client: TestClient, store: SpyStore, payload: dict[str, str]
) -> None:
    response = client.post("/tasks", json=payload)

    assert response.status_code == 422
    assert response.json()["error"]["code"] == "validation_failed"
    assert store.added == []


def test_overrides_belong_to_one_app_instance(app: FastAPI) -> None:
    other = TestClient(create_app())  # no overrides: uses the default wiring

    assert other.get("/tasks/100").status_code == 404
    assert TestClient(app).get("/tasks").json()["total"] == 0
