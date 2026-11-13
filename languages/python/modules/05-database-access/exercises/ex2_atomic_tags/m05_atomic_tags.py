"""Exercise 2: multi-statement changes that must be atomic."""

import sqlite3

SCHEMA = """
CREATE TABLE tasks (id INTEGER PRIMARY KEY, title TEXT NOT NULL);
CREATE TABLE task_tags (
    task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    tag TEXT NOT NULL,
    PRIMARY KEY (task_id, tag)
);
"""


def connect(path: str) -> sqlite3.Connection:
    conn = sqlite3.connect(path)
    conn.executescript(SCHEMA)
    return conn


def create_task(conn: sqlite3.Connection, title: str, tags: list[str]) -> int:
    """Insert the task and its tags in ONE transaction and return the task id.

    If any tag insert fails (e.g. a duplicate tag raises sqlite3.IntegrityError),
    the exception propagates and no task row remains.
    """
    raise NotImplementedError


def set_tags(conn: sqlite3.Connection, task_id: int, tags: list[str]) -> None:
    """Replace all tags of a task atomically.

    Raise LookupError if the task does not exist. If inserting the new tags fails,
    the old tags must still be there afterwards.
    """
    raise NotImplementedError


def get_tags(conn: sqlite3.Connection, task_id: int) -> list[str]:
    """Tags of the task, sorted alphabetically."""
    raise NotImplementedError
