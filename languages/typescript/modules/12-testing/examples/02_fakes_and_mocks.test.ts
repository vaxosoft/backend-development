import assert from 'node:assert/strict';
import { describe, mock, test } from 'node:test';
import {
  createMemoryRepo,
  debounce,
  NotFoundError,
  type Notifier,
  type Task,
  TaskService,
} from './_shared.ts';

const NOW = new Date('2030-01-10T12:00:00Z');

/** Fresh collaborators for every test: nothing is shared, so test order cannot matter. */
function setup() {
  const repo = createMemoryRepo(() => NOW); // a fake: a real, simple implementation
  const notifier = { taskCompleted: mock.fn(async (_task: Task) => {}) }; // a mock: records calls
  const service = new TaskService({
    repo,
    notifier: notifier satisfies Notifier,
    clock: () => NOW,
  });
  return { repo, notifier, service };
}

describe('TaskService.complete', () => {
  test('marks the task done and notifies exactly once', async () => {
    const { repo, notifier, service } = setup();
    const { id } = repo.create({ title: 'Write docs' });

    const done = await service.complete(id);

    assert.equal(done.status, 'done');
    assert.equal(repo.get(id)?.status, 'done');
    assert.equal(notifier.taskCompleted.mock.callCount(), 1);
    assert.equal(notifier.taskCompleted.mock.calls[0]?.arguments[0].id, id);
  });

  test('completing a done task again does not notify twice', async () => {
    const { repo, notifier, service } = setup();
    const { id } = repo.create({ title: 'x', status: 'done' });

    await service.complete(id);

    assert.equal(notifier.taskCompleted.mock.callCount(), 0);
  });

  test('unknown ids reject with NotFoundError', async () => {
    const { service } = setup();
    // `await` the assertion: a forgotten await makes the test pass before the promise settles.
    await assert.rejects(service.complete(99), NotFoundError);
  });

  test('a failing notifier surfaces its error', async () => {
    const { repo, notifier, service } = setup();
    const { id } = repo.create({ title: 'x' });
    notifier.taskCompleted.mock.mockImplementationOnce(async () => {
      throw new Error('smtp down');
    });

    await assert.rejects(service.complete(id), /smtp down/);
  });
});

describe('TaskService.overdue', () => {
  test('uses the injected clock, ignoring done and undated tasks', () => {
    const { repo, service } = setup();
    repo.create({ title: 'late', due_date: '2030-01-09' });
    repo.create({ title: 'today', due_date: '2030-01-10' });
    repo.create({ title: 'finished', due_date: '2030-01-01', status: 'done' });
    repo.create({ title: 'no date' });

    assert.deepEqual(
      service.overdue().map((t) => t.title),
      ['late'],
    );
  });
});

describe('spies with t.mock.method', () => {
  test('wrap a real method and keep its behaviour', (t) => {
    const repo = createMemoryRepo();
    const spy = t.mock.method(repo, 'create'); // restored automatically after the test

    repo.create({ title: 'a' });
    repo.create({ title: 'b' });

    assert.equal(spy.mock.callCount(), 2);
    assert.equal(repo.list().total, 2);
  });
});

describe('fake timers', () => {
  test('debounce fires once, after the quiet period, without sleeping', (t) => {
    t.mock.timers.enable({ apis: ['setTimeout'] });
    const save = mock.fn();
    const debounced = debounce(save, 100);

    debounced('draft 1');
    t.mock.timers.tick(60);
    debounced('draft 2');
    t.mock.timers.tick(99);
    assert.equal(save.mock.callCount(), 0);

    t.mock.timers.tick(1);
    assert.equal(save.mock.callCount(), 1);
    assert.deepEqual(save.mock.calls[0]?.arguments, ['draft 2']);
  });
});
