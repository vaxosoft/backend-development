import { createReadStream, createWriteStream } from 'node:fs';
import { Readable } from 'node:stream';
import { pipeline } from 'node:stream/promises';
import { StringDecoder } from 'node:string_decoder';
import { createGunzip, createGzip } from 'node:zlib';

/** Re-chunks a byte stream into lines; a multi-byte character may straddle two chunks. */
async function* splitLines(source: AsyncIterable<Buffer>): AsyncGenerator<string> {
  const decoder = new StringDecoder('utf8');
  let pending = '';
  for await (const chunk of source) {
    pending += decoder.write(chunk);
    let newline = pending.indexOf('\n');
    while (newline !== -1) {
      yield pending.slice(0, newline).replace(/\r$/, '');
      pending = pending.slice(newline + 1);
      newline = pending.indexOf('\n');
    }
  }
  pending += decoder.end();
  if (pending !== '') {
    yield pending;
  }
}

/**
 * Writes `records` to `file` as gzip-compressed NDJSON: one `JSON.stringify` per line, `\n` terminated.
 * `records` may be a (lazy) sync or async iterable; do not collect it into an array first.
 * If the iterable throws, the returned promise rejects with that error.
 * Resolves with the number of records written.
 */
export async function writeNdjsonGz(
  file: string,
  records: Iterable<unknown> | AsyncIterable<unknown>,
): Promise<number> {
  let count = 0;
  async function* lines(): AsyncGenerator<string> {
    for await (const record of records) {
      count++;
      yield `${JSON.stringify(record)}\n`;
    }
  }
  await pipeline(Readable.from(lines()), createGzip(), createWriteStream(file));
  return count;
}

/**
 * Streams `file` (gzip NDJSON), calling `handler(record, lineNumber)` once per non-blank line, in order.
 * - `lineNumber` is the 1-based physical line number (blank lines are skipped but still counted)
 * - a line that is not valid JSON rejects with an Error whose message contains `line <n>`
 * - if `handler` throws or rejects, reading stops and the promise rejects with that error
 * - a missing file rejects with the underlying ENOENT error
 * Resolves with the number of records handled.
 */
export async function forEachNdjsonGz(
  file: string,
  handler: (record: unknown, lineNumber: number) => void | Promise<void>,
): Promise<number> {
  let handled = 0;
  await pipeline(
    createReadStream(file),
    createGunzip(),
    splitLines,
    async (lines: AsyncIterable<string>) => {
      let lineNumber = 0;
      for await (const line of lines) {
        lineNumber++;
        if (line.trim() === '') {
          continue;
        }
        let record: unknown;
        try {
          record = JSON.parse(line);
        } catch (err) {
          throw new Error(`invalid JSON on line ${lineNumber}`, { cause: err });
        }
        await handler(record, lineNumber);
        handled++;
      }
    },
  );
  return handled;
}
