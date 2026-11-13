export class QueueFullError extends Error {
  constructor() {
    super('queue is full');
    this.name = 'QueueFullError';
  }
}

export class PoolClosedError extends Error {
  constructor() {
    super('pool is closed');
    this.name = 'PoolClosedError';
  }
}

export interface PoolOptions {
  /** Maximum number of tasks running at the same time (>= 1). */
  workers: number;
  /** Maximum number of tasks waiting for a free worker (>= 0). */
  capacity: number;
}

/**
 * A bounded pool for async tasks.
 *
 * - `run(fn)` starts `fn` right away if fewer than `workers` tasks are running, otherwise it waits in
 *   a FIFO queue. When the queue already holds `capacity` tasks, `run` rejects with QueueFullError.
 * - `run` resolves or rejects with whatever `fn` does; a failing task never harms the pool.
 *   A synchronous throw inside `fn` is a rejection too.
 * - `options.signal` is passed to `fn` as its argument. If it aborts while the task is still queued,
 *   the task is removed (fn is never called, its slot is free again) and `run` rejects with `signal.reason`.
 * - `close()` rejects all later `run` calls with PoolClosedError, waits until queued and running tasks
 *   have finished, and may be called several times.
 * - `active` is the number of running tasks, `pending` the number of queued ones.
 */
export class TaskPool {
  constructor(_options: PoolOptions) {
    throw new Error('TODO');
  }

  run<T>(
    _fn: (signal: AbortSignal) => Promise<T>,
    _options?: { signal?: AbortSignal },
  ): Promise<T> {
    throw new Error('TODO');
  }

  close(): Promise<void> {
    throw new Error('TODO');
  }

  get active(): number {
    throw new Error('TODO');
  }

  get pending(): number {
    throw new Error('TODO');
  }
}
