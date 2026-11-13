import assert from 'node:assert/strict';
import { describe, test } from 'node:test';
import { createLogger } from './logger.ts';

function capture() {
  const lines: string[] = [];
  const sink = {
    write(line: string) {
      lines.push(line);
    },
  };
  const records = (): Record<string, unknown>[] => lines.map((l) => JSON.parse(l));
  return { sink, records };
}

describe('createLogger', () => {
  test('emits one JSON line with service, level and msg but no pid/hostname', () => {
    const { sink, records } = capture();
    createLogger({ service: 'task-tracker', sink }).info({ port: 8080 }, 'listening');

    const [line] = records();
    assert.equal(line?.service, 'task-tracker');
    assert.equal(line?.msg, 'listening');
    assert.equal(line?.port, 8080);
    assert.equal(line?.level, 30);
    assert.ok(!('pid' in (line ?? {})));
    assert.ok(!('hostname' in (line ?? {})));
  });

  test('defaults to info and honours an explicit level', () => {
    const quiet = capture();
    createLogger({ service: 's', sink: quiet.sink }).debug('hidden');
    assert.equal(quiet.records().length, 0);

    const verbose = capture();
    createLogger({ service: 's', level: 'debug', sink: verbose.sink }).debug('shown');
    assert.equal(verbose.records().length, 1);

    const errorsOnly = capture();
    const logger = createLogger({ service: 's', level: 'error', sink: errorsOnly.sink });
    logger.warn('hidden');
    logger.error('shown');
    assert.deepEqual(
      errorsOnly.records().map((r) => r.msg),
      ['shown'],
    );
  });

  test('redacts secrets at the top level and one level down', () => {
    const { sink, records } = capture();
    createLogger({ service: 's', sink }).info(
      {
        password: 'p1',
        token: 't1',
        authorization: 'Bearer a',
        user: { name: 'ann', password: 'p2' },
      },
      'login',
    );

    const [line] = records();
    assert.equal(line?.password, '[REDACTED]');
    assert.equal(line?.token, '[REDACTED]');
    assert.equal(line?.authorization, '[REDACTED]');
    assert.deepEqual(line?.user, { name: 'ann', password: '[REDACTED]' });
  });

  test('redacts sensitive request headers but keeps the others', () => {
    const { sink, records } = capture();
    createLogger({ service: 's', sink }).info(
      { req: { headers: { authorization: 'Bearer x', cookie: 'sid=1', accept: 'text/html' } } },
      'incoming',
    );

    assert.deepEqual(records()[0]?.req, {
      headers: { authorization: '[REDACTED]', cookie: '[REDACTED]', accept: 'text/html' },
    });
  });

  test('child loggers keep the base fields and add their own', () => {
    const { sink, records } = capture();
    createLogger({ service: 'svc', sink }).child({ request_id: 'r-1' }).info('hello');

    const [line] = records();
    assert.equal(line?.service, 'svc');
    assert.equal(line?.request_id, 'r-1');
  });

  test('serialises errors under err with type, message and stack', () => {
    const { sink, records } = capture();
    createLogger({ service: 's', sink }).error({ err: new RangeError('boom') }, 'failed');

    const err = records()[0]?.err as { type: string; message: string; stack: string };
    assert.equal(err.type, 'RangeError');
    assert.equal(err.message, 'boom');
    assert.match(err.stack, /RangeError: boom/);
  });
});
