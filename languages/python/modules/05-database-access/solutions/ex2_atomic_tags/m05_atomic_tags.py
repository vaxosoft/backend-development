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
    with conn:  # commits on success, rolls back on any exception
        cur = conn.execute("INSERT INTO tasks (title) VALUES (?)", (title,))
        task_id = cur.lastrowid
        assert task_id is not None
        conn.executemany("INSERT INTO task_tags VALUES (?, ?)", [(task_id, t) for t in tags])
    return task_id


def set_tags(conn: sqlite3.Connection, task_id: int, tags: list[str]) -> None:
    with conn:
        if conn.execute("SELECT 1 FROM tasks WHERE id = ?", (task_id,)).fetchone() is None:
            raise LookupError(f"no task {task_id}")
        conn.execute("DELETE FROM task_tags WHERE task_id = ?", (task_id,))
        conn.executemany("INSERT INTO task_tags VALUES (?, ?)", [(task_id, t) for t in tags])


def get_tags(conn: sqlite3.Connection, task_id: int) -> list[str]:
    rows = conn.execute("SELECT tag FROM task_tags WHERE task_id = ? ORDER BY tag", (task_id,))
    return [row[0] for row in rows]
