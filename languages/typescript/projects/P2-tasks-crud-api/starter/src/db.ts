import Database from 'better-sqlite3';

/** Opens the database and creates the schema when it does not exist yet. */
export function openDatabase(path: string): Database.Database {
  const db = new Database(path);
  db.exec(`
    CREATE TABLE IF NOT EXISTS tasks (
      id         INTEGER PRIMARY KEY AUTOINCREMENT,
      title      TEXT NOT NULL,
      status     TEXT NOT NULL CHECK (status IN ('todo', 'in_progress', 'done')),
      due_date   TEXT,
      tags       TEXT NOT NULL DEFAULT '[]', -- JSON array of strings
      owner_id   INTEGER,
      created_at TEXT NOT NULL
    );
    CREATE INDEX IF NOT EXISTS tasks_status ON tasks (status);
  `);
  return db;
}
