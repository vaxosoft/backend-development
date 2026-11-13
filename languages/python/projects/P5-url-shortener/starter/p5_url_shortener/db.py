import sqlite3

SCHEMA = """
CREATE TABLE IF NOT EXISTS users (
    id            INTEGER PRIMARY KEY,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS links (
    slug       TEXT PRIMARY KEY,
    url        TEXT NOT NULL,
    owner_id   INTEGER NOT NULL REFERENCES users(id),
    hits       INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS links_owner ON links(owner_id);
"""


def connect(path: str) -> sqlite3.Connection:
    """Open the database and create the schema.

    One connection is shared by all request threads (FastAPI runs sync endpoints in a
    thread pool), so `Store` serialises access with a lock.
    """
    conn = sqlite3.connect(path, check_same_thread=False)
    conn.row_factory = sqlite3.Row
    conn.execute("PRAGMA foreign_keys = ON")
    conn.executescript(SCHEMA)
    return conn
