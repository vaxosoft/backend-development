import itertools
from collections.abc import Callable
from datetime import UTC, datetime, timedelta
from pathlib import Path
from typing import Any

import pytest
from fastapi.testclient import TestClient
from p2_tasks_crud_api.app import Config, create_app

Json = dict[str, Any]
START = datetime(2030, 1, 1, 9, 0, 0, tzinfo=UTC)


def ticking_clock() -> Callable[[], datetime]:
    """A clock that moves one second forward on every call, so created_at is predictable."""
    ticks = itertools.count()
    return lambda: START + timedelta(seconds=next(ticks))


@pytest.fixture
def client() -> TestClient:
    return TestClient(create_app(Config(clock=ticking_clock())))


def make(client: TestClient, **fields: Any) -> Json:
    body = {"title": "Write docs", **fields}
    response = client.post("/tasks", json=body)
    assert response.status_code == 201, response.text
    result: Json = response.json()
    return result


def assert_error(response: Any, status: int, code: str) -> Json:
    assert response.status_code == status, response.text
    assert response.headers["content-type"].startswith("application/json")
    error: Json = response.json()["error"]
    assert error["code"] == code
    assert error["message"]
    return error


# --- create ---


def test_create_returns_201_location_and_defaults(client: TestClient) -> None:
    r = client.post("/tasks", json={"title": "Write docs"})
    assert r.status_code == 201
    task = r.json()
    assert r.headers["location"] == f"/tasks/{task['id']}"
    assert task == {
        "id": task["id"],
        "title": "Write docs",
        "status": "todo",
        "due_date": None,
        "tags": [],
        "owner_id": None,
        "created_at": "2030-01-01T09:00:00Z",
    }


def test_create_stores_every_field_and_trims_the_title(client: TestClient) -> None:
    body = {
        "title": "  Ship it ",
        "status": "in_progress",
        "due_date": "2030-01-31",
        "tags": ["docs", "p2"],
        "owner_id": 7,
    }
    task = make(client, **body)
    assert task["title"] == "Ship it"
    assert (task["status"], task["due_date"], task["tags"], task["owner_id"]) == (
        "in_progress",
        "2030-01-31",
        ["docs", "p2"],
        7,
    )


def test_create_assigns_distinct_ids(client: TestClient) -> None:
    assert make(client)["id"] != make(client)["id"]


@pytest.mark.parametrize(
    ("body", "field"),
    [
        ({}, "title"),
        ({"title": "   "}, "title"),
        ({"title": "x" * 201}, "title"),
        ({"title": 42}, "title"),
        ({"title": "ok", "status": "finished"}, "status"),
        ({"title": "ok", "due_date": "31-01-2030"}, "due_date"),
        ({"title": "ok", "due_date": "2030-02-30"}, "due_date"),
        ({"title": "ok", "due_date": "20300131"}, "due_date"),
        ({"title": "ok", "tags": [f"t{i}" for i in range(11)]}, "tags"),
        ({"title": "ok", "tags": ["Docs"]}, "tags"),
        ({"title": "ok", "tags": [""]}, "tags"),
        ({"title": "ok", "tags": ["x" * 31]}, "tags"),
        ({"title": "ok", "tags": ["a", "a"]}, "tags"),
        ({"title": "ok", "owner_id": "seven"}, "owner_id"),
    ],
)
def test_create_rejects_invalid_data_with_422_and_the_field_name(
    client: TestClient, body: Json, field: str
) -> None:
    error = assert_error(client.post("/tasks", json=body), 422, "validation_failed")
    assert field in error["fields"]
    assert error["fields"][field]


def test_create_accepts_the_limits(client: TestClient) -> None:
    tags = [f"t{i}" for i in range(9)] + ["x" * 30]
    task = make(client, title="t" * 200, tags=tags)
    assert len(task["title"]) == 200
    assert task["tags"] == tags


def test_malformed_json_is_400_bad_request(client: TestClient) -> None:
    r = client.post("/tasks", content=b'{"title": ', headers={"content-type": "application/json"})
    assert_error(r, 400, "bad_request")


# --- read ---


def test_get_returns_the_task(client: TestClient) -> None:
    created = make(client, tags=["docs"])
    r = client.get(f"/tasks/{created['id']}")
    assert r.status_code == 200
    assert r.json() == created


def test_unknown_or_non_numeric_id_is_404(client: TestClient) -> None:
    assert_error(client.get("/tasks/999"), 404, "not_found")
    assert_error(client.get("/tasks/abc"), 404, "not_found")


# --- replace ---


def test_put_replaces_everything_but_id_and_created_at(client: TestClient) -> None:
    created = make(client, status="done", due_date="2030-05-05", tags=["old"], owner_id=3)
    r = client.put(f"/tasks/{created['id']}", json={"title": "New title"})
    assert r.status_code == 200
    assert r.json() == {
        "id": created["id"],
        "title": "New title",
        "status": "todo",
        "due_date": None,
        "tags": [],
        "owner_id": None,
        "created_at": created["created_at"],
    }
    assert client.get(f"/tasks/{created['id']}").json() == r.json()


def test_put_validates_and_404s_on_unknown_id(client: TestClient) -> None:
    created = make(client)
    assert_error(
        client.put(f"/tasks/{created['id']}", json={"title": ""}), 422, "validation_failed"
    )
    assert_error(client.put("/tasks/999", json={"title": "x"}), 404, "not_found")
    assert client.get(f"/tasks/{created['id']}").json() == created


# --- patch ---


def test_patch_changes_only_the_given_fields(client: TestClient) -> None:
    created = make(client, due_date="2030-05-05", tags=["a"], owner_id=3)
    r = client.patch(f"/tasks/{created['id']}", json={"status": "done"})
    assert r.status_code == 200
    assert r.json() == {**created, "status": "done"}


def test_patch_null_due_date_clears_it(client: TestClient) -> None:
    created = make(client, due_date="2030-05-05")
    r = client.patch(f"/tasks/{created['id']}", json={"due_date": None})
    assert r.json()["due_date"] is None


def test_patch_updates_title_and_tags_and_ignores_read_only_fields(client: TestClient) -> None:
    created = make(client, owner_id=3)
    r = client.patch(
        f"/tasks/{created['id']}",
        json={"title": " Renamed ", "tags": ["x"], "id": 99, "created_at": "1999-01-01T00:00:00Z"},
    )
    assert r.json() == {**created, "title": "Renamed", "tags": ["x"]}


@pytest.mark.parametrize(
    "body",
    [{"title": ""}, {"title": None}, {"status": "nope"}, {"tags": None}, {"due_date": "tomorrow"}],
)
def test_patch_validates_the_result(client: TestClient, body: Json) -> None:
    created = make(client)
    error = assert_error(
        client.patch(f"/tasks/{created['id']}", json=body), 422, "validation_failed"
    )
    assert next(iter(body)) in error["fields"]
    assert client.get(f"/tasks/{created['id']}").json() == created


def test_patch_unknown_id_is_404(client: TestClient) -> None:
    assert_error(client.patch("/tasks/999", json={"title": "x"}), 404, "not_found")


# --- delete ---


def test_delete_returns_204_and_the_task_is_gone(client: TestClient) -> None:
    created = make(client)
    r = client.delete(f"/tasks/{created['id']}")
    assert r.status_code == 204
    assert r.content == b""
    assert_error(client.get(f"/tasks/{created['id']}"), 404, "not_found")


def test_delete_unknown_id_is_404(client: TestClient) -> None:
    assert_error(client.delete("/tasks/999"), 404, "not_found")


# --- list ---


def titles(response: Any) -> list[str]:
    assert response.status_code == 200, response.text
    return [item["title"] for item in response.json()["items"]]


def test_list_returns_the_envelope_with_defaults(client: TestClient) -> None:
    make(client, title="a")
    body = client.get("/tasks").json()
    assert (body["total"], body["limit"], body["offset"]) == (1, 20, 0)
    assert [item["title"] for item in body["items"]] == ["a"]
    assert client.get("/tasks?status=done").json() == {
        "items": [],
        "total": 0,
        "limit": 20,
        "offset": 0,
    }


def test_list_defaults_to_newest_first_and_sorts_by_created_at(client: TestClient) -> None:
    for title in ("first", "second", "third"):
        make(client, title=title)
    assert titles(client.get("/tasks")) == ["third", "second", "first"]
    assert titles(client.get("/tasks?sort=created_at")) == ["first", "second", "third"]
    assert titles(client.get("/tasks?sort=-created_at")) == ["third", "second", "first"]


def test_list_sorts_by_due_date_with_missing_dates_last(client: TestClient) -> None:
    make(client, title="none")
    make(client, title="late", due_date="2030-09-01")
    make(client, title="early", due_date="2030-02-01")
    assert titles(client.get("/tasks?sort=due_date")) == ["early", "late", "none"]
    assert titles(client.get("/tasks?sort=-due_date")) == ["late", "early", "none"]


def test_list_filters_by_status_and_tag(client: TestClient) -> None:
    make(client, title="a", status="done", tags=["docs", "p2"])
    make(client, title="b", status="todo", tags=["docs"])
    make(client, title="c", status="done", tags=["code"])
    assert titles(client.get("/tasks?status=done&sort=created_at")) == ["a", "c"]
    assert titles(client.get("/tasks?tag=docs&sort=created_at")) == ["a", "b"]
    assert titles(client.get("/tasks?tag=docs&status=done")) == ["a"]
    assert titles(client.get("/tasks?tag=doc")) == []


def test_list_paginates_and_reports_the_total_of_all_matches(client: TestClient) -> None:
    for n in range(5):
        make(client, title=f"t{n}")
    page = client.get("/tasks?sort=created_at&limit=2&offset=2").json()
    assert [item["title"] for item in page["items"]] == ["t2", "t3"]
    assert (page["total"], page["limit"], page["offset"]) == (5, 2, 2)
    assert titles(client.get("/tasks?sort=created_at&limit=2&offset=4")) == ["t4"]


def test_list_clamps_limit_to_100(client: TestClient) -> None:
    assert client.get("/tasks?limit=1000").json()["limit"] == 100


@pytest.mark.parametrize(
    "query", ["limit=abc", "limit=0", "offset=-1", "status=nope", "sort=title", "sort=; DROP"]
)
def test_list_rejects_bad_query_values_with_400(client: TestClient, query: str) -> None:
    assert_error(client.get(f"/tasks?{query}"), 400, "bad_request")


# --- persistence ---


def test_tasks_survive_a_restart_on_the_same_database_file(tmp_path: Path) -> None:
    db = str(tmp_path / "tasks.db")
    with TestClient(create_app(Config(db_path=db))) as first:
        created = make(first, title="persist me", tags=["keep"])
    with TestClient(create_app(Config(db_path=db))) as second:
        assert second.get(f"/tasks/{created['id']}").json() == created
