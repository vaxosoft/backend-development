import type { FastifyBaseLogger } from 'fastify';
import type { PoolConfig } from './config.ts';
import type { JobRunner } from './runner.ts';
import type { JobRequest } from './schema.ts';
import type { Job } from './types.ts';

export type SubmitResult = { ok: true; job: Job } | { ok: false; reason: 'full' | 'closed' };

export type CancelResult = 'cancelled' | 'not_found' | 'finished';

/**
 * A fixed set of workers pulling jobs from a bounded queue.
 *
 * TODO: implement every method. Suggested design:
 * - the constructor starts `config.workers` worker loops (`async` functions that
 *   take the next entry, process it, and exit once the pool is closed and drained)
 * - an idle worker waits on a promise; `submit` hands the entry straight to it,
 *   otherwise it appends to the queue (`full` when `queueCapacity` entries wait)
 * - each entry owns an AbortController; one attempt gets
 *   `AbortSignal.any([controller.signal, AbortSignal.timeout(jobTimeoutMs)])`
 * - a failed attempt is retried after `retryDelayMs` until `maxAttempts`
 * - `cancel` marks the job cancelled at once and aborts its controller
 * - log one `job finished` line (jobId, type, status, attempts, durationMs) per processed job
 */
export class WorkerPool {
  readonly config: PoolConfig;
  readonly run: JobRunner;
  readonly log: FastifyBaseLogger;

  constructor(config: PoolConfig, run: JobRunner, log: FastifyBaseLogger) {
    this.config = config;
    this.run = run;
    this.log = log;
  }

  submit(_request: JobRequest): SubmitResult {
    throw new Error('TODO');
  }

  get(_id: number): Job | undefined {
    throw new Error('TODO');
  }

  /** Newest first. */
  list(): Job[] {
    throw new Error('TODO');
  }

  /** Queued or running jobs become `cancelled`; finished ones are left alone. */
  cancel(_id: number): CancelResult {
    throw new Error('TODO');
  }

  /**
   * Stop accepting jobs, let running and queued jobs finish for up to
   * `drainTimeoutMs`, then cancel whatever is left. Resolves when all workers are done.
   */
  async shutdown(): Promise<void> {
    throw new Error('TODO');
  }
}
