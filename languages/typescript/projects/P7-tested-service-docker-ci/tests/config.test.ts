import assert from 'node:assert/strict';
import { test } from 'node:test';
import { loadConfig } from '../starter/src/config.ts';

test('loadConfig falls back to the documented defaults', () => {
  assert.deepEqual(loadConfig({}), {
    ok: true,
    value: { host: '127.0.0.1', port: 8080, logLevel: 'info', env: 'development' },
  });
});

test('loadConfig reads ADDR, LOG_LEVEL and ENV, and PORT overrides the ADDR port', () => {
  const result = loadConfig({
    ADDR: '0.0.0.0:3000',
    PORT: '9090',
    LOG_LEVEL: 'debug',
    ENV: 'production',
  });
  assert.deepEqual(result, {
    ok: true,
    value: { host: '0.0.0.0', port: 9090, logLevel: 'debug', env: 'production' },
  });
});

test('loadConfig rejects a PORT that is not an integer in range and names the variable', () => {
  for (const bad of ['abc', '70000', '-1', '80.5']) {
    const result = loadConfig({ PORT: bad });
    assert.equal(result.ok, false, `PORT=${bad} should be rejected`);
    if (!result.ok) {
      assert.match(result.errors.join('\n'), new RegExp(`PORT.*"${bad}"`));
    }
  }
});

test('loadConfig reports every invalid variable at once', () => {
  const result = loadConfig({ PORT: 'x', LOG_LEVEL: 'verbose', ENV: 'staging', ADDR: 'nope' });
  assert.equal(result.ok, false);
  if (!result.ok) {
    assert.equal(result.errors.length, 4);
    const text = result.errors.join('\n');
    for (const name of ['PORT', 'LOG_LEVEL', 'ENV', 'ADDR']) {
      assert.ok(text.includes(name), `no message mentions ${name}`);
    }
    assert.ok(
      text.includes('debug, info, warn, error'),
      'LOG_LEVEL message lists the allowed values',
    );
  }
});
