import assert from 'node:assert/strict';
import { describe, test } from 'node:test';
import { parseCreateTask, parsePatchTask } from './task-input.ts';

function fieldsOf(result: ReturnType<typeof parseCreateTask> | ReturnType<typeof parsePatchTask>) {
  assert.equal(result.ok, false, 'expected a validation failure');
  return result.ok ? {} : result.fields;
}

describe('parseCreateTask', () => {
  test('applies defaults and trims the title', () => {
    assert.deepEqual(parseCreateTask({ title: '  Write docs ' }), {
      ok: true,
      value: { title: 'Write docs', status: 'todo', due_date: null, tags: [], owner_id: null },
    });
  });

  test('accepts a fully specified task', () => {
    const body = {
      title: 'Ship',
      status: 'in_progress',
      due_date: '2030-02-28',
      tags: ['docs', 'p1'],
      owner_id: 7,
    };
    assert.deepEqual(parseCreateTask(body), { ok: true, value: body });
  });

  test('rejects bad titles', () => {
    for (const title of ['', '   ', 'x'.repeat(201), 42, null]) {
      assert.ok('title' in fieldsOf(parseCreateTask({ title })), String(title));
    }
    assert.equal(parseCreateTask({ title: 'x'.repeat(200) }).ok, true);
  });

  test('rejects an unknown status and a missing title', () => {
    assert.ok('status' in fieldsOf(parseCreateTask({ title: 'a', status: 'finished' })));
    assert.ok('title' in fieldsOf(parseCreateTask({})));
  });

  test('due_date must be a real calendar date or null', () => {
    assert.equal(parseCreateTask({ title: 'a', due_date: null }).ok, true);
    assert.equal(parseCreateTask({ title: 'a', due_date: '2032-02-29' }).ok, true);
    for (const due_date of ['2030-02-30', '30/01/2030', '2030-01-31T00:00:00Z', 20300131]) {
      assert.ok(
        'due_date' in fieldsOf(parseCreateTask({ title: 'a', due_date })),
        String(due_date),
      );
    }
  });

  test('tags: at most 10, 1-30 chars, lowercase, unique', () => {
    const eleven = Array.from({ length: 11 }, (_, i) => `t${i}`);
    assert.equal(parseCreateTask({ title: 'a', tags: eleven.slice(0, 10) }).ok, true);
    for (const tags of [eleven, ['Docs'], ['a', 'a'], [''], ['x'.repeat(31)], 'docs', [1]]) {
      assert.ok('tags' in fieldsOf(parseCreateTask({ title: 'a', tags })), JSON.stringify(tags));
    }
  });

  test('owner_id must be an integer or null', () => {
    assert.ok('owner_id' in fieldsOf(parseCreateTask({ title: 'a', owner_id: 1.5 })));
    assert.ok('owner_id' in fieldsOf(parseCreateTask({ title: 'a', owner_id: '1' })));
  });

  test('reports every problem at once and names unknown keys', () => {
    const fields = fieldsOf(parseCreateTask({ title: '', status: 'x', admin: true }));
    assert.deepEqual(Object.keys(fields).sort(), ['admin', 'status', 'title']);
    for (const message of Object.values(fields)) assert.ok(message.length > 0);
  });

  test('a body that is not an object is reported under "_"', () => {
    for (const body of [null, 'x', 5, [], undefined]) {
      assert.deepEqual(Object.keys(fieldsOf(parseCreateTask(body))), ['_']);
    }
  });
});

describe('parsePatchTask', () => {
  test('an empty patch is valid and has no keys', () => {
    const result = parsePatchTask({});
    assert.ok(result.ok);
    assert.deepEqual(Object.keys(result.value), []);
  });

  test('absent keys stay absent, no defaults are applied', () => {
    const result = parsePatchTask({ title: ' New ' });
    assert.deepEqual(result, { ok: true, value: { title: 'New' } });
    assert.ok(result.ok && !('status' in result.value) && !('tags' in result.value));
  });

  test('due_date null is kept so the caller can clear the date', () => {
    const result = parsePatchTask({ due_date: null });
    assert.ok(result.ok);
    assert.ok('due_date' in result.value);
    assert.equal(result.value.due_date, null);
  });

  test('values are validated like in create', () => {
    assert.ok('title' in fieldsOf(parsePatchTask({ title: '' })));
    assert.ok('status' in fieldsOf(parsePatchTask({ status: 'nope' })));
    assert.ok('due_date' in fieldsOf(parsePatchTask({ due_date: '2030-13-01' })));
    assert.ok('tags' in fieldsOf(parsePatchTask({ tags: ['A'] })));
  });

  test('owner_id and unknown keys are not patchable', () => {
    assert.deepEqual(Object.keys(fieldsOf(parsePatchTask({ owner_id: 3, id: 1 }))).sort(), [
      'id',
      'owner_id',
    ]);
  });
});
