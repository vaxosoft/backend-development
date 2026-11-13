import assert from 'node:assert/strict';
import { describe, test } from 'node:test';
import { describeBuild, parseBuildInfo, satisfiesNode } from './build-info.ts';

const pkg = (overrides: Record<string, unknown> = {}) =>
  JSON.stringify({
    name: 'task-tracker',
    version: '1.4.0',
    engines: { node: '>=24' },
    ...overrides,
  });

describe('parseBuildInfo', () => {
  test('reads name, version and engine range', () => {
    assert.deepEqual(parseBuildInfo(pkg(), {}), {
      name: 'task-tracker',
      version: '1.4.0',
      commit: 'unknown',
      nodeRange: '>=24',
    });
  });

  test('shortens GIT_COMMIT to 7 lowercase characters', () => {
    const full = '3F9C2AB1D4E5F60718293A4B5C6D7E8F90123456';
    assert.equal(parseBuildInfo(pkg(), { GIT_COMMIT: full }).commit, '3f9c2ab');
    assert.equal(parseBuildInfo(pkg(), { GIT_COMMIT: ' abcdef0\n' }).commit, 'abcdef0');
  });

  test('a missing or malformed GIT_COMMIT is "unknown"', () => {
    for (const value of [undefined, '', 'abc12', 'not-a-hash!', 'zzzzzzz']) {
      assert.equal(parseBuildInfo(pkg(), { GIT_COMMIT: value }).commit, 'unknown', String(value));
    }
  });

  test('accepts pre-release versions', () => {
    assert.equal(parseBuildInfo(pkg({ version: '2.0.0-rc.1' }), {}).version, '2.0.0-rc.1');
  });

  test('rejects text that is not JSON and keeps the cause', () => {
    assert.throws(
      () => parseBuildInfo('{nope', {}),
      (error: unknown) => error instanceof Error && error.cause instanceof SyntaxError,
    );
  });

  test('rejects anything that is not a valid package manifest', () => {
    assert.throws(() => parseBuildInfo('[]', {}), Error);
    assert.throws(() => parseBuildInfo('null', {}), Error);
    assert.throws(() => parseBuildInfo(pkg({ name: '' }), {}), Error);
    assert.throws(() => parseBuildInfo(pkg({ name: 42 }), {}), Error);
    assert.throws(() => parseBuildInfo(pkg({ version: '1.4' }), {}), Error);
    assert.throws(() => parseBuildInfo(pkg({ version: 'v1.4.0' }), {}), Error);
    assert.throws(() => parseBuildInfo(pkg({ engines: undefined }), {}), Error);
    assert.throws(() => parseBuildInfo(pkg({ engines: { npm: '>=10' } }), {}), Error);
  });
});

describe('describeBuild', () => {
  test('includes the commit only when it is known', () => {
    const info = { name: 'task-tracker', version: '1.4.0', commit: '3f9c2ab', nodeRange: '>=24' };
    assert.equal(describeBuild(info), 'task-tracker 1.4.0 (3f9c2ab)');
    assert.equal(describeBuild({ ...info, commit: 'unknown' }), 'task-tracker 1.4.0');
  });
});

describe('satisfiesNode', () => {
  test('compares numerically, not as strings', () => {
    assert.equal(satisfiesNode('>=24', '26.13.0'), true);
    assert.equal(satisfiesNode('>=24', 'v24.0.0'), true);
    assert.equal(satisfiesNode('>=24', '22.11.0'), false);
    assert.equal(satisfiesNode('>=9', '10.0.0'), true);
    assert.equal(satisfiesNode('>=24.10', '24.9.5'), false);
  });

  test('minor and patch parts count when given', () => {
    assert.equal(satisfiesNode('>=24.2', '24.2.0'), true);
    assert.equal(satisfiesNode('>=24.2.1', '24.2.0'), false);
    assert.equal(satisfiesNode('>=24.2.1', '24.3.0'), true);
    assert.equal(satisfiesNode('>=24.2.1', '25.0.0'), true);
  });

  test('unsupported ranges and malformed versions throw', () => {
    assert.throws(() => satisfiesNode('^24', '24.0.0'), Error);
    assert.throws(() => satisfiesNode('>=24 <26', '24.0.0'), Error);
    assert.throws(() => satisfiesNode('>=24', 'latest'), Error);
  });
});
