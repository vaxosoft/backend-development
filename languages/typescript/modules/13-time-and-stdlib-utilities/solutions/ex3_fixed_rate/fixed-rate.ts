import { setTimeout as timerSleep } from 'node:timers/promises';

export interface Clock {
  /** Monotonic time in milliseconds. */
  now(): number;
  /** Resolves after `ms`; rejects with the abort reason when `signal` aborts first. */
  sleep(ms: number, signal: AbortSignal): Promise<void>;
}

export const systemClock: Clock = {
  now: () => performance.now(),
  sleep: (ms, signal) => timerSleep(ms, undefined, { signal }),
};

export interface FixedRateOptions {
  intervalMs: number;
  task: () => Promise<void> | void;
  clock: Clock;
  signal: AbortSignal;
  onError?: (error: unknown) => void;
}

/**
 * Runs `task` at start, start + interval, start + 2 * interval, ... without drift.
 *
 * - Runs never overlap: the next run waits for the current one.
 * - If a run takes longer than the interval, the missed ticks are skipped (no burst of catch-up runs):
 *   the next run happens at the next multiple of the interval counted from the start.
 * - A failing task is reported to `onError` and the schedule continues.
 * - Resolves (does not reject) once `signal` is aborted, also while sleeping.
 */
export async function runFixedRate(options: FixedRateOptions): Promise<void> {
  const { intervalMs, task, clock, signal, onError } = options;
  const start = clock.now();
  let tick = 0;
  while (!signal.aborted) {
    try {
      await task();
    } catch (error) {
      onError?.(error);
    }
    if (signal.aborted) return;
    const elapsed = clock.now() - start;
    tick = Math.max(tick + 1, Math.ceil(elapsed / intervalMs));
    try {
      await clock.sleep(tick * intervalMs - elapsed, signal);
    } catch (error) {
      if (signal.aborted) return;
      throw error;
    }
  }
}
