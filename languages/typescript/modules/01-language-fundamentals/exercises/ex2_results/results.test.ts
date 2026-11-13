import assert from 'node:assert/strict';
import { describe, test } from 'node:test';
import {
  err,
  mapResult,
  ok,
  ParseError,
  parseDraft,
  tryParseJson,
  unwrap,
  ValidationError,
} from './results.ts';

describe('mapResult', () => {
  test('maps the value of a success', () => {
    assert.deepEqual(
      mapResult(ok(2), (n) => n * 10),
      ok(20),
    );
  });

  test('passes a failure through without calling the function', () => {
    const failure = err('boom');
    let called = false;
    const mapped = mapResult(failure, () => {
      called = true;
      return 1;
    });
    assert.equal(mapped, failure);
    assert.equal(called, false);
  });
});

describe('tryParseJson', () => {
  test('returns the parsed value, including falsy ones', () => {
    assert.deepEqual(tryParseJson('{"a":[1,2]}'), ok({ a: [1, 2] }));
    assert.deepEqual(tryParseJson('null'), ok(null));
    assert.deepEqual(tryParseJson('0'), ok(0));
  });

  test('wraps syntax errors in a ParseError that keeps the cause', () => {
    const result = tryParseJson('{oops');
    assert.equal(result.ok, false);
    if (result.ok) return;
    assert.ok(result.error instanceof ParseError);
    assert.ok(result.error instanceof Error);
    assert.equal(result.error.name, 'ParseError');
    assert.ok(result.error.cause instanceof SyntaxError);
  });
});

describe('parseDraft', () => {
  test('accepts a valid draft, trimming the title and defaulting the status', () => {
    assert.deepEqual(
      parseDraft({ title: '  Write docs ' }),
      ok({ title: 'Write docs', status: 'todo' }),
    );
    assert.deepEqual(
      parseDraft({ title: 'x', status: 'done', extra: true }),
      ok({ title: 'x', status: 'done' }),
    );
  });

  test('rejects anything that is not a plain object', () => {
    for (const input of [null, undefined, 'text', 42, ['a']]) {
      const result = parseDraft(input);
      assert.equal(result.ok, false);
      if (!result.ok) assert.deepEqual(Object.keys(result.error.fields), ['body']);
    }
  });

  test('reports every invalid field at once', () => {
    const result = parseDraft({ title: '   ', status: 'finished' });
    assert.equal(result.ok, false);
    if (result.ok) return;
    assert.ok(result.error instanceof ValidationError);
    assert.deepEqual(Object.keys(result.error.fields).sort(), ['status', 'title']);
  });

  test('rejects non-string titles and titles over 200 characters', () => {
    const tooLong = parseDraft({ title: 'x'.repeat(201) });
    const wrongType = parseDraft({ title: 123 });
    assert.equal(tooLong.ok, false);
    assert.equal(wrongType.ok, false);
    assert.equal(parseDraft({ title: 'x'.repeat(200) }).ok, true);
  });
});

describe('unwrap', () => {
  test('returns the value of a success', () => {
    assert.equal(unwrap(ok('fine')), 'fine');
  });

  test('throws the very same error object on failure', () => {
    const failure = new ParseError('nope');
    assert.throws(
      () => unwrap(err(failure)),
      (thrown: unknown) => thrown === failure,
    );
  });
});
