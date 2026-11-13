export interface TimeOfDay {
  hour: number;
  minute: number;
}

/**
 * The first moment strictly after `after` at which the wall clock in `timeZone` shows `at`.
 *
 * - A time that does not exist on a given day (spring-forward gap) runs at the first valid moment after it.
 * - A time that happens twice (fall-back overlap) runs once, at its first occurrence.
 */
export function nextDailyRun(
  _after: Temporal.Instant,
  _timeZone: string,
  _at: TimeOfDay,
): Temporal.Instant {
  throw new Error('TODO');
}

/** The next `count` daily runs, each strictly after the previous one. */
export function upcomingRuns(
  _after: Temporal.Instant,
  _timeZone: string,
  _at: TimeOfDay,
  _count: number,
): Temporal.Instant[] {
  throw new Error('TODO');
}
