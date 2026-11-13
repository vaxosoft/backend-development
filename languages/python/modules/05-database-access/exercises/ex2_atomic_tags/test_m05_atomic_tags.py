import sqlite3
from collections.abc import Iterator
from contextlib import closing
from pathlib import Path

import pytest
from m05_atomic_tags import connect, create_task, get_tags, set_tags


@pytest.fixture
def db_path(tmp_path: Path) -> str:
    return str(tmp_path / "tasks.db")


@pytest.fixture
def conn(db_path: str) -> Iterator[sqlite3.Connection]:
    with closing(connect(db_path)) as c:
        yield c


def count(conn: sqlite3.Connection, table: str) -> int:
    queries = {
        "tasks": "SELECT count(*) FROM tasks",
        "task_tags": "SELECT count(*) FROM task_tags",
    }
    return int(conn.execute(queries[table]).fetchone()[0])


def test_create_task_stores_task_and_tags(conn: sqlite3.Connection) -> None:
    task_id = create_task(conn, "Write docs", ["p1", "docs"])
    assert get_tags(conn, task_id) == ["docs", "p1"]
    assert conn.execute("SELECT title FROM tasks WHERE id = ?", (task_id,)).fetchone() == (
        "Write docs",
    )


def test_create_task_is_committed(conn: sqlite3.Connection, db_path: str) -> None:
    task_id = create_task(conn, "Visible", ["a"])
    with closing(sqlite3.connect(db_path)) as other:
        assert get_tags(other, task_id) == ["a"]


def test_create_task_rolls_back_everything_on_a_duplicate_tag(conn: sqlite3.Connection) -> None:
    with pytest.raises(sqlite3.IntegrityError):
        create_task(conn, "Broken", ["a", "b", "a"])
    assert count(conn, "tasks") == 0
    assert count(conn, "task_tags") == 0
    assert not conn.in_transaction


def test_connection_is_usable_after_a_failure(conn: sqlite3.Connection) -> None:
    with pytest.raises(sqlite3.IntegrityError):
        create_task(conn, "Broken", ["x", "x"])
    task_id = create_task(conn, "Fine", ["x"])
    assert get_tags(conn, task_id) == ["x"]


def test_set_tags_replaces_the_old_ones(conn: sqlite3.Connection) -> None:
    task_id = create_task(conn, "t", ["old1", "old2"])
    set_tags(conn, task_id, ["new"])
    assert get_tags(conn, task_id) == ["new"]
    set_tags(conn, task_id, [])
    assert get_tags(conn, task_id) == []


def test_set_tags_keeps_old_tags_when_the_new_ones_are_invalid(conn: sqlite3.Connection) -> None:
    task_id = create_task(conn, "t", ["keep"])
    with pytest.raises(sqlite3.IntegrityError):
        set_tags(conn, task_id, ["dup", "dup"])
    assert get_tags(conn, task_id) == ["keep"]


def test_set_tags_on_missing_task(conn: sqlite3.Connection) -> None:
    with pytest.raises(LookupError):
        set_tags(conn, 404, ["a"])
    assert count(conn, "task_tags") == 0
