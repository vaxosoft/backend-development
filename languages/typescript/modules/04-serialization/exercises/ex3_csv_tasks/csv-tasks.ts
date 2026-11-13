export type Status = 'todo' | 'in_progress' | 'done';

export interface TaskRow {
  id: number;
  title: string;
  status: Status;
  due_date: string | null;
  tags: string[];
}

export interface ImportError {
  /** 1-based record number where the header is row 1, so the first task is row 2. */
  row: number;
  message: string;
}

export interface ImportResult {
  tasks: TaskRow[];
  errors: ImportError[];
}

/**
 * Header `id,title,status,due_date,tags`, one line per task, tags joined with `|`,
 * a null due_date written as an empty cell. Quoting is the CSV library's job.
 */
export function tasksToCsv(_tasks: TaskRow[]): string {
  throw new Error('TODO');
}

/**
 * Parses CSV produced by tasksToCsv. Valid rows are returned; each invalid row adds an
 * ImportError whose message names the offending column. Rules per column:
 * id positive integer, title trimmed 1-200 chars, status one of the three values,
 * due_date empty (null) or a real YYYY-MM-DD date, tags `|`-separated (empty cell = none),
 * at most 10, unique, lowercase, 1-30 chars each.
 * Throws an Error naming the column when the header lacks a required column.
 */
export function tasksFromCsv(_text: string): ImportResult {
  throw new Error('TODO');
}
