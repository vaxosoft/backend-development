import assert from 'node:assert/strict';
import { describe, test } from 'node:test';
import { setTimeout as sleep } from 'node:timers/promises';
import { collect, firstSuccess, TimeoutError, withTimeout } from './settle-all.ts';

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (error: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

const timers = () => process.getActiveResourcesInfo().filter((r) => r === 'Timeout').length;

describe('collect', () => {
  test('keeps successes in input order even when they finish out of order', async () => {
    const result = await collect([
      async () => {
        await sleep(15);
        return 'a';
      },
      async () => 'b',
      async () => {
        await sleep(5);
        return 'c';
      },
    ]);
    assert.deepEqual(result, { values: ['a', 'b', 'c'], errors: [] });
  });

  test('starts every task before any of them finishes', async () => {
    const gates = [deferred<number>(), deferred<number>(), deferred<number>()];
    let started = 0;
    const pending = collect(
      gates.map((gate) => () => {
        started++;
        return gate.promise;
      }),
    );
    assert.equal(started, 3, 'all tasks must be started synchronously');
    gates.forEach((gate, i) => {
      gate.resolve(i);
    });
    assert.deepEqual((await pending).values, [0, 1, 2]);
  });

  test('reports failures with their index and still returns the successes', async () => {
    const boom = new Error('boom');
    const result = await collect([
      async () => 1,
      async () => {
        throw boom;
      },
      async () => 3,
    ]);
    assert.deepEqual(result.values, [1, 3]);
    assert.deepEqual(result.errors, [{ index: 1, error: boom }]);
  });

  test('a task that throws synchronously is an error entry, not an exception', async () => {
    const result = await collect([
      () => {
        throw new Error('sync');
      },
      async () => 'ok',
    ]);
    assert.deepEqual(result.values, ['ok']);
    assert.equal(result.errors[0]?.index, 0);
  });

  test('no tasks gives empty results', async () => {
    assert.deepEqual(await collect([]), { values: [], errors: [] });
  });
});

describe('firstSuccess', () => {
  test('returns the first success and skips earlier failures', async () => {
    const value = await firstSuccess([
      async () => {
        throw new Error('fast failure');
      },
      async () => {
        await sleep(20);
        return 'slow';
      },
      async () => {
        await sleep(5);
        return 'quick';
      },
    ]);
    assert.equal(value, 'quick');
  });

  test('rejects with an AggregateError when everything fails', async () => {
    await assert.rejects(
      firstSuccess([
        async () => {
          throw new Error('one');
        },
        async () => {
          throw new Error('two');
        },
      ]),
      (err: unknown) => {
        assert.ok(err instanceof AggregateError);
        assert.deepEqual(
          err.errors.map((e: Error) => e.message),
          ['one', 'two'],
        );
        return true;
      },
    );
  });

  test('no tasks is a failure too', async () => {
    await assert.rejects(firstSuccess([]), AggregateError);
  });
});

describe('withTimeout', () => {
  test('passes the value through when the work is fast enough', async () => {
    assert.equal(await withTimeout(sleep(2, 'done'), 1000), 'done');
  });

  test('passes the rejection through', async () => {
    await assert.rejects(
      withTimeout(Promise.reject(new Error('inner')), 1000),
      (err: unknown) => err instanceof Error && err.message === 'inner',
    );
  });

  test('rejects with TimeoutError when the work is too slow', async () => {
    await assert.rejects(withTimeout(new Promise<never>(() => {}), 10), (err: unknown) => {
      assert.ok(err instanceof TimeoutError);
      assert.match(err.message, /10 ms/);
      return true;
    });
  });

  test('does not leave a timer running after the work settled', async () => {
    const before = timers();
    await withTimeout(Promise.resolve('x'), 5000);
    assert.equal(timers(), before);
  });
});
