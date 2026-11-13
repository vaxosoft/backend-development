import assert from 'node:assert/strict';
import { describe, test } from 'node:test';
import { normalizeTags, parseDueDate, ValidationError } from './_shared.ts';

describe('normalizeTags', () => {
  // Table-driven: one row per behaviour, one generated test per row.
  const cases: { name: string; input: string[]; want: string[] }[] = [
    { name: 'keeps clean tags', input: ['docs', 'p1'], want: ['docs', 'p1'] },
    { name: 'lowercases and trims', input: ['  Docs ', 'P1'], want: ['docs', 'p1'] },
    { name: 'drops empty tags', input: ['a', '', '   '], want: ['a'] },
    { name: 'removes duplicates, first one wins', input: ['b', 'a', 'B'], want: ['b', 'a'] },
    { name: 'accepts no tags', input: [], want: [] },
  ];

  for (const { name, input, want } of cases) {
    test(name, () => {
      assert.deepEqual(normalizeTags(input), want);
    });
  }

  test('rejects a tag longer than 30 characters', () => {
    assert.throws(
      () => normalizeTags(['x'.repeat(31)]),
      (err) =>
        err instanceof ValidationError && err.fields.tags === 'each tag must be 1-30 characters',
    );
  });

  test('rejects more than 10 distinct tags but not 10 duplicates', () => {
    const eleven = Array.from({ length: 11 }, (_, i) => `t${i}`);
    assert.throws(() => normalizeTags(eleven), ValidationError);
    assert.equal(normalizeTags(Array(11).fill('same')).length, 1);
  });

  test('does not mutate its input', () => {
    const input = ['B', 'a'];
    normalizeTags(input);
    assert.deepEqual(input, ['B', 'a']);
  });
});

describe('parseDueDate', () => {
  const cases: [text: string, want: string | null][] = [
    ['2030-01-31', '2030-01-31'],
    ['2028-02-29', '2028-02-29'], // leap year
    ['2030-02-29', null], // not a leap year
    ['2030-13-01', null],
    ['2030-1-5', null],
    ['31/01/2030', null],
    ['', null],
  ];

  for (const [text, want] of cases) {
    test(`${JSON.stringify(text)} -> ${want}`, () => {
      assert.equal(parseDueDate(text), want);
    });
  }
});
