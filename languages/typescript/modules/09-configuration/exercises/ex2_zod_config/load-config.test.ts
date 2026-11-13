import assert from 'node:assert/strict';
import { describe, test } from 'node:test';
import { ConfigError, loadConfig } from './load-config.ts';

const valid = { DATABASE_URL: 'postgres://app@db.internal/tracker' };

function issuesOf(env: Record<string, string | undefined>): string[] {
  try {
    loadConfig(env);
  } catch (err) {
    assert.ok(err instanceof ConfigError, 'expected a ConfigError');
    return err.issues;
  }
  assert.fail('expected loadConfig to throw');
}

describe('loadConfig', () => {
  test('applies defaults', () => {
    assert.deepEqual(loadConfig(valid), {
      port: 8080,
      logLevel: 'info',
      environment: 'development',
      databaseUrl: 'postgres://app@db.internal/tracker',
      debug: false,
      allowedOrigins: [],
      apiToken: undefined,
    });
  });

  test('converts strings to typed values', () => {
    const config = loadConfig({
      ...valid,
      PORT: '9090',
      LOG_LEVEL: 'debug',
      ENV: 'test',
      DEBUG: 'yes',
      ALLOWED_ORIGINS: 'https://a.test, https://b.test ,',
      API_TOKEN: 'abcdefghijklmnop',
    });
    assert.equal(config.port, 9090);
    assert.equal(config.logLevel, 'debug');
    assert.equal(config.environment, 'test');
    assert.equal(config.debug, true);
    assert.deepEqual(config.allowedOrigins, ['https://a.test', 'https://b.test']);
    assert.equal(config.apiToken, 'abcdefghijklmnop');
  });

  test('the string "false" is false, not truthy', () => {
    assert.equal(loadConfig({ ...valid, DEBUG: 'false' }).debug, false);
    assert.equal(loadConfig({ ...valid, DEBUG: '0' }).debug, false);
  });

  test('empty strings count as unset', () => {
    const config = loadConfig({ ...valid, PORT: '', LOG_LEVEL: '', API_TOKEN: '' });
    assert.equal(config.port, 8080);
    assert.equal(config.logLevel, 'info');
    assert.equal(config.apiToken, undefined);
  });

  test('reports every problem at once, prefixed with the variable name', () => {
    const issues = issuesOf({ PORT: '99999', LOG_LEVEL: 'verbose', DEBUG: 'maybe' });
    for (const name of ['PORT', 'LOG_LEVEL', 'DEBUG', 'DATABASE_URL']) {
      assert.ok(
        issues.some((issue) => issue.startsWith(`${name}:`)),
        `no issue for ${name}: ${JSON.stringify(issues)}`,
      );
    }
    assert.equal(issues.length, 4);
  });

  test('rejects non-integer and non-numeric ports', () => {
    assert.ok(issuesOf({ ...valid, PORT: '80.5' }).some((i) => i.startsWith('PORT:')));
    assert.ok(issuesOf({ ...valid, PORT: 'abc' }).some((i) => i.startsWith('PORT:')));
  });

  test('DATABASE_URL must be a URL', () => {
    assert.ok(issuesOf({ DATABASE_URL: 'not a url' }).some((i) => i.startsWith('DATABASE_URL:')));
  });

  test('production requires an API token', () => {
    const issues = issuesOf({ ...valid, ENV: 'production' });
    assert.deepEqual(
      issues.map((i) => i.split(':')[0]),
      ['API_TOKEN'],
    );
    assert.equal(
      loadConfig({ ...valid, ENV: 'production', API_TOKEN: 'x'.repeat(16) }).environment,
      'production',
    );
  });

  test('a too-short token is rejected without echoing it', () => {
    const issues = issuesOf({ ...valid, API_TOKEN: 'hunter2' });
    assert.equal(issues.length, 1);
    assert.ok(issues[0]?.startsWith('API_TOKEN:'));
    assert.ok(!issues.join('\n').includes('hunter2'));
  });

  test('the error message lists the issues', () => {
    assert.throws(() => loadConfig({}), /DATABASE_URL/);
  });
});
