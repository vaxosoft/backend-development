import assert from 'node:assert/strict';
import { describe, test } from 'node:test';
import { type Clock, runFixedRate, systemClock } from './fixed-rate.ts';

// Virtual time: sleeping advances the clock instantly, so no test waits for real time.
function fakeClock() {
  let time = 0;
  const clock: Clock = {
    now: () => time,
    async sleep(ms, signal) {
      signal.throwIfAborted();
      time += ms;
    },
  };
  const advance = (ms: number) => {
    time += ms;
  };
  return { clock, advance, now: () => time };
}

describe('runFixedRate', () => {
  test('starts immediately and does not drift when the task takes time', async () => {
    const fake = fakeClock();
    const controller = new AbortController();
    const starts: number[] = [];
    await runFixedRate({
      intervalMs: 100,
      clock: fake.clock,
      signal: controller.signal,
      task: () => {
        starts.push(fake.now());
        fake.advance(30);
        if (starts.length === 4) controller.abort();
      },
    });
    assert.deepEqual(starts, [0, 100, 200, 300]);
  });

  test('instant tasks still run once per interval', async () => {
    const fake = fakeClock();
    const controller = new AbortController();
    const starts: number[] = [];
    await runFixedRate({
      intervalMs: 50,
      clock: fake.clock,
      signal: controller.signal,
      task: () => {
        starts.push(fake.now());
        if (starts.length === 3) controller.abort();
      },
    });
    assert.deepEqual(starts, [0, 50, 100]);
  });

  test('skips missed ticks after an overrun instead of bursting', async () => {
    const fake = fakeClock();
    const controller = new AbortController();
    const starts: number[] = [];
    const durations = [250, 10, 10];
    await runFixedRate({
      intervalMs: 100,
      clock: fake.clock,
      signal: controller.signal,
      task: () => {
        starts.push(fake.now());
        fake.advance(durations[starts.length - 1] ?? 0);
        if (starts.length === 3) controller.abort();
      },
    });
    assert.deepEqual(starts, [0, 300, 400]);
  });

  test('runs never overlap and async tasks are awaited', async () => {
    const fake = fakeClock();
    const controller = new AbortController();
    let running = 0;
    let maxRunning = 0;
    let runs = 0;
    await runFixedRate({
      intervalMs: 10,
      clock: fake.clock,
      signal: controller.signal,
      task: async () => {
        running += 1;
        maxRunning = Math.max(maxRunning, running);
        await Promise.resolve();
        fake.advance(25);
        running -= 1;
        runs += 1;
        if (runs === 3) controller.abort();
      },
    });
    assert.equal(maxRunning, 1);
    assert.equal(runs, 3);
  });

  test('a failing task is reported and the schedule continues', async () => {
    const fake = fakeClock();
    const controller = new AbortController();
    const errors: unknown[] = [];
    let runs = 0;
    await runFixedRate({
      intervalMs: 10,
      clock: fake.clock,
      signal: controller.signal,
      onError: (error) => errors.push(error),
      task: () => {
        runs += 1;
        if (runs === 1) throw new Error('boom');
        if (runs === 3) controller.abort();
      },
    });
    assert.equal(runs, 3);
    assert.equal(errors.length, 1);
    assert.equal((errors[0] as Error).message, 'boom');
  });

  test('an already aborted signal runs nothing and resolves', async () => {
    const fake = fakeClock();
    const controller = new AbortController();
    controller.abort();
    let runs = 0;
    await runFixedRate({
      intervalMs: 10,
      clock: fake.clock,
      signal: controller.signal,
      task: () => {
        runs += 1;
      },
    });
    assert.equal(runs, 0);
  });

  test('aborting while sleeping resolves the promise (no AbortError escapes)', async () => {
    const controller = new AbortController();
    let runs = 0;
    const done = runFixedRate({
      intervalMs: 60_000,
      clock: systemClock,
      signal: controller.signal,
      task: () => {
        runs += 1;
        setImmediate(() => controller.abort());
      },
    });
    await done;
    assert.equal(runs, 1);
  });

  test('works with the real clock', async () => {
    const controller = new AbortController();
    let runs = 0;
    await runFixedRate({
      intervalMs: 5,
      clock: systemClock,
      signal: controller.signal,
      task: () => {
        runs += 1;
        if (runs === 3) controller.abort();
      },
    });
    assert.equal(runs, 3);
  });
});
