"""Untrusted input: SQL injection vs bound parameters, XSS vs escaping."""

import html
import sqlite3


def make_db() -> sqlite3.Connection:
    db = sqlite3.connect(":memory:")
    db.execute("CREATE TABLE tasks (id INTEGER PRIMARY KEY, title TEXT, owner TEXT)")
    db.executemany(
        "INSERT INTO tasks (title, owner) VALUES (?, ?)",
        [("Write docs", "ana"), ("Pay rent", "bob")],
    )
    return db


def titles_vulnerable(db: sqlite3.Connection, owner: str) -> list[str]:
    sql = f"SELECT title FROM tasks WHERE owner = '{owner}'"  # the bug, on purpose
    return [row[0] for row in db.execute(sql)]


def titles_safe(db: sqlite3.Connection, owner: str) -> list[str]:
    cursor = db.execute("SELECT title FROM tasks WHERE owner = ?", (owner,))
    return [row[0] for row in cursor]


def render_title(title: str) -> str:
    return f"<li>{html.escape(title)}</li>"  # escape at the output boundary


def main() -> None:
    db = make_db()
    attack = "x' OR '1'='1"

    print(f"vulnerable, owner=ana: {titles_vulnerable(db, 'ana')}")
    print(f"vulnerable, attack:    {titles_vulnerable(db, attack)}")
    print(f"safe, attack:          {titles_safe(db, attack)}")

    print(render_title("<script>alert('x')</script>"))
    print(render_title("Fish & chips"))


if __name__ == "__main__":
    main()
