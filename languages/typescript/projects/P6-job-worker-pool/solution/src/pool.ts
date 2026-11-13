import { performance } from 'node:perf_hooks';
import { setTimeout as sleep } from 'node:timers/promises';
import type { FastifyBaseLogger } from 'fastify';
import type { PoolConfig } from './config.ts';
import type { JobRunner } from './runner.ts';
import type { JobRequest } from './schema.ts';
import type { Job } from './types.ts';

export type SubmitResult = { ok: true; job: Job } | { ok: false; reason: 'full' | 'closed' };

export type CancelResult = 'cancelled' | 'not_found' | 'finished';

interface Entry {
  job: Job;
  request: JobRequest;
  controller: AbortController;
}

/** A fixed set of workers pulling jobs from a bounded queue. */
export class WorkerPool {
  readonly #config: PoolConfig;
  readonly #run: JobRunner;
  readonly #log: FastifyBaseLogger;
  readonly #entries = new Map<number, Entry>();
  readonly #queue: Entry[] = [];
  // Idle workers wait here; resolving with `undefined` tells a worker to exit.
  readonly #idle: Array<(entry: Entry | undefined) => void> = [];
  readonly #workers: Promise<void>[];
  #nextId = 1;
  #closed = false;

  constructor(config: PoolConfig, run: JobRunner, log: FastifyBaseLogger) {
    this.#config = config;
    this.#run = run;
    this.#log = log;
    this.#workers = Array.from({ length: config.workers }, () => this.#work());
  }

  submit(request: JobRequest): SubmitResult {
    if (this.#closed) {
      return { ok: false, reason: 'closed' };
    }
    const worker = this.#idle.shift();
    if (!worker && this.#queue.length >= this.#config.queueCapacity) {
      return { ok: false, reason: 'full' };
    }
    const job: Job = {
      id: this.#nextId++,
      type: request.type,
      status: 'queued',
      attempts: 0,
      result: null,
      error: null,
    };
    const entry: Entry = { job, request, controller: new AbortController() };
    this.#entries.set(job.id, entry);
    if (worker) {
      worker(entry);
    } else {
      this.#queue.push(entry);
    }
    return { ok: true, job: { ...job } };
  }

  get(id: number): Job | undefined {
    const entry = this.#entries.get(id);
    return entry && { ...entry.job };
  }

  /** Newest first. */
  list(): Job[] {
    return [...this.#entries.values()].reverse().map((entry) => ({ ...entry.job }));
  }

  cancel(id: number): CancelResult {
    const entry = this.#entries.get(id);
    if (!entry) {
      return 'not_found';
    }
    if (entry.job.status !== 'queued' && entry.job.status !== 'running') {
      return 'finished';
    }
    const queued = this.#queue.indexOf(entry);
    if (queued >= 0) {
      this.#queue.splice(queued, 1);
    }
    entry.job.status = 'cancelled';
    entry.controller.abort();
    return 'cancelled';
  }

  async shutdown(): Promise<void> {
    this.#closed = true;
    for (const wake of this.#idle.splice(0)) {
      wake(undefined);
    }
    const timer = new AbortController();
    const expired = sleep(this.#config.drainTimeoutMs, true, { signal: timer.signal }).catch(
      () => false,
    );
    const drained = Promise.all(this.#workers).then(() => false);
    const timedOut = await Promise.race([drained, expired]);
    timer.abort();
    if (timedOut) {
      this.#log.warn('drain timeout reached, cancelling remaining jobs');
      for (const id of this.#entries.keys()) {
        this.cancel(id);
      }
      await Promise.all(this.#workers);
    }
  }

  #take(): Promise<Entry | undefined> {
    const entry = this.#queue.shift();
    if (entry) {
      return Promise.resolve(entry);
    }
    if (this.#closed) {
      return Promise.resolve(undefined);
    }
    return new Promise((resolve) => this.#idle.push(resolve));
  }

  async #work(): Promise<void> {
    for (let entry = await this.#take(); entry; entry = await this.#take()) {
      await this.#process(entry);
    }
  }

  async #process(entry: Entry): Promise<void> {
    const { job, controller } = entry;
    if (controller.signal.aborted) {
      return;
    }
    const started = performance.now();
    job.status = 'running';
    while (!controller.signal.aborted) {
      job.attempts++;
      try {
        const result = await this.#attempt(entry);
        if (!controller.signal.aborted) {
          job.status = 'succeeded';
          job.result = result;
          job.error = null;
        }
        break;
      } catch (err) {
        if (controller.signal.aborted) {
          break;
        }
        job.error = err instanceof Error ? err.message : String(err);
        if (job.attempts >= this.#config.maxAttempts) {
          job.status = 'failed';
          break;
        }
        await sleep(this.#config.retryDelayMs, undefined, { signal: controller.signal }).catch(
          () => undefined,
        );
      }
    }
    this.#log.info(
      {
        jobId: job.id,
        type: job.type,
        status: job.status,
        attempts: job.attempts,
        durationMs: Math.round(performance.now() - started),
      },
      'job finished',
    );
  }

  async #attempt(entry: Entry): Promise<string> {
    const timeout = AbortSignal.timeout(this.#config.jobTimeoutMs);
    const signal = AbortSignal.any([entry.controller.signal, timeout]);
    try {
      return await this.#run(entry.request, signal);
    } catch (err) {
      if (timeout.aborted) {
        throw new Error(`timed out after ${this.#config.jobTimeoutMs}ms`, { cause: err });
      }
      throw err;
    }
  }
}
