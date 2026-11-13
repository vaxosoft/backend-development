import assert from 'node:assert/strict';
import { describe, test } from 'node:test';
import { LruCache, memoize } from './lru-cache.ts';

describe('LruCache', () => {
  test('stores and returns values', () => {
    const cache = new LruCache<string, number>(2);
    cache.set('a', 1).set('b', 2);
    assert.equal(cache.get('a'), 1);
    assert.equal(cache.get('missing'), undefined);
    assert.equal(cache.size, 2);
  });

  test('evicts the least recently used entry when full', () => {
    const cache = new LruCache<string, number>(2);
    cache.set('a', 1).set('b', 2).set('c', 3);
    assert.equal(cache.has('a'), false);
    assert.deepEqual(
      [...cache],
      [
        ['b', 2],
        ['c', 3],
      ],
    );
  });

  test('get refreshes recency but peek does not', () => {
    const cache = new LruCache<string, number>(2);
    cache.set('a', 1).set('b', 2);
    cache.get('a');
    cache.set('c', 3);
    assert.deepEqual(
      [...cache].map(([key]) => key),
      ['a', 'c'],
    );

    const other = new LruCache<string, number>(2);
    other.set('a', 1).set('b', 2);
    other.peek('a');
    other.set('c', 3);
    assert.deepEqual(
      [...other].map(([key]) => key),
      ['b', 'c'],
    );
  });

  test('updating an existing key refreshes it without growing the cache', () => {
    const cache = new LruCache<string, number>(2);
    cache.set('a', 1).set('b', 2).set('a', 10);
    assert.equal(cache.size, 2);
    assert.deepEqual(
      [...cache],
      [
        ['b', 2],
        ['a', 10],
      ],
    );
  });

  test('can store undefined values and delete keys', () => {
    const cache = new LruCache<string, undefined>(2);
    cache.set('a', undefined);
    assert.equal(cache.has('a'), true);
    assert.equal(cache.delete('a'), true);
    assert.equal(cache.delete('a'), false);
  });

  test('rejects invalid capacities', () => {
    for (const capacity of [0, -1, 1.5, Number.NaN]) {
      assert.throws(() => new LruCache(capacity), RangeError);
    }
  });

  test('works with object keys (reference identity)', () => {
    const key = { id: 1 };
    const cache = new LruCache<object, string>(1);
    cache.set(key, 'x');
    assert.equal(cache.get(key), 'x');
    assert.equal(cache.get({ id: 1 }), undefined);
  });
});

describe('memoize', () => {
  test('calls the function once per distinct argument', () => {
    const calls: number[] = [];
    const square = memoize((n: number) => {
      calls.push(n);
      return n * n;
    }, new LruCache<number, number>(10));
    assert.deepEqual([square(3), square(3), square(4), square(3)], [9, 9, 16, 9]);
    assert.deepEqual(calls, [3, 4]);
  });

  test('recomputes after eviction and caches falsy results', () => {
    let calls = 0;
    const lookup = memoize((key: string) => {
      calls++;
      return key === 'zero' ? 0 : undefined;
    }, new LruCache<string, number | undefined>(1));
    lookup('zero');
    lookup('zero');
    assert.equal(calls, 1);
    lookup('other');
    lookup('zero');
    assert.equal(calls, 3);
  });
});
