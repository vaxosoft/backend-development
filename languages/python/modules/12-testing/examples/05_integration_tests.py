"""Contract tests run against every repository implementation, then an end-to-end test on SQLite.

Run: pytest modules/12-testing/examples/05_integration_tests.py -vv --no-header
"""

from collections.abc import Iterator
from datetime import UTC, datetime
from pathlib import Path

import pytest
from fastapi.testclient import TestClient
from m12_app import create_app, get_service
from m12_tasks import InMemoryTaskRepository, SqliteTaskRepository, TaskRepository, TaskService

NOON = datetime(2030, 1, 1, 12, 0, tzinfo=UTC)


@pytest.fixture(params=["memory", "sqlite-memory", "sqlite-file"])
def repo(request: pytest.FixtureRequest, tmp_path: Path) -> Iterator[TaskRepository]:
    """Every test below runs once per implementation: that is the contract."""
    if request.param == "memory":
        yield InMemoryTaskRepository()
        return
    path = ":memory:" if request.param == "sqlite-memory" else str(tmp_path / "tasks.db")
    sqlite_repo = SqliteTaskRepository(path)
    yield sqlite_repo
    sqlite_repo.close()


def test_ids_are_assigned_in_order(repo: TaskRepository) -> None:
    first = repo.add("one", (), NOON)
    second = repo.add("two", (), NOON)

    assert (first.id, second.id) == (1, 2)
    assert first.status == "todo"


def test_get_round_trips_every_field(repo: TaskRepository) -> None:
    created = repo.add("Write docs", ("docs", "p1"), NOON)

    assert repo.get(created.id) == created
    assert created.tags == ("docs", "p1")
    assert created.created_at == NOON


def test_a_task_without_tags_has_an_empty_tuple(repo: TaskRepository) -> None:
    assert repo.add("plain", (), NOON).tags == ()


def test_missing_task_is_none(repo: TaskRepository) -> None:
    assert repo.get(404) is None
    assert repo.set_status(404, "done") is None


def test_list_returns_tasks_in_creation_order(repo: TaskRepository) -> None:
    for title in ("b", "a", "c"):
        repo.add(title, (), NOON)

    assert [t.title for t in repo.list_all()] == ["b", "a", "c"]


def test_set_status_persists(repo: TaskRepository) -> None:
    task = repo.add("ship", (), NOON)

    updated = repo.set_status(task.id, "done")

    assert updated is not None
    assert updated.status == "done"
    assert repo.get(task.id) == updated


def test_http_to_sqlite_file_and_back(tmp_path: Path) -> None:
    db_path = str(tmp_path / "e2e.db")
    writer = SqliteTaskRepository(db_path)
    service = TaskService(writer, clock=lambda: NOON)
    app = create_app()
    app.dependency_overrides[get_service] = lambda: service
    client = TestClient(app)

    try:
        assert (
            client.post("/tasks", json={"title": "persist me", "tags": ["db"]}).status_code == 201
        )
    finally:
        writer.close()

    reopened = SqliteTaskRepository(db_path)  # a new connection sees the committed row
    try:
        assert [t.title for t in reopened.list_all()] == ["persist me"]
    finally:
        reopened.close()


if __name__ == "__main__":
    raise SystemExit(pytest.main([__file__, "-vv", "--no-header"]))
