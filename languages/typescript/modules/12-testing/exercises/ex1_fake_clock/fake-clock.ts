export type TimerHandle = number;

/** The slice of time that production code depends on; tests inject a `FakeClock` instead of the real one. */
export interface Clock {
  now(): number;
  setTimeout(callback: () => void, ms: number): TimerHandle;
  clearTimeout(handle: TimerHandle): void;
}

/**
 * A manually driven clock.
 *
 * - `now()` starts at `start` (milliseconds) and only moves when `advance` is called.
 * - `setTimeout` schedules `callback` at `now() + ms` and returns a handle (unique per timer).
 * - `clearTimeout` cancels a pending timer; unknown or already fired handles are ignored.
 * - `advance(ms)` moves time forward and runs every timer that becomes due, in chronological order
 *   (timers due at the same instant run in the order they were scheduled). While a callback runs,
 *   `now()` equals that timer's due time. Timers scheduled by a callback that fall inside the
 *   advanced window run too. After `advance`, `now()` is exactly the old time plus `ms`.
 *   A negative or non-finite `ms` throws a `RangeError`.
 * - `pending()` is the number of timers that have not fired and were not cleared.
 */
export class FakeClock implements Clock {
  constructor(_start = 0) {
    throw new Error('TODO');
  }

  now(): number {
    throw new Error('TODO');
  }

  setTimeout(_callback: () => void, _ms: number): TimerHandle {
    throw new Error('TODO');
  }

  clearTimeout(_handle: TimerHandle): void {
    throw new Error('TODO');
  }

  advance(_ms: number): void {
    throw new Error('TODO');
  }

  pending(): number {
    throw new Error('TODO');
  }
}
