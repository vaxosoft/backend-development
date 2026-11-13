import assert from 'node:assert/strict';
import { describe, mock, test } from 'node:test';
import { setImmediate as flush } from 'node:timers/promises';
import { FakeClock } from './fake-clock.ts';
import { type DueTask, startReminders } from './reminders.ts';

const MINUTE = 60_000;

function task(id: number, dueInMs: number | null, status: DueTask['status'] = 'todo'): DueTask {
  return { id, title: `task ${id}`, status, dueAt: dueInMs };
}

/** A fresh clock, task list and spies for every test. */
function setup(tasks: DueTask[], notifyImpl?: (t: DueTask) => Promise<void>) {
  const clock = new FakeClock(0);
  const notify = mock.fn(notifyImpl ?? (async (_t: DueTask) => {}));
  const onError = mock.fn((_error: unknown, _task: DueTask) => {});
  const reminders = startReminders({
    clock,
    listTasks: () => tasks,
    notify,
    onError,
    intervalMs: MINUTE,
    leadMs: 10 * MINUTE,
  });
  const notifiedIds = () => notify.mock.calls.map((c) => c.arguments[0].id);
  return { clock, notify, onError, reminders, notifiedIds };
}

describe('startReminders', () => {
  test('does nothing before the first interval has elapsed', async () => {
    const { clock, notify, reminders } = setup([task(1, 5 * MINUTE)]);

    clock.advance(MINUTE - 1);
    await flush();

    assert.equal(notify.mock.callCount(), 0);
    reminders.stop();
  });

  test('notifies a task once it is within the lead window, on the first tick that sees it', async () => {
    const { clock, notifiedIds, reminders } = setup([task(1, 25 * MINUTE)]);

    clock.advance(14 * MINUTE); // 11 minutes left: still too early
    assert.deepEqual(notifiedIds(), []);

    clock.advance(MINUTE); // 10 minutes left
    assert.deepEqual(notifiedIds(), [1]);
    reminders.stop();
  });

  test('notifies each task only once, however many ticks follow', async () => {
    const { clock, notifiedIds, reminders } = setup([task(1, 2 * MINUTE)]);

    clock.advance(10 * MINUTE);
    await flush();

    assert.deepEqual(notifiedIds(), [1]);
    reminders.stop();
  });

  test('includes overdue tasks and skips done, undated and far-away ones', async () => {
    const { clock, notifiedIds, reminders } = setup([
      task(1, -5 * MINUTE),
      task(2, 3 * MINUTE, 'done'),
      task(3, null),
      task(4, 60 * MINUTE),
      task(5, 4 * MINUTE, 'in_progress'),
    ]);

    clock.advance(MINUTE);

    assert.deepEqual(notifiedIds(), [1, 5]);
    reminders.stop();
  });

  test('does not notify a task again while its first notification is still pending', async () => {
    let finish: () => void = () => {};
    const { clock, notify, reminders } = setup(
      [task(1, 0)],
      () => new Promise<void>((resolve) => (finish = resolve)),
    );

    clock.advance(3 * MINUTE);
    assert.equal(notify.mock.callCount(), 1);

    finish();
    await flush();
    clock.advance(3 * MINUTE);
    assert.equal(notify.mock.callCount(), 1);
    reminders.stop();
  });

  test('reports a failed notification, retries it on the next tick and keeps the others going', async () => {
    let attempts = 0;
    const { clock, notify, onError, reminders } = setup([task(1, 0), task(2, 0)], async (t) => {
      if (t.id === 1 && ++attempts === 1) throw new Error('smtp down');
    });

    clock.advance(MINUTE);
    await flush();

    assert.equal(onError.mock.callCount(), 1);
    const [error, failedTask] = onError.mock.calls[0]?.arguments ?? [];
    assert.match(String(error), /smtp down/);
    assert.equal(failedTask?.id, 1);
    assert.deepEqual(
      notify.mock.calls.map((c) => c.arguments[0].id),
      [1, 2],
    );

    clock.advance(MINUTE);
    await flush();

    assert.deepEqual(
      notify.mock.calls.map((c) => c.arguments[0].id),
      [1, 2, 1],
    );
    assert.equal(onError.mock.callCount(), 1);
    reminders.stop();
  });

  test('sees tasks that appear later', async () => {
    const tasks: DueTask[] = [];
    const clock = new FakeClock(0);
    const notify = mock.fn(async (_t: DueTask) => {});
    const reminders = startReminders({
      clock,
      listTasks: () => tasks,
      notify,
      onError: () => {},
      intervalMs: MINUTE,
      leadMs: MINUTE,
    });

    clock.advance(5 * MINUTE);
    tasks.push(task(7, 6 * MINUTE));
    clock.advance(MINUTE);

    assert.equal(notify.mock.callCount(), 1);
    reminders.stop();
  });

  test('stop() ends the loop, and may be called twice', async () => {
    const { clock, notify, reminders } = setup([task(1, 100 * MINUTE)]);

    clock.advance(MINUTE);
    reminders.stop();
    reminders.stop();
    clock.advance(200 * MINUTE);

    assert.equal(notify.mock.callCount(), 0);
  });
});
