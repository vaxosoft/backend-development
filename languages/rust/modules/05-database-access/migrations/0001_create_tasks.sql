CREATE TABLE tasks (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    title      TEXT NOT NULL CHECK (length(trim(title)) BETWEEN 1 AND 200),
    status     TEXT NOT NULL DEFAULT 'todo' CHECK (status IN ('todo', 'in_progress', 'done')),
    due_date   TEXT,
    owner_id   INTEGER,
    created_at TEXT NOT NULL
);
