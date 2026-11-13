// Migrations: ordered, versioned schema changes tracked with PRAGMA user_version.
import Database from 'better-sqlite3';

interface Migration {
  version: number;
  sql: string;
}

const MIGRATIONS: Migration[] = [
  {
    version: 1,
    sql: `CREATE TABLE tasks (
      id INTEGER PRIMARY KEY,
      title TEXT NOT NULL,
      status TEXT NOT NULL DEFAULT 'todo' CHECK (status IN ('todo','in_progress','done')),
      created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
    )`,
  },
  { version: 2, sql: 'ALTER TABLE tasks ADD COLUMN due_date TEXT' },
  {
    version: 3,
    sql: `CREATE TABLE task_tags (
      task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
      tag TEXT NOT NULL,
      PRIMARY KEY (task_id, tag)
    );
    CREATE INDEX idx_tasks_status ON tasks(status)`,
  },
];

function currentVersion(db: Database.Database): number {
  return db.pragma('user_version', { simple: true }) as number;
}

/** Applies every migration newer than the stored version; each one commits together with its version bump. */
function migrate(db: Database.Database, migrations: Migration[]): number[] {
  const applied: number[] = [];
  for (const m of [...migrations].sort((a, b) => a.version - b.version)) {
    if (m.version <= currentVersion(db)) continue;
    db.transaction(() => {
      db.exec(m.sql);
      db.pragma(`user_version = ${m.version}`); // PRAGMA cannot take bound parameters; m.version is a number we own
    })();
    applied.push(m.version);
  }
  return applied;
}

const db = new Database(':memory:');
console.log('fresh database at version', currentVersion(db));
console.log('applied:', migrate(db, MIGRATIONS));
console.log('applied again:', migrate(db, MIGRATIONS), '(idempotent)');
console.log(
  db.prepare("SELECT name FROM sqlite_master WHERE type = 'table' ORDER BY name").pluck().all(),
);

// A failing migration rolls back completely and leaves the version where it was.
try {
  migrate(db, [
    ...MIGRATIONS,
    { version: 4, sql: 'CREATE TABLE ok (id INTEGER); CREATE TABLE ok (id INTEGER)' },
  ]);
} catch (err: unknown) {
  if (err instanceof Database.SqliteError) console.log('migration 4 failed:', err.message);
}
console.log(
  'version',
  currentVersion(db),
  'has table "ok":',
  db.prepare("SELECT 1 FROM sqlite_master WHERE name = 'ok'").get() !== undefined,
);

// ':memory:' is private to one connection: a second connection sees a brand-new empty database.
const other = new Database(':memory:');
console.log(
  'second connection version:',
  currentVersion(other),
  'tables:',
  other.prepare('SELECT count(*) FROM sqlite_master').pluck().get(),
);

other.close();
db.close();
