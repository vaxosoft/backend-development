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
  after: Temporal.Instant,
  timeZone: string,
  at: TimeOfDay,
): Temporal.Instant {
  const startDay = after.toZonedDateTimeISO(timeZone).toPlainDate();
  for (let offset = 0; ; offset += 1) {
    const candidate = startDay
      .add({ days: offset })
      .toZonedDateTime({ timeZone, plainTime: Temporal.PlainTime.from(at) });
    if (Temporal.Instant.compare(candidate.toInstant(), after) > 0) return candidate.toInstant();
  }
}

/** The next `count` daily runs, each strictly after the previous one. */
export function upcomingRuns(
  after: Temporal.Instant,
  timeZone: string,
  at: TimeOfDay,
  count: number,
): Temporal.Instant[] {
  const runs: Temporal.Instant[] = [];
  let cursor = after;
  for (let i = 0; i < count; i += 1) {
    cursor = nextDailyRun(cursor, timeZone, at);
    runs.push(cursor);
  }
  return runs;
}
