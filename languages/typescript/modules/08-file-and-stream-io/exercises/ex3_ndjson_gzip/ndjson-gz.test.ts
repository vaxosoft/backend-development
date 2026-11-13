import assert from 'node:assert/strict';
import { mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { afterEach, beforeEach, describe, test } from 'node:test';
import { gunzipSync, gzipSync } from 'node:zlib';
import { forEachNdjsonGz, writeNdjsonGz } from './ndjson-gz.ts';

describe('ndjson-gz', () => {
  let dir = '';
  beforeEach(async () => {
    dir = await mkdtemp(join(tmpdir(), 'm08-ex3-'));
  });
  afterEach(async () => {
    await rm(dir, { recursive: true, force: true });
  });

  test('round-trips many records in order', async () => {
    const file = join(dir, 'tasks.ndjson.gz');
    async function* source() {
      for (let id = 1; id <= 5000; id++) {
        yield { id, title: `line\nbreak ${id} é`, tags: id % 2 === 0 ? ['docs'] : [] };
      }
    }
    assert.equal(await writeNdjsonGz(file, source()), 5000);

    const raw = await readFile(file);
    assert.deepEqual([...raw.subarray(0, 2)], [0x1f, 0x8b]);
    const lines = gunzipSync(raw).toString('utf8').split('\n');
    assert.equal(lines.length, 5001);
    assert.equal(lines[5000], '');

    const seen: unknown[] = [];
    const count = await forEachNdjsonGz(file, (record) => {
      seen.push(record);
    });
    assert.equal(count, 5000);
    assert.deepEqual(seen[0], { id: 1, title: 'line\nbreak 1 é', tags: [] });
    assert.deepEqual((seen[4999] as { id: number }).id, 5000);
  });

  test('accepts a plain array and awaits an async handler', async () => {
    const file = join(dir, 'a.gz');
    await writeNdjsonGz(file, [1, 'two', null]);
    const seen: unknown[] = [];
    await forEachNdjsonGz(file, async (record) => {
      await Promise.resolve();
      seen.push(record);
    });
    assert.deepEqual(seen, [1, 'two', null]);
  });

  test('a failing source rejects with its error', async () => {
    async function* broken() {
      yield { ok: true };
      throw new Error('cursor lost');
    }
    await assert.rejects(writeNdjsonGz(join(dir, 'b.gz'), broken()), /cursor lost/);
  });

  test('skips blank lines but keeps physical line numbers', async () => {
    const file = join(dir, 'c.gz');
    await writeFile(file, gzipSync('{"a":1}\n\n{"a":2}\r\n'));
    const lineNumbers: number[] = [];
    const count = await forEachNdjsonGz(file, (_record, lineNumber) => {
      lineNumbers.push(lineNumber);
    });
    assert.equal(count, 2);
    assert.deepEqual(lineNumbers, [1, 3]);
  });

  test('reports the line of malformed JSON after handling the earlier lines', async () => {
    const file = join(dir, 'd.gz');
    await writeFile(file, gzipSync('{"a":1}\n{oops\n{"a":3}\n'));
    const seen: unknown[] = [];
    await assert.rejects(
      forEachNdjsonGz(file, (record) => {
        seen.push(record);
      }),
      /line 2/,
    );
    assert.deepEqual(seen, [{ a: 1 }]);
  });

  test('stops reading when the handler throws', async () => {
    const file = join(dir, 'e.gz');
    await writeNdjsonGz(
      file,
      Array.from({ length: 1000 }, (_, i) => ({ i })),
    );
    let calls = 0;
    await assert.rejects(
      forEachNdjsonGz(file, () => {
        calls++;
        if (calls === 3) {
          throw new Error('stop here');
        }
      }),
      /stop here/,
    );
    assert.equal(calls, 3);
  });

  test('a missing file rejects with ENOENT', async () => {
    await assert.rejects(
      forEachNdjsonGz(join(dir, 'nope.gz'), () => {}),
      { code: 'ENOENT' },
    );
  });
});
