export type TaskStatus = 'todo' | 'in_progress' | 'done';

export interface Task {
  id: number;
  title: string;
  status: TaskStatus;
  due_date: string | null;
  tags: string[];
  owner_id: number | null;
  created_at: string;
}

export type NewTask = Omit<Task, 'id'>;

/** Everything the HTTP layer needs from persistence; tests can swap in a fake. */
export interface TaskStore {
  create(task: NewTask): Task;
  get(id: number): Task | undefined;
  list(limit: number, offset: number): { items: Task[]; total: number };
}

export function errorBody(code: string, message: string, fields?: Record<string, string>) {
  return { error: { code, message, ...(fields ? { fields } : {}) } };
}
