import assert from 'node:assert/strict';
import { test } from 'node:test';
import { decodeJson, decodeWithDates, encodeJson } from './json-codec.ts';

test('encodeJson turns bigint into a string instead of throwing', () => {
  assert.equal(encodeJson({ id: 2n ** 63n }), '{"id":"9223372036854775808"}');
});

test('encodeJson writes Set as array and Map as object, also when nested', () => {
  const value = { tags: new Set(['a', 'b']), meta: new Map([['k', new Set([1])]]) };
  assert.equal(encodeJson(value), '{"tags":["a","b"],"meta":{"k":[1]}}');
});

test('encodeJson keeps the default behaviour for everything else', () => {
  const value = {
    when: new Date('2030-01-01T09:00:00Z'),
    skipped: undefined,
    n: null,
    list: [1, 'x'],
  };
  assert.equal(encodeJson(value), '{"when":"2030-01-01T09:00:00.000Z","n":null,"list":[1,"x"]}');
});

test('decodeJson returns the value for valid JSON, including falsy values', () => {
  assert.deepEqual(decodeJson('{"a":[1,2]}'), { ok: true, value: { a: [1, 2] } });
  assert.deepEqual(decodeJson('null'), { ok: true, value: null });
  assert.deepEqual(decodeJson('0'), { ok: true, value: 0 });
});

test('decodeJson reports malformed input instead of throwing', () => {
  for (const text of ['', '{"a":', "{'a':1}", 'undefined']) {
    const result = decodeJson(text);
    assert.equal(result.ok, false, text);
    if (!result.ok) assert.ok(result.error.length > 0);
  }
});

test('decodeWithDates revives *_at strings at any depth and leaves other strings alone', () => {
  const result = decodeWithDates(
    '{"created_at":"2030-01-01T09:00:00Z","title":"2030-01-01T09:00:00Z","nested":{"seen_at":"2031-05-06T07:08:09Z"},"retries_at":5}',
  );
  assert.ok(result.ok);
  const value = result.value as {
    created_at: Date;
    title: string;
    nested: { seen_at: Date };
    retries_at: number;
  };
  assert.ok(value.created_at instanceof Date);
  assert.equal(value.created_at.toISOString(), '2030-01-01T09:00:00.000Z');
  assert.equal(value.title, '2030-01-01T09:00:00Z');
  assert.equal(value.nested.seen_at.getUTCFullYear(), 2031);
  assert.equal(value.retries_at, 5);
});

test('decodeWithDates also reports malformed input', () => {
  assert.equal(decodeWithDates('{').ok, false);
});
