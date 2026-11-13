import assert from 'node:assert/strict';
import { describe, test } from 'node:test';
import { setTimeout as sleep } from 'node:timers/promises';
import {
  createContextLogger,
  currentRequestId,
  resolveRequestId,
  runWithRequestContext,
} from './request-context.ts';

describe('resolveRequestId', () => {
  const generate = () => 'generated-id';

  test('reuses a well-formed client id', () => {
    assert.equal(resolveRequestId('abc-123_X.y', generate), 'abc-123_X.y');
  });

  test('generates an id when the header is missing or empty', () => {
    assert.equal(resolveRequestId(undefined, generate), 'generated-id');
    assert.equal(resolveRequestId('', generate), 'generated-id');
  });

  test('rejects ids that could forge or break log lines', () => {
    for (const hostile of ['has space', 'line\nbreak', 'quote"', 'tab\t', 'a'.repeat(65), '<b>']) {
      assert.equal(resolveRequestId(hostile, generate), 'generated-id', JSON.stringify(hostile));
    }
    assert.equal(resolveRequestId('a'.repeat(64), generate), 'a'.repeat(64));
  });

  test('generates a distinct non-empty id by default', () => {
    const a = resolveRequestId(undefined);
    const b = resolveRequestId(undefined);
    assert.ok(a.length > 0);
    assert.notEqual(a, b);
  });
});

describe('request context', () => {
  test('is undefined outside a context', () => {
    assert.equal(currentRequestId(), undefined);
  });

  test('survives awaits and nested calls', async () => {
    async function deep(): Promise<string | undefined> {
      await sleep(1);
      return currentRequestId();
    }
    const seen = await runWithRequestContext('req-1', async () => {
      await sleep(1);
      return deep();
    });
    assert.equal(seen, 'req-1');
    assert.equal(currentRequestId(), undefined);
  });

  test('keeps concurrent requests apart', async () => {
    const run = (id: string, delayMs: number) =>
      runWithRequestContext(id, async () => {
        await sleep(delayMs);
        return currentRequestId();
      });

    const ids = await Promise.all([run('slow', 8), run('fast', 1), run('medium', 4)]);
    assert.deepEqual(ids, ['slow', 'fast', 'medium']);
  });

  test('inner contexts shadow outer ones and restore them afterwards', () => {
    runWithRequestContext('outer', () => {
      runWithRequestContext('inner', () => assert.equal(currentRequestId(), 'inner'));
      assert.equal(currentRequestId(), 'outer');
    });
  });
});

describe('createContextLogger', () => {
  function capture() {
    const lines: string[] = [];
    const sink = {
      write(line: string) {
        lines.push(line);
      },
    };
    return { sink, records: (): Record<string, unknown>[] => lines.map((l) => JSON.parse(l)) };
  }

  test('adds request_id inside a context and omits it outside', () => {
    const { sink, records } = capture();
    const log = createContextLogger(sink);

    log.info('before');
    runWithRequestContext('req-9', () => log.info('during'));
    log.info('after');

    const [before, during, after] = records();
    assert.ok(!('request_id' in (before ?? {})));
    assert.equal(during?.request_id, 'req-9');
    assert.equal(during?.msg, 'during');
    assert.ok(!('request_id' in (after ?? {})));
  });

  test('lines from interleaved requests carry their own ids', async () => {
    const { sink, records } = capture();
    const log = createContextLogger(sink);

    async function handle(id: string, delayMs: number) {
      await runWithRequestContext(id, async () => {
        log.info(`${id} start`);
        await sleep(delayMs);
        log.info(`${id} end`);
      });
    }
    await Promise.all([handle('a', 6), handle('b', 1)]);

    for (const line of records()) {
      assert.equal(String(line.msg).split(' ')[0], line.request_id);
    }
    assert.equal(records().length, 4);
  });
});
