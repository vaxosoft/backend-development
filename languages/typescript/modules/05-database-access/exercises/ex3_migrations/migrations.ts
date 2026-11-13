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
export function schemaVersion(_db: Database.Database): number {
  throw new Error('TODO');
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
  _db: Database.Database,
  _migrations: Migration[],
  _target?: number,
): { from: number; to: number } {
  throw new Error('TODO');
}
