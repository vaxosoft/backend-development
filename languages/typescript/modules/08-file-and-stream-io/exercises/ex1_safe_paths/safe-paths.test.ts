import assert from 'node:assert/strict';
import { join } from 'node:path';
import { describe, test } from 'node:test';
import { PathEscapeError, resolveInside, sanitizeFilename } from './safe-paths.ts';

describe('resolveInside', () => {
  const root = join('/srv', 'uploads');

  test('returns the absolute path for names inside the root', () => {
    assert.equal(resolveInside(root, 'report.pdf'), join(root, 'report.pdf'));
    assert.equal(resolveInside(root, 'a/b/c.txt'), join(root, 'a', 'b', 'c.txt'));
    assert.equal(resolveInside(root, 'a/../b.txt'), join(root, 'b.txt'));
  });

  test('refuses anything that leaves the root', () => {
    for (const bad of ['../x', '../../etc/passwd', 'a/../../x', '/etc/passwd', '..']) {
      assert.throws(() => resolveInside(root, bad), PathEscapeError, bad);
    }
  });

  test('refuses the root itself', () => {
    assert.throws(() => resolveInside(root, ''), PathEscapeError);
    assert.throws(() => resolveInside(root, '.'), PathEscapeError);
    assert.throws(() => resolveInside(root, 'a/..'), PathEscapeError);
  });

  test('refuses a sibling directory that shares the root as a string prefix', () => {
    assert.throws(() => resolveInside('/srv/up', '../up-evil/x'), PathEscapeError);
  });
});

describe('sanitizeFilename', () => {
  test('keeps ordinary names unchanged', () => {
    assert.equal(sanitizeFilename('report.final.pdf'), 'report.final.pdf');
    assert.equal(sanitizeFilename('my file (1).txt'), 'my file (1).txt');
  });

  test('drops directory parts of both separators', () => {
    assert.equal(sanitizeFilename('../../etc/passwd'), 'passwd');
    assert.equal(sanitizeFilename('C:\\Users\\bob\\evil.exe'), 'evil.exe');
    assert.equal(sanitizeFilename('a/b\\c.txt'), 'c.txt');
  });

  test('removes control characters and surrounding whitespace', () => {
    assert.equal(sanitizeFilename('a\u0000b\nc\u007f.txt'), 'abc.txt');
    assert.equal(sanitizeFilename('  spaced.txt \t'), 'spaced.txt');
  });

  test('never returns an empty or dot-only name', () => {
    for (const name of ['', '   ', '.', '..', 'dir/', '../..', '\u0000']) {
      assert.equal(sanitizeFilename(name), 'unnamed', JSON.stringify(name));
    }
  });

  test('limits the length to 100 characters', () => {
    assert.equal(sanitizeFilename('x'.repeat(300)).length, 100);
  });
});
