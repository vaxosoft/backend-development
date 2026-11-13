// Task Tracker export: tasks -> NDJSON -> gzip -> file, then read back line by line and checksum the file.
import { createHash } from 'node:crypto';
import { createReadStream, createWriteStream } from 'node:fs';
import { mkdtemp, rm, stat } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { Readable } from 'node:stream';
import { pipeline } from 'node:stream/promises';
import { StringDecoder } from 'node:string_decoder';
import { createGunzip, createGzip } from 'node:zlib';

interface Task {
  id: number;
  title: string;
  status: 'todo' | 'in_progress' | 'done';
  due_date: string | null;
  tags: string[];
  owner_id: number | null;
  created_at: string;
}

const STATUSES = ['todo', 'in_progress', 'done'] as const;

// Stands in for a database cursor: rows are produced one at a time, never as one big array.
async function* allTasks(count: number): AsyncGenerator<Task> {
  for (let id = 1; id <= count; id++) {
    yield {
      id,
      title: `Task ${id}`,
      status: STATUSES[id % 3] ?? 'todo',
      due_date: id % 4 === 0 ? '2030-01-31' : null,
      tags: id % 2 === 0 ? ['docs'] : [],
      owner_id: null,
      created_at: '2030-01-01T09:00:00Z',
    };
  }
}

async function* toNdjson(tasks: AsyncIterable<Task>): AsyncGenerator<string> {
  for await (const task of tasks) {
    yield `${JSON.stringify(task)}\n`;
  }
}

// A pipeline stage written as an async generator: bytes in, one string per line out.
async function* splitLines(source: AsyncIterable<Buffer>): AsyncGenerator<string> {
  const decoder = new StringDecoder('utf8');
  let pending = '';
  for await (const chunk of source) {
    pending += decoder.write(chunk);
    let newline = pending.indexOf('\n');
    while (newline !== -1) {
      yield pending.slice(0, newline);
      pending = pending.slice(newline + 1);
      newline = pending.indexOf('\n');
    }
  }
  pending += decoder.end();
  if (pending !== '') {
    yield pending;
  }
}

const dir = await mkdtemp(join(tmpdir(), 'm08-'));
try {
  const file = join(dir, 'tasks.ndjson.gz');
  await pipeline(Readable.from(toNdjson(allTasks(10_000))), createGzip(), createWriteStream(file));
  console.log('compressed file is non-empty:', (await stat(file)).size > 0);

  // pipeline() (not .pipe()) so an error in the file or in gunzip rejects instead of hanging.
  const perStatus: Record<string, number> = {};
  let first: Task | undefined;
  await pipeline(
    createReadStream(file),
    createGunzip(),
    splitLines,
    async (lines: AsyncIterable<string>) => {
      for await (const line of lines) {
        const task = JSON.parse(line) as Task;
        first ??= task;
        perStatus[task.status] = (perStatus[task.status] ?? 0) + 1;
      }
    },
  );
  console.log('first:', first?.title);
  console.log('per status:', perStatus);

  const hash = createHash('sha256');
  await pipeline(createReadStream(file), hash);
  console.log('sha256 length:', hash.digest('hex').length);
} finally {
  await rm(dir, { recursive: true, force: true });
}
