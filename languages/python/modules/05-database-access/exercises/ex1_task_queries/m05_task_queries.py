"""Exercise 1: parameterised queries and row mapping."""

import sqlite3
from dataclasses import dataclass
from datetime import date

SCHEMA = """
CREATE TABLE tasks (
    id INTEGER PRIMARY KEY,
    title TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'todo',
    due_date TEXT
);
"""


@dataclass(frozen=True, slots=True)
class Task:
    id: int
    title: str
    status: str
    due_date: date | None


def add_task(
    conn: sqlite3.Connection, title: str, status: str = "todo", due_date: date | None = None
) -> int:
    """Insert a task, COMMIT, and return its new id. `due_date` is stored as ISO text or NULL."""
    raise NotImplementedError


def get_task(conn: sqlite3.Connection, task_id: int) -> Task | None:
    """Return the task, or None if there is no such id. NULL due_date becomes None."""
    raise NotImplementedError


def search_tasks(conn: sqlite3.Connection, term: str, status: str | None = None) -> list[Task]:
    """Tasks whose title contains `term` (case-insensitive), optionally with this status, by id.

    `term` is user input: bind it as a parameter. A literal `%` or `_` in it must match
    only itself (use LIKE ... ESCAPE).
    """
    raise NotImplementedError
