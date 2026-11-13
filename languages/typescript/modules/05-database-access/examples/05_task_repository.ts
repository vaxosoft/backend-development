// The repository pattern in the Task Tracker domain: SQL stays behind one small interface.
import Database from 'better-sqlite3';

type Status = 'todo' | 'in_progress' | 'done';

interface Task {
  id: number;
  title: string;
  status: Status;
  due_date: string | null;
  tags: string[];
  owner_id: number | null;
  created_at: string;
}

type NewTask = Pick<Task, 'title'> &
  Partial<Pick<Task, 'status' | 'due_date' | 'tags' | 'owner_id'>>;
type TaskPatch = Partial<Pick<Task, 'title' | 'status' | 'due_date' | 'tags'>>;

const SORTS = {
  created_at: 'created_at ASC',
  '-created_at': 'created_at DESC',
  due_date: 'due_date IS NULL, due_date ASC',
  '-due_date': 'due_date IS NULL, due_date DESC',
} as const;

interface ListQuery {
  status?: Status;
  tag?: string;
  sort?: keyof typeof SORTS;
  limit?: number;
  offset?: number;
}

interface Page<T> {
  items: T[];
  total: number;
  limit: number;
  offset: number;
}

interface TaskRepository {
  create(input: NewTask): Task;
  get(id: number): Task | null;
  list(query: ListQuery): Page<Task>;
  update(id: number, patch: TaskPatch): Task | null;
  delete(id: number): boolean;
}

/** What the driver returns: snake_case columns, NULL as null, no tags (they live in another table). */
type TaskDbRow = Omit<Task, 'tags'>;

const SCHEMA = `
  CREATE TABLE tasks (
    id INTEGER PRIMARY KEY,
    title TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'todo' CHECK (status IN ('todo','in_progress','done')),
    due_date TEXT,
    owner_id INTEGER,
    created_at TEXT NOT NULL
  );
  CREATE TABLE task_tags (
    task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    tag TEXT NOT NULL,
    PRIMARY KEY (task_id, tag)
  );
`;

function rfc3339(date: Date): string {
  return date.toISOString().replace('.000Z', 'Z');
}

class SqliteTaskRepository implements TaskRepository {
  readonly #db: Database.Database;
  readonly #now: () => Date;

  constructor(db: Database.Database, now: () => Date) {
    this.#db = db;
    this.#now = now;
    db.pragma('foreign_keys = ON');
  }

  create(input: NewTask): Task {
    return this.#db.transaction(() => {
      const { lastInsertRowid } = this.#db
        .prepare(
          `INSERT INTO tasks (title, status, due_date, owner_id, created_at)
           VALUES (@title, @status, @due_date, @owner_id, @created_at)`,
        )
        .run({
          title: input.title,
          status: input.status ?? 'todo',
          due_date: input.due_date ?? null,
          owner_id: input.owner_id ?? null,
          created_at: rfc3339(this.#now()),
        });
      const id = Number(lastInsertRowid);
      this.#writeTags(id, input.tags ?? []);
      return this.#mustGet(id);
    })();
  }

  get(id: number): Task | null {
    const row = this.#db.prepare<[number], TaskDbRow>('SELECT * FROM tasks WHERE id = ?').get(id);
    return row ? { ...row, tags: this.#tagsFor([id]).get(id) ?? [] } : null;
  }

  list(query: ListQuery): Page<Task> {
    const limit = Math.min(Math.max(query.limit ?? 20, 1), 100);
    const offset = Math.max(query.offset ?? 0, 0);
    const where: string[] = [];
    const params: unknown[] = [];
    if (query.status) {
      where.push('status = ?');
      params.push(query.status);
    }
    if (query.tag) {
      where.push('EXISTS (SELECT 1 FROM task_tags tt WHERE tt.task_id = tasks.id AND tt.tag = ?)');
      params.push(query.tag);
    }
    const clause = where.length > 0 ? `WHERE ${where.join(' AND ')}` : '';
    // Only the column list comes from a whitelist object; user values are always bound.
    const order = SORTS[query.sort ?? '-created_at'];

    const total = this.#db
      .prepare(`SELECT count(*) FROM tasks ${clause}`)
      .pluck()
      .get(...params) as number;
    const rows = this.#db
      .prepare<unknown[], TaskDbRow>(
        `SELECT * FROM tasks ${clause} ORDER BY ${order}, id LIMIT ? OFFSET ?`,
      )
      .all(...params, limit, offset);

    const tags = this.#tagsFor(rows.map((r) => r.id)); // one query for the whole page, not one per task
    return { items: rows.map((r) => ({ ...r, tags: tags.get(r.id) ?? [] })), total, limit, offset };
  }

  update(id: number, patch: TaskPatch): Task | null {
    return this.#db.transaction(() => {
      const current = this.get(id);
      if (!current) return null;
      const next = { ...current, ...patch }; // absent keys keep their value, due_date: null clears it
      this.#db
        .prepare('UPDATE tasks SET title = ?, status = ?, due_date = ? WHERE id = ?')
        .run(next.title, next.status, next.due_date, id);
      if (patch.tags) this.#writeTags(id, patch.tags);
      return this.#mustGet(id);
    })();
  }

  delete(id: number): boolean {
    return this.#db.prepare('DELETE FROM tasks WHERE id = ?').run(id).changes > 0;
  }

  #mustGet(id: number): Task {
    const task = this.get(id);
    if (!task) throw new Error(`task ${id} vanished inside its transaction`);
    return task;
  }

  #writeTags(id: number, tags: string[]): void {
    this.#db.prepare('DELETE FROM task_tags WHERE task_id = ?').run(id);
    const insert = this.#db.prepare('INSERT INTO task_tags (task_id, tag) VALUES (?, ?)');
    for (const tag of tags) insert.run(id, tag);
  }

  #tagsFor(ids: number[]): Map<number, string[]> {
    const byTask = new Map<number, string[]>();
    if (ids.length === 0) return byTask;
    const marks = ids.map(() => '?').join(',');
    const rows = this.#db
      .prepare<number[], { task_id: number; tag: string }>(
        `SELECT task_id, tag FROM task_tags WHERE task_id IN (${marks}) ORDER BY tag`,
      )
      .all(...ids);
    for (const { task_id, tag } of rows) byTask.set(task_id, [...(byTask.get(task_id) ?? []), tag]);
    return byTask;
  }
}

// A fake clock makes created_at deterministic: each call moves one minute forward.
let minutes = 0;
const clock = () => new Date(Date.UTC(2030, 0, 1, 9, minutes++));

const db = new Database(':memory:');
db.exec(SCHEMA);
const repo: TaskRepository = new SqliteTaskRepository(db, clock);

repo.create({ title: 'Write docs', tags: ['docs', 'p1'], due_date: '2030-02-01' });
repo.create({ title: 'Fix bug', status: 'in_progress', tags: ['p1'] });
repo.create({ title: 'Ship', due_date: '2030-01-15', owner_id: 7 });

console.log(repo.get(1));
console.log(repo.get(99));

const page = repo.list({ tag: 'p1', limit: 1 });
console.log(
  page.total,
  page.items.map((t) => t.title),
);
console.log(repo.list({ sort: 'due_date' }).items.map((t) => `${t.title}:${t.due_date}`));
console.log(repo.list({ status: 'todo', limit: 1000 }).limit);

console.log(repo.update(1, { status: 'done', due_date: null, tags: ['docs'] }));
console.log(repo.update(99, { title: 'x' }));
console.log(repo.delete(2), repo.delete(2));
console.log(repo.list({}).total);

db.close();
