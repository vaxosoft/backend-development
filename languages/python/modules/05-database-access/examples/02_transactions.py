"""Transactions: commit/rollback, atomic multi-statement changes and what other connections see."""

import sqlite3
import tempfile
from collections.abc import Iterator
from contextlib import closing, contextmanager
from pathlib import Path

SCHEMA = """
CREATE TABLE tasks (id INTEGER PRIMARY KEY, title TEXT NOT NULL);
CREATE TABLE task_tags (
    task_id INTEGER NOT NULL REFERENCES tasks(id),
    tag TEXT NOT NULL,
    PRIMARY KEY (task_id, tag)
);
"""


def count_tasks(conn: sqlite3.Connection) -> int:
    return int(conn.execute("SELECT count(*) FROM tasks").fetchone()[0])


def count_tags(conn: sqlite3.Connection) -> int:
    return int(conn.execute("SELECT count(*) FROM task_tags").fetchone()[0])


def create_task(conn: sqlite3.Connection, title: str, tags: list[str]) -> int:
    """Both inserts succeed or neither does: `with conn` commits or rolls back for us."""
    with conn:
        cur = conn.execute("INSERT INTO tasks (title) VALUES (?)", (title,))
        task_id = cur.lastrowid
        assert task_id is not None
        conn.executemany("INSERT INTO task_tags VALUES (?, ?)", [(task_id, t) for t in tags])
        return task_id


@contextmanager
def transaction(conn: sqlite3.Connection) -> Iterator[sqlite3.Connection]:
    """Explicit BEGIN/COMMIT for a connection opened with isolation_level=None (autocommit)."""
    conn.execute("BEGIN IMMEDIATE")  # take the write lock now instead of at the first write
    try:
        yield conn
    except BaseException:
        conn.execute("ROLLBACK")
        raise
    conn.execute("COMMIT")


with tempfile.TemporaryDirectory() as tmp:
    path = Path(tmp) / "tasks.db"

    with closing(sqlite3.connect(path)) as writer, closing(sqlite3.connect(path)) as reader:
        writer.executescript(SCHEMA)

        print("created", create_task(writer, "Write docs", ["docs", "p1"]))

        # Duplicate tag: the second tag insert fails and the task insert is rolled back too.
        try:
            create_task(writer, "Broken", ["a", "a"])
        except sqlite3.IntegrityError as err:
            print("IntegrityError:", err)
        print("tasks:", count_tasks(writer), "tags:", count_tags(writer))

        # Without commit, other connections do not see the change (and we hold the write lock).
        writer.execute("INSERT INTO tasks (title) VALUES ('uncommitted')")
        print("in transaction:", writer.in_transaction, "| reader sees", count_tasks(reader))
        writer.commit()
        print("after commit: reader sees", count_tasks(reader))

    # `with conn:` does NOT close the connection; closing() does.
    conn = sqlite3.connect(path, isolation_level=None)
    with closing(conn):
        try:
            with transaction(conn):
                conn.execute("INSERT INTO tasks (title) VALUES ('rolled back')")
                raise RuntimeError("boom")
        except RuntimeError as err:
            print("RuntimeError:", err, "| tasks:", count_tasks(conn))
        with transaction(conn):
            conn.execute("INSERT INTO tasks (title) VALUES ('committed')")
        print("tasks:", count_tasks(conn))
