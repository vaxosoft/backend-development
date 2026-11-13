import { parse } from 'csv-parse/sync';
import { stringify } from 'csv-stringify/sync';
import { z } from 'zod';

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

const COLUMNS = ['id', 'title', 'status', 'due_date', 'tags'] as const;

const RowSchema = z.object({
  id: z
    .string()
    .regex(/^[1-9]\d*$/, 'must be a positive integer')
    .transform(Number),
  title: z.string().trim().min(1, 'must be 1-200 characters').max(200, 'must be 1-200 characters'),
  status: z.enum(['todo', 'in_progress', 'done']),
  due_date: z.union([z.literal('').transform(() => null), z.iso.date('must be a YYYY-MM-DD date')]),
  tags: z
    .string()
    .transform((cell) => (cell === '' ? [] : cell.split('|')))
    .pipe(
      z
        .array(
          z
            .string()
            .min(1)
            .max(30)
            .refine((t) => t === t.toLowerCase(), 'must be lowercase'),
        )
        .max(10)
        .refine((tags) => new Set(tags).size === tags.length, 'must be unique'),
    ),
});

/**
 * Header `id,title,status,due_date,tags`, one line per task, tags joined with `|`,
 * a null due_date written as an empty cell. Quoting is the CSV library's job.
 */
export function tasksToCsv(tasks: TaskRow[]): string {
  return stringify(
    tasks.map((t) => [t.id, t.title, t.status, t.due_date ?? '', t.tags.join('|')]),
    { header: true, columns: [...COLUMNS] },
  );
}

/**
 * Parses CSV produced by tasksToCsv. Valid rows are returned; each invalid row adds an
 * ImportError whose message names the offending column. Rules per column:
 * id positive integer, title trimmed 1-200 chars, status one of the three values,
 * due_date empty (null) or a real YYYY-MM-DD date, tags `|`-separated (empty cell = none),
 * at most 10, unique, lowercase, 1-30 chars each.
 * Throws an Error naming the column when the header lacks a required column.
 */
export function tasksFromCsv(text: string): ImportResult {
  const records: Record<string, string>[] = parse(text, {
    columns: (header: string[]) => {
      for (const column of COLUMNS) {
        if (!header.includes(column)) throw new Error(`missing column: ${column}`);
      }
      return header;
    },
    skip_empty_lines: true,
    relax_column_count: true,
  });

  const result: ImportResult = { tasks: [], errors: [] };
  records.forEach((record, index) => {
    const parsed = RowSchema.safeParse({ ...record, tags: record.tags ?? '' });
    if (parsed.success) {
      result.tasks.push(parsed.data);
      return;
    }
    const issue = parsed.error.issues[0];
    result.errors.push({
      row: index + 2,
      message: `${issue?.path.join('.') ?? 'row'}: ${issue?.message ?? 'invalid'}`,
    });
  });
  return result;
}
