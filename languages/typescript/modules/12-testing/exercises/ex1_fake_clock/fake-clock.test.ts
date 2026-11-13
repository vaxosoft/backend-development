import assert from 'node:assert/strict';
import { describe, test } from 'node:test';
import { type Clock, FakeClock } from './fake-clock.ts';

describe('FakeClock', () => {
  test('time only moves when advanced', () => {
    const clock = new FakeClock(1000);
    assert.equal(clock.now(), 1000);
    clock.advance(250);
    assert.equal(clock.now(), 1250);
    clock.advance(0);
    assert.equal(clock.now(), 1250);
  });

  test('a timer fires once its delay has fully elapsed, not before', () => {
    const clock = new FakeClock();
    const fired: number[] = [];
    clock.setTimeout(() => fired.push(clock.now()), 100);

    clock.advance(99);
    assert.deepEqual(fired, []);
    clock.advance(1);
    assert.deepEqual(fired, [100]);
    clock.advance(1000);
    assert.deepEqual(fired, [100]);
  });

  test('timers run in chronological order, ties in scheduling order, seeing their own due time', () => {
    const clock = new FakeClock(10);
    const log: string[] = [];
    clock.setTimeout(() => log.push(`c@${clock.now()}`), 50);
    clock.setTimeout(() => log.push(`a@${clock.now()}`), 20);
    clock.setTimeout(() => log.push(`b@${clock.now()}`), 20);

    clock.advance(100);

    assert.deepEqual(log, ['a@30', 'b@30', 'c@60']);
    assert.equal(clock.now(), 110);
  });

  test('timers scheduled by a callback run in the same advance when they are due', () => {
    const clock = new FakeClock();
    const log: number[] = [];
    clock.setTimeout(() => {
      log.push(clock.now());
      clock.setTimeout(() => log.push(clock.now()), 30);
      clock.setTimeout(() => log.push(clock.now()), 500);
    }, 10);

    clock.advance(100);

    assert.deepEqual(log, [10, 40]);
    assert.equal(clock.pending(), 1);
    clock.advance(410);
    assert.deepEqual(log, [10, 40, 510]);
  });

  test('clearTimeout cancels; unknown and fired handles are ignored', () => {
    const clock = new FakeClock();
    let calls = 0;
    const cancelled = clock.setTimeout(() => calls++, 10);
    const fired = clock.setTimeout(() => calls++, 5);
    assert.equal(clock.pending(), 2);

    clock.clearTimeout(cancelled);
    clock.advance(10);
    clock.clearTimeout(fired);
    clock.clearTimeout(9999);

    assert.equal(calls, 1);
    assert.equal(clock.pending(), 0);
  });

  test('handles are unique', () => {
    const clock = new FakeClock();
    const handles = new Set([1, 2, 3].map(() => clock.setTimeout(() => {}, 1)));
    assert.equal(handles.size, 3);
  });

  test('rejects negative and non-finite advances', () => {
    const clock = new FakeClock();
    assert.throws(() => clock.advance(-1), RangeError);
    assert.throws(() => clock.advance(Number.NaN), RangeError);
    assert.throws(() => clock.advance(Number.POSITIVE_INFINITY), RangeError);
  });
});

// The reason to have a fake clock: code written against the `Clock` interface runs instantly in tests.
function debounce(clock: Clock, fn: (value: string) => void, waitMs: number) {
  let handle: number | undefined;
  return (value: string) => {
    if (handle !== undefined) clock.clearTimeout(handle);
    handle = clock.setTimeout(() => fn(value), waitMs);
  };
}

describe('a debouncer driven by FakeClock', () => {
  test('saves once, with the last value, after the quiet period', () => {
    const clock = new FakeClock();
    const saved: string[] = [];
    const save = debounce(clock, (v) => saved.push(v), 300);

    save('d1');
    clock.advance(200);
    save('d2');
    clock.advance(200);
    assert.deepEqual(saved, []);
    clock.advance(100);
    assert.deepEqual(saved, ['d2']);
  });
});
