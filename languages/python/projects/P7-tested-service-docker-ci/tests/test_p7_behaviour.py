"""Behaviour tests: health, readiness, metrics, tasks and an injected store."""

from datetime import UTC, datetime

from fastapi.testclient import TestClient
from p7_tested_service_docker_ci.app import create_app
from p7_tested_service_docker_ci.models import Task, TaskCreate
from p7_tested_service_docker_ci.store import InMemoryTaskStore

FIXED_NOW = datetime(2030, 1, 1, 9, 0, tzinfo=UTC)


def make_client() -> TestClient:
    store = InMemoryTaskStore(clock=lambda: FIXED_NOW)
    return TestClient(create_app(store=store))


def test_healthz_reports_ok() -> None:
    response = make_client().get("/healthz")

    assert response.status_code == 200
    assert response.json() == {"status": "ok"}


def test_readyz_is_503_until_startup_has_finished() -> None:
    client = make_client()  # not used as a context manager: the lifespan has not run

    response = client.get("/readyz")

    assert response.status_code == 503
    assert response.json() == {"status": "not_ready"}


def test_readyz_is_200_once_the_app_has_started() -> None:
    with make_client() as client:
        response = client.get("/readyz")

    assert response.status_code == 200
    assert response.json() == {"status": "ready"}


def test_metrics_is_plain_text_with_requests_total_line() -> None:
    response = make_client().get("/metrics")

    assert response.status_code == 200
    assert response.headers["content-type"].startswith("text/plain")
    assert "requests_total " in response.text


def test_metrics_counts_every_request_including_the_scrape_itself() -> None:
    client = make_client()
    for _ in range(3):
        client.get("/healthz")
    client.get("/tasks/999")  # a 404 is still a request

    response = client.get("/metrics")

    assert response.text.splitlines() == ["requests_total 5"]


def test_create_task_returns_201_location_and_defaults() -> None:
    response = make_client().post("/tasks", json={"title": "  Write docs  ", "tags": ["docs"]})

    assert response.status_code == 201
    assert response.headers["location"] == "/tasks/1"
    assert response.json() == {
        "id": 1,
        "title": "Write docs",
        "status": "todo",
        "due_date": None,
        "tags": ["docs"],
        "created_at": "2030-01-01T09:00:00Z",
    }


def test_get_task_returns_the_stored_task() -> None:
    client = make_client()
    client.post("/tasks", json={"title": "first"})
    client.post("/tasks", json={"title": "second", "status": "done", "due_date": "2030-02-01"})

    response = client.get("/tasks/2")

    assert response.status_code == 200
    assert response.json()["title"] == "second"
    assert response.json()["due_date"] == "2030-02-01"


def test_unknown_task_is_404_with_error_envelope() -> None:
    response = make_client().get("/tasks/42")

    assert response.status_code == 404
    assert response.json()["error"]["code"] == "not_found"


def test_list_tasks_returns_tasks_in_creation_order() -> None:
    client = make_client()
    for title in ("a", "b", "c"):
        client.post("/tasks", json={"title": title})

    response = client.get("/tasks")

    assert [task["title"] for task in response.json()] == ["a", "b", "c"]


def test_invalid_task_is_422_with_field_messages() -> None:
    response = make_client().post("/tasks", json={"title": "", "tags": ["UPPER"]})

    assert response.status_code == 422
    error = response.json()["error"]
    assert error["code"] == "validation_failed"
    assert {"title", "tags"} <= set(error["fields"])


class RecordingStore:
    """A hand-written fake: the app only needs the TaskStore methods."""

    def __init__(self) -> None:
        self.added: list[str] = []

    def add(self, data: TaskCreate) -> Task:
        self.added.append(data.title)
        return Task(id=7, created_at=FIXED_NOW, **data.model_dump())

    def get(self, task_id: int) -> Task | None:
        return None

    def list_all(self) -> list[Task]:
        return []


def test_app_talks_to_the_store_interface_so_a_fake_can_be_injected() -> None:
    store = RecordingStore()
    client = TestClient(create_app(store=store))

    response = client.post("/tasks", json={"title": "via fake"})

    assert response.headers["location"] == "/tasks/7"
    assert store.added == ["via fake"]
