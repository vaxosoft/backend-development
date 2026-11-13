// SQLite implementation of the `TaskRepo` port. Not runnable.
import Database from 'better-sqlite3';
import type { Clock, ListOptions, NewTask, Status, Task, TaskRepo } from './_shared.ts';

interface Row {
  id: number;
  title: string;
  status: Status;
  due_date: string | null;
  tags: string;
  owner_id: number | null;
  created_at: string;
}

const toTask = (row: Row): Task => ({ ...row, tags: JSON.parse(row.tags) as string[] });

/** Opens a fresh schema on `db` (default: a private in-memory database) and returns the repo and the handle. */
export function createSqliteRepo(
  clock: Clock = () => new Date(),
  db: Database.Database = new Database(':memory:'),
): TaskRepo & { close(): void } {
  db.exec(`
    CREATE TABLE tasks (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      title TEXT NOT NULL,
      status TEXT NOT NULL DEFAULT 'todo' CHECK (status IN ('todo', 'in_progress', 'done')),
      due_date TEXT,
      tags TEXT NOT NULL DEFAULT '[]',
      owner_id INTEGER,
      created_at TEXT NOT NULL
    )`);

  const insert = db.prepare(
    'INSERT INTO tasks (title, status, due_date, tags, owner_id, created_at) VALUES (?, ?, ?, ?, ?, ?)',
  );
  const byId = db.prepare('SELECT * FROM tasks WHERE id = ?');

  const get = (id: number): Task | undefined => {
    const row = byId.get(id) as Row | undefined;
    return row && toTask(row);
  };

  return {
    create(input: NewTask) {
      const createdAt = clock()
        .toISOString()
        .replace(/\.\d{3}Z$/, 'Z');
      const { lastInsertRowid } = insert.run(
        input.title,
        input.status ?? 'todo',
        input.due_date ?? null,
        JSON.stringify(input.tags ?? []),
        input.owner_id ?? null,
        createdAt,
      );
      const created = get(Number(lastInsertRowid));
      if (!created) throw new Error('insert did not produce a row');
      return created;
    },

    get,

    list({ status, limit = 20, offset = 0 }: ListOptions = {}) {
      const size = Math.min(Math.max(limit, 1), 100);
      const where = status === undefined ? '' : 'WHERE status = ?';
      const args = status === undefined ? [] : [status];
      const rows = db
        .prepare(`SELECT * FROM tasks ${where} ORDER BY id DESC LIMIT ? OFFSET ?`)
        .all(...args, size, offset) as Row[];
      const { total } = db.prepare(`SELECT COUNT(*) AS total FROM tasks ${where}`).get(...args) as {
        total: number;
      };
      return { items: rows.map(toTask), total };
    },

    update(id, patch) {
      const current = get(id);
      if (!current) return undefined;
      const next = {
        ...current,
        ...Object.fromEntries(Object.entries(patch).filter(([, v]) => v !== undefined)),
      };
      db.prepare('UPDATE tasks SET title = ?, status = ?, due_date = ?, tags = ? WHERE id = ?').run(
        next.title,
        next.status,
        next.due_date,
        JSON.stringify(next.tags),
        id,
      );
      return get(id);
    },

    delete(id) {
      return db.prepare('DELETE FROM tasks WHERE id = ?').run(id).changes > 0;
    },

    close() {
      db.close();
    },
  };
}
