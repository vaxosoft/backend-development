import sqlite3
from collections.abc import Iterator
from contextlib import closing
from datetime import date
from pathlib import Path

import pytest
from m05_task_queries import SCHEMA, Task, add_task, get_task, search_tasks


@pytest.fixture
def conn() -> Iterator[sqlite3.Connection]:
    with closing(sqlite3.connect(":memory:")) as c:
        c.executescript(SCHEMA)
        yield c


def test_add_and_get_round_trip(conn: sqlite3.Connection) -> None:
    task_id = add_task(conn, "Write docs", "in_progress", date(2030, 1, 31))
    assert get_task(conn, task_id) == Task(task_id, "Write docs", "in_progress", date(2030, 1, 31))


def test_defaults_and_null_due_date(conn: sqlite3.Connection) -> None:
    task_id = add_task(conn, "No date")
    task = get_task(conn, task_id)
    assert task == Task(task_id, "No date", "todo", None)
    assert conn.execute("SELECT due_date FROM tasks").fetchone() == (None,)


def test_unknown_id_is_none(conn: sqlite3.Connection) -> None:
    assert get_task(conn, 404) is None


def test_add_task_commits(tmp_path: Path) -> None:
    path = tmp_path / "tasks.db"
    with closing(sqlite3.connect(path)) as writer, closing(sqlite3.connect(path)) as reader:
        writer.executescript(SCHEMA)
        task_id = add_task(writer, "Visible to others")
        assert get_task(reader, task_id) is not None


def test_search_is_case_insensitive_and_ordered(conn: sqlite3.Connection) -> None:
    a = add_task(conn, "Write DOCS")
    add_task(conn, "Deploy")
    b = add_task(conn, "update docs index", "done")
    assert [t.id for t in search_tasks(conn, "docs")] == [a, b]
    assert [t.id for t in search_tasks(conn, "docs", status="done")] == [b]
    assert search_tasks(conn, "nothing") == []


def test_search_treats_input_as_data(conn: sqlite3.Connection) -> None:
    add_task(conn, "keep me")
    hostile = "x' OR '1'='1"
    assert search_tasks(conn, hostile) == []
    assert search_tasks(conn, "'; DROP TABLE tasks; --") == []
    assert get_task(conn, 1) is not None


def test_like_wildcards_in_the_term_match_literally(conn: sqlite3.Connection) -> None:
    percent = add_task(conn, "100% done")
    underscore = add_task(conn, "snake_case")
    add_task(conn, "plain text")
    assert [t.id for t in search_tasks(conn, "%")] == [percent]
    assert [t.id for t in search_tasks(conn, "_")] == [underscore]
