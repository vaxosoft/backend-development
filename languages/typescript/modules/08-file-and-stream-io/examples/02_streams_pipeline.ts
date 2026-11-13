// Streams: a lazy source, a Transform and a file sink connected with pipeline() (backpressure and cleanup).
import { createReadStream, createWriteStream } from 'node:fs';
import { mkdtemp, rm, stat } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createInterface } from 'node:readline';
import { Readable, Transform } from 'node:stream';
import { pipeline } from 'node:stream/promises';

async function* numberedLines(count: number): AsyncGenerator<string> {
  for (let i = 1; i <= count; i++) {
    yield `line ${i}\n`;
  }
}

const upperCase = new Transform({
  transform(chunk: Buffer, _encoding, callback) {
    callback(null, chunk.toString('utf8').toUpperCase());
  },
});

const dir = await mkdtemp(join(tmpdir(), 'm08-'));
try {
  const file = join(dir, 'lines.txt');

  // 200_000 lines are produced on demand; memory stays flat because pipeline() respects backpressure.
  await pipeline(Readable.from(numberedLines(200_000)), upperCase, createWriteStream(file));
  console.log('size', (await stat(file)).size);

  // Read it back line by line without loading the file.
  const lines = createInterface({ input: createReadStream(file), crlfDelay: Infinity });
  let count = 0;
  let last = '';
  for await (const line of lines) {
    count++;
    last = line;
  }
  console.log(count, last);

  // Errors anywhere in the chain reject the promise and destroy every stream.
  try {
    await pipeline(
      createReadStream(join(dir, 'missing.txt')),
      createWriteStream(join(dir, 'copy.txt')),
    );
  } catch (err) {
    console.log('pipeline failed:', (err as NodeJS.ErrnoException).code);
  }
} finally {
  await rm(dir, { recursive: true, force: true });
}
