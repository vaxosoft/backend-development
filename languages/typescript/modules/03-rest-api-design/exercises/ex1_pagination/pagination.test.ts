import assert from 'node:assert/strict';
import { describe, test } from 'node:test';
import { paginate, parsePageParams } from './pagination.ts';

function parsed(query: Record<string, string | string[]>): { limit: number; offset: number } {
  const result = parsePageParams(query);
  assert.ok(result.ok, JSON.stringify(query));
  return result.value;
}

describe('parsePageParams', () => {
  test('uses defaults when nothing is given', () => {
    assert.deepEqual(parsed({}), { limit: 20, offset: 0 });
  });

  test('reads valid values', () => {
    assert.deepEqual(parsed({ limit: '5', offset: '10' }), { limit: 5, offset: 10 });
    assert.deepEqual(parsed({ limit: '100', offset: '0' }), { limit: 100, offset: 0 });
  });

  test('clamps a limit above 100 instead of rejecting it', () => {
    assert.deepEqual(parsed({ limit: '101' }), { limit: 100, offset: 0 });
    assert.deepEqual(parsed({ limit: '99999' }), { limit: 100, offset: 0 });
  });

  test('rejects values that are not whole numbers', () => {
    for (const limit of ['', 'abc', '1.5', '-1', ' 5', '1e3', '0x10', '0']) {
      const result = parsePageParams({ limit });
      assert.equal(result.ok, false, `limit=${JSON.stringify(limit)}`);
      if (!result.ok) assert.match(result.message, /limit/);
    }
  });

  test('rejects a bad offset and names it', () => {
    for (const offset of ['', 'x', '-5', '2.0']) {
      const result = parsePageParams({ offset });
      assert.equal(result.ok, false, `offset=${JSON.stringify(offset)}`);
      if (!result.ok) assert.match(result.message, /offset/);
    }
  });

  test('rejects repeated parameters', () => {
    assert.equal(parsePageParams({ limit: ['1', '2'] }).ok, false);
    assert.equal(parsePageParams({ offset: ['1', '2'] }).ok, false);
  });
});

describe('paginate', () => {
  const items = [1, 2, 3, 4, 5];

  test('returns the requested slice with the envelope fields', () => {
    assert.deepEqual(paginate(items, { limit: 2, offset: 1 }), {
      items: [2, 3],
      total: 5,
      limit: 2,
      offset: 1,
    });
  });

  test('the last page may be short and an offset past the end is empty', () => {
    assert.deepEqual(paginate(items, { limit: 2, offset: 4 }).items, [5]);
    const beyond = paginate(items, { limit: 2, offset: 50 });
    assert.deepEqual(beyond.items, []);
    assert.equal(beyond.total, 5);
  });

  test('does not modify its input', () => {
    const frozen = Object.freeze([...items]);
    assert.deepEqual(paginate(frozen, { limit: 10, offset: 0 }).items, items);
  });
});
