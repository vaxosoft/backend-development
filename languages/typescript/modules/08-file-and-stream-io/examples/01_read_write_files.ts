// Files with node:fs/promises: write, append, read, stat, list, exclusive create, missing files.
import { appendFile, mkdtemp, readdir, readFile, rm, stat, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

const dir = await mkdtemp(join(tmpdir(), 'm08-'));
try {
  const file = join(dir, 'notes.txt');
  await writeFile(file, 'first line\n', 'utf8');
  await appendFile(file, 'second line\n');
  console.log(JSON.stringify(await readFile(file, 'utf8')));

  const buffer = await readFile(file); // no encoding: a Buffer of raw bytes
  console.log(buffer.length, 'bytes', buffer.subarray(0, 5).toString('hex'));

  const info = await stat(file);
  console.log(info.isFile(), info.isDirectory(), info.size);

  // flag 'wx' fails if the file exists: an atomic "create only" with no exists() race.
  try {
    await writeFile(file, 'overwrite?', { flag: 'wx' });
  } catch (err) {
    console.log('wx ->', (err as NodeJS.ErrnoException).code);
  }

  try {
    await readFile(join(dir, 'missing.txt'));
  } catch (err) {
    if (err instanceof Error && 'code' in err && err.code === 'ENOENT') {
      console.log('missing file -> ENOENT');
    } else {
      throw err;
    }
  }

  await writeFile(join(dir, 'b.txt'), 'b');
  const entries = await readdir(dir, { withFileTypes: true });
  console.log(entries.map((e) => `${e.name}${e.isDirectory() ? '/' : ''}`).sort());
} finally {
  await rm(dir, { recursive: true, force: true });
}
