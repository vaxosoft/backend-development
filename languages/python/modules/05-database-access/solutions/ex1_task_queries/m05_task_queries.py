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
    with conn:
        cur = conn.execute(
            "INSERT INTO tasks (title, status, due_date) VALUES (?, ?, ?)",
            (title, status, due_date.isoformat() if due_date else None),
        )
    assert cur.lastrowid is not None
    return cur.lastrowid


def _to_task(row: tuple[int, str, str, str | None]) -> Task:
    task_id, title, status, due = row
    return Task(task_id, title, status, date.fromisoformat(due) if due else None)


def get_task(conn: sqlite3.Connection, task_id: int) -> Task | None:
    row = conn.execute(
        "SELECT id, title, status, due_date FROM tasks WHERE id = ?", (task_id,)
    ).fetchone()
    return _to_task(row) if row else None


def search_tasks(conn: sqlite3.Connection, term: str, status: str | None = None) -> list[Task]:
    escaped = term.replace("\\", "\\\\").replace("%", "\\%").replace("_", "\\_")
    sql = "SELECT id, title, status, due_date FROM tasks WHERE title LIKE ? ESCAPE '\\'"
    params: list[object] = [f"%{escaped}%"]
    if status is not None:
        sql += " AND status = ?"
        params.append(status)
    return [_to_task(row) for row in conn.execute(sql + " ORDER BY id", params)]
