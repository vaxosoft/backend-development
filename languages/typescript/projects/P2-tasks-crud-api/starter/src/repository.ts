import type Database from 'better-sqlite3';
import type { ListQuery, Page, Task, TaskInput, TaskPatch } from './types.ts';

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

export class SqliteTaskRepository implements TaskRepository {
  readonly db: Database.Database;
  readonly clock: () => Date;

  constructor(db: Database.Database, clock: () => Date) {
    this.db = db;
    this.clock = clock;
  }

  create(_input: TaskInput): Task {
    throw new Error('TODO');
  }

  get(_id: number): Task | undefined {
    throw new Error('TODO');
  }

  replace(_id: number, _input: TaskInput): Task | undefined {
    throw new Error('TODO');
  }

  patch(_id: number, _patch: TaskPatch): Task | undefined {
    throw new Error('TODO');
  }

  remove(_id: number): boolean {
    throw new Error('TODO');
  }

  list(_query: ListQuery): Page<Task> {
    throw new Error('TODO');
  }
}
