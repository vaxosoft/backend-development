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
  db: Database.Database,
  title: string,
  tags: string[],
): TaskWithTags {
  return db.transaction(() => {
    const id = Number(
      db.prepare('INSERT INTO tasks (title) VALUES (?)').run(title).lastInsertRowid,
    );
    const insert = db.prepare('INSERT INTO task_tags (task_id, tag) VALUES (?, ?)');
    for (const tag of tags) insert.run(id, tag);
    return { id, title, tags: [...tags].sort() };
  })();
}

/**
 * Renames a tag on every task that has it, atomically, and returns how many tasks changed.
 * A task that already has `to` just loses `from` (no duplicate row, no error).
 */
export function renameTag(db: Database.Database, from: string, to: string): number {
  return db.transaction(() => {
    const ids = db
      .prepare<[string], number>('SELECT task_id FROM task_tags WHERE tag = ?')
      .pluck()
      .all(from);
    db.prepare(
      `INSERT INTO task_tags (task_id, tag)
       SELECT task_id, ? FROM task_tags old
       WHERE old.tag = ? AND NOT EXISTS (SELECT 1 FROM task_tags t WHERE t.task_id = old.task_id AND t.tag = ?)`,
    ).run(to, from, to);
    db.prepare('DELETE FROM task_tags WHERE tag = ?').run(from);
    return ids.length;
  })();
}
