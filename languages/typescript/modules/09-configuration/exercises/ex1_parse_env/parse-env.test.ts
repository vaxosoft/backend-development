import assert from 'node:assert/strict';
import { describe, test } from 'node:test';
import { ConfigError, parseBoolean, parseDuration, parseInteger, parseList } from './parse-env.ts';

describe('parseBoolean', () => {
  test('understands the usual words in any case', () => {
    for (const word of ['1', 'true', 'TRUE', ' Yes ', 'on']) {
      assert.equal(parseBoolean('DEBUG', word, false), true, word);
    }
    for (const word of ['0', 'false', 'False', 'no', ' OFF']) {
      assert.equal(parseBoolean('DEBUG', word, true), false, word);
    }
  });

  test('uses the fallback when unset or empty', () => {
    assert.equal(parseBoolean('DEBUG', undefined, true), true);
    assert.equal(parseBoolean('DEBUG', '', false), false);
  });

  test('rejects anything else and names the variable', () => {
    assert.throws(
      () => parseBoolean('LOG_JSON', 'maybe', false),
      (err: unknown) => {
        assert.ok(err instanceof ConfigError);
        assert.match(err.message, /LOG_JSON/);
        assert.match(err.message, /maybe/);
        return true;
      },
    );
  });
});

describe('parseInteger', () => {
  test('parses strict integers', () => {
    assert.equal(parseInteger('PORT', '8080'), 8080);
    assert.equal(parseInteger('OFFSET', ' -5 '), -5);
    assert.equal(parseInteger('PORT', '0'), 0);
  });

  test('uses the fallback, or complains when there is none', () => {
    assert.equal(parseInteger('PORT', undefined, { fallback: 3000 }), 3000);
    assert.equal(parseInteger('PORT', '', { fallback: 0 }), 0);
    assert.throws(() => parseInteger('PORT', undefined), /PORT is required/);
  });

  test('rejects junk that Number() or parseInt() would let through', () => {
    for (const junk of ['8080abc', '2.5', '1e3', '0x10', ' ', 'NaN', '--1']) {
      assert.throws(() => parseInteger('PORT', junk, { fallback: 1 }), ConfigError, junk);
    }
  });

  test('enforces min and max', () => {
    assert.equal(parseInteger('PORT', '65535', { min: 1, max: 65535 }), 65535);
    assert.throws(() => parseInteger('PORT', '0', { min: 1, max: 65535 }), /PORT/);
    assert.throws(() => parseInteger('PORT', '65536', { min: 1, max: 65535 }), /PORT/);
  });
});

describe('parseDuration', () => {
  test('converts units to milliseconds', () => {
    assert.equal(parseDuration('TIMEOUT', '500ms', 0), 500);
    assert.equal(parseDuration('TIMEOUT', '2s', 0), 2000);
    assert.equal(parseDuration('TIMEOUT', '5m', 0), 300_000);
    assert.equal(parseDuration('TIMEOUT', ' 1h ', 0), 3_600_000);
  });

  test('falls back when unset', () => {
    assert.equal(parseDuration('TIMEOUT', undefined, 1234), 1234);
    assert.equal(parseDuration('TIMEOUT', '', 99), 99);
  });

  test('a bare number or unknown text is an error naming the variable', () => {
    for (const bad of ['30', '2.5s', '1d', 's', '-1s', 'fast']) {
      assert.throws(() => parseDuration('TIMEOUT', bad, 0), /TIMEOUT/, bad);
    }
  });
});

describe('parseList', () => {
  test('splits on commas, trims and drops empty items', () => {
    assert.deepEqual(parseList('docs, p1 ,,urgent'), ['docs', 'p1', 'urgent']);
    assert.deepEqual(parseList('single'), ['single']);
  });

  test('unset or blank means an empty list', () => {
    assert.deepEqual(parseList(undefined), []);
    assert.deepEqual(parseList(''), []);
    assert.deepEqual(parseList(' , '), []);
  });
});
