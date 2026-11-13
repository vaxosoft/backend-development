import type Database from 'better-sqlite3';

/*
 * Schema used by the tests:
 *   tasks(id INTEGER PRIMARY KEY, title TEXT NOT NULL)
 *   task_tags(task_id REFERENCES tasks(id), tag TEXT NOT NULL CHECK (length(tag) <= 30), PRIMARY KEY (task_id, tag))
 */

export interface TaskWithTags {
  id: number;
  title: string;
  tags: string[];
}

/**
 * Inserts the task and all its tags as ONE atomic unit: if any tag insert fails
 * (duplicate tag, too long) nothing may remain in either table, and the original
 * error is rethrown. It must also work when called inside an outer db.transaction().
 * Returns the task with tags sorted alphabetically.
 */
export function createTaskWithTags(
  _db: Database.Database,
  _title: string,
  _tags: string[],
): TaskWithTags {
  throw new Error('TODO');
}

/**
 * Renames a tag on every task that has it, atomically, and returns how many tasks changed.
 * A task that already has `to` just loses `from` (no duplicate row, no error).
 */
export function renameTag(_db: Database.Database, _from: string, _to: string): number {
  throw new Error('TODO');
}
