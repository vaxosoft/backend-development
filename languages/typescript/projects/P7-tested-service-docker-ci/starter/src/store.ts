import type { NewTask, Task, TaskStore } from './types.ts';

export class InMemoryTaskStore implements TaskStore {
  readonly #tasks: Task[] = [];

  create(task: NewTask): Task {
    const created: Task = { id: this.#tasks.length + 1, ...task };
    this.#tasks.push(created);
    return created;
  }

  get(id: number): Task | undefined {
    return this.#tasks[id - 1];
  }

  list(limit: number, offset: number): { items: Task[]; total: number } {
    return { items: this.#tasks.slice(offset, offset + limit), total: this.#tasks.length };
  }
}
