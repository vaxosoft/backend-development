export interface DueTask {
  due_date: string | null;
  status: 'todo' | 'in_progress' | 'done';
}

/**
 * Parses a task due date. Only the exact wire format `YYYY-MM-DD` of a real calendar day is
 * accepted; anything else (other formats, date-times, impossible days) returns `null`.
 */
export function parseDueDate(_text: string): Temporal.PlainDate | null {
  throw new Error('TODO');
}

/**
 * Whole calendar days from "today" (as seen in `timeZone`) until `due`.
 * Negative when the due date is in the past, 0 when it is today.
 */
export function daysUntilDue(_due: string, _now: Temporal.Instant, _timeZone: string): number {
  throw new Error('TODO');
}

/** A task is overdue when it has a due date before today (in `timeZone`) and is not done. */
export function isOverdue(_task: DueTask, _now: Temporal.Instant, _timeZone: string): boolean {
  throw new Error('TODO');
}
