import { createHash } from 'node:crypto';
import { createReadStream, createWriteStream } from 'node:fs';
import { rm } from 'node:fs/promises';
import { Transform } from 'node:stream';
import { pipeline } from 'node:stream/promises';

export interface CopyResult {
  bytes: number;
  sha256: string;
}

/**
 * Copies `src` to `dest` as a stream (never read the whole file into memory) and computes the
 * SHA-256 of the copied bytes on the way, as lowercase hex.
 * If anything fails (for example `src` does not exist) the returned promise rejects with that error
 * and `dest` must not exist afterwards.
 */
export async function copyWithSha256(src: string, dest: string): Promise<CopyResult> {
  const hash = createHash('sha256');
  let bytes = 0;
  const tap = new Transform({
    transform(chunk: Buffer, _encoding, callback) {
      hash.update(chunk);
      bytes += chunk.length;
      callback(null, chunk);
    },
  });
  try {
    await pipeline(createReadStream(src), tap, createWriteStream(dest));
  } catch (err) {
    await rm(dest, { force: true });
    throw err;
  }
  return { bytes, sha256: hash.digest('hex') };
}
