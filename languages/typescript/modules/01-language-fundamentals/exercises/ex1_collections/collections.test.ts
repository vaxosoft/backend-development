import assert from 'node:assert/strict';
import { describe, test } from 'node:test';
import { countBy, sortedNumbers, uniqueTags } from './collections.ts';

describe('sortedNumbers', () => {
  test('sorts numerically, not as strings', () => {
    assert.deepEqual(sortedNumbers([10, 9, 1, 100]), [1, 9, 10, 100]);
  });

  test('does not mutate its input', () => {
    const input = Object.freeze([3, 1, 2]);
    assert.deepEqual(sortedNumbers(input), [1, 2, 3]);
    assert.deepEqual(input, [3, 1, 2]);
  });

  test('handles negatives and empty input', () => {
    assert.deepEqual(sortedNumbers([0, -5, 5, -10]), [-10, -5, 0, 5]);
    assert.deepEqual(sortedNumbers([]), []);
  });
});

describe('uniqueTags', () => {
  test('trims, lowercases and removes duplicates keeping first position', () => {
    assert.deepEqual(uniqueTags(['  Docs', 'P1', 'docs', 'p1 ', 'API']), ['docs', 'p1', 'api']);
  });

  test('drops empty and whitespace-only tags', () => {
    assert.deepEqual(uniqueTags(['', '   ', 'a']), ['a']);
  });
});

describe('countBy', () => {
  test('counts per key in first-seen order', () => {
    const counts = countBy(['apple', 'avocado', 'banana', 'blueberry', 'cherry', 'apricot'], (s) =>
      s.charAt(0),
    );
    assert.deepEqual(
      [...counts],
      [
        ['a', 3],
        ['b', 2],
        ['c', 1],
      ],
    );
  });

  test('works with objects and an empty list', () => {
    const tasks = [{ status: 'todo' }, { status: 'done' }, { status: 'todo' }];
    assert.equal(countBy(tasks, (t) => t.status).get('todo'), 2);
    assert.equal(countBy([], String).size, 0);
  });
});
