import assert from 'node:assert/strict';
import { describe, test } from 'node:test';
import { validateTaskInput } from './validation.ts';

function valid(body: unknown) {
  const result = validateTaskInput(body);
  assert.equal(result.ok, true, JSON.stringify(result));
  if (!result.ok) throw new Error('unreachable');
  return result.value;
}

function fieldsOf(body: unknown): string[] {
  const result = validateTaskInput(body);
  assert.equal(result.ok, false, `expected ${JSON.stringify(body)} to be rejected`);
  return result.ok ? [] : Object.keys(result.fields).sort();
}

describe('valid input', () => {
  test('a title alone is enough; defaults fill the rest', () => {
    assert.deepEqual(valid({ title: 'Write docs' }), {
      title: 'Write docs',
      status: 'todo',
      due_date: null,
      tags: [],
      owner_id: null,
    });
  });

  test('every field is accepted and the title is trimmed', () => {
    assert.deepEqual(
      valid({
        title: '  Ship it  ',
        status: 'in_progress',
        due_date: '2030-02-28',
        tags: ['docs', 'p1'],
        owner_id: 7,
      }),
      {
        title: 'Ship it',
        status: 'in_progress',
        due_date: '2030-02-28',
        tags: ['docs', 'p1'],
        owner_id: 7,
      },
    );
  });

  test('null is a valid due_date and owner_id; unknown fields are dropped', () => {
    const value = valid({ title: 'x', due_date: null, owner_id: null, is_admin: true });
    assert.deepEqual(Object.keys(value).sort(), [
      'due_date',
      'owner_id',
      'status',
      'tags',
      'title',
    ]);
  });

  test('boundary sizes are accepted', () => {
    const tags = Array.from({ length: 10 }, (_, i) => `t${i}`);
    assert.equal(valid({ title: 'x'.repeat(200), tags }).tags.length, 10);
    assert.equal(valid({ title: 'x', tags: ['a'.repeat(30)] }).tags[0]?.length, 30);
    assert.equal(valid({ title: 'x', due_date: '2028-02-29' }).due_date, '2028-02-29');
  });
});

describe('invalid input', () => {
  test('anything that is not an object is rejected under `body`', () => {
    for (const body of [null, undefined, 'text', 12, true, []]) {
      assert.deepEqual(fieldsOf(body), ['body'], JSON.stringify(body));
    }
  });

  test('title rules', () => {
    for (const title of [undefined, '', '   ', 'x'.repeat(201), 42, null]) {
      assert.deepEqual(fieldsOf({ title }), ['title'], JSON.stringify(title));
    }
    assert.deepEqual(fieldsOf({}), ['title']);
  });

  test('status must be one of the three values', () => {
    assert.deepEqual(fieldsOf({ title: 'x', status: 'finished' }), ['status']);
    assert.deepEqual(fieldsOf({ title: 'x', status: 'DONE' }), ['status']);
    assert.deepEqual(fieldsOf({ title: 'x', status: null }), ['status']);
  });

  test('due_date must be a real YYYY-MM-DD date', () => {
    for (const due_date of [
      '2030-02-30',
      '2030-13-01',
      '30-01-2030',
      '2030-1-5',
      '2030-01-31T10:00:00Z',
      20300131,
    ]) {
      assert.deepEqual(fieldsOf({ title: 'x', due_date }), ['due_date'], JSON.stringify(due_date));
    }
  });

  test('tags rules point at the element or the list', () => {
    assert.deepEqual(fieldsOf({ title: 'x', tags: 'docs' }), ['tags']);
    assert.deepEqual(fieldsOf({ title: 'x', tags: ['a', 'a'] }), ['tags']);
    assert.deepEqual(
      fieldsOf({ title: 'x', tags: Array.from({ length: 11 }, (_, i) => `t${i}`) }),
      ['tags'],
    );
    assert.deepEqual(fieldsOf({ title: 'x', tags: ['ok', 'Docs'] }), ['tags.1']);
    assert.deepEqual(fieldsOf({ title: 'x', tags: ['', 'a'.repeat(31), 5] }), [
      'tags.0',
      'tags.1',
      'tags.2',
    ]);
  });

  test('owner_id must be an integer or null', () => {
    for (const owner_id of [1.5, '1', true, Number.NaN]) {
      assert.deepEqual(fieldsOf({ title: 'x', owner_id }), ['owner_id'], String(owner_id));
    }
  });

  test('all problems are reported at once', () => {
    assert.deepEqual(
      fieldsOf({ title: '', status: 'nope', due_date: 'tomorrow', tags: ['A'], owner_id: 'me' }),
      ['due_date', 'owner_id', 'status', 'tags.0', 'title'],
    );
  });

  test('every message is a non-empty string', () => {
    const result = validateTaskInput({ title: '' });
    assert.equal(result.ok, false);
    if (!result.ok) {
      for (const message of Object.values(result.fields)) assert.ok(message.length > 0);
    }
  });
});
