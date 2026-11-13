import type Database from 'better-sqlite3';
import type { ListQuery, Page, Sort, Status, Task, TaskInput, TaskPatch } from './types.ts';

/** Everything the HTTP layer needs from storage; swap the implementation freely. */
export interface TaskRepository {
  create(input: TaskInput): Task;
  get(id: number): Task | undefined;
  /** Full replace; keeps `id` and `created_at`. */
  replace(id: number, input: TaskInput): Task | undefined;
  patch(id: number, patch: TaskPatch): Task | undefined;
  remove(id: number): boolean;
  list(query: ListQuery): Page<Task>;
}

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

// Column names cannot be bound as parameters, so sorting maps a validated enum to fixed SQL.
const ORDER_BY: Record<Sort, string> = {
  created_at: 'created_at ASC, id ASC',
  '-created_at': 'created_at DESC, id DESC',
  due_date: 'due_date IS NULL, due_date ASC, id ASC',
  '-due_date': 'due_date IS NULL, due_date DESC, id ASC',
};

const timestamp = (date: Date) => date.toISOString().replace(/\.\d{3}Z$/, 'Z');

export class SqliteTaskRepository implements TaskRepository {
  readonly db: Database.Database;
  readonly clock: () => Date;

  constructor(db: Database.Database, clock: () => Date) {
    this.db = db;
    this.clock = clock;
  }

  create(input: TaskInput): Task {
    const created_at = timestamp(this.clock());
    const info = this.db
      .prepare(
        `INSERT INTO tasks (title, status, due_date, tags, owner_id, created_at)
         VALUES (@title, @status, @due_date, @tags, @owner_id, @created_at)`,
      )
      .run({ ...input, tags: JSON.stringify(input.tags), created_at });
    return { id: Number(info.lastInsertRowid), ...input, created_at };
  }

  get(id: number): Task | undefined {
    const row = this.db.prepare('SELECT * FROM tasks WHERE id = ?').get(id) as Row | undefined;
    return row && toTask(row);
  }

  replace(id: number, input: TaskInput): Task | undefined {
    const info = this.db
      .prepare(
        `UPDATE tasks SET title = @title, status = @status, due_date = @due_date,
                tags = @tags, owner_id = @owner_id WHERE id = @id`,
      )
      .run({ ...input, tags: JSON.stringify(input.tags), id });
    return info.changes === 0 ? undefined : this.get(id);
  }

  patch(id: number, patch: TaskPatch): Task | undefined {
    return this.db.transaction(() => {
      const current = this.get(id);
      if (!current) return undefined;
      const { title, status, due_date, tags, owner_id } = { ...current, ...patch };
      return this.replace(id, { title, status, due_date, tags, owner_id });
    })();
  }

  remove(id: number): boolean {
    return this.db.prepare('DELETE FROM tasks WHERE id = ?').run(id).changes > 0;
  }

  list(query: ListQuery): Page<Task> {
    const conditions: string[] = [];
    const params: (string | number)[] = [];
    if (query.status) {
      conditions.push('status = ?');
      params.push(query.status);
    }
    if (query.tag) {
      conditions.push('EXISTS (SELECT 1 FROM json_each(tasks.tags) WHERE value = ?)');
      params.push(query.tag);
    }
    const where = conditions.length > 0 ? `WHERE ${conditions.join(' AND ')}` : '';

    const { total } = this.db
      .prepare(`SELECT COUNT(*) AS total FROM tasks ${where}`)
      .get(...params) as {
      total: number;
    };
    const rows = this.db
      .prepare(`SELECT * FROM tasks ${where} ORDER BY ${ORDER_BY[query.sort]} LIMIT ? OFFSET ?`)
      .all(...params, query.limit, query.offset) as Row[];
    return { items: rows.map(toTask), total };
  }
}
