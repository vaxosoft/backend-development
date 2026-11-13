export interface DueTask {
  due_date: string | null;
  status: 'todo' | 'in_progress' | 'done';
}

const DUE_DATE = /^\d{4}-\d{2}-\d{2}$/;

/**
 * Parses a task due date. Only the exact wire format `YYYY-MM-DD` of a real calendar day is
 * accepted; anything else (other formats, date-times, impossible days) returns `null`.
 */
export function parseDueDate(text: string): Temporal.PlainDate | null {
  if (!DUE_DATE.test(text)) return null;
  try {
    return Temporal.PlainDate.from(text, { overflow: 'reject' });
  } catch (error) {
    if (error instanceof RangeError) return null;
    throw error;
  }
}

/**
 * Whole calendar days from "today" (as seen in `timeZone`) until `due`.
 * Negative when the due date is in the past, 0 when it is today.
 */
export function daysUntilDue(due: string, now: Temporal.Instant, timeZone: string): number {
  const today = now.toZonedDateTimeISO(timeZone).toPlainDate();
  return today.until(Temporal.PlainDate.from(due), { largestUnit: 'day' }).days;
}

/** A task is overdue when it has a due date before today (in `timeZone`) and is not done. */
export function isOverdue(task: DueTask, now: Temporal.Instant, timeZone: string): boolean {
  if (task.due_date === null || task.status === 'done') return false;
  return daysUntilDue(task.due_date, now, timeZone) < 0;
}
