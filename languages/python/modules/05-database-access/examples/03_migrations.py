"""Schema migrations with PRAGMA user_version, plus the per-connection pragmas to set."""

import sqlite3
from collections.abc import Iterator
from contextlib import closing, contextmanager

MIGRATIONS: list[tuple[str, ...]] = [
    # version 1
    (
        """CREATE TABLE tasks (
            id INTEGER PRIMARY KEY,
            title TEXT NOT NULL,
            status TEXT NOT NULL DEFAULT 'todo'
        )""",
    ),
    # version 2
    (
        "ALTER TABLE tasks ADD COLUMN due_date TEXT",
        """CREATE TABLE task_tags (
            task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
            tag TEXT NOT NULL,
            PRIMARY KEY (task_id, tag)
        )""",
    ),
]


def open_db(path: str) -> sqlite3.Connection:
    # isolation_level=None: we issue BEGIN/COMMIT ourselves, so DDL is transactional too.
    conn = sqlite3.connect(path, isolation_level=None)
    conn.execute("PRAGMA foreign_keys = ON")  # off by default and per connection!
    return conn


@contextmanager
def transaction(conn: sqlite3.Connection) -> Iterator[None]:
    conn.execute("BEGIN IMMEDIATE")
    try:
        yield
    except BaseException:
        conn.execute("ROLLBACK")
        raise
    conn.execute("COMMIT")


def schema_version(conn: sqlite3.Connection) -> int:
    return int(conn.execute("PRAGMA user_version").fetchone()[0])


def migrate(conn: sqlite3.Connection) -> None:
    for version, statements in enumerate(MIGRATIONS, start=1):
        if version <= schema_version(conn):
            continue
        with transaction(conn):
            for statement in statements:
                conn.execute(statement)
            conn.execute(f"PRAGMA user_version = {version}")  # PRAGMA takes no bound parameters


with closing(open_db(":memory:")) as conn:
    print("before:", schema_version(conn))
    migrate(conn)
    print("after:", schema_version(conn))
    migrate(conn)  # safe to run on every start-up
    print("again:", schema_version(conn))
    print([r[1] for r in conn.execute("PRAGMA table_info(tasks)")])

    conn.execute("INSERT INTO tasks (title) VALUES ('Write docs')")
    conn.execute("INSERT INTO task_tags VALUES (1, 'docs')")
    try:
        conn.execute("INSERT INTO task_tags VALUES (42, 'orphan')")
    except sqlite3.IntegrityError as err:
        print("IntegrityError:", err)
    conn.execute("DELETE FROM tasks WHERE id = 1")  # cascades to task_tags
    print("tags left:", conn.execute("SELECT count(*) FROM task_tags").fetchone()[0])
