"""sqlite3 and the DB-API: connect, execute with bound parameters, fetch, map rows."""

import sqlite3

conn = sqlite3.connect(":memory:")
conn.row_factory = sqlite3.Row  # rows behave like read-only mappings: row["title"]

conn.execute(
    """CREATE TABLE tasks (
        id INTEGER PRIMARY KEY,
        title TEXT NOT NULL,
        status TEXT NOT NULL DEFAULT 'todo' CHECK (status IN ('todo', 'in_progress', 'done')),
        due_date TEXT
    )"""
)

cur = conn.execute(
    "INSERT INTO tasks (title, due_date) VALUES (?, ?)", ("Write docs", "2030-01-31")
)
print("new id:", cur.lastrowid)
conn.executemany(
    "INSERT INTO tasks (title, status) VALUES (:title, :status)",
    [{"title": "Review PR", "status": "in_progress"}, {"title": "Deploy", "status": "done"}],
)

# NULL columns arrive as None.
for row in conn.execute("SELECT id, title, status, due_date FROM tasks ORDER BY id"):
    print(row["id"], row["title"], row["status"], row["due_date"])

print(conn.execute("SELECT count(*) FROM tasks WHERE status = ?", ("done",)).fetchone()[0])
print(dict(conn.execute("SELECT * FROM tasks WHERE id = ?", (1,)).fetchone()))
print(conn.execute("SELECT * FROM tasks WHERE id = ?", (99,)).fetchone())  # no row -> None

# Parameters are sent separately from the SQL text, so hostile input stays data.
hostile = "x'; DROP TABLE tasks; --"
conn.execute("INSERT INTO tasks (title) VALUES (?)", (hostile,))
found = conn.execute("SELECT title FROM tasks WHERE title = ?", (hostile,)).fetchone()
print("stored literally:", found["title"])
print("table still there:", conn.execute("SELECT count(*) FROM tasks").fetchone()[0])

# Placeholders bind values, never identifiers: validate those against an allow-list.
try:
    conn.execute("SELECT * FROM ? ", ("tasks",))
except sqlite3.OperationalError as err:
    print("OperationalError:", err)

# The CHECK constraint turns bad data into an exception.
try:
    conn.execute("INSERT INTO tasks (title, status) VALUES (?, ?)", ("Bad", "blocked"))
except sqlite3.IntegrityError as err:
    print("IntegrityError:", err)

conn.close()
