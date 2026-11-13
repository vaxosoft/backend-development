// A bounded in-process job pool: N async workers, a bounded queue, cancellation and graceful close.
import { setTimeout as sleep } from 'node:timers/promises';

type JobType = 'reverse' | 'sleep' | 'fail';
type JobStatus = 'queued' | 'running' | 'succeeded' | 'failed' | 'cancelled';

interface Job {
  id: number;
  type: JobType;
  payload: Record<string, unknown>;
  status: JobStatus;
  result: string | null;
  error: string | null;
}

type Submit = { ok: true; job: Job } | { ok: false; reason: 'queue_full' | 'closed' };

async function runJob(job: Job, signal: AbortSignal): Promise<string | null> {
  switch (job.type) {
    case 'reverse':
      return [...String(job.payload.text)].reverse().join('');
    case 'sleep':
      await sleep(Number(job.payload.ms), undefined, { signal });
      return null;
    case 'fail':
      throw new Error('this job always fails');
  }
}

class JobPool {
  readonly #capacity: number;
  readonly #queue: Job[] = [];
  readonly #waiting: Array<(job: Job | undefined) => void> = []; // idle workers parked on a promise
  readonly #jobs = new Map<number, Job>();
  readonly #aborts = new Map<number, AbortController>();
  readonly #workers: Promise<void>[];
  #closed = false;
  #nextId = 1;

  constructor(options: { workers: number; capacity: number }) {
    this.#capacity = options.capacity;
    this.#workers = Array.from({ length: options.workers }, () => this.#work());
  }

  submit(type: JobType, payload: Record<string, unknown> = {}): Submit {
    if (this.#closed) return { ok: false, reason: 'closed' };
    if (this.#waiting.length === 0 && this.#queue.length >= this.#capacity) {
      return { ok: false, reason: 'queue_full' };
    }
    const job: Job = {
      id: this.#nextId++,
      type,
      payload,
      status: 'queued',
      result: null,
      error: null,
    };
    this.#jobs.set(job.id, job);
    const idle = this.#waiting.shift();
    if (idle)
      idle(job); // hand the job straight to an idle worker; it never occupies a queue slot
    else this.#queue.push(job);
    return { ok: true, job };
  }

  cancel(id: number): boolean {
    const job = this.#jobs.get(id);
    if (!job) return false;
    if (job.status === 'queued') {
      const index = this.#queue.indexOf(job);
      if (index >= 0) this.#queue.splice(index, 1);
      job.status = 'cancelled'; // a worker that was already handed this job skips it
      return true;
    }
    if (job.status === 'running') {
      this.#aborts.get(id)?.abort();
      return true;
    }
    return false; // already finished
  }

  all(): Job[] {
    return [...this.#jobs.values()];
  }

  /** Stops accepting jobs, lets queued and running jobs finish, then resolves. */
  async close(): Promise<void> {
    this.#closed = true;
    for (const wake of this.#waiting.splice(0)) wake(undefined);
    await Promise.all(this.#workers);
  }

  async #work(): Promise<void> {
    while (true) {
      const job =
        this.#queue.shift() ??
        (this.#closed
          ? undefined
          : await new Promise<Job | undefined>((resolve) => this.#waiting.push(resolve)));
      if (!job) return; // closed and nothing left to do
      await this.#execute(job);
    }
  }

  async #execute(job: Job): Promise<void> {
    if (job.status === 'cancelled') return;
    const controller = new AbortController();
    this.#aborts.set(job.id, controller);
    job.status = 'running';
    try {
      job.result = await runJob(job, controller.signal);
      job.status = 'succeeded';
    } catch (err: unknown) {
      if (controller.signal.aborted) {
        job.status = 'cancelled';
      } else {
        job.status = 'failed';
        job.error = err instanceof Error ? err.message : String(err);
      }
    } finally {
      this.#aborts.delete(job.id);
    }
  }
}

const pool = new JobPool({ workers: 2, capacity: 2 });

for (let i = 0; i < 5; i++) {
  const outcome = pool.submit('sleep', { ms: 30 });
  console.log(outcome.ok ? `accepted job ${outcome.job.id}` : `rejected: ${outcome.reason}`);
}

await sleep(5); // let the two workers pick up jobs 1 and 2

console.log('cancel queued job 3:', pool.cancel(3));
console.log('cancel running job 1:', pool.cancel(1));
await sleep(5);
console.log(
  pool
    .all()
    .map((j) => `${j.id}:${j.status}`)
    .join(' '),
);

pool.submit('reverse', { text: 'tracker' });
pool.submit('fail');

await pool.close();
console.log('after close:', pool.submit('reverse', { text: 'x' }));
for (const job of pool.all()) {
  console.log(job.id, job.type.padEnd(7), job.status.padEnd(9), job.result ?? job.error ?? '');
}
