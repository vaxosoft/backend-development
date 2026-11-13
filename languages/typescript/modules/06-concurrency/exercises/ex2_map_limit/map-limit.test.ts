import assert from 'node:assert/strict';
import { describe, test } from 'node:test';
import { setTimeout as sleep } from 'node:timers/promises';
import { mapLimit } from './map-limit.ts';

function tracker() {
  let inFlight = 0;
  let peak = 0;
  const started: number[] = [];
  return {
    started,
    get peak() {
      return peak;
    },
    async run<T>(index: number, ms: number, value: T): Promise<T> {
      started.push(index);
      inFlight++;
      peak = Math.max(peak, inFlight);
      await sleep(ms);
      inFlight--;
      return value;
    },
  };
}

describe('mapLimit results', () => {
  test('keeps input order even when later items finish first', async () => {
    const result = await mapLimit([30, 1, 10], 3, async (ms, i) => {
      await sleep(ms);
      return `${i}:${ms}`;
    });
    assert.deepEqual(result, ['0:30', '1:1', '2:10']);
  });

  test('empty input resolves to an empty array without calling fn', async () => {
    let calls = 0;
    assert.deepEqual(
      await mapLimit([], 3, async () => {
        calls++;
      }),
      [],
    );
    assert.equal(calls, 0);
  });

  test('a limit larger than the input is fine', async () => {
    assert.deepEqual(await mapLimit([1, 2], 50, async (n) => n * 2), [2, 4]);
  });

  test('limit must be a positive integer', async () => {
    for (const limit of [0, -1, 1.5, Number.NaN]) {
      await assert.rejects(
        mapLimit([1], limit, async (n) => n),
        RangeError,
        String(limit),
      );
    }
  });
});

describe('mapLimit concurrency', () => {
  test('never exceeds the limit but does use all of it', async () => {
    const t = tracker();
    const items = Array.from({ length: 10 }, (_, i) => i);
    await mapLimit(items, 3, (i) => t.run(i, 4, i));
    assert.equal(t.peak, 3);
    assert.deepEqual(t.started, items);
  });

  test('limit 1 runs strictly one after another', async () => {
    const t = tracker();
    await mapLimit([0, 1, 2, 3], 1, (i) => t.run(i, 2, i));
    assert.equal(t.peak, 1);
  });

  test('starts the next item as soon as a slot frees up, not in batches', async () => {
    const order: string[] = [];
    await mapLimit([40, 5, 5, 5], 2, async (ms, i) => {
      order.push(`start ${i}`);
      await sleep(ms);
      order.push(`end ${i}`);
    });
    assert.deepEqual(order.slice(0, 5), ['start 0', 'start 1', 'end 1', 'start 2', 'end 2']);
  });
});

describe('mapLimit failure and cancellation', () => {
  test('fails fast with the original error and starts nothing new afterwards', async () => {
    const t = tracker();
    const boom = new Error('boom');
    const items = Array.from({ length: 12 }, (_, i) => i);
    await assert.rejects(
      mapLimit(items, 2, async (i) => {
        await t.run(i, 3, i);
        if (i === 2) throw boom;
      }),
      (err: unknown) => err === boom,
    );
    const startedAtRejection = t.started.length;
    await sleep(30);
    assert.equal(t.started.length, startedAtRejection, 'no calls may start after the rejection');
    assert.ok(startedAtRejection < 8, `started ${startedAtRejection} of 12`);
  });

  test('a pre-aborted signal rejects with its reason and never calls fn', async () => {
    const reason = new Error('stop');
    let calls = 0;
    await assert.rejects(
      mapLimit(
        [1, 2],
        2,
        async () => {
          calls++;
        },
        { signal: AbortSignal.abort(reason) },
      ),
      (err: unknown) => err === reason,
    );
    assert.equal(calls, 0);
  });

  test('aborting during the run rejects and stops starting new items', async () => {
    const controller = new AbortController();
    const t = tracker();
    const reason = new Error('enough');
    const items = Array.from({ length: 20 }, (_, i) => i);
    const run = mapLimit(items, 2, (i) => t.run(i, 3, i), { signal: controller.signal });
    setTimeout(() => controller.abort(reason), 8);
    await assert.rejects(run, (err: unknown) => err === reason);
    const startedAtAbort = t.started.length;
    await sleep(30);
    assert.equal(t.started.length, startedAtAbort);
    assert.ok(startedAtAbort < 20);
  });
});
