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

interface Entry {
  start: () => void;
  cancel: (reason: unknown) => void;
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
  readonly #workers: number;
  readonly #capacity: number;
  readonly #queue: Entry[] = [];
  readonly #drained: Array<() => void> = [];
  #active = 0;
  #closed = false;

  constructor(options: PoolOptions) {
    this.#workers = options.workers;
    this.#capacity = options.capacity;
  }

  run<T>(
    fn: (signal: AbortSignal) => Promise<T>,
    options: { signal?: AbortSignal } = {},
  ): Promise<T> {
    if (this.#closed) return Promise.reject(new PoolClosedError());
    const { signal } = options;
    if (signal?.aborted) return Promise.reject(signal.reason);
    const hasWorker = this.#active < this.#workers;
    if (!hasWorker && this.#queue.length >= this.#capacity)
      return Promise.reject(new QueueFullError());

    return new Promise<T>((resolve, reject) => {
      const entry: Entry = {
        start: () => {
          signal?.removeEventListener('abort', onAbort);
          this.#active++;
          const settle = () => {
            this.#active--;
            this.#startNext();
          };
          let result: Promise<T>;
          try {
            result = fn(signal ?? new AbortController().signal);
          } catch (err: unknown) {
            result = Promise.reject(err);
          }
          // Update the counters before the caller resumes, so `active` is already correct there.
          result.then(
            (value) => {
              settle();
              resolve(value);
            },
            (err: unknown) => {
              settle();
              reject(err);
            },
          );
        },
        cancel: reject,
      };
      const onAbort = () => {
        this.#queue.splice(this.#queue.indexOf(entry), 1);
        entry.cancel(signal?.reason);
      };

      if (hasWorker) {
        entry.start();
      } else {
        signal?.addEventListener('abort', onAbort, { once: true });
        this.#queue.push(entry);
      }
    });
  }

  close(): Promise<void> {
    this.#closed = true;
    if (this.#active === 0 && this.#queue.length === 0) return Promise.resolve();
    return new Promise((resolve) => this.#drained.push(resolve));
  }

  get active(): number {
    return this.#active;
  }

  get pending(): number {
    return this.#queue.length;
  }

  #startNext(): void {
    const next = this.#queue.shift();
    if (next) {
      next.start();
    } else if (this.#active === 0) {
      for (const resolve of this.#drained.splice(0)) resolve();
    }
  }
}
