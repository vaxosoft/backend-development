import type Database from 'better-sqlite3';

export interface Migration {
  version: number;
  up: string;
  down: string;
}

export class MigrationError extends Error {
  readonly version: number;
  readonly direction: 'up' | 'down';

  constructor(version: number, direction: 'up' | 'down', options?: ErrorOptions) {
    super(`migration ${version} (${direction}) failed`, options);
    this.name = 'MigrationError';
    this.version = version;
    this.direction = direction;
  }
}

/** The schema version stored in PRAGMA user_version (0 for a new database). */
export function schemaVersion(db: Database.Database): number {
  return db.pragma('user_version', { simple: true }) as number;
}

/**
 * Moves the schema to `target` (default: the highest version) and returns `{ from, to }`.
 *
 * - Versions must be unique positive integers; otherwise throw RangeError BEFORE touching the database.
 * - A target that is neither 0 nor a known version throws RangeError.
 * - Going up runs `up` of every version in (current, target] ascending; going down runs `down` of every
 *   version in (target, current] descending. Nothing to do is not an error.
 * - Each step runs in its own transaction together with its user_version update. If a step fails,
 *   that step is rolled back, earlier steps stay applied, and a MigrationError (with `cause`) is thrown.
 */
export function migrateTo(
  db: Database.Database,
  migrations: Migration[],
  target?: number,
): { from: number; to: number } {
  const sorted = [...migrations].sort((a, b) => a.version - b.version);
  const versions = new Set<number>();
  for (const { version } of sorted) {
    if (!Number.isInteger(version) || version < 1 || versions.has(version)) {
      throw new RangeError(`invalid or duplicate migration version: ${version}`);
    }
    versions.add(version);
  }
  const goal = target ?? sorted.at(-1)?.version ?? 0;
  if (goal !== 0 && !versions.has(goal)) throw new RangeError(`unknown target version: ${goal}`);

  const from = schemaVersion(db);
  const run = (m: Migration, direction: 'up' | 'down', resultVersion: number) => {
    try {
      db.transaction(() => {
        db.exec(direction === 'up' ? m.up : m.down);
        db.pragma(`user_version = ${resultVersion}`);
      })();
    } catch (err: unknown) {
      throw new MigrationError(m.version, direction, { cause: err });
    }
  };

  if (goal > from) {
    for (const m of sorted.filter((m) => m.version > from && m.version <= goal))
      run(m, 'up', m.version);
  } else {
    const down = sorted.filter((m) => m.version > goal && m.version <= from).reverse();
    for (const m of down) {
      const previous = sorted.filter((p) => p.version < m.version).at(-1)?.version ?? 0;
      run(m, 'down', previous);
    }
  }
  return { from, to: schemaVersion(db) };
}
