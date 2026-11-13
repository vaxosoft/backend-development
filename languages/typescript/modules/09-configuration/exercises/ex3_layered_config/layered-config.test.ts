import assert from 'node:assert/strict';
import { describe, test } from 'node:test';
import { ConfigError, DEFAULTS, redact, resolveSettings } from './layered-config.ts';

describe('resolveSettings', () => {
  test('with nothing supplied the defaults apply', () => {
    const { settings, sources } = resolveSettings({ env: {}, argv: [] });
    assert.deepEqual(settings, DEFAULTS);
    assert.deepEqual(sources, {
      port: 'default',
      logLevel: 'default',
      dataDir: 'default',
      apiKey: 'default',
    });
  });

  test('the file overrides defaults', () => {
    const file = JSON.stringify({ port: 9000, logLevel: 'warn' });
    const { settings, sources } = resolveSettings({ file, env: {}, argv: [] });
    assert.equal(settings.port, 9000);
    assert.equal(settings.logLevel, 'warn');
    assert.equal(settings.dataDir, './data');
    assert.equal(sources.port, 'file');
    assert.equal(sources.dataDir, 'default');
  });

  test('env overrides the file, flags override env', () => {
    const file = JSON.stringify({ port: 9000, logLevel: 'warn', dataDir: '/file' });
    const env = { APP_PORT: '9100', APP_LOG_LEVEL: 'debug' };
    const { settings, sources } = resolveSettings({
      file,
      env,
      argv: ['--port', '9200', '--data-dir=/flag'],
    });
    assert.equal(settings.port, 9200);
    assert.equal(settings.logLevel, 'debug');
    assert.equal(settings.dataDir, '/flag');
    assert.deepEqual(sources, {
      port: 'flag',
      logLevel: 'env',
      dataDir: 'flag',
      apiKey: 'default',
    });
  });

  test('supports the short -p flag and the api key from env', () => {
    const { settings, sources } = resolveSettings({
      env: { APP_API_KEY: 'k-123' },
      argv: ['-p', '7000'],
    });
    assert.equal(settings.port, 7000);
    assert.equal(settings.apiKey, 'k-123');
    assert.equal(sources.apiKey, 'env');
  });

  test('an empty env var counts as unset', () => {
    const { settings, sources } = resolveSettings({
      env: { APP_PORT: '', APP_LOG_LEVEL: '' },
      argv: [],
    });
    assert.equal(settings.port, 8080);
    assert.equal(sources.port, 'default');
    assert.equal(sources.logLevel, 'default');
  });

  test('invalid values name the key and the layer', () => {
    assert.throws(
      () => resolveSettings({ env: { APP_PORT: '99999' }, argv: [] }),
      (err: unknown) => err instanceof ConfigError && /port \(env\)/.test(err.message),
    );
    assert.throws(
      () => resolveSettings({ env: {}, argv: ['--log-level', 'loud'] }),
      (err: unknown) => err instanceof ConfigError && /logLevel \(flag\)/.test(err.message),
    );
    assert.throws(
      () => resolveSettings({ file: '{"port":"abc"}', env: {}, argv: [] }),
      (err: unknown) => err instanceof ConfigError && /port \(file\)/.test(err.message),
    );
  });

  test('unknown file keys, unknown flags and broken JSON are errors', () => {
    assert.throws(() => resolveSettings({ file: '{"prot":1}', env: {}, argv: [] }), ConfigError);
    assert.throws(() => resolveSettings({ env: {}, argv: ['--prot', '1'] }), ConfigError);
    assert.throws(() => resolveSettings({ file: '{oops', env: {}, argv: [] }), ConfigError);
  });
});

describe('redact', () => {
  test('hides the api key but keeps everything else', () => {
    const redacted = redact({ ...DEFAULTS, apiKey: 'super-secret' });
    assert.equal(redacted.apiKey, '[REDACTED]');
    assert.equal(redacted.port, 8080);
    assert.ok(!JSON.stringify(redacted).includes('super-secret'));
  });

  test('an unset key stays unset and the input is not mutated', () => {
    assert.equal(redact(DEFAULTS).apiKey, undefined);
    const settings = { ...DEFAULTS, apiKey: 'k' };
    redact(settings);
    assert.equal(settings.apiKey, 'k');
  });
});
