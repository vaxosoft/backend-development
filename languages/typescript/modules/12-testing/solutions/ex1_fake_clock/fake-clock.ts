export type TimerHandle = number;

/** The slice of time that production code depends on; tests inject a `FakeClock` instead of the real one. */
export interface Clock {
  now(): number;
  setTimeout(callback: () => void, ms: number): TimerHandle;
  clearTimeout(handle: TimerHandle): void;
}

interface Timer {
  handle: TimerHandle;
  dueAt: number;
  callback: () => void;
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
  #now: number;
  #nextHandle = 1;
  #timers: Timer[] = [];

  constructor(start = 0) {
    this.#now = start;
  }

  now(): number {
    return this.#now;
  }

  setTimeout(callback: () => void, ms: number): TimerHandle {
    const handle = this.#nextHandle++;
    this.#timers.push({ handle, dueAt: this.#now + Math.max(0, ms), callback });
    return handle;
  }

  clearTimeout(handle: TimerHandle): void {
    this.#timers = this.#timers.filter((t) => t.handle !== handle);
  }

  advance(ms: number): void {
    if (!Number.isFinite(ms) || ms < 0) throw new RangeError('ms must be a non-negative number');
    const target = this.#now + ms;
    for (;;) {
      // Stable sort: equal due times keep scheduling order.
      const next = this.#timers
        .filter((t) => t.dueAt <= target)
        .sort((a, b) => a.dueAt - b.dueAt)[0];
      if (!next) break;
      this.#timers = this.#timers.filter((t) => t !== next);
      this.#now = next.dueAt;
      next.callback();
    }
    this.#now = target;
  }

  pending(): number {
    return this.#timers.length;
  }
}
