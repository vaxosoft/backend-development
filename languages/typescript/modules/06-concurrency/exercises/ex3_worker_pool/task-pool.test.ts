import assert from 'node:assert/strict';
import { describe, test } from 'node:test';
import { setTimeout as sleep } from 'node:timers/promises';
import { PoolClosedError, QueueFullError, TaskPool } from './task-pool.ts';

function deferred<T = void>() {
  let resolve!: (value: T) => void;
  let reject!: (error: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

const tick = () => sleep(1);

describe('TaskPool running tasks', () => {
  test('returns the task result and propagates its rejection', async () => {
    const pool = new TaskPool({ workers: 2, capacity: 2 });
    assert.equal(await pool.run(async () => 42), 42);
    await assert.rejects(
      pool.run(async () => {
        throw new Error('task failed');
      }),
      /task failed/,
    );
    await pool.close();
  });

  test('a synchronous throw is a rejection and the pool keeps working', async () => {
    const pool = new TaskPool({ workers: 1, capacity: 1 });
    await assert.rejects(
      pool.run(() => {
        throw new Error('sync');
      }),
      /sync/,
    );
    assert.equal(await pool.run(async () => 'still alive'), 'still alive');
    assert.equal(pool.active, 0);
    await pool.close();
  });

  test('runs at most `workers` tasks at once', async () => {
    const pool = new TaskPool({ workers: 3, capacity: 20 });
    let inFlight = 0;
    let peak = 0;
    const jobs = Array.from({ length: 12 }, () =>
      pool.run(async () => {
        inFlight++;
        peak = Math.max(peak, inFlight);
        await sleep(3);
        inFlight--;
      }),
    );
    await Promise.all(jobs);
    assert.equal(peak, 3);
    await pool.close();
  });

  test('queued tasks start in FIFO order as workers free up', async () => {
    const pool = new TaskPool({ workers: 1, capacity: 5 });
    const started: number[] = [];
    const gates = [deferred(), deferred(), deferred()];
    const jobs = gates.map((gate, i) =>
      pool.run(async () => {
        started.push(i);
        await gate.promise;
      }),
    );
    await tick();
    assert.deepEqual(started, [0]);
    gates[0]?.resolve();
    await tick();
    assert.deepEqual(started, [0, 1]);
    gates[1]?.resolve();
    gates[2]?.resolve();
    await Promise.all(jobs);
    assert.deepEqual(started, [0, 1, 2]);
    await pool.close();
  });
});

describe('TaskPool back-pressure', () => {
  test('rejects with QueueFullError when workers and queue are full, and recovers', async () => {
    const pool = new TaskPool({ workers: 1, capacity: 1 });
    const gate = deferred();
    const running = pool.run(() => gate.promise);
    const queued = pool.run(async () => 'queued');
    assert.equal(pool.active, 1);
    assert.equal(pool.pending, 1);

    await assert.rejects(
      pool.run(async () => 'overflow'),
      QueueFullError,
    );

    gate.resolve();
    await running;
    assert.equal(await queued, 'queued');
    assert.equal(await pool.run(async () => 'again'), 'again');
    await pool.close();
  });

  test('capacity 0 means no waiting at all', async () => {
    const pool = new TaskPool({ workers: 1, capacity: 0 });
    const gate = deferred();
    const running = pool.run(() => gate.promise);
    await assert.rejects(
      pool.run(async () => 1),
      QueueFullError,
    );
    gate.resolve();
    await running;
    await pool.close();
  });
});

describe('TaskPool cancellation', () => {
  test('aborting a queued task removes it and frees its slot', async () => {
    const pool = new TaskPool({ workers: 1, capacity: 1 });
    const gate = deferred();
    const running = pool.run(() => gate.promise);
    const controller = new AbortController();
    let called = false;
    const queued = pool.run(
      async () => {
        called = true;
      },
      { signal: controller.signal },
    );
    const reason = new Error('changed my mind');
    controller.abort(reason);
    await assert.rejects(queued, (err: unknown) => err === reason);
    assert.equal(pool.pending, 0);

    const replacement = pool.run(async () => 'took the slot');
    gate.resolve();
    await running;
    assert.equal(await replacement, 'took the slot');
    assert.equal(called, false);
    await pool.close();
  });

  test('an already aborted signal never runs the task', async () => {
    const pool = new TaskPool({ workers: 2, capacity: 2 });
    let called = false;
    const reason = new Error('too late');
    await assert.rejects(
      pool.run(
        async () => {
          called = true;
        },
        { signal: AbortSignal.abort(reason) },
      ),
      (err: unknown) => err === reason,
    );
    assert.equal(called, false);
    assert.equal(pool.active, 0);
    await pool.close();
  });

  test('the signal reaches a running task, which decides how to stop', async () => {
    const pool = new TaskPool({ workers: 1, capacity: 0 });
    const controller = new AbortController();
    const job = pool.run(
      (signal) =>
        new Promise<string>((_, reject) => {
          signal.addEventListener('abort', () => reject(new Error('task saw abort')));
        }),
      { signal: controller.signal },
    );
    await tick();
    controller.abort();
    await assert.rejects(job, /task saw abort/);
    await pool.close();
  });
});

describe('TaskPool close', () => {
  test('waits for running and queued tasks, then refuses new work', async () => {
    const pool = new TaskPool({ workers: 1, capacity: 2 });
    const done: string[] = [];
    const gate = deferred();
    const a = pool.run(async () => {
      await gate.promise;
      done.push('a');
    });
    const b = pool.run(async () => {
      done.push('b');
    });

    let closed = false;
    const closing = pool.close().then(() => {
      closed = true;
    });
    await tick();
    assert.equal(closed, false, 'close must wait for the running task');
    await assert.rejects(
      pool.run(async () => 'late'),
      PoolClosedError,
    );

    gate.resolve();
    await closing;
    await Promise.all([a, b]);
    assert.deepEqual(done, ['a', 'b']);
    assert.equal(pool.active, 0);
    assert.equal(pool.pending, 0);
  });

  test('close on an idle pool resolves and can be repeated', async () => {
    const pool = new TaskPool({ workers: 2, capacity: 2 });
    await pool.close();
    await pool.close();
  });
});
