import type { Clock } from './fake-clock.ts';

export interface DueTask {
  id: number;
  title: string;
  status: 'todo' | 'in_progress' | 'done';
  /** Due time in milliseconds on the same scale as `Clock.now()`; `null` when the task has no due date. */
  dueAt: number | null;
}

export interface ReminderDeps {
  clock: Clock;
  listTasks(): DueTask[];
  notify(task: DueTask): Promise<void>;
  /** Called with the failure of a `notify` call; reminders must keep running afterwards. */
  onError(error: unknown, task: DueTask): void;
  /** How often to look for tasks that need a reminder. */
  intervalMs: number;
  /** A task is reminded when `dueAt - now <= leadMs` (overdue tasks included). */
  leadMs: number;
}

/**
 * Starts a background reminder loop and returns `{ stop }`.
 *
 * Every `intervalMs` (measured by `clock`, never by real timers) it looks at `listTasks()` and calls
 * `notify(task)` for each task that is not `done`, has a `dueAt`, and is within `leadMs` of it.
 *
 * - A task is notified at most once: not again on later ticks, and not again while its first
 *   `notify` is still pending (the promise has not settled).
 * - When `notify` rejects, `onError(error, task)` is called, the other tasks are unaffected, and the
 *   task is retried on the next tick.
 * - Nothing runs at start-up: the first look happens after the first interval.
 * - `stop()` cancels the loop; no further ticks happen. Calling it twice is harmless.
 */
export function startReminders({
  clock,
  listTasks,
  notify,
  onError,
  intervalMs,
  leadMs,
}: ReminderDeps): { stop(): void } {
  const handled = new Set<number>(); // in flight or successfully notified

  const cancel = clock.setInterval(() => {
    const now = clock.now();
    for (const task of listTasks()) {
      if (task.status === 'done' || task.dueAt === null || handled.has(task.id)) continue;
      if (task.dueAt - now > leadMs) continue;
      handled.add(task.id);
      notify(task).catch((error: unknown) => {
        handled.delete(task.id);
        onError(error, task);
      });
    }
  }, intervalMs);

  return { stop: cancel };
}
