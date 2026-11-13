import { setTimeout as sleep } from 'node:timers/promises';

export interface Failure {
  index: number;
  error: unknown;
}

export interface Collected<T> {
  /** Values of the tasks that succeeded, in input order. */
  values: T[];
  /** One entry per failed task, in input order. */
  errors: Failure[];
}

export class TimeoutError extends Error {
  constructor(ms: number) {
    super(`timed out after ${ms} ms`);
    this.name = 'TimeoutError';
  }
}

/**
 * Starts ALL tasks immediately (before awaiting any of them) and waits for every one to settle.
 * Never rejects: a task that rejects or throws synchronously is reported in `errors`.
 */
export async function collect<T>(tasks: Array<() => Promise<T>>): Promise<Collected<T>> {
  // The async wrapper turns a synchronous throw into a rejection but still calls the task right away.
  const settled = await Promise.allSettled(tasks.map(async (task) => task()));
  const result: Collected<T> = { values: [], errors: [] };
  settled.forEach((outcome, index) => {
    if (outcome.status === 'fulfilled') result.values.push(outcome.value);
    else result.errors.push({ index, error: outcome.reason });
  });
  return result;
}

/**
 * Resolves with the first task to succeed and ignores failures before that.
 * If every task fails (or there are none) rejects with an AggregateError holding all the errors.
 */
export async function firstSuccess<T>(tasks: Array<() => Promise<T>>): Promise<T> {
  return Promise.any(tasks.map(async (task) => task()));
}

/**
 * Resolves or rejects like `work`, but rejects with TimeoutError if it takes longer than `ms`.
 * The timer must not stay alive after `work` settles.
 */
export async function withTimeout<T>(work: Promise<T>, ms: number): Promise<T> {
  const timer = new AbortController();
  const timeout = sleep(ms, undefined, { signal: timer.signal }).then(() => {
    throw new TimeoutError(ms);
  });
  try {
    return await Promise.race([work, timeout]);
  } finally {
    timer.abort();
    timeout.catch(() => {}); // the aborted sleep rejects; nobody is waiting for it any more
  }
}
