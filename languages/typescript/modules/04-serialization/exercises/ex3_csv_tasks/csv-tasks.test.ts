import assert from 'node:assert/strict';
import { test } from 'node:test';
import { type TaskRow, tasksFromCsv, tasksToCsv } from './csv-tasks.ts';

const sample: TaskRow[] = [
  { id: 1, title: 'Write docs', status: 'todo', due_date: '2030-01-31', tags: ['docs', 'p1'] },
  { id: 2, title: 'Say "hi", loudly', status: 'done', due_date: null, tags: [] },
];

test('tasksToCsv writes a header, joins tags with | and leaves null dates empty', () => {
  assert.equal(
    tasksToCsv(sample),
    'id,title,status,due_date,tags\n1,Write docs,todo,2030-01-31,docs|p1\n2,"Say ""hi"", loudly",done,,\n',
  );
});

test('tasksToCsv of no tasks is just the header', () => {
  assert.equal(tasksToCsv([]), 'id,title,status,due_date,tags\n');
});

test('round trip keeps commas, quotes and newlines in titles', () => {
  const tasks: TaskRow[] = [
    ...sample,
    { id: 3, title: 'two\nlines', status: 'in_progress', due_date: '2030-12-31', tags: ['a'] },
  ];
  assert.deepEqual(tasksFromCsv(tasksToCsv(tasks)), { tasks, errors: [] });
});

test('tasksFromCsv converts types and trims titles', () => {
  const { tasks, errors } = tasksFromCsv('id,title,status,due_date,tags\n10,  Padded ,todo,,x|y\n');
  assert.deepEqual(errors, []);
  assert.deepEqual(tasks, [
    { id: 10, title: 'Padded', status: 'todo', due_date: null, tags: ['x', 'y'] },
  ]);
});

test('column order in the header does not matter', () => {
  const { tasks } = tasksFromCsv('tags,status,title,id,due_date\nq,done,T,5,2030-02-01\n');
  assert.deepEqual(tasks, [
    { id: 5, title: 'T', status: 'done', due_date: '2030-02-01', tags: ['q'] },
  ]);
});

test('invalid rows are reported with their row number and the good rows are kept', () => {
  const csv = [
    'id,title,status,due_date,tags',
    '1,Good,todo,,',
    'abc,Bad id,todo,,',
    '3,,todo,,',
    '4,Bad status,finished,,',
    '5,Bad date,todo,2030-02-30,',
    '6,Bad tags,todo,,Docs|docs',
    '7,Also good,done,2030-01-01,a',
    '',
  ].join('\n');
  const { tasks, errors } = tasksFromCsv(csv);
  assert.deepEqual(
    tasks.map((t) => t.id),
    [1, 7],
  );
  assert.deepEqual(
    errors.map((e) => e.row),
    [3, 4, 5, 6, 7],
  );
  const columns = ['id', 'title', 'status', 'due_date', 'tags'];
  errors.forEach((error, i) => {
    assert.ok(error.message.includes(columns[i] ?? ''), `${error.row}: ${error.message}`);
  });
});

test('a header without a required column is an error', () => {
  assert.throws(() => tasksFromCsv('id,title,status,tags\n1,a,todo,\n'), /due_date/);
});
