import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { access, mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { afterEach, beforeEach, describe, test } from 'node:test';
import { copyWithSha256 } from './copy-with-hash.ts';

describe('copyWithSha256', () => {
  let dir = '';
  beforeEach(async () => {
    dir = await mkdtemp(join(tmpdir(), 'm08-ex2-'));
  });
  afterEach(async () => {
    await rm(dir, { recursive: true, force: true });
  });

  test('copies the bytes and reports size and hash', async () => {
    const data = Buffer.alloc(3 * 1024 * 1024 + 17);
    for (let i = 0; i < data.length; i++) {
      data[i] = (i * 31 + (i >> 8)) & 0xff;
    }
    await writeFile(join(dir, 'in.bin'), data);

    const result = await copyWithSha256(join(dir, 'in.bin'), join(dir, 'out.bin'));

    assert.equal(result.bytes, data.length);
    assert.equal(result.sha256, createHash('sha256').update(data).digest('hex'));
    assert.deepEqual(await readFile(join(dir, 'out.bin')), data);
  });

  test('handles an empty file', async () => {
    await writeFile(join(dir, 'empty'), '');
    const result = await copyWithSha256(join(dir, 'empty'), join(dir, 'copy'));
    assert.equal(result.bytes, 0);
    assert.equal(result.sha256, createHash('sha256').digest('hex'));
  });

  test('overwrites an existing destination', async () => {
    await writeFile(join(dir, 'in.txt'), 'new');
    await writeFile(join(dir, 'out.txt'), 'old old old');
    await copyWithSha256(join(dir, 'in.txt'), join(dir, 'out.txt'));
    assert.equal(await readFile(join(dir, 'out.txt'), 'utf8'), 'new');
  });

  test('rejects with ENOENT for a missing source and leaves no destination behind', async () => {
    await assert.rejects(
      copyWithSha256(join(dir, 'missing.txt'), join(dir, 'out.txt')),
      (err: unknown) => (err as NodeJS.ErrnoException).code === 'ENOENT',
    );
    await assert.rejects(access(join(dir, 'out.txt')), { code: 'ENOENT' });
  });
});
