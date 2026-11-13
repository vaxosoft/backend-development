"""The store contract: every implementation must pass the same tests."""

import pytest
from m12_store import InMemoryTaskStore, SqliteTaskStore, TaskStore


@pytest.fixture(params=[InMemoryTaskStore, SqliteTaskStore], ids=["memory", "sqlite"])
def store(request: pytest.FixtureRequest) -> TaskStore:
    factory: type[TaskStore] = request.param
    return factory()


def test_add_assigns_sequential_ids_and_defaults(store: TaskStore) -> None:
    first = store.add("Write docs", ["docs", "p1"])
    second = store.add("Ship it")

    assert (first.id, second.id) == (1, 2)
    assert first.status == "todo"
    assert first.tags == ("docs", "p1")
    assert second.tags == ()


def test_get(store: TaskStore) -> None:
    task = store.add("Write docs")

    assert store.get(task.id) == task
    assert store.get(999) is None


def test_list_is_ordered_by_id_and_paginated(store: TaskStore) -> None:
    for n in range(1, 6):
        store.add(f"task {n}")

    assert [t.id for t in store.list_tasks()] == [1, 2, 3, 4, 5]
    assert [t.id for t in store.list_tasks(limit=2)] == [1, 2]
    assert [t.id for t in store.list_tasks(limit=2, offset=3)] == [4, 5]
    assert store.list_tasks(offset=10) == []


def test_list_filters_by_status(store: TaskStore) -> None:
    done = store.add("done one")
    store.add("still todo")
    store.set_status(done.id, "done")

    assert [t.title for t in store.list_tasks(status="done")] == ["done one"]
    assert [t.title for t in store.list_tasks(status="todo")] == ["still todo"]
    assert store.list_tasks(status="in_progress") == []


def test_list_returns_a_new_list_each_time(store: TaskStore) -> None:
    store.add("one")

    store.list_tasks().clear()

    assert len(store.list_tasks()) == 1


def test_set_status_updates_and_persists(store: TaskStore) -> None:
    task = store.add("ship")

    updated = store.set_status(task.id, "in_progress")

    assert updated is not None
    assert updated.status == "in_progress"
    assert updated.title == "ship"
    assert store.get(task.id) == updated


def test_set_status_on_an_unknown_id_is_none(store: TaskStore) -> None:
    assert store.set_status(999, "done") is None


@pytest.mark.parametrize("bad", ["", "DONE", "finished"])
def test_invalid_statuses_are_rejected_everywhere(store: TaskStore, bad: str) -> None:
    task = store.add("x")

    with pytest.raises(ValueError, match="invalid status"):
        store.set_status(task.id, bad)
    with pytest.raises(ValueError, match="invalid status"):
        store.set_status(999, bad)
    with pytest.raises(ValueError, match="invalid status"):
        store.list_tasks(status=bad)


def test_delete_reports_whether_something_was_removed(store: TaskStore) -> None:
    task = store.add("temp")

    assert store.delete(task.id) is True
    assert store.delete(task.id) is False
    assert store.get(task.id) is None


def test_ids_are_never_reused_after_a_delete(store: TaskStore) -> None:
    store.add("one")
    second = store.add("two")
    store.delete(second.id)

    assert store.add("three").id == 3
