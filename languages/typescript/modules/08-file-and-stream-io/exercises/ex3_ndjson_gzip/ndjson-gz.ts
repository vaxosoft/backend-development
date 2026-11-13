/**
 * Writes `records` to `file` as gzip-compressed NDJSON: one `JSON.stringify` per line, `\n` terminated.
 * `records` may be a (lazy) sync or async iterable; do not collect it into an array first.
 * If the iterable throws, the returned promise rejects with that error.
 * Resolves with the number of records written.
 */
export async function writeNdjsonGz(
  _file: string,
  _records: Iterable<unknown> | AsyncIterable<unknown>,
): Promise<number> {
  throw new Error('TODO');
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
  _file: string,
  _handler: (record: unknown, lineNumber: number) => void | Promise<void>,
): Promise<number> {
  throw new Error('TODO');
}
