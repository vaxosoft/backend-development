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
export async function collect<T>(_tasks: Array<() => Promise<T>>): Promise<Collected<T>> {
  throw new Error('TODO');
}

/**
 * Resolves with the first task to succeed and ignores failures before that.
 * If every task fails (or there are none) rejects with an AggregateError holding all the errors.
 */
export async function firstSuccess<T>(_tasks: Array<() => Promise<T>>): Promise<T> {
  throw new Error('TODO');
}

/**
 * Resolves or rejects like `work`, but rejects with TimeoutError if it takes longer than `ms`.
 * The timer must not stay alive after `work` settles.
 */
export async function withTimeout<T>(_work: Promise<T>, _ms: number): Promise<T> {
  throw new Error('TODO');
}
